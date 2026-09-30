package gui

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

var avatarPattern = regexp.MustCompile(`^/?[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)
var stickerHashPattern = regexp.MustCompile(`^[a-fA-F0-9]{1,128}$`)

func avatarPath(value string) string {
	if len(value) == 0 || len(value) > 2048 || !avatarPattern.MatchString(value) {
		return ""
	}
	return strings.TrimPrefix(value, "/")
}
func stickerPath(s Sticker) string {
	if !numericID.MatchString(s.ID) {
		return ""
	}
	if s.Hash == "" {
		return "https://stickershop.line-scdn.net/stickershop/v1/sticker/" + s.ID + "/android/sticker.png"
	}
	if !stickerHashPattern.MatchString(s.Hash) {
		return ""
	}
	return "https://stickershop.line-scdn.net/stickershop/v2/sticker/" + s.ID + "/" + s.Hash + "/android/sticker.png"
}

var imageHTTP = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{DisableCompression: true}}

func fetchCDN(ctx context.Context, rawURL string, maxDimension int) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("Accept", "image/png, image/jpeg")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := imageHTTP.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	mime := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if mime != "image/png" && mime != "image/jpeg" {
		return ""
	}
	encoding := response.Header.Get("Content-Encoding")
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		return ""
	}
	if response.ContentLength > 65536 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 || response.ContentLength >= 0 && int64(len(data)) != response.ContentLength {
		return ""
	}
	return sanitizedImage(data, 65536, maxDimension)
}
func fetchAvatarCDN(ctx context.Context, path string) string {
	safe := avatarPath(path)
	if safe == "" {
		return ""
	}
	return fetchCDN(ctx, "https://profile.line-scdn.net/"+safe+"/large", 512)
}
func fetchStickerCDN(ctx context.Context, s Sticker) string {
	url := stickerPath(s)
	if url == "" {
		return ""
	}
	return fetchCDN(ctx, url, 512)
}
func imageCost(id, data string) int { return len(id) + len(data) + 6 }
func isDataImage(data string) bool {
	if len(data) > 87407 {
		return false
	}
	var encoded string
	if strings.HasPrefix(data, "data:image/png;base64,") {
		encoded = strings.TrimPrefix(data, "data:image/png;base64,")
	} else if strings.HasPrefix(data, "data:image/jpeg;base64,") {
		encoded = strings.TrimPrefix(data, "data:image/jpeg;base64,")
	} else {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	return err == nil && len(decoded) <= 65536
}
func (e *Engine) syncAvatars(retry bool) {
	if e.state.Mode != "live" || e.state.Login.Active || e.disposed {
		return
	}
	if retry {
		e.avatarFailed = map[string]bool{}
	}
	priority := []string{e.state.Account.ID, e.state.SelectedID}
	for _, m := range e.state.Messages {
		priority = append(priority, m.SenderID)
	}
	for _, c := range e.state.Chats {
		if !c.Group {
			priority = append(priority, c.ID)
		}
	}
	for _, c := range e.state.Contacts {
		priority = append(priority, c.ID)
	}
	seen := map[string]bool{}
	for _, id := range priority {
		if e.avatarActive >= 4 || len(seen) >= 500 {
			break
		}
		if seen[id] || !fullID.MatchString(id) || !strings.EqualFold(id[:1], "u") {
			continue
		}
		seen[id] = true
		path := avatarPath(e.picturePaths[id])
		if path == "" {
			continue
		}
		if cached := e.avatarData[path]; cached != "" {
			e.addAvatar(id, cached)
			continue
		}
		if e.avatarLoading[path] || e.avatarFailed[path] {
			continue
		}
		e.avatarLoading[path] = true
		e.avatarActive++
		epoch := e.epoch
		fetch := e.ops.FetchAvatar
		if fetch == nil {
			fetch = fetchAvatarCDN
		}
		ctx := e.resourceCtx
		e.spawn(func() {
			data := fetch(ctx, path)
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.disposed || e.epoch != epoch {
				return
			}
			e.avatarActive--
			delete(e.avatarLoading, path)
			if !isDataImage(data) || !e.cacheAvatar(path, data) {
				e.avatarFailed[path] = true
			}
			e.syncAvatars(false)
			e.publish()
		})
	}
}
func (e *Engine) cacheAvatar(path, data string) bool {
	if len(data) > 2<<20 {
		return false
	}
	if old := e.avatarData[path]; old != "" {
		e.avatarCacheBytes -= len(old)
	}
	priority := []string{e.state.Account.ID, e.state.SelectedID}
	for _, m := range e.state.Messages {
		priority = append(priority, m.SenderID)
	}
	for _, c := range e.state.Chats {
		if !c.Group {
			priority = append(priority, c.ID)
		}
	}
	for _, c := range e.state.Contacts {
		priority = append(priority, c.ID)
	}
	ranks := map[string]int{}
	for i, id := range priority {
		p := avatarPath(e.picturePaths[id])
		if p != "" {
			if _, ok := ranks[p]; !ok {
				ranks[p] = i
			}
		}
	}
	rank := func(p string) int {
		if n, ok := ranks[p]; ok {
			return n
		}
		return len(priority) + 1
	}
	if e.avatarCacheBytes+len(data) > 2<<20 {
		candidates := []string{}
		for oldPath := range e.avatarData {
			if oldPath != path && rank(oldPath) > rank(path) {
				candidates = append(candidates, oldPath)
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return rank(candidates[i]) > rank(candidates[j]) })
		for _, oldPath := range candidates {
			if e.avatarCacheBytes+len(data) <= 2<<20 {
				break
			}
			e.avatarCacheBytes -= len(e.avatarData[oldPath])
			delete(e.avatarData, oldPath)
			e.avatarFailed[oldPath] = true
			for id, p := range e.picturePaths {
				if avatarPath(p) == oldPath {
					delete(e.state.Avatars, id)
				}
			}
		}
	}
	if e.avatarCacheBytes+len(data) > 2<<20 {
		return false
	}
	e.avatarData[path] = data
	e.avatarCacheBytes += len(data)
	return true
}
func (e *Engine) addAvatar(id, data string) {
	if e.state.Avatars[id] == data {
		return
	}
	bytes := 2
	for key, value := range e.state.Avatars {
		if key != id {
			bytes += imageCost(key, value)
		}
	}
	if bytes+imageCost(id, data) > 2<<20 {
		return
	}
	e.state.Avatars[id] = data
}
func (e *Engine) syncStickerImages() {
	if e.disposed || e.state.Login.Active {
		return
	}
	stickers := []Sticker{}
	v := e.state.Stickers
	if v.Selected != nil {
		stickers = append(stickers, *v.Selected)
	}
	if v.Open {
		for _, p := range v.Products {
			if p.ID == v.PackageID && p.Poster != nil {
				stickers = append(stickers, *p.Poster)
			}
		}
		stickers = append(stickers, v.Items...)
	}
	for i := len(e.state.Messages) - 1; i >= 0; i-- {
		if e.state.Messages[i].Sticker != nil {
			stickers = append(stickers, *e.state.Messages[i].Sticker)
		}
	}
	seen := map[string]bool{}
	for _, s := range stickers {
		if e.stickerActive >= 4 || len(seen) >= 140 {
			break
		}
		key := s.ID
		if s.Hash != "" {
			key += "-" + s.Hash
		}
		if seen[key] || stickerPath(s) == "" {
			continue
		}
		seen[key] = true
		if e.state.Mode == "demo" {
			e.addSticker(key, demoSage)
			continue
		}
		if data := e.stickerData[key]; data != "" {
			e.addSticker(key, data)
			continue
		}
		if e.stickerAttempted[key] {
			continue
		}
		e.stickerAttempted[key] = true
		e.stickerActive++
		epoch := e.epoch
		generation := e.stickerGeneration
		fetch := e.ops.FetchSticker
		if fetch == nil {
			fetch = fetchStickerCDN
		}
		ctx := e.stickerCtx
		e.spawn(func() {
			data := fetch(ctx, s)
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.disposed || e.epoch != epoch || e.stickerGeneration != generation {
				return
			}
			e.stickerActive--
			if isDataImage(data) {
				total := 0
				for _, old := range e.stickerData {
					total += len(old)
				}
				if total+len(data) <= 1<<20 {
					e.stickerData[key] = data
					e.addSticker(key, data)
				}
			}
			e.syncStickerImages()
			e.publish()
		})
	}
}
func (e *Engine) addSticker(key, data string) {
	if e.state.StickerImages[key] == data {
		return
	}
	bytes := 2
	for k, v := range e.state.StickerImages {
		if k != key {
			bytes += imageCost(k, v)
		}
	}
	if bytes+imageCost(key, data) > 1<<20 {
		return
	}
	e.state.StickerImages[key] = data
}
