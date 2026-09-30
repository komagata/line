package gui

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchOperationSchedulesBoundedResync(t *testing.T) {
	var reads atomic.Int32
	id := "u" + strings.Repeat("a", 43)
	e := NewEngine(Operations{Snapshot: func(context.Context) (Snapshot, error) {
		reads.Add(1)
		return Snapshot{Account: Account{ID: id, Name: "Self"}, Chats: []Chat{{ID: id}}, Contacts: []Contact{}}, nil
	}}, nil)
	e.mu.Lock()
	e.state.Status = "ready"
	e.resyncDelay = 10 * time.Millisecond
	e.watchEvent(WatchEvent{Kind: "operation", Revision: "1"})
	e.mu.Unlock()
	deadline := time.After(time.Second)
	for reads.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("operation did not resync")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	e.Close()
}
func TestFreshRemoteWatchNotifiesOnceUnlessFocused(t *testing.T) {
	var notices atomic.Int32
	id := "u" + strings.Repeat("b", 43)
	e := NewEngine(Operations{Notify: func(context.Context) bool { notices.Add(1); return true }}, nil)
	e.mu.Lock()
	e.state.Status = "ready"
	e.state.SelectedID = id
	e.state.Chats = []Chat{{ID: id}}
	e.noticeDelay = time.Millisecond
	e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "100", Text: "private", Timestamp: time.Now().UnixMilli()}})
	e.mu.Unlock()
	deadline := time.After(time.Second)
	for notices.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("no notification")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	v := e.View()
	e.Handle(Command{Action: "focus", ID: id, Session: v.Session, Selection: v.Selection, Focused: true})
	e.mu.Lock()
	e.watchEvent(WatchEvent{Kind: "message", Revision: "2", ChatID: id, Message: &Message{ID: "101", Text: "private", Timestamp: time.Now().UnixMilli()}})
	e.mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	if notices.Load() != 1 {
		t.Fatal("focused chat notified")
	}
	e.Close()
}
func TestAvatarPathRejectsURLAndTraversal(t *testing.T) {
	for _, p := range []string{"https://example.invalid/x", "../secret", "a//b", "a?b", "a%2Fb", "a\\b"} {
		if avatarPath(p) != "" {
			t.Fatalf("accepted %q", p)
		}
	}
	if avatarPath("/abc-DEF_123") != "abc-DEF_123" {
		t.Fatal("valid opaque path rejected")
	}
}
func TestWriterPrioritizesCoreStateOverOptionalImages(t *testing.T) {
	var output lockedBuffer
	writer := NewWriter(&output)
	view := initial()
	view.Status = "ready"
	view.Messages = []Message{{ID: "1", Text: "important"}}
	view.Preview = "data:image/png;base64," + strings.Repeat("A", 5<<20)
	if err := writer.Publish(view); err != nil {
		t.Fatalf("optional image killed state: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { writer.Run(ctx); close(done) }()
	deadline := time.After(time.Second)
	for output.Len() == 0 {
		select {
		case <-deadline:
			t.Fatal("no frame")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	<-done
	if !strings.Contains(output.String(), "important") || strings.Contains(output.String(), strings.Repeat("A", 1000)) {
		t.Fatal("core text or optional budget wrong")
	}
}
func TestDemoRetainsFictionalConversationAndPhotos(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	if len(v.Chats) != 5 || len(v.Messages) != 5 || len(v.Avatars) < 3 {
		t.Fatalf("demo parity: chats=%d messages=%d avatars=%d", len(v.Chats), len(v.Messages), len(v.Avatars))
	}
	e.Close()
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *lockedBuffer) Len() int       { b.mu.Lock(); defer b.mu.Unlock(); return b.b.Len() }
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }
func TestAvatarSamePathFetchedOnlyOnce(t *testing.T) {
	id1 := "u" + strings.Repeat("1", 43)
	id2 := "u" + strings.Repeat("2", 43)
	gate := make(chan struct{})
	var count atomic.Int32
	e := NewEngine(Operations{FetchAvatar: func(context.Context, string) string { count.Add(1); <-gate; return demoSage }}, nil)
	e.mu.Lock()
	e.state.Chats = []Chat{{ID: id1}, {ID: id2}}
	e.picturePaths[id1] = "shared"
	e.picturePaths[id2] = "shared"
	e.syncAvatars(false)
	e.mu.Unlock()
	deadline := time.After(time.Second)
	for count.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("fetch did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if count.Load() != 1 {
		t.Fatalf("duplicate CDN fetch: %d", count.Load())
	}
	close(gate)
	e.Wait()
	e.Close()
}
func TestQRDebugModeDoesNotEncodeSecret(t *testing.T) {
	t.Setenv("QRCODE_DEBUG", "1")
	if _, err := qrPNG("https://example.invalid/secret"); err == nil {
		t.Fatal("QR debug mode accepted")
	}
}
func TestNewHighPriorityAvatarEvictsLowerPriorityCache(t *testing.T) {
	hi := "u" + strings.Repeat("a", 43)
	lo := "u" + strings.Repeat("b", 43)
	e := NewEngine(Operations{}, nil)
	e.mu.Lock()
	e.state.SelectedID = hi
	e.state.Chats = []Chat{{ID: lo}}
	e.picturePaths[hi] = "high"
	e.picturePaths[lo] = "low"
	e.avatarData["low"] = strings.Repeat("x", (2<<20)-len(demoSage)+1)
	e.avatarCacheBytes = len(e.avatarData["low"])
	accepted := e.cacheAvatar("high", demoSage)
	_, old := e.avatarData["low"]
	e.mu.Unlock()
	if !accepted || old {
		t.Fatal("high-priority avatar was not admitted")
	}
	e.Close()
}
