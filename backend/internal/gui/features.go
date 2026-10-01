package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"golang.org/x/sys/unix"
	"slices"
	"strings"
	"time"
)

var reactions = []string{"like", "love", "laugh", "surprise", "sad", "angry"}

func (e *Engine) target(c Command) *Message {
	if !numericID.MatchString(c.MessageID) {
		return nil
	}
	for i := range e.state.Messages {
		if e.state.Messages[i].ID == c.MessageID {
			return &e.state.Messages[i]
		}
	}
	return nil
}
func (e *Engine) feature(c Command) {
	if c.Action == "preview-close" || c.Action == "file-cancel" {
		if e.current(c) {
			e.state.Preview = ""
			e.fileGeneration++
			e.fileBusy = false
			e.state.FileStatus = "idle"
			if e.fileCancel != nil {
				e.fileCancel()
			}
		}
		return
	}
	if c.Action == "attach-cancel" {
		if e.current(c) {
			e.fileGeneration++
			e.fileBusy = false
			e.state.Attachment = nil
		}
		return
	}
	if !e.available(c) {
		return
	}
	if c.Action == "reply-cancel" {
		delete(e.replies, c.ID)
		e.state.Reply = nil
		return
	}
	if c.Action == "attach" {
		e.attach(c)
		return
	}
	target := e.target(c)
	if target == nil {
		return
	}
	if c.Action == "reply" {
		if len(e.replies) >= 50 && e.replies[c.ID].ID == "" {
			return
		}
		source := *target
		r := Reply{ID: target.ID, Text: target.Text, Sender: target.Sender, Source: &source}
		if len(r.Text) > 160 {
			r.Text = r.Text[:160]
		}
		e.replies[c.ID] = r
		e.state.Reply = &r
		return
	}
	if c.Action == "download" || c.Action == "preview" {
		if target.Downloadable {
			e.download(c)
		}
		return
	}
	if c.Action != "react" && c.Action != "unsend" {
		return
	}
	if c.Action == "unsend" && (!target.Own || !c.Confirmed) {
		return
	}
	if c.Action == "react" && ((!c.Remove && !slices.Contains(reactions, c.Reaction)) || (c.Remove && c.Reaction != "")) {
		return
	}
	e.pending = true
	e.mutationActive = true
	e.state.SendStatus = "pending"
	e.state.SendNote = ""
	epoch := e.epoch
	ctx := e.resourceCtx
	mode := e.state.Mode
	e.publish()
	e.spawn(func() {
		var result ActionResult
		var err error
		if mode == "demo" {
			action := c.Action
			if c.Remove {
				action = "remove_reaction"
			}
			result = ActionResult{Action: action, ChatID: c.ID, MessageID: c.MessageID}
		} else if e.ops.Action != nil {
			result, err = e.ops.Action(ctx, c.Action, c.ID, c.MessageID, c.Reaction, c.Remove)
		} else {
			err = errors.New("action unavailable")
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		e.mutationActive = false
		if e.disposed || e.epoch != epoch {
			return
		}
		e.pending = false
		if errors.Is(err, ErrUnauthenticated) {
			e.loseAuthentication()
			return
		}
		action := c.Action
		if c.Remove {
			action = "remove_reaction"
		}
		if err != nil || result.Action != action || result.ChatID != c.ID || result.MessageID != c.MessageID {
			e.state.SendStatus = "uncertain"
			e.state.SendNote = "操作結果を確認できません。LINE側で確認してから、必要な場合だけ再実行してください"
			e.publish()
			return
		}
		e.state.SendStatus = "idle"
		if c.Action == "unsend" {
			e.state.Messages = slices.DeleteFunc(e.state.Messages, func(m Message) bool { return m.ID == c.MessageID })
			if e.state.Mode == "demo" {
				e.demoMessages[c.ID] = slices.DeleteFunc(e.demoMessages[c.ID], func(m Message) bool { return m.ID == c.MessageID })
			}
		} else if e.state.Mode == "demo" {
			e.state.Messages = demoReact(e.state.Messages, c.MessageID, c.Reaction, c.Remove)
			e.demoMessages[c.ID] = demoReact(e.demoMessages[c.ID], c.MessageID, c.Reaction, c.Remove)
		}
		if e.state.Mode == "live" && e.current(c) {
			e.selectChat(c.ID)
		}
		e.publish()
	})
}
func (e *Engine) attach(c Command) {
	if e.fileBusy {
		return
	}
	e.fileBusy = true
	epoch := e.epoch
	generation := e.fileGeneration + 1
	e.fileGeneration = generation
	e.spawn(func() {
		file, err := snapshotFile(c.URL)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch || e.fileGeneration != generation || !e.current(c) {
			return
		}
		e.fileBusy = false
		if err != nil {
			e.state.SendNote = "添付は20 MiB以下の通常ファイルを選んでください。リンクや特殊ファイルは使えません"
		} else {
			e.state.Attachment = file
			e.state.SendNote = ""
		}
		e.publish()
	})
}
func (e *Engine) download(c Command) {
	if e.fileBusy || e.ops.Download == nil && e.state.Mode != "demo" {
		return
	}
	targetFD, targetName := -1, ""
	if c.Action == "download" {
		var err error
		targetFD, targetName, err = openParent(c.URL)
		if err != nil {
			e.state.SendNote = "保存先を確認してください"
			return
		}
	}
	e.fileBusy = true
	e.state.FileStatus = "pending"
	e.state.SendNote = ""
	ctx, cancel := context.WithTimeout(e.resourceCtx, 130*time.Second)
	e.fileCancel = cancel
	generation := e.fileGeneration + 1
	e.fileGeneration = generation
	epoch := e.epoch
	mode := e.state.Mode
	e.publish()
	e.spawn(func() {
		if targetFD >= 0 {
			defer unix.Close(targetFD)
		}
		var data []byte
		var err error
		if mode == "demo" {
			data, _ = base64.StdEncoding.DecodeString(strings.TrimPrefix(demoSage, "data:image/png;base64,"))
		} else {
			data, err = e.ops.Download(ctx, c.ID, c.MessageID)
		}
		if err == nil && len(data) > MaxFile {
			err = errors.New("download too large")
		}
		if err == nil && ctx.Err() == nil {
			if c.Action == "preview" {
				preview := sanitizedImage(data, 2<<20, 4096)
				if preview == "" {
					err = errors.New("invalid image")
				} else {
					e.mu.Lock()
					if !e.disposed && e.epoch == epoch && e.fileGeneration == generation && e.current(c) {
						e.state.Preview = preview
					}
					e.mu.Unlock()
				}
			} else {
				err = publishFile(ctx, targetFD, targetName, data, func(link func() error) error {
					e.mu.Lock()
					defer e.mu.Unlock()
					if e.disposed || e.epoch != epoch || e.fileGeneration != generation || !e.current(c) || ctx.Err() != nil {
						return context.Canceled
					}
					return link()
				})
			}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		cancel()
		if e.disposed || e.epoch != epoch {
			return
		}
		if errors.Is(err, ErrUnauthenticated) {
			e.loseAuthentication()
			return
		}
		if e.fileGeneration != generation {
			return
		}
		e.fileBusy = false
		e.state.FileStatus = "idle"
		if err != nil {
			if errors.Is(err, context.Canceled) {
				e.state.SendNote = "ファイルの取得を中止しました"
			} else if c.Action == "preview" {
				e.state.SendNote = "プレビューできません（PNG/JPEG・2 MiB・4096×4096まで）。保存を利用してください"
			} else {
				e.state.SendNote = "保存結果を確認できません。既存のファイルは上書きしません。保存先を確認してください"
			}
		} else if c.Action == "download" {
			e.state.SendNote = "ファイルを保存しました"
		}
		e.publish()
	})
}

func demoReact(rows []Message, id, reaction string, remove bool) []Message {
	out := append([]Message(nil), rows...)
	for i := range out {
		if out[i].ID != id {
			continue
		}
		current := []Reaction{}
		for _, r := range out[i].Reactions {
			if r.Own {
				r.Count--
				r.Own = false
			}
			if r.Count > 0 {
				current = append(current, r)
			}
		}
		if !remove {
			found := false
			for j := range current {
				if current[j].Name == reaction {
					current[j].Count++
					current[j].Own = true
					found = true
					break
				}
			}
			if !found {
				current = append(current, Reaction{Name: reaction, Count: 1, Own: true})
			}
		}
		out[i].Reactions = current
	}
	return out
}
