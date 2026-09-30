package gui

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRemote struct {
	mu    sync.Mutex
	sends int
	gate  chan struct{}
}

func (f *fakeRemote) Send(ctx context.Context, chat, body, reply string, file *Attachment) (SendResult, error) {
	f.mu.Lock()
	f.sends++
	f.mu.Unlock()
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return SendResult{}, ctx.Err()
		}
	}
	return SendResult{ID: "123", ChatID: chat, Encrypted: true}, nil
}
func (f *fakeRemote) Count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.sends }

func TestDemoModeRejectsStaleDraft(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	if v.Mode != "demo" || v.Status != "ready" || v.SelectedID == "" {
		t.Fatalf("demo state: %+v", v)
	}
	e.Handle(Command{Action: "draft", ID: v.SelectedID, Session: "stale", Selection: v.Selection, Text: "wrong", Sequence: 1})
	if e.View().Draft != "" {
		t.Fatal("stale session changed draft")
	}
	e.Handle(Command{Action: "draft", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, Text: "hello", Sequence: 2})
	if e.View().Draft != "hello" || e.View().DraftAck != 2 {
		t.Fatal("current draft not acknowledged")
	}
}

func TestDuplicateSendWhilePendingMakesOneCall(t *testing.T) {
	f := &fakeRemote{gate: make(chan struct{})}
	e := NewEngine(Operations{Send: f.Send}, nil)
	id := "u" + strings.Repeat("a", 43)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	e.mu.Lock()
	e.state.Mode = "live"
	e.state.SelectedID = id
	e.state.Chats = []Chat{{ID: id, Name: "Alice"}}
	e.mu.Unlock()
	v := e.View()
	draft := Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello", Sequence: 1}
	e.Handle(draft)
	send := Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"}
	e.Handle(send)
	e.Handle(send)
	deadline := time.After(time.Second)
	for f.Count() == 0 {
		select {
		case <-deadline:
			t.Fatal("send did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if f.Count() != 1 {
		t.Fatalf("%d sends", f.Count())
	}
	close(f.gate)
	e.Wait()
	if f.Count() != 1 || e.View().SendStatus != "idle" {
		t.Fatalf("send result %+v / %d", e.View(), f.Count())
	}
}

func TestBoundedLinesRejectsOversizeAndRate(t *testing.T) {
	valid := NewReader(strings.NewReader(strings.Repeat(" ", 5000)+"{}\n"), func(json.RawMessage) {})
	if err := valid.Run(context.Background()); err != nil {
		t.Fatalf("valid long line rejected: %v", err)
	}
	var seen int
	r := NewReader(strings.NewReader(strings.Repeat("x", 65537)+"\n"), func(json.RawMessage) { seen++ })
	if err := r.Run(context.Background()); err == nil || seen != 0 {
		t.Fatalf("oversize accepted: %v / %d", err, seen)
	}
}

func TestReplyMustRemainInSelectedHistory(t *testing.T) {
	f := &fakeRemote{}
	e := NewEngine(Operations{Send: f.Send}, nil)
	id := "u" + strings.Repeat("b", 43)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	e.mu.Lock()
	e.state.Mode = "live"
	e.state.SelectedID = id
	e.state.Chats = []Chat{{ID: id}}
	e.state.Messages = []Message{{ID: "42", Text: "old", Sender: "A"}}
	e.mu.Unlock()
	v := e.View()
	base := Command{ID: id, Session: v.Session, Selection: v.Selection}
	e.Handle(Command{Action: "reply", ID: id, Session: base.Session, Selection: base.Selection, MessageID: "42"})
	e.mu.Lock()
	e.state.Messages = nil
	e.mu.Unlock()
	e.Handle(Command{Action: "draft", ID: id, Session: base.Session, Selection: base.Selection, Text: "hello"})
	e.Handle(Command{Action: "send", ID: id, Session: base.Session, Selection: base.Selection, Text: "hello", ReplyTo: "42"})
	e.Wait()
	if f.Count() != 0 {
		t.Fatal("sent with missing reply target")
	}
}
func TestUnsendRequiresOwnMessageAndConfirmation(t *testing.T) {
	var calls int
	e := NewEngine(Operations{Action: func(context.Context, string, string, string, string, bool) (ActionResult, error) {
		calls++
		return ActionResult{Action: "unsend", ChatID: "demo-0", MessageID: "101"}, nil
	}}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	c := Command{Action: "unsend", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, MessageID: "101", Confirmed: true}
	e.Handle(c)
	e.Wait()
	if calls != 0 {
		t.Fatal("unsent another sender's message")
	}
	c.MessageID = "102"
	c.Confirmed = false
	e.Handle(c)
	e.Wait()
	if calls != 0 {
		t.Fatal("unsent without confirmation")
	}
}
func TestLoginCancelInvalidatesLateCallback(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	e := NewEngine(Operations{Login: func(ctx context.Context, _ <-chan struct{}, emit func(LoginEvent) error) error {
		close(entered)
		<-release
		return emit(LoginEvent{Stage: "scan", Image: "late"})
	}}, nil)
	e.Handle(Command{Action: "login", Request: "ui-1"})
	<-entered
	e.Handle(Command{Action: "login-cancel"})
	close(release)
	e.Wait()
	if v := e.View(); v.Login.Stage != "cancelled" || v.Login.Image != "" {
		t.Fatalf("late login event: %+v", v.Login)
	}
}
func TestStickerCannotSendWithoutOwnedSelection(t *testing.T) {
	var sends int
	e := NewEngine(Operations{SendSticker: func(context.Context, string, string, string, string) (SendResult, error) {
		sends++
		return SendResult{ID: "1", ChatID: "demo-0"}, nil
	}}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	e.Handle(Command{Action: "sticker-send", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, PackageID: "1", StickerID: "1001", Text: ""})
	e.Wait()
	if sends != 0 {
		t.Fatal("unowned sticker send")
	}
}
func TestReaderRejectsMoreThan200CommandsPerSecond(t *testing.T) {
	var input strings.Builder
	for i := 0; i < 201; i++ {
		input.WriteString("{}\n")
	}
	reader := NewReader(strings.NewReader(input.String()), func(json.RawMessage) {})
	if err := reader.Run(context.Background()); err == nil {
		t.Fatal("201 commands accepted")
	}
}
func TestEngineBoundsConcurrentJobsAt32(t *testing.T) {
	gate := make(chan struct{})
	e, id := readyEngine(Operations{History: func(context.Context, string) ([]Message, error) { <-gate; return nil, nil }})
	for i := 0; i < 32; i++ {
		e.Handle(Command{Action: "select", ID: id})
	}
	before := e.View().Selection
	e.Handle(Command{Action: "select", ID: id})
	if got := e.View().Selection; got != before {
		t.Fatalf("33rd job accepted: %d -> %d", before, got)
	}
	close(gate)
	e.Wait()
	e.Close()
}
