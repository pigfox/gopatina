package subset

// StdFuncs lists the standard-library functions patina mode supports in
// phase 1, by package path and name. Everything else in the standard
// library is rejected with an error naming it.
var StdFuncs = map[string]bool{
	"fmt.Print":    true,
	"fmt.Println":  true,
	"fmt.Printf":   true,
	"fmt.Sprint":   true,
	"fmt.Sprintln": true,
	"fmt.Sprintf":  true,
	"fmt.Fprint":   true,
	"fmt.Fprintln": true,
	"fmt.Fprintf":  true,
	"fmt.Errorf":   true,
	"errors.New":   true,
	// RFC 0012: the provided errors package's first API.
	"errors.Is":         true,
	"errors.As":         true,
	"errors.Unwrap":     true,
	"strings.Contains":  true,
	"strings.HasPrefix": true,
	"strings.HasSuffix": true,
	"strings.Index":     true,
	"strings.Join":      true,
	"strings.Repeat":    true,
	"strings.TrimSpace": true,
	// RFC 0011: the first provided API of package strings.
	"strings.ToLower":    true,
	"strings.ToUpper":    true,
	"strings.EqualFold":  true,
	"strings.TrimPrefix": true,
	"strings.TrimSuffix": true,
	"strings.Split":      true,
	"strings.Cut":        true,
	"strings.IndexByte":  true,
	"strings.ReplaceAll": true,
	"os.Exit":            true,
	// The provided math functions that are exact by IEEE 754: Rust gives
	// Go's result bit for bit, NaNs included, on amd64 and arm64
	// (testdata/golden/math_exact). Max and Min are not among them: Go's
	// NaN result differs between amd64 and arm64.
	"math.Sqrt":            true,
	"math.Abs":             true,
	"math.Floor":           true,
	"math.Ceil":            true,
	"math.Trunc":           true,
	"math.Copysign":        true,
	"math.Signbit":         true,
	"math.IsNaN":           true,
	"math.IsInf":           true,
	"math.Inf":             true,
	"math.NaN":             true,
	"math.Float64bits":     true,
	"math.Float64frombits": true,
	// math.Exp: Go's own algorithm for the target (exp_amd64.s, with or
	// without FMA as the processor has it; exp_arm64.s), bit for bit.
	"math.Exp": true,
	// math.Log: Go's own for the target (log_amd64.s; log.go as gc
	// compiles it on arm64, fusing ten of its multiply-adds), bit for bit.
	"math.Log": true,
	// math.Sin, math.Cos: sin.go as gc compiles it for the target, fusing
	// what it fuses there (all on arm64; the outer adds from GOAMD64=v3).
	"math.Sin": true,
	"math.Cos": true,
	// math.Pow: pow.go, with Exp and Log as above. gc fuses its square
	// and doubling on arm64 and from GOAMD64=v3, which rounds to the same
	// value.
	"math.Pow":     true,
	"slices.Clone": true,
	"bytes.Clone":  true,
	// Sorting a slice of integers or strings (sortProblem): Rust's sort
	// and Go's give the same order, since equal elements are the same.
	"slices.Sort":           true,
	"slices.IsSorted":       true,
	"sort.Ints":             true,
	"sort.Strings":          true,
	"sort.IntsAreSorted":    true,
	"sort.StringsAreSorted": true,
}

// Sorts are the functions of StdFuncs that sort a slice in place.
var Sorts = map[string]bool{"slices.Sort": true, "sort.Ints": true, "sort.Strings": true}

// StdVars lists the standard-library variables patina mode supports.
var StdVars = map[string]bool{
	"os.Stdout": true,
	"os.Stderr": true,
}
