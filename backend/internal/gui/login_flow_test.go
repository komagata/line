package gui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGUIReplacementQRPinSuccessFlow(t *testing.T) {
	entered := make(chan struct{})
	var refreshes atomic.Int32
	id := "u" + strings.Repeat("e", 43)
	e := NewEngine(Operations{Login: func(ctx context.Context, confirm <-chan struct{}, emit func(LoginEvent) error) error {
		if err := emit(LoginEvent{Stage: "replace-confirmation"}); err != nil {
			return err
		}
		close(entered)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-confirm:
		}
		for _, stage := range []LoginEvent{{Stage: "scan", Image: "data:image/png;base64,fictional"}, {Stage: "phone", PIN: "123456"}, {Stage: "saving"}} {
			if err := emit(stage); err != nil {
				return err
			}
		}
		return nil
	}, Snapshot: func(context.Context) (Snapshot, error) {
		refreshes.Add(1)
		return Snapshot{Account: Account{ID: id, Name: "Me"}}, nil
	}}, nil)
	e.Handle(Command{Action: "login", Request: "ui-1"})
	<-entered
	v := e.View()
	if v.Login.Stage != "replace-confirmation" || !v.Login.CanConfirm {
		t.Fatal("replacement confirmation missing")
	}
	e.Handle(Command{Action: "login-confirm", Attempt: v.Login.Attempt})
	e.Wait()
	v = e.View()
	if v.Login.Stage != "success" || v.Login.Image != "" || v.Login.PIN != "" || refreshes.Load() != 1 {
		t.Fatalf("login result %+v refreshes %d", v.Login, refreshes.Load())
	}
	e.Close()
}
func TestGUILoginStorageFailureDoesNotClaimSuccess(t *testing.T) {
	var refreshes atomic.Int32
	e := NewEngine(Operations{Login: func(context.Context, <-chan struct{}, func(LoginEvent) error) error {
		return errors.New("storage failure")
	}, Snapshot: func(context.Context) (Snapshot, error) { refreshes.Add(1); return Snapshot{}, nil }}, nil)
	e.Handle(Command{Action: "login", Request: "ui-2"})
	e.Wait()
	v := e.View()
	if v.Login.Stage != "error" || refreshes.Load() != 0 {
		t.Fatalf("bad failure handling: %+v %d", v.Login, refreshes.Load())
	}
	e.Close()
}
func TestGUICancelAtPINSuppressesLateSuccess(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	e := NewEngine(Operations{Login: func(ctx context.Context, _ <-chan struct{}, emit func(LoginEvent) error) error {
		if err := emit(LoginEvent{Stage: "phone", PIN: "123456"}); err != nil {
			return err
		}
		close(entered)
		<-release
		return emit(LoginEvent{Stage: "saving"})
	}}, nil)
	e.Handle(Command{Action: "login", Request: "ui-3"})
	<-entered
	e.Handle(Command{Action: "login-cancel"})
	close(release)
	e.Wait()
	v := e.View()
	if v.Login.Stage != "uncertain" || v.Login.PIN != "" {
		t.Fatalf("cancel at PIN: %+v", v.Login)
	}
	e.Close()
}
