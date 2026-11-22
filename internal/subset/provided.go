package subset

import "go/types"

// ProvidedTypes are the named types of provided packages (RFC 0011) that
// patina mode supports, by package path and name. Each is its underlying
// type in Rust; its methods are runtime functions (ProvidedMethods).
var ProvidedTypes = map[string]bool{
	"time.Duration": true,
}

// ProvidedMethods maps each supported method of a provided type, by
// package path, type and name, to the runtime function that implements it
// with Go's behaviour. The receiver is passed first, by value. None of
// them panics, so fmt may print through String.
var ProvidedMethods = map[string]string{
	"time.Duration.String":       "duration_string",
	"time.Duration.Nanoseconds":  "duration_nanoseconds",
	"time.Duration.Microseconds": "duration_microseconds",
	"time.Duration.Milliseconds": "duration_milliseconds",
	"time.Duration.Seconds":      "duration_seconds",
	"time.Duration.Minutes":      "duration_minutes",
	"time.Duration.Hours":        "duration_hours",
	"time.Duration.Truncate":     "duration_truncate",
	"time.Duration.Round":        "duration_round",
	"time.Duration.Abs":          "duration_abs",
}

// providedName is the path.Name of t if it is a provided type, or "".
func providedName(t types.Type) string {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	name := n.Obj().Pkg().Path() + "." + n.Obj().Name()
	if !ProvidedTypes[name] {
		return ""
	}
	return name
}

// ProvidedMethod returns the runtime function of m, a method of a provided
// type, and its Go name (time.Duration.Round); the function is "" when
// patina mode does not provide m yet, and both are "" when m is not a
// method of a provided type.
func ProvidedMethod(m *types.Func) (fn, name string) {
	recv := m.Signature().Recv()
	if recv == nil {
		return "", ""
	}
	typ := providedName(recv.Type()) // value receivers only, so far
	if typ == "" {
		return "", ""
	}
	name = typ + "." + m.Name()
	return ProvidedMethods[name], name
}
