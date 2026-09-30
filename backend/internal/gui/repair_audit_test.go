package gui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kongesque/line-cli/internal/messaging"
	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
)

func TestAuditMessageProjectionPreservesFormatting(t *testing.T) {
	self := "u" + strings.Repeat("a", 43)
	want := "first\nsecond\tthird\r四行目"
	got := projectMessage(messaging.Message{ID: "1", From: self, CreatedTime: json.Number("1"), ContentType: 0, Text: want}, Account{ID: self, Name: "Me"}, nil)
	if got.Text != want {
		t.Fatalf("message formatting changed: want %q, got %q", want, got.Text)
	}
}

type blockedPreflightAPI struct {
	session.API
	entered chan struct{}
	release chan struct{}
	sends   atomic.Int32
}

func (a *blockedPreflightAPI) GetBlockedContactIds() ([]string, error) {
	close(a.entered)
	<-a.release
	return nil, nil
}
func (a *blockedPreflightAPI) SendMessage(_ int64, _ *line.Message) (*line.Message, error) {
	a.sends.Add(1)
	return &line.Message{ID: "88"}, nil
}

func TestAuditDirectCancelBeforeMutationMustNotDispatch(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	store := &memoryStore{s: &session.State{Version: 1, MID: self, AccessToken: "fictional", NoE2EE: true}}
	api := &blockedPreflightAPI{entered: make(chan struct{}), release: make(chan struct{})}
	manager := session.NewManager(store)
	manager.NewClient = func(string) session.API { return api }
	direct := &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := direct.Send(ctx, peer, "fictional", "", nil); done <- err }()
	<-api.entered
	cancel()
	close(api.release)
	<-done
	if got := api.sends.Load(); got != 0 {
		t.Fatalf("cancelled direct send dispatched %d remote mutations", got)
	}
}

func TestAuditModeChangeCancelsPendingMutationContext(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var dispatched atomic.Int32
	e, id := readyEngine(Operations{Send: func(ctx context.Context, chat, body, reply string, file *Attachment) (SendResult, error) {
		close(entered)
		<-release
		if ctx.Err() == nil {
			dispatched.Add(1)
		}
		return SendResult{}, ctx.Err()
	}})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.Handle(Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	<-entered
	e.Handle(Command{Action: "mode", Mode: "demo"})
	close(release)
	e.Wait()
	e.Close()
	if dispatched.Load() != 0 {
		t.Fatal("mode change left pending mutation context active")
	}
}

type incompleteContactAPI struct {
	session.API
	self, peer     string
	groupAuthError bool
	includeDetails bool
	blocked        bool
}

func (a *incompleteContactAPI) GetProfileContext(context.Context) (*line.Profile, error) {
	return &line.Profile{Mid: a.self, DisplayName: "Self"}, nil
}
func (a *incompleteContactAPI) GetAllContactIds() ([]string, error) { return []string{a.peer}, nil }
func (a *incompleteContactAPI) GetContactsV2([]string) (*line.ContactsResponse, error) {
	if a.includeDetails {
		return &line.ContactsResponse{Contacts: map[string]line.ContactWrapper{a.peer: {Contact: line.Contact{Mid: a.peer, DisplayName: "Peer"}}}}, nil
	}
	return &line.ContactsResponse{}, nil
}
func (a *incompleteContactAPI) GetBlockedContactIds() ([]string, error) {
	if a.blocked {
		return []string{a.peer}, nil
	}
	return nil, nil
}
func (a *incompleteContactAPI) GetMessageBoxes(line.MessageBoxesOptions) (*line.MessageBoxesResponse, error) {
	if a.groupAuthError {
		return &line.MessageBoxesResponse{MessageBoxes: []line.MessageBox{{ID: "c" + strings.Repeat("c", 43)}}}, nil
	}
	return &line.MessageBoxesResponse{}, nil
}
func (a *incompleteContactAPI) GetChats([]string, bool, bool) (*line.GetChatsResponse, error) {
	return nil, session.ErrSessionInvalidated
}

func auditDirect(api session.API, self string) *Direct {
	store := &memoryStore{s: &session.State{Version: 1, MID: self, AccessToken: "fictional", NoE2EE: true}}
	manager := session.NewManager(store)
	manager.NewClient = func(string) session.API { return api }
	return &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
}
func TestAuditOmittedContactDetailsMustNotGrantStartChatAuthority(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	snapshot, err := auditDirect(&incompleteContactAPI{self: self, peer: peer}, self).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Contacts) != 0 || !snapshot.ContactsFailed {
		t.Fatalf("unverified contact entered ready authority: %+v", snapshot.Contacts)
	}
}
func TestAuditGroupMetadataAuthLossMustPropagate(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	_, err := auditDirect(&incompleteContactAPI{self: self, peer: peer, groupAuthError: true}, self).Snapshot(context.Background())
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("group metadata auth loss ignored: %v", err)
	}
}

