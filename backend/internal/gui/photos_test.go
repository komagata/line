package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kongesque/line-cli/internal/messaging"
	"github.com/kongesque/line-cli/pkg/line"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fictionalJPEG(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 251), uint8(y % 251), 120, 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
	return b.Bytes()
}
func TestOrdinaryPhotoNeedsThumbnail(t *testing.T) {
	data := fictionalJPEG(2048, 1536)
	if got := photoThumbnail(data); got == "" {
		t.Fatal("ordinary phone photo rejected by current inline sanitizer")
	}
}

func TestVisiblePhotoUsesAuthenticatedDownload(t *testing.T) {
	var calls atomic.Int32
	data := fictionalJPEG(1024, 768)
	e, id := readyEngine(Operations{Download: func(ctx context.Context, chat, message string) ([]byte, error) {
		if message != "123" {
			return nil, context.Canceled
		}
		calls.Add(1)
		return data, nil
	}})
	defer e.Close()
	e.mu.Lock()
	e.state.Messages = []Message{{ID: "123", ContentType: 1, Downloadable: true}}
	e.mu.Unlock()
	v := e.View()
	e.Handle(Command{Action: "photos-visible", ID: id, Session: v.Session, Selection: v.Selection, MessageIDs: []string{"123"}})
	e.Wait()
	if calls.Load() != 1 {
		t.Fatal("visible photo never reaches authenticated download operation")
	}
}

func TestOfflinePhotoUsesReadOnlyAuthenticatedDownload(t *testing.T) {
	var calls atomic.Int32
	data := fictionalJPEG(640, 480)
	e, id := readyEngine(Operations{Download: func(ctx context.Context, chat, message string) ([]byte, error) {
		calls.Add(1)
		return bytes.Clone(data), nil
	}})
	defer e.Close()
	ids := photoRows(e, 1)
	e.mu.Lock()
	e.state.Status = "offline" // Watch failure stops the stream but keeps the session authenticated.
	e.mu.Unlock()
	e.Handle(photoCommand(e, id, ids...))
	e.Wait()
	if calls.Load() != 1 || e.View().PhotoThumbnails[ids[0]].Status != "ready" {
		t.Fatal("offline read-only thumbnail did not complete")
	}
	// A lost authentication response must still reset identity from offline.
	e.mu.Lock()
	e.state.PhotoThumbnails = map[string]PhotoThumbnail{}
	e.state.Status = "offline"
	e.mu.Unlock()
	e.ops.Download = func(context.Context, string, string) ([]byte, error) { return nil, ErrUnauthenticated }
	retry := photoCommand(e, id, ids...)
	e.Handle(retry)
	e.Wait()
	if e.View().Status != "unauthenticated" || e.View().SelectedID != "" {
		t.Fatal("unauthenticated offline download did not reset identity")
	}
}

func TestPhotoStatusGateRejectsNonReadyOfflineStatesAndStaleContext(t *testing.T) {
	var calls atomic.Int32
	e, id := readyEngine(Operations{Download: func(context.Context, string, string) ([]byte, error) {
		calls.Add(1)
		return fictionalJPEG(20, 20), nil
	}})
	defer e.Close()
	ids := photoRows(e, 1)
	for _, status := range []string{"idle", "checking", "error"} {
		e.mu.Lock()
		e.state.Status = status
		e.mu.Unlock()
		e.Handle(photoCommand(e, id, ids...))
		e.Wait()
	}
	stale := photoCommand(e, id, ids...)
	stale.Selection++
	e.mu.Lock()
	e.state.Status = "offline"
	e.mu.Unlock()
	e.Handle(stale)
	e.Wait()
	if calls.Load() != 0 {
		t.Fatal("disallowed state or stale context downloaded")
	}
}

