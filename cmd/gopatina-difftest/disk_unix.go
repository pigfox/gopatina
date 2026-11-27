//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// freePercent returns the percent of the disk holding dir that is free to
// an unprivileged user, or 100 when it cannot tell, dir being no directory
// (a -work of /dev/null would measure devfs, 0% free on macOS) and the run
// failing on it with its own error.
func freePercent(dir string) float64 {
	var st syscall.Statfs_t
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || syscall.Statfs(dir, &st) != nil || st.Blocks == 0 {
		return 100
	}
	return 100 * float64(st.Bavail) / float64(st.Blocks)
}

// alive reports whether process pid exists.
func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// ownGroup starts cmd in a process group of its own, all of it killed
// when cmd is stopped. A process the group was forking as it was killed
// can escape the kill on macOS, keeping the build's output open and
// writing on; the group is killed again shortly after to catch it.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := -cmd.Process.Pid
		for _, d := range []time.Duration{20 * time.Millisecond, 200 * time.Millisecond} {
			time.AfterFunc(d, func() { syscall.Kill(pgid, syscall.SIGKILL) })
		}
		return syscall.Kill(pgid, syscall.SIGKILL)
	}
}
