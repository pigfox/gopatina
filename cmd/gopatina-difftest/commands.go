package main

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// Commands. A run's builds and programs each start in a process group of
// their own, so stopping one, at its time limit or because the run is
// stopped, also stops what it started: go's compilers and linkers,
// cargo's rustc. A run stopped by a signal stops them all, and waits for
// them, before removing its working directory, which a build still
// running would fill again.

// commands counts the commands a run has running, and stops them all.
type commands struct {
	ctx     context.Context
	stop    context.CancelFunc
	mu      sync.Mutex
	idle    *sync.Cond // n reached 0
	n       int
	stopped bool
}

// errStopped is the error of a build not started because the run stopped.
var errStopped = errors.New("stopped")

func newCommands() *commands {
	k := &commands{}
	k.ctx, k.stop = context.WithCancel(context.Background())
	k.idle = sync.NewCond(&k.mu)
	return k
}

// start counts a command about to start; false once the run has stopped.
func (k *commands) start() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stopped {
		return false
	}
	k.n++
	return true
}

// end counts a command ended.
func (k *commands) end() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.n--; k.n == 0 {
		k.idle.Broadcast()
	}
}

// halt stops every running command, with what it started, waits for them
// to end, and starts no more.
func (k *commands) halt() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.stopped = true
	k.stop()
	for k.n > 0 {
		k.idle.Wait()
	}
}

// halted reports whether the run has stopped its commands: a build or
// program failing now may have been stopped, not failed.
func (k *commands) halted() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.stopped
}

// command returns name with args, run in a process group of its own that
// is killed when ctx is done.
func command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	ownGroup(cmd)
	cmd.WaitDelay = 5 * time.Second // output a stray process still holds open
	return cmd
}
