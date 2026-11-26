//go:build !unix

package main

import "os/exec"

// freePercent cannot tell the free space here: the disk guard is off.
func freePercent(string) float64 { return 100 }

// alive cannot tell here: no run's directory is swept.
func alive(int) bool { return true }

// ownGroup leaves cmd as it is: stopping it stops it alone.
func ownGroup(*exec.Cmd) {}
