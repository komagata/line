package gui

import (
	"context"
	"strings"
	"testing"
)

func TestLocalChatAndDraftSurviveRefresh(t *testing.T) {
	me := "u" + strings.Repeat("a", 43)
	target := "u" + strings.Repeat("b", 43)
	ops := Operations{Snapshot: func(context.Context) (Snapshot, error) {
		return Snapshot{Account: Account{ID: me, Name: "Me"}, Chats: []Chat{}, Contacts: []Contact{{ID: target, Name: "Friend"}}}, nil
	}, History: func(context.Context, string) ([]Message, error) { return nil, nil }}
	e := NewEngine(ops, nil)
	e.mu.Lock()
	e.state.Account = Account{ID: me, Name: "Me"}
	e.state.Status = "ready"
	e.state.ContactsStatus = "ready"
	e.state.Contacts = []Contact{{ID: target, Name: "Friend"}}
	e.known[target] = Contact{ID: target, Name: "Friend"}
	e.mu.Unlock()
	v := e.View()
	e.Handle(Command{Action: "start-chat", ID: "", Session: v.Session, Selection: v.Selection, ContactID: target})
	e.Wait()
	v = e.View()
	e.Handle(Command{Action: "draft", ID: target, Session: v.Session, Selection: v.Selection, Text: "remember"})
	e.Handle(Command{Action: "refresh"})
	e.Wait()
	v = e.View()
	if v.SelectedID != target || v.Draft != "remember" || len(v.Chats) != 1 {
		t.Fatalf("local chat lost: %+v", v)
	}
	e.Close()
}
func TestDemoReactionAndUnsendPersistAcrossSelection(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	base := Command{ID: v.SelectedID, Session: v.Session, Selection: v.Selection}
	e.Handle(Command{Action: "react", ID: base.ID, Session: base.Session, Selection: base.Selection, MessageID: "101", Reaction: "like"})
	e.Wait()
	if e.View().Messages[0].Reactions[0].Name != "like" {
		t.Fatal("demo reaction not shown")
	}
	e.Handle(Command{Action: "unsend", ID: base.ID, Session: base.Session, Selection: base.Selection, MessageID: "102", Confirmed: true})
	e.Wait()
	e.Handle(Command{Action: "select", ID: "demo-2"})
	e.Handle(Command{Action: "select", ID: "demo-0"})
	for _, m := range e.View().Messages {
		if m.ID == "102" {
			t.Fatal("unsent message returned")
		}
	}
	e.Close()
}
func TestLiveSendUpdatesChatPreview(t *testing.T) {
	peer := "u" + strings.Repeat("c", 43)
	e, id := readyEngine(Operations{Send: func(context.Context, string, string, string, *Attachment) (SendResult, error) {
		return SendResult{ID: "99", ChatID: peer}, nil
	}})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.Handle(Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "hello"})
	e.Wait()
	if e.View().Chats[0].Preview != "hello" {
		t.Fatal("chat preview not updated")
	}
	e.Close()
}