func photoCommand(e *Engine, id string, ids ...string) Command {
	v := e.View()
	return Command{Action: "photos-visible", ID: id, Session: v.Session, Selection: v.Selection, MessageIDs: ids}
}
func photoRows(e *Engine, n int) []string {
	ids := []string{}
	rows := []Message{}
	for i := 0; i < n; i++ {
		id := strconv.Itoa(100 + i)
		ids = append(ids, id)
		rows = append(rows, Message{ID: id, ContentType: 1, Downloadable: true, Text: "Fictional caption"})
	}
	e.mu.Lock()
	e.state.Messages = rows
	e.mu.Unlock()
	return ids
}
func waitPhoto(t *testing.T, ch <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-ch:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("thumbnail did not start")
		return nil
	}
}
func TestPhotoConcurrencyQueueDedupAndIgnoredCancellation(t *testing.T) {
	started := make(chan context.Context, 20)
	gate := make(chan struct{})
	var calls atomic.Int32
	data := fictionalJPEG(900, 600)
	e, id := readyEngine(Operations{Download: func(ctx context.Context, chat, message string) ([]byte, error) {
		calls.Add(1)
		started <- ctx
		<-gate
		return bytes.Clone(data), nil
	}})
	defer e.Close()
	defer close(gate)
	ids := photoRows(e, 100)
	bad := photoCommand(e, id, ids...)
	e.Handle(bad)
	if calls.Load() != 0 {
		t.Fatal("100-message request must be rejected")
	}
	c := photoCommand(e, id, ids[:8]...)
	e.Handle(c)
	ctx1 := waitPhoto(t, started)
	ctx2 := waitPhoto(t, started)
	e.Handle(c)
	e.Handle(c)
	e.mu.Lock()
	active, queue := e.photoActive, len(e.photoQueue)
	e.mu.Unlock()
	if active != 2 || queue > 8 || calls.Load() != 2 {
		t.Fatal("unbounded or duplicate downloads")
	}
	hide := c
	hide.Action = "photos-hide"
	e.Handle(hide)
	if ctx1.Err() == nil || ctx2.Err() == nil || len(e.View().PhotoThumbnails) != 0 {
		t.Fatal("hide did not cancel or clear private pictures")
	}
	// Ignoring cancellation cannot release slots or let the next view exceed two.
	e.Handle(c)
	e.mu.Lock()
	active = e.photoActive
	e.mu.Unlock()
	if active != 2 || calls.Load() != 2 {
		t.Fatal("cancel released slots before worker completion")
	}
}
func TestPhotoStaleCompletionAcrossLifecycle(t *testing.T) {
	for _, change := range []string{"select", "mode", "login", "auth", "hide", "dispose"} {
		t.Run(change, func(t *testing.T) {
			started := make(chan context.Context, 1)
			gate := make(chan struct{})
			data := fictionalJPEG(600, 900)
			e, id := readyEngine(Operations{Download: func(ctx context.Context, chat, message string) ([]byte, error) {
				started <- ctx
				<-gate
				return bytes.Clone(data), nil
			}, Login: func(ctx context.Context, confirm <-chan struct{}, emit func(LoginEvent) error) error {
				<-ctx.Done()
				return ctx.Err()
			}})
			ids := photoRows(e, 1)
			c := photoCommand(e, id, ids...)
			e.Handle(c)
			ctx := waitPhoto(t, started)
			switch change {
			case "select":
				e.Handle(Command{Action: "select", ID: id}) // even reselecting revokes selection
			case "mode":
				e.Handle(Command{Action: "mode", Mode: "demo"})
			case "login":
				e.Handle(Command{Action: "login"})
			case "auth":
				e.mu.Lock()
				e.loseAuthentication()
				e.mu.Unlock()
			case "hide":
				c.Action = "photos-hide"
				e.Handle(c)
			case "dispose":
				e.mu.Lock()
				e.disposed = true
				e.epoch++
				e.cancel()
				e.stopJobs()
				e.state.AvatarClear()
				e.mu.Unlock()
			}
			if ctx.Err() == nil {
				t.Fatal("old picture request survived lifecycle change")
			}
			close(gate)
			if change == "login" {
				e.Handle(Command{Action: "login-cancel"})
			}
			e.Wait()
			if len(e.View().PhotoThumbnails) != 0 {
				t.Fatal("stale image published into new context")
			}
			e.Close()
		})
	}
}
func TestPhotoFailureRetryAndNonPhotoRejection(t *testing.T) {
	var calls atomic.Int32
	data := fictionalJPEG(1200, 900)
	e, id := readyEngine(Operations{Download: func(ctx context.Context, chat, message string) ([]byte, error) {
		n := calls.Add(1)
		if n == 1 {
			return nil, errors.New("private transport detail")
		}
		return bytes.Clone(data), nil
	}})
	defer e.Close()
	ids := photoRows(e, 1)
	c := photoCommand(e, id, ids...)
	e.Handle(c)
	e.Wait()
	if e.View().PhotoThumbnails[ids[0]].Status != "error" {
		t.Fatal("failure has no readable state")
	}
	e.Handle(c)
	e.Wait()
	if calls.Load() != 1 {
		t.Fatal("automatic retry loop")
	}
	retry := c
	retry.Action = "photo-retry"
	retry.MessageID = ids[0]
	e.Handle(retry)
	e.Wait()
	if calls.Load() != 2 || e.View().PhotoThumbnails[ids[0]].Status != "ready" {
		t.Fatal("explicit retry did not refetch")
	}
	e.Handle(retry)
	e.Wait()
	if calls.Load() != 3 {
		t.Fatal("explicit retry must refetch even when backend cache is ready")
	}
	bad := c
	bad.MessageIDs = []string{"https://example.org/evil", "../secret", "999"}
	e.Handle(bad)
	e.Wait()
	if calls.Load() != 3 {
		t.Fatal("unsafe/non-photo request downloaded")
	}
	stale := c
	stale.Session = "old"
	e.Handle(stale)
	e.Wait()
	if calls.Load() != 3 {
		t.Fatal("stale authority downloaded")
	}
}
func TestPhotoThumbnailBoundsAndInvalidData(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not an image"), []byte("<svg/>"), make([]byte, MaxFile+1), fictionalJPEG(8193, 1)} {
		if photoThumbnail(data) != "" {
			t.Fatal("unsafe or excessive data accepted")
		}
	}
	// Config alone must reject a decompression bomb before allocating pixel storage.
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	bomb := bytes.Clone(b.Bytes())
	binary.BigEndian.PutUint32(bomb[16:20], 8192)
	binary.BigEndian.PutUint32(bomb[20:24], 8192)
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	cfg, format, err := image.DecodeConfig(bytes.NewReader(bomb))
	if err != nil || format != "png" || cfg.Width != 8192 || cfg.Height != 8192 {
		t.Fatal("bomb fixture does not advertise excessive dimensions")
	}
	if photoThumbnail(bomb) != "" {
		t.Fatal("pixel-count bomb accepted")
	}
	for _, size := range [][2]int{{2048, 1536}, {500, 2000}, {1, 1}} {
		uri := photoThumbnail(fictionalJPEG(size[0], size[1]))
		if !isDataImage(uri) {
			t.Fatal("bounded JPEG not produced")
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/jpeg;base64,"))
		if err != nil || len(data) > 65536 {
			t.Fatal("thumbnail byte limit")
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != "jpeg" || cfg.Width > 512 || cfg.Height > 512 {
			t.Fatal("thumbnail dimension or MIME limit")
		}
		if math.Abs(float64(cfg.Width)/float64(cfg.Height)-float64(size[0])/float64(size[1])) > 0.01 {
			t.Fatal("aspect ratio changed")
		}
	}
	var p bytes.Buffer
	png.Encode(&p, image.NewRGBA(image.Rect(0, 0, 100, 50)))
	if !isDataImage(photoThumbnail(p.Bytes())) {
		t.Fatal("ordinary PNG rejected")
	}
	jpegData := fictionalJPEG(30, 20)
	if photoThumbnail(jpegData[:len(jpegData)/2]) != "" {
		t.Fatal("truncated image decoded")
	}
}
func TestPhotoBudgetAndEviction(t *testing.T) {
	e, id := readyEngine(Operations{})
	defer e.Close()
	_ = id
	photoRows(e, 100)
	e.mu.Lock()
	defer e.mu.Unlock()
	data := "data:image/jpeg;base64," + strings.Repeat("A", 87000)
	e.photoVisible = map[string]bool{}
	for i := 0; i < 100; i++ {
		key := strconv.Itoa(100 + i)
		e.photoVisible[key] = true
		if !e.cachePhoto(key, data) {
			e.setPhoto(key, PhotoThumbnail{Status: "error"})
		}
	}
	total := 2
	for key, p := range e.state.PhotoThumbnails {
		total += photoCost(key, p)
	}
	if total > photoBudget {
		t.Fatal("total cache/frame image budget exceeded")
	}
	// Offscreen data may be reclaimed, without automatically retrying evictions.
	e.photoVisible = map[string]bool{"199": true}
	if !e.cachePhoto("199", data) {
		t.Fatal("offscreen cache did not free capacity")
	}
	if e.state.PhotoThumbnails["100"].Status != "error" {
		t.Fatal("evicted photo could automatically loop")
	}
}

func TestWriterPrioritizesCoreOverPhotoThumbnails(t *testing.T) {
	v := initial()
	v.Messages = []Message{{ID: "123", Text: "Fictional caption", ContentType: 1}}
	v.PhotoThumbnails = map[string]PhotoThumbnail{"123": {Status: "ready", Data: strings.Repeat("x", MaxFrame)}}
	w := NewWriter(nil)
	if err := w.Publish(v); err != nil {
		t.Fatal("optional thumbnails killed core state", err)
	}
	if len(w.latest) > MaxFrame || !bytes.Contains(w.latest, []byte("Fictional caption")) {
		t.Fatal("frame/core contract violated")
	}
	if v.PhotoThumbnails["123"].Data == "" {
		t.Fatal("writer mutated engine image cache")
	}
}

func TestStickerMarkerRecoveryThroughHistoryAndWatch(t *testing.T) {
	decoder := &messaging.Client{}
	raw := &line.Message{ID: "123", ContentType: 7, CreatedTime: "1700000000000", From: "fictional", ContentMetadata: map[string]string{"STKID": "1001", "STKVER": "1", "e2eeVersion": "2", "STKTXT": "Fictional sticker"}}
	projected := projectMessage(decoder.Decode("fixture", raw), Account{}, map[string]string{"fictional": "Fictional sender"})
	if projected.Sticker == nil {
		t.Fatal("safe sticker lost at GUI projection")
	}
	e, id := readyEngine(Operations{History: func(context.Context, string) ([]Message, error) { return []Message{projected}, nil }, FetchSticker: func(context.Context, Sticker) string { return demoSage }})
	defer e.Close()
	e.Handle(Command{Action: "select", ID: id})
	e.Wait()
	if e.View().StickerImages["1001"] == "" || e.View().Messages[0].Sticker == nil {
		t.Fatal("history did not reach data URI")
	}
	raw.ID = "124"
	raw.ContentMetadata["STKID"] = "1002"
	watched := projectMessage(decoder.Decode(id, raw), Account{}, map[string]string{"fictional": "Fictional sender"})
	e.mu.Lock()
	e.watchEvent(WatchEvent{Kind: "message", Revision: "2", ChatID: id, Message: &watched})
	e.mu.Unlock()
	e.Wait()
	if e.View().StickerImages["1002"] == "" {
		t.Fatal("watch did not reach data URI")
	}
}

func TestPhotoLargerThanLegacyTwoMiBLimit(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2048, 1536))
	seed := uint32(1)
	for y := 0; y < 1536; y++ {
		for x := 0; x < 2048; x++ {
			seed = seed*1664525 + 1013904223
			img.SetRGBA(x, y, color.RGBA{uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, &jpeg.Options{Quality: 95})
	if b.Len() <= 2<<20 || b.Len() > MaxFile {
		t.Fatal("fixture must exceed legacy limit within attachment contract")
	}
	uri := photoThumbnail(b.Bytes())
	if !isDataImage(uri) {
		t.Fatal("ordinary large JPEG rejected")
	}
}

func TestCombinedMediaFrameBudget(t *testing.T) {
	v := initial()
	data := "data:image/png;base64,iVBORw0KGgo" + strings.Repeat("A", 86000)
	caption := strings.Repeat("猫", 10000)
	for i := 0; i < 100; i++ {
		v.Messages = append(v.Messages, Message{ID: strconv.Itoa(100 + i), Text: caption, ContentType: 1, Downloadable: true})
	}
	for i := 0; i < 21; i++ {
		v.Avatars[fmt.Sprintf("u%032x", i)] = data
	}
	for i := 0; i < 11; i++ {
		v.StickerImages[strconv.Itoa(1000+i)] = data
	}
	for i := 0; i < 5; i++ {
		v.PhotoThumbnails[strconv.Itoa(100+i)] = PhotoThumbnail{Status: "ready", Data: data}
	}
	v.Preview = "data:image/jpeg;base64,/9j/" + strings.Repeat("A", 2700000)
	w := NewWriter(nil)
	if err := w.Publish(v); err != nil {
		t.Fatal("combined optional budgets exceeded framing contract", err)
	}
	if len(w.latest) > MaxFrame {
		t.Fatal("combined frame larger than 5 MiB")
	}
	var frame struct {
		View View `json:"view"`
	}
	if err := json.Unmarshal(w.latest, &frame); err != nil {
		t.Fatal(err)
	}
	if len(frame.View.Messages) != 100 || frame.View.Messages[99].Text != caption {
		t.Fatal("core user content truncated")
	}
	if frame.View.Preview != "" {
		t.Fatal("oversized combined frame retained large preview")
	}
	if len(v.Avatars) != 21 || len(v.StickerImages) != 11 || v.PhotoThumbnails["100"].Data == "" {
		t.Fatal("writer mutated engine caches")
	}
}
func TestThumbnailAuthenticationLossClearsIdentity(t *testing.T) {
	e, id := readyEngine(Operations{Download: func(context.Context, string, string) ([]byte, error) { return nil, ErrUnauthenticated }})
	defer e.Close()
	ids := photoRows(e, 1)
	e.Handle(photoCommand(e, id, ids...))
	e.Wait()
	if v := e.View(); v.Status != "unauthenticated" || v.SelectedID != "" || len(v.PhotoThumbnails) != 0 {
		t.Fatal("auth failure published a picture or retained old authority")
	}
}
