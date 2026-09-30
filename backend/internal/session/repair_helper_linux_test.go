package session

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestSecretToolCancellationChild(t *testing.T) {
	if path := os.Getenv("LINE_GUI_SECRET_FIXTURE"); path != "" {
		os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
		for {
			time.Sleep(time.Second)
		}
	}
}
func TestSecretToolOperationCancellation(t *testing.T) {
	exe, _ := os.Executable()
	path := filepath.Join(t.TempDir(), "started")
	t.Setenv("LINE_GUI_SECRET_FIXTURE", path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan secretResult, 1)
	go func() {
		done <- runSecretToolContext(ctx, exe, []string{"-test.run=^TestSecretToolCancellationChild$"}, []byte("fictional-secret-stdin"))
	}()
	end := time.Now().Add(3 * time.Second)
	var pid int
	for time.Now().Before(end) {
		b, err := os.ReadFile(path)
		if err == nil {
			pid, _ = strconv.Atoi(string(b))
			break
		}
		time.Sleep(time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("synthetic secret helper did not start")
	}
	cancel()
	select {
	case r := <-done:
		if r.err == nil || len(r.output) != 0 {
			t.Fatal("cancelled helper returned success/output")
		}
	case <-time.After(time.Second):
		t.Fatal("secret helper ignored operation cancellation")
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("secret helper was not reaped: %v", err)
	}
}
