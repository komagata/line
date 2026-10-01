package gui

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var stickerOptions = []string{"", "A", "S", "AS", "P", "PS"}

func validProduct(p OwnedProduct) bool {
	if !numericID.MatchString(p.ID) || !numericID.MatchString(p.Version) || p.ResourceType < 1 || p.ResourceType > 8 || len(p.Ranges) == 0 || len(p.Ranges) > 64 {
		return false
	}
	if _, err := strconv.ParseUint(p.ID, 10, 64); err != nil {
		return false
	}
	count := 0
	var last uint64
	for _, r := range p.Ranges {
		start, err := strconv.ParseUint(r.Start, 10, 64)
		if err != nil || start == 0 || start <= last || r.Size < 1 || r.Size > 1000 || start > ^uint64(0)-uint64(r.Size-1) {
			return false
		}
		last = start + uint64(r.Size-1)
		count += r.Size
		if count > 1000 {
			return false
		}
	}
	return p.Count == count
}
func (e *Engine) sticker(c Command) {
	v := &e.state.Stickers
	if !e.current(c) {
		return
	}
	switch c.Action {
	case "sticker-close":
		if e.pending {
			return
		}
		v.Open = false
		v.Items = []Sticker{}
		v.Selected = nil
		return
	case "sticker-hide":
		v.Open = false
		v.Items = []Sticker{}
		return
	}
	if !e.available(c) {
		return
	}
	switch c.Action {
	case "sticker-open":
		e.openCatalog(c)
	case "sticker-page":
		if v.Open && v.Status == "ready" {
			e.stickerPage(c.PackageID, c.Page)
		}
	case "sticker-choose":
		if !v.Open || v.Status != "ready" {
			return
		}
		for _, s := range v.Items {
			if s.ID == c.StickerID && s.PackageID == c.PackageID {
				selected := s
				v.Selected = &selected
				v.Open = false
				v.Items = []Sticker{}
				return
			}
		}
	case "sticker-send":
		e.sendSticker(c)
	}
}
func (e *Engine) openCatalog(c Command) {
	v := &e.state.Stickers
	v.Open = true
	v.Status = "loading"
	v.Note = "所有スタンプを読み込んでいます…"
	v.Products = []StickerProduct{}
	v.Items = []Sticker{}
	e.products = nil
	epoch := e.epoch
	ctx := e.resourceCtx
	selection := e.state.Selection
	mode := e.state.Mode
	e.publish()
	e.spawn(func() {
		products := []OwnedProduct{{ID: "1", Name: "架空のカラースタンプ", Version: "1", ValidUntil: "-1", ResourceType: 1, Ranges: []OwnedRange{{Start: "1001", Size: 48}}, Count: 48, Supported: true}}
		var err error
		if mode != "demo" {
			if e.ops.Catalog == nil {
				err = errors.New("catalog unavailable")
			} else {
				products, err = e.ops.Catalog(ctx)
			}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		if errors.Is(err, ErrUnauthenticated) {
			e.loseAuthentication()
			return
		}
		if e.state.Selection != selection || !e.current(c) {
			return
		}
		if err != nil || len(products) > 500 {
			v.Status = "error"
			v.Note = "所有スタンプを取得できません（最大500パック）。再試行してください"
			e.publish()
			return
		}
		seen := map[string]bool{}
		for _, p := range products {
			if !validProduct(p) || seen[p.ID] {
				v.Status = "error"
				v.Note = "所有スタンプを取得できません（最大500パック）。再試行してください"
				e.publish()
				return
			}
			seen[p.ID] = true
		}
		e.products = products
		for _, p := range products {
			v.Products = append(v.Products, StickerProduct{ID: p.ID, Name: p.Name, Count: p.Count, Supported: p.Supported, ResourceType: p.ResourceType, Poster: &Sticker{ID: p.Ranges[0].Start, Hash: p.Hash}})
		}
		v.Status = "ready"
		v.Note = ""
		if len(products) > 0 {
			e.stickerPage(products[0].ID, 0)
		}
		e.syncStickerImages()
		e.publish()
	})
}
func (e *Engine) stickerPage(id string, page int) {
	v := &e.state.Stickers
	for _, p := range e.products {
		if p.ID != id {
			continue
		}
		pages := (p.Count + 39) / 40
		if page < 0 || page >= pages {
			return
		}
		v.PackageID = id
		v.Page = page
		v.Pages = pages
		v.Items = []Sticker{}
		v.Note = "静止画で表示・音声なし。選択してから送信してください"
		if !p.Supported {
			v.Note = "名前・文字を変えるカスタムスタンプには対応していません"
			return
		}
		index := 0
		for _, r := range p.Ranges {
			start, _ := strconv.ParseUint(r.Start, 10, 64)
			for i := 0; i < r.Size; i++ {
				if index >= page*40 && index < (page+1)*40 {
					v.Items = append(v.Items, Sticker{ID: fmt.Sprint(start + uint64(i)), PackageID: p.ID, Version: p.Version, Hash: p.Hash, Option: stickerOptions[p.ResourceType-1], Alt: p.Name})
				}
				index++
			}
		}
		return
	}
}
func (e *Engine) sendSticker(c Command) {
	s := e.state.Stickers.Selected
	if s == nil || c.StickerID != s.ID || c.PackageID != s.PackageID || c.Text != "" || e.state.Draft != "" || e.state.Attachment != nil {
		return
	}
	var p *OwnedProduct
	for i := range e.products {
		if e.products[i].ID == s.PackageID {
			p = &e.products[i]
			break
		}
	}
	if p == nil || !p.Supported || !stickerOwned(*p, s.ID) {
		e.state.SendNote = "このスタンプの所有情報を確認できません。スタンプ一覧を読み直すか、選択を解除してください"
		return
	}
	if p.ValidUntil != "-1" {
		end, err := strconv.ParseInt(p.ValidUntil, 10, 64)
		if err != nil || end <= time.Now().UnixMilli() {
			e.state.SendNote = "スタンプの有効期限が切れました。スタンプ一覧を開いて選び直してください"
			return
		}
	}
	reply := e.replies[c.ID]
	if c.ReplyTo != reply.ID || reply.ID != "" && !e.hasMessage(reply.ID) {
		return
	}
	e.pending = true
	e.mutationActive = true
	e.state.SendStatus = "pending"
	epoch := e.epoch
	ctx := e.resourceCtx
	mode := e.state.Mode
	e.publish()
	e.spawn(func() {
		result := SendResult{ID: "900001", ChatID: c.ID}
		if mode == "demo" {
			e.mu.Lock()
			e.demoSeq++
			result.ID = strconv.Itoa(900000 + e.demoSeq)
			e.mu.Unlock()
		}
		var err error
		if mode != "demo" {
			if e.ops.SendSticker == nil {
				err = errors.New("sticker unavailable")
			} else {
				result, err = e.ops.SendSticker(ctx, c.ID, s.PackageID, s.ID, reply.ID)
			}
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
		if err != nil || result.ChatID != c.ID || !numericID.MatchString(result.ID) {
			e.state.SendStatus = "uncertain"
			e.state.SendNote = "送信結果を確認できません。LINE側で確認してから、必要な場合だけ再送してください"
			e.publish()
			return
		}
		e.state.SendStatus = "idle"
		if e.state.SelectedID == c.ID {
			e.state.Stickers.Selected = nil
		}
		now := time.Now()
		e.sentMessage(c.ID, Message{SenderUnavailable: e.state.Account.NameUnavailable || e.state.Account.DemoFixture, GeneratedText: true, ID: result.ID, Text: "［スタンプ］ " + s.Alt, ContentType: 7, Sticker: s, Own: true, Sender: e.state.Account.Name, SenderID: e.state.Account.ID, Timestamp: now.UnixMilli(), Time: now.Format("15:04"), Day: now.Format("2006/01/02"), Status: "ok", ReplyTo: reply.ID})
		delete(e.replies, c.ID)
		if e.state.SelectedID == c.ID {
			e.state.Reply = nil
		}
		e.publish()
	})
}
func stickerOwned(p OwnedProduct, id string) bool {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return false
	}
	for _, r := range p.Ranges {
		start, _ := strconv.ParseUint(r.Start, 10, 64)
		if n >= start && n < start+uint64(r.Size) {
			return true
		}
	}
	return false
}
