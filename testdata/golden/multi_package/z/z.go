// Package z is imported by a, so it initializes before a although its path
// sorts after a's, b's and c's.
package z

import "fmt"

const Tag = "z"

func init() { fmt.Println("init z") }
