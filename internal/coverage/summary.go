package coverage

import (
	"math/rand/v2"
	"regexp"
	"sort"
	"strings"
)

// A Rate is a share with its 95% bootstrap interval: packages are drawn
// with replacement, so one large package cannot carry the number.
type Rate struct {
	Num   int     `json:"num"`
	Den   int     `json:"den"`
	Value float64 `json:"value"`
	Low   float64 `json:"low_95"`
	High  float64 `json:"high_95"`
}

// A Reason is one feature's part in the rejections.
type Reason struct {
	Feature string `json:"feature"`
	// In is how many rejected functions need the feature, among others.
	In int `json:"in_rejected_functions"`
	// Only is how many would be accepted if just this feature were added.
	Only int `json:"would_pass_with_only_this"`
}

// A Step is one feature of the greedy roadmap: added in this order, each
// lets the most remaining functions in.
type Step struct {
	Feature  string  `json:"feature"`
	Unlocked int     `json:"functions_unlocked"`
	Rate     float64 `json:"function_rate_after"`
}

// A Group is the summary of a set of packages.
type Group struct {
	Packages int `json:"packages"`
	// Excluded counts the functions that use reflect or unsafe themselves,
	// or through code of their own package; they are left out of every share
	// below. ViaDeps counts those that reach reflect or unsafe only through
	// another package: they stay in the shares, as rejected.
	Excluded        int      `json:"functions_excluded_reflect_unsafe_direct"`
	ViaDeps         int      `json:"functions_reaching_reflect_unsafe_via_dependencies"`
	Functions       Rate     `json:"functions"`
	Lines           Rate     `json:"lines_in_accepted_functions"`
	Files           Rate     `json:"files"`
	PackagesWhole   Rate     `json:"packages_wholly_accepted"`
	Reasons         []Reason `json:"reasons"`
	Roadmap         []Step   `json:"roadmap"`
	LeaveOneOutLow  float64  `json:"function_rate_leave_one_module_out_low"`
	LeaveOneOutHigh float64  `json:"function_rate_leave_one_module_out_high"`
}

// A Summary is the whole report's numbers.
type Summary struct {
	All        Group              `json:"all"`
	Std        Group              `json:"std"`
	ThirdParty Group              `json:"third_party"`
	Modules    map[string]float64 `json:"function_rate_by_module"`
	// TopMessages are the most frequent diagnostics of each feature.
	TopMessages map[string][]MessageCount `json:"top_messages"`
	// ForeignTypes counts the diagnostics about a type from another package
	// by where the type is declared: the measured package's own module,
	// the standard library, or another module.
	ForeignTypes map[string]int `json:"foreign_types_by_origin"`
}

// A MessageCount is a diagnostic message and how often it was given.
type MessageCount struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// topPerFeature is how many messages TopMessages keeps for each feature.
const topPerFeature = 25

var foreignPathRE = regexp.MustCompile(`type [*\[\]]*([a-z][^ \[]*)\.[A-Za-z_]`)

// origin says where the type named in a "types from other packages"
// diagnostic of a package in module is declared.
func origin(msg, module string) string {
	m := foreignPathRE.FindStringSubmatch(msg)
	switch {
	case m == nil:
		return "unknown"
	case module != "std" && (m[1] == module || strings.HasPrefix(m[1], module+"/")):
		return "same module"
	case !strings.Contains(strings.SplitN(m[1], "/", 2)[0], "."):
		return "standard library"
	}
	return "other modules"
}

// messages computes TopMessages and ForeignTypes.
func messages(pkgs []Package) (map[string][]MessageCount, map[string]int) {
	byFeature := map[string]map[string]int{}
	foreign := map[string]int{}
	for _, p := range pkgs {
		for msg, n := range p.Messages {
			f := Feature(msg)
			if byFeature[f] == nil {
				byFeature[f] = map[string]int{}
			}
			byFeature[f][msg] += n
			if f == "types from other packages" {
				foreign[origin(msg, p.Module)] += n
			}
		}
	}
	top := map[string][]MessageCount{}
	for f, ms := range byFeature {
		var list []MessageCount
		for m, n := range ms {
			list = append(list, MessageCount{m, n})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].Count != list[j].Count {
				return list[i].Count > list[j].Count
			}
			return list[i].Message < list[j].Message
		})
		top[f] = list[:min(len(list), topPerFeature)]
	}
	return top, foreign
}

// Summarize computes the summary of pkgs, with boot bootstrap draws from
// the given seed.
func Summarize(pkgs []Package, boot int, seed uint64) Summary {
	var std, third []Package
	for _, p := range pkgs {
		if p.Module == "std" {
			std = append(std, p)
		} else {
			third = append(third, p)
		}
	}
	s := Summary{
		All: group(pkgs, boot, seed), Std: group(std, boot, seed), ThirdParty: group(third, boot, seed),
		Modules: map[string]float64{},
	}
	s.TopMessages, s.ForeignTypes = messages(pkgs)
	for m, ps := range byModule(pkgs) {
		c := count(ps)
		s.Modules[m] = ratio(c.funcs[0], c.funcs[1])
	}
	return s
}

// counts holds numerator and denominator pairs.
type counts struct {
	funcs, lines, files, whole [2]int
	excluded, viaDeps          int
}

