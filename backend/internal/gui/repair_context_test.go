package gui

import (
	"context"
	"errors"
	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type coordinatorTransport func(*http.Request) (*http.Response, error)

func (f coordinatorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCoordinatorHistoryCancellationReachesHTTP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	done := make(chan struct{})
	store := &memoryStore{s: &session.State{Version: 1, MID: "u" + strings.Repeat("a", 43), AccessToken: "fictional", NoE2EE: true}}
	manager := session.NewManager(store)
	manager.NewClient = func(token string) session.API {
		c := line.NewClient(token)
		c.HTTPClient = &http.Client{Transport: coordinatorTransport(func(r *http.Request) (*http.Response, error) {
			entered <- r.Context()
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-release:
				return nil, errors.New("fixture release")
			}
		})}
		return c
	}
	d := &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
	go func() { defer close(done); d.History(ctx, "u"+strings.Repeat("b", 43)) }()
	var request context.Context
	select {
	case request = <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("HTTP not reached")
	}
	cancel()
	select {
	case <-request.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("cancelled history leaves HTTP request alive")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fixture did not stop")
	}
}
func TestCoordinatorModeSwitchCancelsPendingMutation(t *testing.T) {
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	e := NewEngine(Operations{Send: func(ctx context.Context, chat, text, reply string, a *Attachment) (SendResult, error) {
		entered <- ctx
		<-release
		return SendResult{}, ctx.Err()
	}}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	e.mu.Lock()
	e.state.Mode = "live"
	e.state.SelectedID = "u" + strings.Repeat("b", 43)
	e.state.Chats = []Chat{{ID: e.state.SelectedID}}
	e.mu.Unlock()
	v := e.View()
	e.Handle(Command{Action: "draft", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, Text: "fictional", Sequence: 1})
	e.Handle(Command{Action: "send", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, Text: "fictional"})
	ctx := <-entered
	e.Handle(Command{Action: "mode", Mode: "demo"})
	if ctx.Err() == nil {
		t.Error("mode switch does not cancel pending mutation context")
	}
	close(release)
	e.Wait()
	e.Close()
}

func TestRepairHTTPMutationCancellationBeforeDispatchAndDuringResponse(t *testing.T) {
	for _, stage := range []string{"preflight", "dispatched"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var writes atomic.Int32
			entered := make(chan struct{}, 1)
			store := &memoryStore{s: &session.State{Version: 1, MID: "u" + strings.Repeat("a", 43), AccessToken: "fictional", NoE2EE: true}}
			manager := session.NewManager(store)
			manager.NewClient = func(token string) session.API {
				c := line.NewClient(token)
				c.HTTPClient = &http.Client{Transport: coordinatorTransport(func(r *http.Request) (*http.Response, error) {
					if strings.HasSuffix(r.URL.Path, "getBlockedContactIds") {
						if stage == "preflight" {
							cancel()
						}
						return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":[]}`)), Header: make(http.Header)}, nil
					}
					writes.Add(1)
					entered <- struct{}{}
					<-r.Context().Done()
					return nil, r.Context().Err()
				})}
				return c
			}
			d := &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
			done := make(chan error, 1)
			go func() { _, err := d.Send(ctx, "u"+strings.Repeat("b", 43), "fictional", "", nil); done <- err }()
			if stage == "dispatched" {
				select {
				case <-entered:
					cancel()
				case <-time.After(time.Second):
					t.Fatal("mutation not dispatched")
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled mutation returned success")
				}
			case <-time.After(time.Second):
				t.Fatal("HTTP mutation ignored cancellation")
			}
			want := int32(0)
			if stage == "dispatched" {
				want = 1
			}
			if writes.Load() != want {
				t.Fatalf("mutations=%d want=%d", writes.Load(), want)
			}
		})
	}
}
