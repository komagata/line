// Package helper owns OS helper lifetimes; no credentials belong in argv.
package helper

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Command puts each helper and its descendants in a private process group.
// Cancellation kills the entire group, and Wait joins the child and pipe pumps.
// Pdeathsig is a last resort for abrupt loss of the owning process.
func Command(ctx context.Context, path string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 200 * time.Millisecond
	return cmd
}

// Run also cleans up descendants when a helper exits before its children.
var reapOnce sync.Once
var reapError error

func Run(cmd *exec.Cmd) error {
	reapOnce.Do(func() { reapError = unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) })
	if reapError != nil {
		return reapError
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		pgid := cmd.Process.Pid
		syscall.Kill(-pgid, syscall.SIGKILL)
		// Only adopt/reap descendants of this helper group, never another cmd owner.
		for {
			_, err := syscall.Wait4(-pgid, nil, 0, nil)
			if err == syscall.EINTR {
				continue
			}
			if err != nil {
				break
			}
		}
	}()
	return cmd.Wait()
}
