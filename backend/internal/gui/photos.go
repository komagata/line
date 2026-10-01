package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"slices"
	"time"
)

const photoBudget = 512 << 10
const photoConcurrency = 2
const photoViewportLimit = 8
const photoMaxPixels = 32_000_000

// Original bytes never reach QML or disk. Inspect dimensions before allocating
// the decoded image, and re-encode to strip metadata and bound the bridge frame.
func photoThumbnail(data []byte) string {
	if len(data) == 0 || len(data) > MaxFile {
		return ""
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > photoMaxPixels {
		return ""
	}
	img, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return ""
	}
	w, h := cfg.Width, cfg.Height
	if max(w, h) > 512 {
		w = max(1, w*512/max(cfg.Width, cfg.Height))
		h = max(1, h*512/max(cfg.Width, cfg.Height))
	}
	for {
		small := image.NewRGBA(image.Rect(0, 0, w, h))
		bounds := img.Bounds()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, b, a := img.At(bounds.Min.X+(2*x+1)*cfg.Width/(2*w), bounds.Min.Y+(2*y+1)*cfg.Height/(2*h)).RGBA()
				small.SetRGBA(x, y, color.RGBA{uint8((r + 65535 - a) >> 8), uint8((g + 65535 - a) >> 8), uint8((b + 65535 - a) >> 8), 255})
			}
		}
		for _, quality := range []int{80, 60, 40} {
			var out bytes.Buffer
			if jpeg.Encode(&out, small, &jpeg.Options{Quality: quality}) == nil && out.Len() <= 65536 {
				return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
			}
		}
		if w <= 64 && h <= 64 {
			return ""
		}
		w = max(1, w/2)
		h = max(1, h/2)
	}
}

