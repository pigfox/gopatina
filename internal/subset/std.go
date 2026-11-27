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
	"math.Exp":     true,
	"slices.Clone": true,
	"bytes.Clone":  true,
}

// StdVars lists the standard-library variables patina mode supports.
var StdVars = map[string]bool{
	"os.Stdout": true,
	"os.Stderr": true,
}
