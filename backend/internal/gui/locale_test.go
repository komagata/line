package gui

import (
	"encoding/json"
	"github.com/kongesque/line-cli/internal/messaging"
	"testing"
)

func TestLocaleSwitchPreservesDemoContextAndDraft(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, Text: "設定", Sequence: 1})
	e.Handle(Command{Action: "locale", Text: "en"})
	en := e.View()
	if en.StatusText != "Fictional conversation · Demo" {
		t.Fatalf("English status: %q", en.StatusText)
	}
	if en.Chats[0].Name != "Alex Morgan" || en.Messages[0].Text != "What time shall we meet tomorrow?" {
		t.Fatal("English fictional fixture missing")
	}
	if en.Draft != "設定" || en.Session != v.Session || en.Selection != v.Selection || en.SelectedID != v.SelectedID {
		t.Fatal("locale reset conversation")
	}
	e.Handle(Command{Action: "locale", Text: "invalid"})
	if e.View().StatusText != en.StatusText {
		t.Fatal("invalid locale accepted")
	}
	e.Handle(Command{Action: "locale", Text: "ja"})
	if e.View().Chats[0].Name != v.Chats[0].Name || e.View().Draft != "設定" {
		t.Fatal("Japanese restoration lost data")
	}
	e.Handle(Command{Action: "locale", Text: "en"})
	e.Handle(Command{Action: "mode", Mode: "live"})
	if e.View().StatusText != "Preparing…" {
		t.Fatal("locale lost across mode change")
	}
}

func TestLocaleDoesNotTranslateRealContent(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.mu.Lock()
	e.state.StatusText = "接続中"
	e.state.Account = Account{Name: "自分"}
	e.state.Chats = []Chat{{ID: "real", Name: "山田 太郎", Preview: "トークを開く"}}
	e.state.Messages = []Message{{ID: "101", Text: "このメッセージは復号できませんでした", Sender: "自分", Day: "日時不明", FileName: "［画像］"}}
	e.state.Draft = "準備しています…"
	e.state.Reply = &Reply{ID: "101", Text: "［画像］", Sender: "自分"}
	e.mu.Unlock()
	e.Handle(Command{Action: "locale", Text: "en"})
	v := e.View()
	if v.StatusText != "Connected" {
		t.Fatal(v.StatusText)
	}
	if v.Account.Name != "自分" || v.Chats[0].Name != "山田 太郎" || v.Chats[0].Preview != "トークを開く" || v.Messages[0].Text != "このメッセージは復号できませんでした" || v.Messages[0].Sender != "自分" || v.Messages[0].FileName != "［画像］" || v.Reply.Text != "［画像］" || v.Draft != "準備しています…" {
		t.Fatal("real content translated")
	}
}

func TestLocaleGeneratedMediaAndReplyKeepFilenames(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.mu.Lock()
	e.state.Messages = []Message{{ID: "1", Text: "［ファイル：設定］", ContentType: 14, FileName: "設定", GeneratedText: true}, {ID: "2", Text: "［画像］", ContentType: 1, GeneratedText: true}, {ID: "3", Text: "このメッセージは復号できませんでした", Status: "decryption_failed", GeneratedText: true}, {ID: "4", Text: "［スタンプ］ 元のタイトル", ContentType: 7, GeneratedText: true, Sticker: &Sticker{Alt: "元のタイトル"}}}
	source := e.state.Messages[0]
	e.state.Reply = &Reply{ID: "1", Text: source.Text, Source: &source}
	e.state.Chats = []Chat{{ID: "real", Preview: source.Text, PreviewMessage: &source}}
	e.state.Login = loginView("scan", "attempt", "request", "", "", "")
	e.state.Account = Account{Name: "自分", NameUnavailable: true}
	e.state.Contacts = []Contact{{Name: "名前未設定", NameUnavailable: true}, {Name: "名前未設定"}}
	e.mu.Unlock()
	e.Handle(Command{Action: "locale", Text: "en"})
	v := e.View()
	if v.Messages[0].Text != "[File: 設定]" || v.Messages[0].FileName != "設定" || v.Reply.Text != "[File: 設定]" || v.Chats[0].Preview != "[File: 設定]" {
		t.Fatal("generated file projection lost filename")
	}
	if v.Messages[1].Text != "[Image]" || v.Messages[2].Text != "This message could not be decrypted" || v.Messages[3].Text != "[Sticker] 元のタイトル" || v.Messages[3].Sticker.Alt != "元のタイトル" {
		t.Fatal("generated media projection failed")
	}
	if v.Account.Name != "Me" || v.Contacts[0].Name != "Unnamed contact" || v.Contacts[1].Name != "名前未設定" || v.Login.StatusText != "Scan the QR code with LINE on your phone" {
		t.Fatal("generated names/login projection failed")
	}
	e.Handle(Command{Action: "locale", Text: "ja"})
	if e.View().Messages[0].Text != source.Text {
		t.Fatal("projection mutated canonical media")
	}
}

func TestLocaleKeepsUserCreatedDemoMessageAndPreview(t *testing.T) {
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.Handle(Command{Action: "mode", Mode: "demo"})
	e.mu.Lock()
	e.sentMessage("demo-0", Message{ID: "user-created", Text: "山田 太郎", Timestamp: 100, Day: "2026/10/01"})
	e.mu.Unlock()
	e.Handle(Command{Action: "locale", Text: "en"})
	v := e.View()
	if v.Chats[0].Preview != "山田 太郎" || v.Messages[len(v.Messages)-1].Text != "山田 太郎" {
		t.Fatal("demo switch rewrote user-created message")
	}
	e.Handle(Command{Action: "locale", Text: "ja"})
	if e.View().Messages[len(v.Messages)-1].Text != "山田 太郎" {
		t.Fatal("demo content lost")
	}
}

func TestDirectLocaleMetadataDistinguishesLiteralTextAndMedia(t *testing.T) {
	account := Account{ID: "self", Name: "自分"}
	plain := projectMessage(messaging.Message{ID: "1", From: "other", CreatedTime: json.Number("100"), Text: "［画像］"}, account, map[string]string{"other": "名前未設定"})
	media := projectMessage(messaging.Message{ID: "2", From: "other", CreatedTime: json.Number("100"), ContentType: 1}, account, map[string]string{"other": "名前未設定"}, map[string]bool{"other": true})
	if plain.GeneratedText || plain.SenderUnavailable || !media.GeneratedText || !media.SenderUnavailable {
		t.Fatal("incorrect localization metadata")
	}
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.mu.Lock()
	e.state.Messages = []Message{plain, media}
	e.mu.Unlock()
	e.Handle(Command{Action: "locale", Text: "en"})
	v := e.View()
	if v.Messages[0].Text != "［画像］" || v.Messages[0].Sender != "名前未設定" || v.Messages[1].Text != "[Image]" || v.Messages[1].Sender != "Unnamed contact" {
		t.Fatal("literal text/name mistaken for generated labels")
	}
}
