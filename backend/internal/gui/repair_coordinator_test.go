package gui

import (
	"github.com/kongesque/line-cli/internal/messaging"
	"strings"
	"testing"
)

func TestCoordinatorPreservesMultiline(t *testing.T) {
	text := "一行目\n二行目\t三行目"
	got := projectMessage(messaging.Message{ID: "1", ContentType: 0, Status: "ok", Text: text}, Account{}, nil)
	if got.Text != text {
		t.Fatalf("want %q got %q", text, got.Text)
	}
}
func TestCoordinatorPreservesJapaneseBodyLimit(t *testing.T) {
	text := strings.Repeat("あ", 5000)
	got := projectMessage(messaging.Message{ID: "1", ContentType: 0, Status: "ok", Text: text}, Account{}, nil)
	if got.Text != text {
		t.Fatalf("5000 Japanese chars truncated to %d runes", len([]rune(got.Text)))
	}
}