// Caller holds mu. Cancelled workers retain their global slot until completion,
// even when an operation ignores cancellation. Every publication also checks
// the generation, session, selection and epoch.
func (e *Engine) resetPhotos() {
	if e.photoCancel != nil {
		e.photoCancel()
	}
	e.photoCtx, e.photoCancel = context.WithCancel(e.resourceCtx)
	e.photoGeneration++
	e.photoQueue = nil
	e.photoVisible = map[string]bool{}
	e.state.PhotoThumbnails = map[string]PhotoThumbnail{}
}
func (e *Engine) setPhoto(id string, p PhotoThumbnail) {
	next := make(map[string]PhotoThumbnail, len(e.state.PhotoThumbnails)+1)
	for k, v := range e.state.PhotoThumbnails {
		next[k] = v
	}
	next[id] = p
	e.state.PhotoThumbnails = next
}
func photoCost(id string, p PhotoThumbnail) int { return len(id) + len(p.Data) + 40 }
func (e *Engine) cachePhoto(id, data string) bool {
	// Reserve worst-case JSON status/key overhead for all 100 history rows,
	// including failures that arrive after cached pictures fill the budget.
	limit := photoBudget - 100*60 - 2
	total := 0
	for k, p := range e.state.PhotoThumbnails {
		if k != id {
			total += len(p.Data)
		}
	}
	if total+len(data) > limit {
		// Prefer visible pictures; evicted entries require an explicit retry.
		keys := make([]string, 0, len(e.state.PhotoThumbnails))
		for k, p := range e.state.PhotoThumbnails {
			if !e.photoVisible[k] && p.Data != "" {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			old := e.state.PhotoThumbnails[k]
			total -= len(old.Data)
			e.setPhoto(k, PhotoThumbnail{Status: "error"})
			if total+len(data) <= limit {
				break
			}
		}
	}
	if total+len(data) > limit {
		return false
	}
	e.setPhoto(id, PhotoThumbnail{Status: "ready", Data: data})
	return true
}
func (e *Engine) photos(c Command) {
	if !e.current(c) {
		return
	}
	// A watch update can remove old rows from the 100-message window.
	valid := map[string]bool{}
	for _, m := range e.state.Messages {
		if m.ContentType == 1 && m.Downloadable {
			valid[m.ID] = true
		}
	}
	next := map[string]PhotoThumbnail{}
	for id, p := range e.state.PhotoThumbnails {
		if valid[id] {
			next[id] = p
		}
	}
	e.state.PhotoThumbnails = next
	if c.Action == "photos-hide" {
		e.resetPhotos()
		return
	}
	// Offline means the receive stream stopped; the authenticated session can
	// still serve read-only media through Operations.Download.
	if (e.state.Status != "ready" && e.state.Status != "offline") || e.state.Login.Active {
		return
	}
	if e.photoCtx == nil {
		e.resetPhotos()
	}
	if c.Action == "photo-retry" {
		status := e.state.PhotoThumbnails[c.MessageID].Status
		if !valid[c.MessageID] || !e.photoVisible[c.MessageID] || (status != "error" && status != "ready") {
			return
		}
		e.setPhoto(c.MessageID, PhotoThumbnail{})
		if !slices.Contains(e.photoQueue, c.MessageID) && len(e.photoQueue) < photoViewportLimit {
			e.photoQueue = append(e.photoQueue, c.MessageID)
		}
	} else {
		if len(c.MessageIDs) > photoViewportLimit {
			return
		}
		e.photoQueue = nil
		e.photoVisible = map[string]bool{}
		for _, id := range c.MessageIDs {
			if !numericID.MatchString(id) || e.photoVisible[id] || !slices.ContainsFunc(e.state.Messages, func(m Message) bool { return m.ID == id && m.ContentType == 1 && m.Downloadable }) {
				continue
			}
			e.photoVisible[id] = true
			if e.state.PhotoThumbnails[id].Status == "" {
				e.photoQueue = append(e.photoQueue, id)
			}
		}
	}
	e.pumpPhotos()
}
func (e *Engine) pumpPhotos() {
	for !e.disposed && !e.state.Login.Active && e.photoActive < photoConcurrency && len(e.photoQueue) > 0 {
		id := e.photoQueue[0]
		e.photoQueue = e.photoQueue[1:]
		if !e.photoVisible[id] || e.state.PhotoThumbnails[id].Status != "" {
			continue
		}
		e.setPhoto(id, PhotoThumbnail{Status: "loading"})
		c := Command{ID: e.state.SelectedID, Session: e.state.Session, Selection: e.state.Selection}
		generation, epoch, mode := e.photoGeneration, e.epoch, e.state.Mode
		ctx, cancel := context.WithTimeout(e.photoCtx, 30*time.Second)
		e.photoActive++
		e.spawn(func() {
			defer cancel()
			var data []byte
			var err error
			if mode == "demo" {
				data = demoPhotoBytes()
			} else if e.ops.Download != nil {
				data, err = e.ops.Download(ctx, c.ID, id)
			} else {
				err = errors.New("unavailable")
			}
			thumb := ""
			if err == nil && ctx.Err() == nil {
				thumb = photoThumbnail(data)
			}
			clear(data)
			e.mu.Lock()
			defer e.mu.Unlock()
			e.photoActive--
			if !e.disposed && e.epoch == epoch && e.photoGeneration == generation && e.current(c) && ctx.Err() == nil && slices.ContainsFunc(e.state.Messages, func(m Message) bool { return m.ID == id && m.ContentType == 1 && m.Downloadable }) {
				if errors.Is(err, ErrUnauthenticated) {
					e.loseAuthentication()
					return
				}
				if thumb == "" || !e.cachePhoto(id, thumb) {
					e.setPhoto(id, PhotoThumbnail{Status: "error"})
				}
			} else if !e.disposed && e.epoch == epoch && e.photoGeneration == generation && e.current(c) && slices.ContainsFunc(e.state.Messages, func(m Message) bool { return m.ID == id && m.ContentType == 1 && m.Downloadable }) {
				e.setPhoto(id, PhotoThumbnail{Status: "error"})
			}
			e.pumpPhotos()
			if !e.disposed {
				e.publish()
			}
		})
	}
}
