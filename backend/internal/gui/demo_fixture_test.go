package gui

import (
	"encoding/json"
	"os"
	"testing"
)

// The screenshot harness derives its public fictional data from the same demo
// as the IPC entry point. No live operations or private records are involved.
func TestExportDemoFixture(t *testing.T) {
	path := os.Getenv("LINE_DEMO_FIXTURE")
	if path == "" {
		t.Skip("capture harness only")
	}
	e := NewEngine(Operations{}, nil)
	defer e.Close()
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v := e.View()
	e.Handle(Command{Action: "photos-visible", ID: v.SelectedID, Session: v.Session, Selection: v.Selection, MessageIDs: []string{"104"}})
	e.Wait()
	ja := e.View()
	if ja.PhotoThumbnails["104"].Status != "ready" || ja.StickerImages["1001"] == "" {
		t.Fatal("demo media did not reach sanitized image state")
	}
	e.Handle(Command{Action: "locale", Text: "en"})
	en := e.View()
	ja.Session = "fictional-preview"
	en.Session = "fictional-preview"
	data, err := json.Marshal(map[string]View{"ja": ja, "en": en})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
