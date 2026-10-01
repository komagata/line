package gui

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

const MaxInputLine = 65536
const MaxFrame = 5 << 20

// Reader keeps command parsing bounded before JSON decoding or dispatch.
type Reader struct {
	input  io.Reader
	accept func(json.RawMessage)
}

func NewReader(input io.Reader, accept func(json.RawMessage)) *Reader {
	return &Reader{input: input, accept: accept}
}
func (r *Reader) Run(ctx context.Context) error {
	br := bufio.NewReaderSize(r.input, 4096)
	var second int64
	var count int
	for {
		var line []byte
		for {
			part, err := br.ReadSlice('\n')
			line = append(line, part...)
			if len(line) > MaxInputLine+1 {
				return errors.New("input line exceeds limit")
			}
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			if err == io.EOF {
				if len(line) > 0 {
					return errors.New("unterminated input")
				}
				return nil
			}
			if err != nil {
				return err
			}
			break
		}
		if len(line)-1 > MaxInputLine {
			return errors.New("input line exceeds limit")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		now := time.Now().Unix()
		if now != second {
			second = now
			count = 0
		}
		count++
		if count > 200 {
			return errors.New("command rate exceeded")
		}
		line = bytes.TrimSpace(line)
		if len(line) > 0 && json.Valid(line) {
			r.accept(append(json.RawMessage(nil), line...))
		}
	}
}

// Writer coalesces snapshots under backpressure and has one goroutine owner.
type Writer struct {
	out     io.Writer
	mu      sync.Mutex
	latest  []byte
	wake    chan struct{}
	stopped bool
}

func NewWriter(out io.Writer) *Writer { return &Writer{out: out, wake: make(chan struct{}, 1)} }
func (w *Writer) Publish(v View) error {
	encode := func(value View) ([]byte, error) {
		return json.Marshal(struct {
			Type string `json:"type"`
			View View   `json:"view"`
		}{"state", value})
	}
	frame, err := encode(v)
	if err != nil {
		return err
	}
	if len(frame)+1 > MaxFrame {
		v.Preview = ""
		frame, err = encode(v)
		if err != nil {
			return err
		}
	}
	if len(frame)+1 > MaxFrame {
		photos := make(map[string]PhotoThumbnail, len(v.PhotoThumbnails))
		for id, p := range v.PhotoThumbnails {
			photos[id] = p
		}
		v.PhotoThumbnails = photos
		for id, p := range photos {
			if p.Data == "" {
				continue
			}
			photos[id] = PhotoThumbnail{Status: "error"}
			frame, err = encode(v)
			if err != nil {
				return err
			}
			if len(frame)+1 <= MaxFrame {
				break
			}
		}
	}
	if len(frame)+1 > MaxFrame {
		images := make(map[string]string, len(v.StickerImages))
		for k, data := range v.StickerImages {
			images[k] = data
		}
		v.StickerImages = images
		for k := range images {
			delete(images, k)
			frame, err = encode(v)
			if err != nil {
				return err
			}
			if len(frame)+1 <= MaxFrame {
				break
			}
		}
	}
	if len(frame)+1 > MaxFrame {
		images := make(map[string]string, len(v.Avatars))
		for k, data := range v.Avatars {
			images[k] = data
		}
		v.Avatars = images
		for k := range images {
			delete(images, k)
			frame, err = encode(v)
			if err != nil {
				return err
			}
			if len(frame)+1 <= MaxFrame {
				break
			}
		}
	}
	if len(frame)+1 > MaxFrame {
		return errors.New("output frame exceeds limit")
	}
	frame = append(frame, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped {
		return io.ErrClosedPipe
	}
	w.latest = frame
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return nil
}
func (w *Writer) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.wake:
			w.mu.Lock()
			frame := w.latest
			w.latest = nil
			w.mu.Unlock()
			if len(frame) == 0 {
				continue
			}
			for len(frame) > 0 {
				n, err := w.out.Write(frame)
				if err != nil {
					return err
				}
				if n <= 0 {
					return io.ErrShortWrite
				}
				frame = frame[n:]
			}
		}
	}
}
func (w *Writer) Close() { w.mu.Lock(); w.stopped = true; w.latest = nil; w.mu.Unlock() }
