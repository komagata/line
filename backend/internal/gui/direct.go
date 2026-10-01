package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/kongesque/line-cli/internal/events"
	"github.com/kongesque/line-cli/internal/messaging"
	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
	qrcode "github.com/yeqown/go-qrcode/v2"
)

type Direct struct {
	Lock          func() (func(), error)
	Manager       *session.Manager
	mu            sync.RWMutex
	account       Account
	names         map[string]string
	nameFallbacks map[string]bool
}

func NewDirect() *Direct {
	return &Direct{Manager: session.NewManager(session.KeychainStore{}), Lock: session.Lock, names: map[string]string{}}
}
func (d *Direct) Operations() Operations {
	return Operations{Snapshot: d.Snapshot, History: d.History, Send: d.Send, Action: d.Action, Download: d.Download, Catalog: d.Catalog, SendSticker: d.SendSticker, Login: d.Login, Watch: d.Watch, Notify: staticNotify, NotifyLocalized: localizedNotify, FetchAvatar: fetchAvatarCDN, FetchSticker: fetchStickerCDN, Profiles: d.Profiles}
}
func (d *Direct) acquire(ctx context.Context) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lock := d.Lock
		if lock == nil {
			lock = session.Lock
		}
		unlock, err := lock()
		if errors.Is(err, session.ErrBusy) {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		return unlock, err
	}
}
func cleanText(value string, limit int) string {
	var b strings.Builder
	units := 0
	for _, r := range value {
		if (unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t') || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
			continue
		}
		size := utf16.RuneLen(r)
		if units+size > limit {
			break
		}
		units += size
		b.WriteRune(r)
	}
	return b.String()
}
func label(value string, limit int) string {
	return strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(cleanText(value, limit))
}
func number(value json.Number) int64 {
	n, err := value.Int64()
	if err != nil || n < 0 {
		return 0
	}
	return n
}
func clock(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Local().Format("15:04")
}
func day(ms int64) string {
	if ms <= 0 {
		return "日時不明"
	}
	return time.UnixMilli(ms).Local().Format("2006/01/02")
}
func (d *Direct) Snapshot(ctx context.Context) (Snapshot, error) {
	manager := d.Manager.WithContext(ctx)
	unlock, err := d.acquire(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	defer unlock()
	saved, err := manager.Store.Load()
	if errors.Is(err, session.ErrNotFound) || errors.Is(err, session.ErrSessionInvalidated) || saved != nil && saved.Invalidated {
		return Snapshot{}, ErrUnauthenticated
	}
	if err != nil {
		return Snapshot{}, err
	}
	if saved == nil {
		return Snapshot{}, ErrUnauthenticated
	}
	var profile *line.Profile
	if err = manager.DoContext(ctx, func(api session.API) (err error) { profile, err = api.GetProfileContext(ctx); return }); err != nil {
		return Snapshot{}, classify(err)
	}
	if profile == nil || !fullID.MatchString(profile.Mid) {
		return Snapshot{}, errors.New("invalid profile")
	}
	nameFallbacks := map[string]bool{}
	result := Snapshot{Account: Account{ID: profile.Mid, Name: label(profile.DisplayName, 160)}, Pictures: map[string]string{profile.Mid: profile.PicturePath}}
	if result.Account.Name == "" {
		result.Account.Name = "自分"
		result.Account.NameUnavailable = true
	}
	var ids []string
	if err = manager.Do(func(api session.API) (err error) { ids, err = api.GetAllContactIds(); return }); err != nil {
		if errors.Is(classify(err), ErrUnauthenticated) {
			return Snapshot{}, ErrUnauthenticated
		}
		result.ContactsFailed = true
		result.NamesPartial = true
		ids = nil
	}
	if len(ids) > 10000 {
		result.ContactsFailed = true
		result.NamesPartial = true
		ids = nil
	}
	var blocked []string
	if err = manager.Do(func(api session.API) (err error) { blocked, err = api.GetBlockedContactIds(); return }); err != nil {
		if errors.Is(classify(err), ErrUnauthenticated) {
			return Snapshot{}, ErrUnauthenticated
		}
		result.ContactsFailed = true
		result.NamesPartial = true
		ids = nil
	}
	blockedIDs := map[string]bool{}
	for _, id := range blocked {
		blockedIDs[id] = true
	}
	candidates := []Contact{}
	seen := map[string]bool{}
	for _, id := range ids {
		if len(candidates) >= 500 {
			break
		}
		if !fullID.MatchString(id) || !strings.EqualFold(id[:1], "u") || id == profile.Mid || seen[id] || blockedIDs[id] {
			continue
		}
		seen[id] = true
		candidates = append(candidates, Contact{ID: id, Name: id})
	}
	names := map[string]string{}
	for i := 0; i < len(candidates); i += 100 {
		end := min(i+100, len(candidates))
		batch := make([]string, 0, end-i)
		for _, c := range candidates[i:end] {
			batch = append(batch, c.ID)
		}
		var response *line.ContactsResponse
		if err = manager.Do(func(api session.API) (err error) { response, err = api.GetContactsV2(batch); return }); err != nil || response == nil {
			if errors.Is(classify(err), ErrUnauthenticated) {
				return Snapshot{}, ErrUnauthenticated
			}
			result.ContactsFailed = true
			result.NamesPartial = true
			continue
		}
		for j := i; j < end; j++ {
			c := &candidates[j]
			wrapper, ok := response.Contacts[c.ID]
			if !ok || wrapper.Contact.Mid != c.ID {
				result.ContactsFailed = true
				result.NamesPartial = true
				continue
			}
			name := label(wrapper.Contact.EffectiveDisplayName(), 160)
			if name == "" {
				name = "名前未設定"
				c.NameUnavailable = true
				nameFallbacks[c.ID] = true
			}
			c.Name = name
			result.Contacts = append(result.Contacts, *c)
			names[c.ID] = name
			result.Pictures[c.ID] = wrapper.Contact.PicturePath
		}
	}
	options := line.MessageBoxesOptions{MessageBoxCountLimit: 100, WithUnreadCount: true, LastMessagesPerMessageBoxCount: 1}
	cursors := map[string]bool{}
	chatSeen := map[string]bool{}
	for page := 0; page < 100; page++ {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		var boxes *line.MessageBoxesResponse
		if err = manager.Do(func(api session.API) (err error) { boxes, err = api.GetMessageBoxes(options); return }); err != nil {
			return Snapshot{}, classify(err)
		}
		if boxes == nil || len(boxes.MessageBoxes) > 100 {
			return Snapshot{}, errors.New("invalid chat page")
		}
		for _, box := range boxes.MessageBoxes {
			if !fullID.MatchString(box.ID) || chatSeen[box.ID] {
				continue
			}
			chatSeen[box.ID] = true
			if len(result.Chats) >= 500 {
				continue
			}
			updated := int64(0)
			if box.LastDeliveredMessageID != nil {
				updated = number(box.LastDeliveredMessageID.DeliveredTime)
			}
			for _, m := range box.LastMessages {
				updated = max(updated, number(m.CreatedTime))
			}
			unread := number(box.UnreadCount)
			if unread > 9999 {
				unread = 9999
			}
			name := names[box.ID]
			result.Chats = append(result.Chats, Chat{ID: box.ID, Name: name, Group: strings.ToLower(box.ID[:1]) != "u", Unread: int(unread), UpdatedAt: updated, Time: clock(updated), Preview: "トークを開く", PreviewKey: "トークを開く", NameUnavailable: name == ""})
		}
		if !boxes.HasNext {
			break
		}
		if len(boxes.MessageBoxes) == 0 {
			return Snapshot{}, errors.New("empty chat page")
		}
		next := boxes.MessageBoxes[len(boxes.MessageBoxes)-1].ID
		if next == "" || cursors[next] {
			return Snapshot{}, errors.New("chat cursor did not advance")
		}
		cursors[next] = true
		options.MinChatID = next
		if page == 99 {
			return Snapshot{}, errors.New("too many chat pages")
		}
	}
	// Missing direct-chat and group names are optional metadata; never infer IDs from display text.
	for i := 0; i < len(result.Chats); i += 100 {
		end := min(i+100, len(result.Chats))
		direct := []string{}
		groups := []string{}
		for _, c := range result.Chats[i:end] {
			if c.Group {
				groups = append(groups, c.ID)
			} else if names[c.ID] == "" {
				direct = append(direct, c.ID)
			}
		}
		for j := 0; j < len(direct); j += 100 {
			var response *line.ContactsResponse
			batch := direct[j:min(j+100, len(direct))]
			err = manager.Do(func(api session.API) (err error) { response, err = api.GetContactsV2(batch); return })
			if errors.Is(classify(err), ErrUnauthenticated) {
				return Snapshot{}, ErrUnauthenticated
			}
			if err == nil && response != nil {
				for _, id := range batch {
					w, ok := response.Contacts[id]
					if ok && (w.Contact.Mid == "" || w.Contact.Mid == id) {
						names[id] = label(w.Contact.EffectiveDisplayName(), 160)
						result.Pictures[id] = w.Contact.PicturePath
					}
				}
			} else {
				result.NamesPartial = true
			}
		}
		for j := 0; j < len(groups); j += 20 {
			var response *line.GetChatsResponse
			batch := groups[j:min(j+20, len(groups))]
			err = manager.Do(func(api session.API) (err error) { response, err = api.GetChats(batch, false, false); return })
			if errors.Is(classify(err), ErrUnauthenticated) {
				return Snapshot{}, ErrUnauthenticated
			}
			if err == nil && response != nil {
				for _, group := range response.Chats {
					if fullID.MatchString(group.ChatMid) {
						names[group.ChatMid] = label(group.ChatName, 160)
					}
				}
			} else {
				result.NamesPartial = true
			}
		}
	}
	for i := range result.Chats {
		c := &result.Chats[i]
		if name := names[c.ID]; name != "" {
			c.Name = name
			c.NameUnavailable = nameFallbacks[c.ID]
		} else {
			c.Name = c.ID
			result.NamesPartial = true
		}
	}
	sort.SliceStable(result.Chats, func(i, j int) bool { return result.Chats[i].UpdatedAt > result.Chats[j].UpdatedAt })
	d.mu.Lock()
	d.account = result.Account
	d.names = names
	d.nameFallbacks = nameFallbacks
	d.mu.Unlock()
	return result, nil
}
func classify(err error) error {
	if errors.Is(err, session.ErrNotFound) || errors.Is(err, session.ErrSessionInvalidated) || line.IsAuthError(session.ProtocolError(err)) {
		return ErrUnauthenticated
	}
	return err
}
func projectMessage(raw messaging.Message, account Account, names map[string]string, fallbackMaps ...map[string]bool) Message {
	fallback := raw.From == account.ID && account.NameUnavailable
	if raw.From != account.ID && len(fallbackMaps) > 0 {
		fallback = fallbackMaps[0][raw.From]
	}
	timestamp, _ := strconv.ParseInt(raw.CreatedTime.String(), 10, 64)
	if timestamp < 0 {
		timestamp = 0
	}
	kind := raw.ContentType
	text := ""
	status := "ok"
	switch {
	case raw.Status == "decryption_failed" || raw.Error != "":
		text = "このメッセージは復号できませんでした"
		status = "decryption_failed"
	case kind == 0:
		text = cleanText(raw.Text, 10000)
	case kind == 1:
		text = "［画像］"
	case kind == 2:
		text = "［動画］"
	case kind == 3:
		text = "［音声］"
	case kind == 7:
		text = "［スタンプ］"
	case kind == 14:
		text = "［ファイル：" + label(raw.FileName, 160) + "］"
	default:
		text = "［未対応のメッセージ］"
	}
	sender := account.Name
	if raw.From != account.ID {
		sender = names[raw.From]
		if sender == "" {
			sender = label(raw.From, 160)
		}
	}
	encrypted := raw.Encrypted
	m := Message{GeneratedText: kind != 0 || status == "decryption_failed", SenderUnavailable: fallback, ID: raw.ID, Text: text, SenderID: raw.From, Sender: sender, Own: raw.From == account.ID, Timestamp: timestamp, Time: clock(timestamp), Day: day(timestamp), Encrypted: &encrypted, Status: status, ReplyTo: raw.ReplyTo, ContentType: kind, Downloadable: messaging.IsDownloadable(kind), FileName: label(raw.FileName, 160), Reactions: []Reaction{}}
	if raw.Sticker != nil {
		m.Sticker = &Sticker{ID: raw.Sticker.ID, PackageID: raw.Sticker.PackageID, Version: raw.Sticker.Version, Option: raw.Sticker.Option, Hash: raw.Sticker.Hash, Alt: raw.Sticker.Alt}
	}
	counts := map[string]*Reaction{}
	seen := map[string]bool{}
	for _, r := range raw.Reactions {
		if seen[r.FromUserMID] || !fullID.MatchString(r.FromUserMID) {
			continue
		}
		seen[r.FromUserMID] = true
		typeID := r.ReactionType.PredefinedReactionType
		if typeID < 2 || typeID > 7 {
			continue
		}
		name := reactions[typeID-2]
		if counts[name] == nil {
			counts[name] = &Reaction{Name: name}
		}
		counts[name].Count++
		if r.FromUserMID == account.ID {
			counts[name].Own = true
		}
	}
	for _, name := range reactions {
		if counts[name] != nil {
			m.Reactions = append(m.Reactions, *counts[name])
		}
	}
	return m
}
func (d *Direct) History(ctx context.Context, id string) ([]Message, error) {
	manager := d.Manager.WithContext(ctx)
	unlock, err := d.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	client, err := messaging.New(manager)
	if err != nil {
		return nil, classify(err)
	}
	rows, err := client.History(id, 100)
	if err != nil {
		return nil, classify(err)
	}
	out := make([]Message, 0, len(rows))
	d.mu.RLock()
	account, names, fallbacks := d.account, d.names, d.nameFallbacks
	d.mu.RUnlock()
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out = append(out, projectMessage(r, account, names, fallbacks))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return out, nil
}
func (d *Direct) withClient(ctx context.Context, fn func(*messaging.Client) error) error {
	manager := d.Manager.WithContext(ctx)
	unlock, err := d.acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	client, err := messaging.New(manager)
	if err != nil {
		return classify(err)
	}
	return classify(fn(client))
}
func (d *Direct) Send(ctx context.Context, chat, body, reply string, file *Attachment) (SendResult, error) {
	var result *messaging.SendResult
	err := d.withClient(ctx, func(c *messaging.Client) (err error) {
		if file != nil {
			result, err = c.SendFile(chat, messaging.Attachment{Name: file.Name, Data: file.Data}, reply)
		} else {
			result, err = c.SendReply(chat, body, reply)
		}
		return
	})
	if err != nil || result == nil {
		return SendResult{}, err
	}
	return SendResult{ID: result.ID, ChatID: result.ChatID, Encrypted: result.Encrypted}, nil
}
func (d *Direct) Action(ctx context.Context, action, chat, id, reaction string, remove bool) (ActionResult, error) {
	var result *messaging.ActionResult
	err := d.withClient(ctx, func(c *messaging.Client) (err error) {
		if action == "unsend" {
			result, err = c.Unsend(chat, id)
		} else {
			result, err = c.React(chat, id, reaction, remove)
		}
		return
	})
	if err != nil || result == nil {
		return ActionResult{}, err
	}
	return ActionResult{Action: result.Action, ChatID: result.ChatID, MessageID: result.MessageID}, nil
}
func (d *Direct) Download(ctx context.Context, chat, id string) ([]byte, error) {
	var data []byte
	err := d.withClient(ctx, func(c *messaging.Client) (err error) { data, err = c.DownloadFile(ctx, chat, id); return })
	return data, err
}
func (d *Direct) Catalog(ctx context.Context) ([]OwnedProduct, error) {
	var products []messaging.OwnedStickerProduct
	err := d.withClient(ctx, func(c *messaging.Client) (err error) { products, err = c.OwnedStickers(); return })
	if err != nil {
		return nil, err
	}
	out := make([]OwnedProduct, 0, len(products))
	for _, p := range products {
		r := make([]OwnedRange, 0, len(p.Ranges))
		for _, item := range p.Ranges {
			r = append(r, OwnedRange{Start: item.Start, Size: item.Size})
		}
		out = append(out, OwnedProduct{ID: p.ID, Name: p.Name, Version: p.Version, ValidUntil: p.ValidUntil, ResourceType: p.ResourceType, Hash: p.Hash, Ranges: r, Count: p.Count, Supported: p.Supported})
	}
	return out, nil
}
func (d *Direct) SendSticker(ctx context.Context, chat, pack, sticker, reply string) (SendResult, error) {
	var result *messaging.SendResult
	err := d.withClient(ctx, func(c *messaging.Client) (err error) { result, err = c.SendSticker(chat, pack, sticker, reply); return })
	if err != nil || result == nil {
		return SendResult{}, err
	}
	return SendResult{ID: result.ID, ChatID: result.ChatID, Encrypted: result.Encrypted}, nil
}

type matrixWriter struct{ bitmap [][]bool }

func (w *matrixWriter) Write(m qrcode.Matrix) error { w.bitmap = m.Bitmap(); return nil }
func (*matrixWriter) Close() error                  { return nil }
func qrPNG(value string) (string, error) {
	switch os.Getenv("QRCODE_DEBUG") {
	case "1", "true", "TRUE", "enabled", "ENABLED":
		return "", errors.New("QR debug logging is disabled")
	}
	code, err := qrcode.NewWith(value, qrcode.WithEncodingMode(qrcode.EncModeByte), qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium))
	if err != nil {
		return "", errors.New("QR encoding failed")
	}
	w := new(matrixWriter)
	if err = code.Save(w); err != nil {
		return "", errors.New("QR rendering failed")
	}
	size := len(w.bitmap) + 8
	if size < 1 || size*4 > 1200 {
		return "", errors.New("QR dimension exceeds limit")
	}
	img := image.NewGray(image.Rect(0, 0, size*4, size*4))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := range w.bitmap {
		for x, dark := range w.bitmap[y] {
			if dark {
				for dy := 0; dy < 4; dy++ {
					for dx := 0; dx < 4; dx++ {
						img.SetGray((x+4)*4+dx, (y+4)*4+dy, color.Gray{Y: 0})
					}
				}
			}
		}
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil || buf.Len() > 65536 {
		return "", errors.New("QR image exceeds limit")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
func emitPIN(pin string, emit func(LoginEvent) error) error {
	if len(pin) < 4 || len(pin) > 12 {
		return errors.New("invalid login PIN")
	}
	for i := range pin {
		if pin[i] < '0' || pin[i] > '9' {
			return errors.New("invalid login PIN")
		}
	}
	return emit(LoginEvent{Stage: "phone", PIN: pin})
}
func (d *Direct) Login(ctx context.Context, confirm <-chan struct{}, emit func(LoginEvent) error) error {
	manager := d.Manager.WithContext(ctx)
	unlock, err := d.acquire(ctx)
	if err != nil {
		return err
	}
	snapshot, err := manager.PrepareLogin(ctx)
	if err != nil {
		unlock()
		return err
	}
	saved, loadErr := manager.Store.Load()
	unlock()
	if loadErr != nil && !errors.Is(loadErr, session.ErrNotFound) {
		return loadErr
	}
	if saved != nil && !saved.Invalidated {
		if err := emit(LoginEvent{Stage: "replace-confirmation"}); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-confirm:
		}
	}
	unlock, err = d.acquire(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err = manager.CheckLogin(ctx, snapshot); err != nil {
		return err
	}
	if err = manager.PrepareStorage(); err != nil {
		return err
	}
	_, err = manager.LoginQR(ctx, func(ev session.QRLoginEvent) error {
		switch ev.Kind {
		case session.QRLoginCode:
			image, err := qrPNG(ev.URL)
			if err != nil {
				return err
			}
			return emit(LoginEvent{Stage: "scan", Image: image})
		case session.QRLoginExpired:
			return emit(LoginEvent{Stage: "preparing"})
		case session.QRLoginScanned:
			return emit(LoginEvent{Stage: "phone"})
		case session.QRLoginPIN:
			return emitPIN(ev.PIN, emit)
		case session.QRLoginPhoneAccepted, session.QRLoginApproved:
			return emit(LoginEvent{Stage: "saving"})
		}
		return nil
	})
	if err != nil {
		var outcome *session.LoginError
		if errors.As(err, &outcome) && (outcome.Dispatched || outcome.Approved || outcome.SaveUncertain) {
			return ErrLoginUncertain
		}
		return classify(err)
	}
	return nil
}

type watchSink struct {
	nameFallbacks map[string]bool
	account       Account
	names         map[string]string
	callback      func(WatchEvent)
}

func (w *watchSink) Write(data []byte) (int, error) {
	if len(data) > 131072 {
		return 0, errors.New("watch frame too large")
	}
	var event events.Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &event); err != nil {
		return 0, err
	}
	out := WatchEvent{Kind: event.Event, Revision: event.Revision, ChatID: event.ChatID}
	if event.Message != nil {
		m := projectMessage(*event.Message, w.account, w.names, w.nameFallbacks)
		out.Message = &m
	}
	w.callback(out)
	return len(data), nil
}
func (d *Direct) Watch(ctx context.Context, callback func(WatchEvent)) error {
	manager := d.Manager.WithContext(ctx)
	unlock, err := session.WatchLock()
	if err != nil {
		return err
	}
	defer unlock()
	d.mu.RLock()
	account, names, fallbacks := d.account, d.names, d.nameFallbacks
	d.mu.RUnlock()
	sink := &watchSink{account: account, names: names, nameFallbacks: fallbacks, callback: callback}
	watcher := events.Watcher{Manager: manager, Lock: d.Lock, Out: sink, Err: io.Discard}
	return classify(watcher.Run(ctx))
}
func (d *Direct) Profiles(ctx context.Context, ids []string) (map[string]ProfileInfo, error) {
	manager := d.Manager.WithContext(ctx)
	if len(ids) == 0 || len(ids) > 100 {
		return nil, errors.New("profile batch outside bound")
	}
	for _, id := range ids {
		if !fullID.MatchString(id) || !strings.EqualFold(id[:1], "u") {
			return nil, errors.New("invalid profile ID")
		}
	}
	unlock, err := d.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	var response *line.ContactsResponse
	if err = manager.Do(func(api session.API) (err error) { response, err = api.GetContactsV2(ids); return }); err != nil || response == nil {
		if err != nil {
			return nil, classify(err)
		}
		return nil, errors.New("profile lookup failed")
	}
	out := map[string]ProfileInfo{}
	for _, id := range ids {
		w, ok := response.Contacts[id]
		if !ok || w.Contact.Mid != "" && w.Contact.Mid != id {
			continue
		}
		out[id] = ProfileInfo{Name: label(w.Contact.EffectiveDisplayName(), 160), Path: w.Contact.PicturePath}
	}
	return out, nil
}
