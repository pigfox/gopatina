// Package benchstat decides whether two programs run at the same speed, with
// a stated confidence, from paired timings.
//
// The test is a two one-sided tests (TOST) equivalence test on the paired log
// ratios ln(a_i / b_i). Two programs are called equivalent when the whole
// (1 - 2α) confidence interval of their mean log ratio lies inside
// (-ln(1+margin), +ln(1+margin)). α is the one-sided tail of a normal
// distribution beyond sigma standard deviations, so sigma = 5 means each
// one-sided test is run at α ≈ 2.87e-7. The interval uses Student's t with
// n - 1 degrees of freedom. See bench/README.md.
package benchstat

import (
	"errors"
	"math"
)

// MinPairs is the fewest pairs Equivalence accepts.
const MinPairs = 10

// Verdict is the outcome of an equivalence test.
type Verdict string

const (
	// Equivalent: the whole interval lies inside the margin.
	Equivalent Verdict = "equivalent"
	// Slower: the whole interval lies above the margin (a is slower).
	Slower Verdict = "slower"
	// Faster: the whole interval lies below the margin (a is faster).
	Faster Verdict = "faster"
	// Inconclusive: the interval crosses a margin bound; more or quieter
	// runs are needed to decide.
	Inconclusive Verdict = "inconclusive"
)

// Result is the outcome of Equivalence. Ratios are a / b.
type Result struct {
	Pairs        int     `json:"pairs"`
	Margin       float64 `json:"margin"`
	Sigma        float64 `json:"sigma"`
	Alpha        float64 `json:"alpha_one_sided"`
	MeanLogRatio float64 `json:"mean_log_ratio"`
	SDLogRatio   float64 `json:"sd_log_ratio"`
	Ratio        float64 `json:"ratio"`
	Low          float64 `json:"ratio_low"`
	High         float64 `json:"ratio_high"`
	Verdict      Verdict `json:"verdict"`
}

// Equivalence runs the TOST equivalence test on paired positive timings a
// and b (a[i] and b[i] measured next to each other), with an equivalence
// margin (0.005 is ±0.5%) and a confidence of sigma standard deviations.
func Equivalence(a, b []float64, margin, sigma float64) (Result, error) {
	if len(a) != len(b) {
		return Result{}, errors.New("benchstat: a and b differ in length")
	}
	if len(a) < MinPairs {
		return Result{}, errors.New("benchstat: need at least 10 pairs")
	}
	if !(margin > 0) || !(sigma > 0) {
		return Result{}, errors.New("benchstat: margin and sigma must be positive")
	}
	d := make([]float64, len(a))
	for i := range a {
		if !(a[i] > 0) || !(b[i] > 0) {
			return Result{}, errors.New("benchstat: timings must be positive")
		}
		d[i] = math.Log(a[i] / b[i])
	}
	n := float64(len(d))
	var sum float64
	for _, x := range d {
		sum += x
	}
	mean := sum / n
	var ss float64
	for _, x := range d {
		ss += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(ss / (n - 1))
	alpha := TailAlpha(sigma)
	half := TQuantile(alpha, n-1) * sd / math.Sqrt(n)
	lo, hi := mean-half, mean+half
	bound := math.Log1p(margin)
	r := Result{
		Pairs:        len(d),
		Margin:       margin,
		Sigma:        sigma,
		Alpha:        alpha,
		MeanLogRatio: mean,
		SDLogRatio:   sd,
		Ratio:        math.Exp(mean),
		Low:          math.Exp(lo),
		High:         math.Exp(hi),
	}
	switch {
	case lo > -bound && hi < bound:
		r.Verdict = Equivalent
	case lo >= bound:
		r.Verdict = Slower
	case hi <= -bound:
		r.Verdict = Faster
	default:
		r.Verdict = Inconclusive
	}
	return r, nil
}

// TailAlpha is the one-sided tail probability of a standard normal beyond
// sigma: P(Z > sigma).
func TailAlpha(sigma float64) float64 {
	return 0.5 * math.Erfc(sigma/math.Sqrt2)
}

// TQuantile returns t such that P(T > t) = alpha for Student's t with df
// degrees of freedom (0 < alpha < 0.5, df > 0).
func TQuantile(alpha, df float64) float64 {
	lo, hi := 0.0, 1.0
	for tTail(hi, df) > alpha {
		lo, hi = hi, hi*2
	}
	for range 200 {
		mid := (lo + hi) / 2
		if tTail(mid, df) > alpha {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// tTail is P(T > t) for t >= 0.
func tTail(t, df float64) float64 {
	return 0.5 * betaInc(df/2, 0.5, df/(df+t*t))
}

// betaInc is the regularized incomplete beta function I_x(a, b) for
// 0 <= x <= 1, by its continued fraction (Numerical Recipes, betai).
func betaInc(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	front := math.Exp(lab - la - lb + a*math.Log(x) + b*math.Log1p(-x))
	if x < (a+1)/(a+b+2) {
		return front * betaCF(a, b, x) / a
	}
	return 1 - front*betaCF(b, a, 1-x)/b
}

// betaCF evaluates the continued fraction for betaInc by the modified Lentz
// method.
func betaCF(a, b, x float64) float64 {
	const tiny = 1e-300
	const eps = 1e-16
	clamp := func(v float64) float64 {
		if math.Abs(v) < tiny {
			return tiny
		}
		return v
	}
	qab, qap, qam := a+b, a+1, a-1
	c := 1.0
	d := 1 / clamp(1-qab*x/qap)
	h := d
	for m := 1; m <= 1000; m++ {
		fm := float64(m)
		m2 := 2 * fm
		aa := fm * (b - fm) * x / ((qam + m2) * (a + m2))
		d = 1 / clamp(1+aa*d)
		c = clamp(1 + aa/c)
		h *= d * c
		aa = -(a + fm) * (qab + fm) * x / ((a + m2) * (qap + m2))
		d = 1 / clamp(1+aa*d)
		c = clamp(1 + aa/c)
		del := d * c
		h *= del
		if math.Abs(del-1) < eps {
			break
		}
	}
	return h
}
