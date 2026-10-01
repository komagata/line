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
	ja := e.View()
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
