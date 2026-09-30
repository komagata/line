package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kongesque/line-cli/internal/gui"
	"github.com/kongesque/line-cli/internal/helper"
	"golang.org/x/sys/unix"
)

// Test-only subprocess entrypoints, never selected by the shipped executable.
func TestLifecycleProcess(t *testing.T) {
	mode := os.Getenv("LINE_GUI_LIFECYCLE_FIXTURE")
	if mode == "" {
		return
	}
	exe, _ := os.Executable()
	dir := os.Getenv("LINE_GUI_LIFECYCLE_DIR")
	switch mode {
	case "launcher":
		cmd := exec.Command(exe, "-test.run=^TestLifecycleProcess$")
		cmd.Env = append(os.Environ(), "LINE_GUI_LIFECYCLE_FIXTURE=app")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if cmd.Start() != nil {
			os.Exit(2)
		}
		os.WriteFile(filepath.Join(dir, "app"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600)
		for {
			time.Sleep(time.Second)
		}
	case "app":
		run(gui.Operations{Snapshot: func(ctx context.Context) (gui.Snapshot, error) {
			cmd := helper.Command(ctx, exe, "-test.run=^TestLifecycleProcess$")
			cmd.Env = append(os.Environ(), "LINE_GUI_LIFECYCLE_FIXTURE=helper")
			return gui.Snapshot{}, helper.Run(cmd)
		}})
		os.Exit(0)
	case "helper":
		cmd := exec.Command(exe, "-test.run=^TestLifecycleProcess$")
		cmd.Env = append(os.Environ(), "LINE_GUI_LIFECYCLE_FIXTURE=descendant")
		if cmd.Start() != nil {
			os.Exit(2)
		}
		os.WriteFile(filepath.Join(dir, "helper"), []byte(fmt.Sprintf("%d %d", os.Getpid(), cmd.Process.Pid)), 0600)
		for {
			time.Sleep(time.Second)
		}
	case "descendant":
		for {
			time.Sleep(time.Second)
		}
	}
}
func fixturePID(t *testing.T, path string) []int {
	t.Helper()
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		b, err := os.ReadFile(path)
		if err == nil && len(b) > 0 {
			var pids []int
			for _, s := range strings.Fields(string(b)) {
				p, _ := strconv.Atoi(s)
				pids = append(pids, p)
			}
			return pids
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture did not start: %s", path)
	return nil
}
func TestProcessShutdownWithFullStdoutAndOwnedHelpers(t *testing.T) {
	// Adopt orphaned fixture applications so this test can prove exit and reap it.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"eof", "signal", "parent"} {
		t.Run(mode, func(t *testing.T) {
			exe, _ := os.Executable()
			dir := t.TempDir()
			entry := "app"
			if mode == "parent" {
				entry = "launcher"
			}
			cmd := exec.Command(exe, "-test.run=^TestLifecycleProcess$")
			cmd.Env = append(os.Environ(), "LINE_GUI_LIFECYCLE_FIXTURE="+entry, "LINE_GUI_LIFECYCLE_DIR="+dir)
			input, _ := cmd.StdinPipe()
			output, _ := cmd.StdoutPipe()
			cmd.Stderr = io.Discard
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			app := cmd.Process.Pid
			if mode == "parent" {
				app = fixturePID(t, filepath.Join(dir, "app"))[0]
			}
			defer syscall.Kill(app, syscall.SIGKILL)
			reader := bufio.NewReader(output)
			if _, err := reader.ReadBytes('\n'); err != nil {
				t.Fatal(err)
			}
			io.WriteString(input, "{\"action\":\"mode\",\"mode\":\"demo\"}\n")
			line, err := reader.ReadBytes('\n')
			if err != nil {
				t.Fatal(err)
			}
			var state struct {
				View gui.View `json:"view"`
			}
			if json.Unmarshal(line, &state) != nil {
				t.Fatal("invalid demo state")
			}
			for n := 1; n <= 30; n++ {
				b, _ := json.Marshal(gui.Command{Action: "draft", ID: state.View.SelectedID, Session: state.View.Session, Selection: state.View.Selection, Text: strings.Repeat("x", 10000), Sequence: n})
				input.Write(append(b, '\n'))
				time.Sleep(10 * time.Millisecond)
			}
			io.WriteString(input, "{\"action\":\"mode\",\"mode\":\"live\"}\n{\"action\":\"refresh\"}\n")
			pids := fixturePID(t, filepath.Join(dir, "helper"))
			defer func() {
				for _, pid := range pids {
					syscall.Kill(pid, syscall.SIGKILL)
					syscall.Wait4(pid, nil, syscall.WNOHANG, nil)
				}
			}()
			started := time.Now()
			switch mode {
			case "eof":
				input.Close()
			case "signal":
				syscall.Kill(app, syscall.SIGTERM)
			case "parent":
				cmd.Process.Kill()
			}
			done := make(chan error, 1)
			go func() {
				err := cmd.Wait()
				if mode == "parent" {
					_, err = syscall.Wait4(app, nil, 0, nil)
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("shutdown exceeded 3s")
			}
			for _, pid := range pids {
				if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
					t.Errorf("owned helper %d survived shutdown: %v", pid, err)
				}
			}
			t.Logf("%s: exited and reaped helpers in %s", mode, time.Since(started))
		})
	}
}
