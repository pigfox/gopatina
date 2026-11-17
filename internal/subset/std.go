package subset

// StdFuncs lists the standard-library functions patina mode supports in
// phase 1, by package path and name. Everything else in the standard
// library is rejected with an error naming it.
var StdFuncs = map[string]bool{
	"fmt.Print":         true,
	"fmt.Println":       true,
	"fmt.Printf":        true,
	"fmt.Sprint":        true,
	"fmt.Sprintln":      true,
	"fmt.Sprintf":       true,
	"fmt.Fprint":        true,
	"fmt.Fprintln":      true,
	"fmt.Fprintf":       true,
	"fmt.Errorf":        true,
	"errors.New":        true,
	"strings.Contains":  true,
	"strings.HasPrefix": true,
	"strings.HasSuffix": true,
	"strings.Index":     true,
	"strings.Join":      true,
	"strings.Repeat":    true,
	"strings.TrimSpace": true,
	"os.Exit":           true,
	"slices.Clone":      true,
	"bytes.Clone":       true,
}

// StdVars lists the standard-library variables patina mode supports.
var StdVars = map[string]bool{
	"os.Stdout": true,
	"os.Stderr": true,
}
