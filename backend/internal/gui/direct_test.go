package gui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kongesque/line-cli/internal/messaging"
)

func TestProjectMessageSanitizesUntrustedTextAndKeepsAttachment(t *testing.T) {
	raw := messaging.Message{ID: "42", From: "u" + strings.Repeat("a", 43), CreatedTime: json.Number("123"), ContentType: 14, Status: "attachment", FileName: "x\u001b[31m.txt"}
	got := projectMessage(raw, Account{ID: "u" + strings.Repeat("b", 43), Name: "Me"}, nil)
	if got.Text != "［ファイル：x[31m.txt］" || !got.Downloadable || got.Status != "ok" {
		t.Fatalf("projection: %+v", got)
	}
}
func TestQRCodeGeneratedInProcessWithinImageBound(t *testing.T) {
	image, err := qrPNG("https://example.invalid/qr?secret=fictional")
	if err != nil || !strings.HasPrefix(image, "data:image/png;base64,") || len(image) > 87407 {
		t.Fatalf("QR: %v, length %d", err, len(image))
	}
}