func TestAuditBlockedContactMustBeExcludedFromStartChat(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	snapshot, err := auditDirect(&incompleteContactAPI{self: self, peer: peer, includeDetails: true, blocked: true}, self).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, contact := range snapshot.Contacts {
		if contact.ID == peer {
			t.Fatalf("blocked contact appeared as selectable: %+v", contact)
		}
	}
}

func TestAuditSendWatchEchoMustNotDuplicateMessageID(t *testing.T) {
	gate := make(chan struct{})
	id := "u" + strings.Repeat("c", 43)
	e, _ := readyEngine(Operations{Send: func(context.Context, string, string, string, *Attachment) (SendResult, error) {
		<-gate
		return SendResult{ID: "88", ChatID: id}, nil
	}})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello", Sequence: 1})
	e.Handle(Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.mu.Lock()
	e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "88", Text: "hello", Timestamp: time.Now().UnixMilli(), Status: "ok"}})
	e.mu.Unlock()
	close(gate)
	e.Wait()
	count := 0
	for _, m := range e.View().Messages {
		if m.ID == "88" {
			count++
		}
	}
	e.Close()
	if count != 1 {
		t.Fatalf("send/watch echo left %d copies of message 88", count)
	}
}

func TestAuditRefreshPreservesRecentPreview(t *testing.T) {
	id := "u" + strings.Repeat("c", 43)
	e, _ := readyEngine(Operations{Snapshot: func(context.Context) (Snapshot, error) {
		return Snapshot{Account: Account{ID: "u" + strings.Repeat("d", 43), Name: "Me"}, Chats: []Chat{{ID: id, Preview: "トークを開く"}}, Contacts: []Contact{}}, nil
	}})
	e.mu.Lock()
	e.state.Chats[0].Preview = "recent message"
	e.state.SelectedID = ""
	e.mu.Unlock()
	e.Handle(Command{Action: "refresh"})
	e.Wait()
	got := e.View().Chats[0].Preview
	e.Close()
	if got != "recent message" {
		t.Fatalf("refresh erased recent preview: %q", got)
	}
}

func TestAuditStickerSendUpdatesChatPreview(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	previous := v.Chats[0].Preview
	c := Command{ID: v.SelectedID, Session: v.Session, Selection: v.Selection}
	c.Action = "sticker-open"
	e.Handle(c)
	e.Wait()
	v = e.View()
	if len(v.Stickers.Items) == 0 {
		t.Fatal("demo catalog did not populate")
	}
	s := v.Stickers.Items[0]
	c.Action, c.PackageID, c.StickerID = "sticker-choose", s.PackageID, s.ID
	e.Handle(c)
	c.Action = "sticker-send"
	e.Handle(c)
	e.Wait()
	got := e.View().Chats[0].Preview
	e.Close()
	if got == previous {
		t.Fatalf("sticker send kept stale preview %q", got)
	}
}
