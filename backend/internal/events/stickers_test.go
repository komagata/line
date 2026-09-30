package events

import (
	"context"
	"strings"
	"testing"
)

func TestWatchStickerMetadataProjection(t *testing.T) {
	w, f, _, out := setup(t)
	f.streams = []stream{{frames: []frame{{"operation", `{"revision":"11","type":26,"message":{"id":"1","from":"u-peer","to":"u-self","contentType":7,"hasContent":true,"contentMetadata":{"STKID":"10","STKPKGID":"1","STKVER":"1","STKOPT":"S","private":"never project"}}}`}}}}
	if err := w.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := outputEvents(t, out)
	if len(events) != 1 || events[0].Message.Sticker == nil || events[0].Message.Sticker.ID != "10" || strings.Contains(out.String(), "never project") {
		t.Fatal("watch projection")
	}
}