func count(pkgs []Package) counts {
	var c counts
	for _, p := range pkgs {
		for _, f := range p.Funcs {
			if f.Excluded {
				c.excluded++
				continue
			}
			for _, x := range f.Features {
				if x == ViaDeps {
					c.viaDeps++
				}
			}
			c.funcs[1]++
			c.lines[1] += f.Lines
			if f.Accepted {
				c.funcs[0]++
				c.lines[0] += f.Lines
			}
		}
		c.files[0] += p.FilesOK
		c.files[1] += p.Files
		c.whole[1]++
		if p.Accepted {
			c.whole[0]++
		}
	}
	return c
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func group(pkgs []Package, boot int, seed uint64) Group {
	g := Group{Packages: len(pkgs)}
	c := count(pkgs)
	g.Excluded, g.ViaDeps = c.excluded, c.viaDeps
	// Bootstrap: draw len(pkgs) packages with replacement, boot times.
	r := rand.New(rand.NewPCG(seed, 1))
	var draws [4][]float64
	for range boot {
		var d counts
		for range pkgs {
			one := count(pkgs[r.IntN(len(pkgs)):][:1])
			for i := range 2 {
				d.funcs[i] += one.funcs[i]
				d.lines[i] += one.lines[i]
				d.files[i] += one.files[i]
				d.whole[i] += one.whole[i]
			}
		}
		for i, pair := range [][2]int{d.funcs, d.lines, d.files, d.whole} {
			draws[i] = append(draws[i], ratio(pair[0], pair[1]))
		}
	}
	rate := func(pair [2]int, ds []float64) Rate {
		lo, hi := percentile(ds, 0.025), percentile(ds, 0.975)
		return Rate{Num: pair[0], Den: pair[1], Value: ratio(pair[0], pair[1]), Low: lo, High: hi}
	}
	g.Functions = rate(c.funcs, draws[0])
	g.Lines = rate(c.lines, draws[1])
	g.Files = rate(c.files, draws[2])
	g.PackagesWhole = rate(c.whole, draws[3])
	g.Reasons, g.Roadmap = reasons(pkgs, c.funcs[1])
	g.LeaveOneOutLow, g.LeaveOneOutHigh = g.Functions.Value, g.Functions.Value
	mods := byModule(pkgs)
	if len(mods) > 1 {
		for m := range mods {
			var rest []Package
			for _, p := range pkgs {
				if p.Module != m {
					rest = append(rest, p)
				}
			}
			rc := count(rest)
			v := ratio(rc.funcs[0], rc.funcs[1])
			g.LeaveOneOutLow = min(g.LeaveOneOutLow, v)
			g.LeaveOneOutHigh = max(g.LeaveOneOutHigh, v)
		}
	}
	return g
}

// percentile returns the p-quantile of ds (nearest rank), or 0 for none.
func percentile(ds []float64, p float64) float64 {
	if len(ds) == 0 {
		return 0
	}
	s := append([]float64(nil), ds...)
	sort.Float64s(s)
	i := int(p * float64(len(s)-1))
	return s[i]
}

func byModule(pkgs []Package) map[string][]Package {
	m := map[string][]Package{}
	for _, p := range pkgs {
		m[p.Module] = append(m[p.Module], p)
	}
	return m
}

// reasons ranks the features that keep functions out, and the greedy
// roadmap: the order of features that lets the most functions in first.
func reasons(pkgs []Package, total int) ([]Reason, []Step) {
	in, only := map[string]int{}, map[string]int{}
	var rejected [][]string
	accepted := 0
	for _, p := range pkgs {
		for _, f := range p.Funcs {
			if f.Excluded {
				continue
			}
			if f.Accepted {
				accepted++
				continue
			}
			rejected = append(rejected, f.Features)
			for _, x := range f.Features {
				in[x]++
			}
			if len(f.Features) == 1 {
				only[f.Features[0]]++
			}
		}
	}
	var rs []Reason
	for x, n := range in {
		rs = append(rs, Reason{Feature: x, In: n, Only: only[x]})
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].In != rs[j].In {
			return rs[i].In > rs[j].In
		}
		return rs[i].Feature < rs[j].Feature
	})
	have := map[string]bool{}
	var steps []Step
	for len(rejected) > 0 {
		best, bestN := "", -1
		for _, r := range rs {
			if have[r.Feature] {
				continue
			}
			have[r.Feature] = true
			n := 0
			for _, fs := range rejected {
				if covered(fs, have) {
					n++
				}
			}
			delete(have, r.Feature)
			if n > bestN {
				best, bestN = r.Feature, n
			}
		}
		// With nothing unlocked by any one feature, best is still the
		// remaining feature most functions need (rs is in that order),
		// so the roadmap goes on until every feature is in.
		if best == "" {
			break // a rejected function with no feature: cannot happen
		}
		have[best] = true
		var rest [][]string
		for _, fs := range rejected {
			if !covered(fs, have) {
				rest = append(rest, fs)
			}
		}
		rejected = rest
		accepted += bestN
		steps = append(steps, Step{Feature: best, Unlocked: bestN, Rate: ratio(accepted, total)})
	}
	return rs, steps
}

func covered(fs []string, have map[string]bool) bool {
	for _, x := range fs {
		if !have[x] {
			return false
		}
	}
	return true
}
