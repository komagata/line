package gui

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func readyEngine(ops Operations) (*Engine, string) {
	id := "u" + strings.Repeat("c", 43)
	e := NewEngine(ops, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	e.mu.Lock()
	e.state.Mode = "live"
	e.state.SelectedID = id
	e.state.Chats = []Chat{{ID: id, Name: "Alice"}}
	e.state.Account = Account{ID: "u" + strings.Repeat("d", 43), Name: "Me"}
	e.mu.Unlock()
	return e, id
}
func TestAuthLossDuringHistoryRevokesEditor(t *testing.T) {
	e, id := readyEngine(Operations{History: func(context.Context, string) ([]Message, error) { return nil, ErrUnauthenticated }})
	e.Handle(Command{Action: "select", ID: id})
	e.Wait()
	v := e.View()
	if v.Status != "unauthenticated" || v.SelectedID != "" || len(v.Chats) != 0 {
		t.Fatalf("auth context survived: %+v", v)
	}
	e.Close()
}
func TestHistoryCompletionMergesWatchMessage(t *testing.T) {
	gate := make(chan struct{})
	e, id := readyEngine(Operations{History: func(context.Context, string) ([]Message, error) {
		<-gate
		return []Message{{ID: "100", Text: "old", Status: "ok"}}, nil
	}})
	e.Handle(Command{Action: "select", ID: id})
	e.mu.Lock()
	e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "101", Text: "new", Timestamp: time.Now().UnixMilli(), Status: "ok"}})
	e.mu.Unlock()
	close(gate)
	e.Wait()
	if len(e.View().Messages) != 2 {
		t.Fatalf("watch message lost: %+v", e.View().Messages)
	}
	e.Close()
}
func TestAttachmentCancelledDuringSnapshotStaysCancelled(t *testing.T) {
	e, id := readyEngine(Operations{})
	dir := t.TempDir()
	name := dir + "/file"
	if err := os.WriteFile(name, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	v := e.View()
	e.Handle(Command{Action: "attach", ID: id, Session: v.Session, Selection: v.Selection, URL: fileURL(name)})
	e.Handle(Command{Action: "attach-cancel", ID: id, Session: v.Session, Selection: v.Selection})
	e.Wait()
	if e.View().Attachment != nil {
		t.Fatal("cancelled attachment returned")
	}
	e.Close()
}
func TestModeChangeDropsUnknownMutationResult(t *testing.T) {
	gate := make(chan struct{})
	e, id := readyEngine(Operations{Send: func(context.Context, string, string, string, *Attachment) (SendResult, error) {
		<-gate
		return SendResult{}, errors.New("unknown")
	}})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.Handle(Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.Handle(Command{Action: "mode", Mode: "demo"})
	close(gate)
	e.Wait()
	if got := e.View(); got.Mode != "demo" || got.SendStatus != "idle" {
		t.Fatalf("stale mutation changed new mode: %+v", got)
	}
	e.Close()
}
func TestWatchStopsOnModeChangeAndDropsLateMessage(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	var callback func(WatchEvent)
	e, id := readyEngine(Operations{Watch: func(ctx context.Context, cb func(WatchEvent)) error {
		callback = cb
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}})
	e.mu.Lock()
	e.startWatch()
	e.mu.Unlock()
	<-started
	e.Handle(Command{Action: "mode", Mode: "demo"})
	<-stopped
	callback(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "111", Text: "late", Timestamp: time.Now().UnixMilli()}})
	e.Wait()
	v := e.View()
	if v.Mode != "demo" || v.Watching || len(v.Messages) != 5 {
		t.Fatalf("watch survived mode: %+v", v)
	}
	e.Close()
}
func TestDuplicateWatchMessageDoesNotDoubleUnread(t *testing.T) {
	e, id := readyEngine(Operations{})
	e.mu.Lock()
	e.state.SelectedID = ""
	e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "222", Text: "hello", Timestamp: time.Now().UnixMilli()}})
	e.watchEvent(WatchEvent{Kind: "message", Revision: "2", ChatID: id, Message: &Message{ID: "222", Text: "hello", Timestamp: time.Now().UnixMilli()}})
	unread := e.state.Chats[0].Unread
	e.mu.Unlock()
	if unread != 1 {
		t.Fatalf("duplicate incremented unread to %d", unread)
	}
	e.Close()
}
func TestSelectionChangeClearsStagedSticker(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	e.mu.Lock()
	e.state.Stickers.Selected = &Sticker{ID: "1001", PackageID: "1"}
	e.mu.Unlock()
	e.Handle(Command{Action: "select", ID: "demo-2"})
	if e.View().Stickers.Selected != nil {
		t.Fatal("staged sticker crossed chat selection")
	}
	e.Close()
	_ = v
}
