// Command blank_imports imports packages for their initialization only,
// as a program that imports a package for its side effects does. The
// packages patina mode provides have none a program can observe; a
// translated one (unicode/utf8, hash/fnv) has its initialization
// translated.
package main

import (
	"fmt"

	_ "bytes"
	_ "errors"
	_ "io"
	_ "os"
	_ "slices"
	_ "strings"
	_ "time"

	_ "hash/fnv"
	_ "unicode/utf8"
)

func main() {
	fmt.Println("initialized")
}
