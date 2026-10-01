package gui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

var fullID = regexp.MustCompile(`^(?:[uUcCrR][A-Za-z0-9_-]{43}|[ucr][0-9a-f]{32})$`)
var numericID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

type Command struct {
	Action       string `json:"action"`
	Mode         string `json:"mode,omitempty"`
	Locale       string `json:"locale,omitempty"`
	ID           string `json:"id,omitempty"`
	Session      string `json:"session,omitempty"`
	Selection    int    `json:"selection,omitempty"`
	Text         string `json:"text,omitempty"`
	Sequence     int    `json:"sequence,omitempty"`
	ReplyTo      string `json:"replyTo,omitempty"`
	AttachmentID string `json:"attachmentId,omitempty"`
	ContactID    string `json:"contactId,omitempty"`
	MessageID    string `json:"messageId,omitempty"`
	Reaction     string `json:"reaction,omitempty"`
	Remove       bool   `json:"remove,omitempty"`
	Confirmed    bool   `json:"confirmed,omitempty"`
	URL          string `json:"url,omitempty"`
	Enabled      bool   `json:"enabled,omitempty"`
	Focused      bool   `json:"focused,omitempty"`
	Request      string `json:"request,omitempty"`
	Attempt      string `json:"attempt,omitempty"`
	PackageID    string `json:"packageId,omitempty"`
	StickerID    string `json:"stickerId,omitempty"`
	Page         int    `json:"page,omitempty"`
}
type Chat struct {
	PreviewMessage  *Message `json:"-"`
	PreviewKey      string   `json:"-"`
	DemoFixture     bool     `json:"-"`
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Preview         string   `json:"preview"`
	Time            string   `json:"time"`
	Unread          int      `json:"unread"`
	Group           bool     `json:"group"`
	UpdatedAt       int64    `json:"updatedAt"`
	NameUnavailable bool     `json:"nameUnavailable"`
}
type Contact struct {
	NameUnavailable bool   `json:"nameUnavailable,omitempty"`
	DemoFixture     bool   `json:"-"`
	ID              string `json:"id"`
	Name            string `json:"name"`
}
type Account struct {
	NameUnavailable bool   `json:"nameUnavailable,omitempty"`
	DemoFixture     bool   `json:"-"`
	ID              string `json:"id"`
	Name            string `json:"name"`
}
type Reaction struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Own   bool   `json:"own"`
}
type Sticker struct {
	ID        string `json:"id"`
	PackageID string `json:"packageId"`
	Version   string `json:"version"`
	Option    string `json:"option"`
	Hash      string `json:"hash"`
	Alt       string `json:"alt"`
}
type Message struct {
	GeneratedText     bool       `json:"generatedText,omitempty"`
	DemoFixture       bool       `json:"-"`
	SenderUnavailable bool       `json:"senderUnavailable,omitempty"`
	ID                string     `json:"id"`
	Text              string     `json:"text"`
	SenderID          string     `json:"senderId"`
	Sender            string     `json:"sender"`
	Own               bool       `json:"own"`
	Timestamp         int64      `json:"timestamp"`
	Time              string     `json:"time"`
	Day               string     `json:"day"`
	Encrypted         *bool      `json:"encrypted"`
	Status            string     `json:"status"`
	ReplyTo           string     `json:"replyTo"`
	ContentType       int        `json:"contentType"`
	Downloadable      bool       `json:"downloadable"`
	FileName          string     `json:"fileName"`
	Reactions         []Reaction `json:"reactions"`
	Sticker           *Sticker   `json:"sticker"`
}
type Attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int    `json:"size"`
	Data []byte `json:"-"`
}
type Reply struct {
	Source *Message `json:"-"`
	ID     string   `json:"id"`
	Text   string   `json:"text"`
	Sender string   `json:"sender"`
}
type Login struct {
	Stage      string `json:"stage"`
	Attempt    string `json:"attempt"`
	Request    string `json:"request"`
	Active     bool   `json:"active"`
	Label      string `json:"label"`
	StatusText string `json:"statusText"`
	Image      string `json:"image"`
	PIN        string `json:"pin"`
	CanConfirm bool   `json:"canConfirm"`
	CanCancel  bool   `json:"canCancel"`
	CanRetry   bool   `json:"canRetry"`
}
type StickerProduct struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Count        int      `json:"count"`
	Supported    bool     `json:"supported"`
	ResourceType int      `json:"resourceType"`
	Poster       *Sticker `json:"poster"`
}
type StickerView struct {
	Open      bool             `json:"open"`
	Status    string           `json:"status"`
	Note      string           `json:"note"`
	Products  []StickerProduct `json:"products"`
	PackageID string           `json:"packageId"`
	Page      int              `json:"page"`
	Pages     int              `json:"pages"`
	Items     []Sticker        `json:"items"`
	Selected  *Sticker         `json:"selected"`
}
type View struct {
	Mode             string            `json:"mode"`
	Session          string            `json:"session"`
	Selection        int               `json:"selection"`
	Status           string            `json:"status"`
	StatusText       string            `json:"statusText"`
	Login            Login             `json:"login"`
	Contacts         []Contact         `json:"contacts"`
	ContactsStatus   string            `json:"contactsStatus"`
	ContactNote      string            `json:"contactNote"`
	Avatars          map[string]string `json:"avatars"`
	StickerImages    map[string]string `json:"stickerImages"`
	Stickers         StickerView       `json:"stickers"`
	Chats            []Chat            `json:"chats"`
	Messages         []Message         `json:"messages"`
	SelectedID       string            `json:"selectedId"`
	Draft            string            `json:"draft"`
	DraftVersion     int               `json:"draftVersion"`
	DraftAck         int               `json:"draftAck"`
	Reply            *Reply            `json:"reply"`
	Attachment       *Attachment       `json:"attachment"`
	Preview          string            `json:"preview"`
	FileStatus       string            `json:"fileStatus"`
	Notifications    bool              `json:"notifications"`
	NotificationNote string            `json:"notificationNote"`
	Account          Account           `json:"account"`
	HistoryStatus    string            `json:"historyStatus"`
	HistoryNote      string            `json:"historyNote"`
	SendStatus       string            `json:"sendStatus"`
	SendNote         string            `json:"sendNote"`
	Watching         bool              `json:"watching"`
	NamesPartial     bool              `json:"namesPartial"`
	Busy             bool              `json:"busy"`
}
type Snapshot struct {
	Account        Account
	Chats          []Chat
	Contacts       []Contact
	ContactsFailed bool
	Pictures       map[string]string
	NamesPartial   bool
}
type SendResult struct {
	ID        string
	ChatID    string
	Encrypted bool
}
type ActionResult struct {
	Action    string
	ChatID    string
	MessageID string
}
type LoginEvent struct {
	Stage  string
	Image  string
	PIN    string
	Reason string
}
type Operations struct {
	Snapshot        func(context.Context) (Snapshot, error)
	History         func(context.Context, string) ([]Message, error)
	Send            func(context.Context, string, string, string, *Attachment) (SendResult, error)
	Action          func(context.Context, string, string, string, string, bool) (ActionResult, error)
	Download        func(context.Context, string, string) ([]byte, error)
	Catalog         func(context.Context) ([]OwnedProduct, error)
	SendSticker     func(context.Context, string, string, string, string) (SendResult, error)
	Login           func(context.Context, <-chan struct{}, func(LoginEvent) error) error
	Watch           func(context.Context, func(WatchEvent)) error
	Notify          func(context.Context) bool
	NotifyLocalized func(context.Context, string) bool
	FetchAvatar     func(context.Context, string) string
	FetchSticker    func(context.Context, Sticker) string
	Profiles        func(context.Context, []string) (map[string]ProfileInfo, error)
}
type ProfileInfo struct {
	Name string
	Path string
}
type OwnedProduct struct {
	ID, Name, Version, ValidUntil, Hash string
	ResourceType                        int
	Ranges                              []OwnedRange
	Count                               int
	Supported                           bool
}
type OwnedRange struct {
	Start string
	Size  int
}
type WatchEvent struct {
	Kind, Revision, ChatID string
	Message                *Message
}

type Engine struct {
	locale            string
	mu                sync.Mutex
	ops               Operations
	emit              func(View)
	state             View
	ctx               context.Context
	cancel            context.CancelFunc
	resourceCtx       context.Context
	resourceCancel    context.CancelFunc
	stickerCtx        context.Context
	stickerCancel     context.CancelFunc
	wg                sync.WaitGroup
	epoch             uint64
	historyGeneration uint64
	loginGeneration   uint64
	watchCancel       context.CancelFunc
	historyCancel     context.CancelFunc
	loginCancel       context.CancelFunc
	loginConfirm      chan struct{}
	fileCancel        context.CancelFunc
	drafts            map[string]string
	replies           map[string]Reply
	local             map[string]Chat
	known             map[string]Contact
	pending           bool
	mutationActive    bool
	fileBusy          bool
	fileGeneration    uint64
	products          []OwnedProduct
	disposed          bool
	inflight          int
	demoSeq           int
	demoMessages      map[string][]Message
	seenRevision      map[string]struct{}
	revisionOrder     []string
	seenMessages      map[string]struct{}
	messageOrder      []string
	lastResync        time.Time
	resyncTimer       *time.Timer
	resyncDelay       time.Duration
	noticeDelay       time.Duration
	noticeTimer       *time.Timer
	lastNotice        time.Time
	noticePending     map[string]time.Time
	focusID           string
	focusSession      string
	focusSelection    int
	startedAt         time.Time
	picturePaths      map[string]string
	profileAttempted  map[string]bool
	avatarLoading     map[string]bool
	avatarFailed      map[string]bool
	avatarCacheBytes  int
	avatarData        map[string]string
	avatarActive      int
	stickerAttempted  map[string]bool
	stickerData       map[string]string
	stickerActive     int
	stickerGeneration uint64
}

func token() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func initial() View {
	return View{Mode: "live", Session: token(), Status: "idle", StatusText: "準備しています…", Login: Login{Stage: "idle"}, Contacts: []Contact{}, ContactsStatus: "idle", Avatars: map[string]string{}, StickerImages: map[string]string{}, Stickers: StickerView{Status: "idle", Products: []StickerProduct{}, Items: []Sticker{}}, Chats: []Chat{}, Messages: []Message{}, Account: Account{Name: "自分", NameUnavailable: true}, HistoryStatus: "idle", SendStatus: "idle", FileStatus: "idle", Notifications: true}
}
func NewEngine(ops Operations, emit func(View)) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	resourceCtx, resourceCancel := context.WithCancel(ctx)
	stickerCtx, stickerCancel := context.WithCancel(resourceCtx)
	return &Engine{resourceCtx: resourceCtx, resourceCancel: resourceCancel, stickerCtx: stickerCtx, stickerCancel: stickerCancel, ops: ops, emit: emit, state: initial(), ctx: ctx, cancel: cancel, drafts: map[string]string{}, replies: map[string]Reply{}, local: map[string]Chat{}, known: map[string]Contact{}, seenRevision: map[string]struct{}{}, seenMessages: map[string]struct{}{}, noticePending: map[string]time.Time{}, resyncDelay: 5 * time.Second, noticeDelay: 500 * time.Millisecond, startedAt: time.Now(), picturePaths: map[string]string{}, profileAttempted: map[string]bool{}, avatarLoading: map[string]bool{}, avatarFailed: map[string]bool{}, avatarData: map[string]string{}, stickerAttempted: map[string]bool{}, stickerData: map[string]string{}, demoMessages: map[string][]Message{}}
}
func (e *Engine) View() View { e.mu.Lock(); defer e.mu.Unlock(); return e.localizedView() }
func (e *Engine) publish() {
	if e.emit != nil {
		e.emit(e.localizedView())
	}
}                       // caller holds mu; emit must enqueue only
func (e *Engine) Wait() { e.wg.Wait() }
func (e *Engine) Close() {
	e.mu.Lock()
	if !e.disposed {
		e.disposed = true
		e.epoch++
		e.cancel()
		e.stopJobs()
		e.state.AvatarClear()
		e.publish()
	}
	e.mu.Unlock()
	e.wg.Wait()
}
func (v *View) AvatarClear() {
	v.Avatars = map[string]string{}
	v.StickerImages = map[string]string{}
	v.Preview = ""
}
func (e *Engine) loseAuthentication() {
	e.resetIdentity()
	e.state.Chats = []Chat{}
	e.state.Contacts = []Contact{}
	e.state.Account = Account{Name: "自分", NameUnavailable: true}
	e.state.Status = "unauthenticated"
	e.state.StatusText = "LINEへログインしてください"
	e.state.ContactsStatus = "idle"
	e.state.HistoryStatus = "idle"
	e.state.Watching = false
	e.state.Busy = false
	e.publish()
}
func (e *Engine) stopJobs() {
	if e.stickerCancel != nil {
		e.stickerCancel()
	}
	if e.resourceCancel != nil {
		e.resourceCancel()
	}
	e.resourceCtx, e.resourceCancel = context.WithCancel(e.ctx)
	e.stickerCtx, e.stickerCancel = context.WithCancel(e.resourceCtx)
	if e.watchCancel != nil {
		e.watchCancel()
		e.watchCancel = nil
	}
	if e.historyCancel != nil {
		e.historyCancel()
		e.historyCancel = nil
	}
	if e.loginCancel != nil {
		e.loginCancel()
		e.loginCancel = nil
	}
	if e.fileCancel != nil {
		e.fileCancel()
		e.fileCancel = nil
	}
	if e.resyncTimer != nil {
		e.resyncTimer.Stop()
		e.resyncTimer = nil
	}
	if e.noticeTimer != nil {
		e.noticeTimer.Stop()
		e.noticeTimer = nil
	}
}
func (e *Engine) spawn(fn func()) {
	e.inflight++
	e.wg.Add(1)
	go func() { defer e.wg.Done(); defer func() { e.mu.Lock(); e.inflight--; e.mu.Unlock() }(); fn() }()
}
func (e *Engine) current(c Command) bool {
	return c.ID != "" && c.ID == e.state.SelectedID && c.Session == e.state.Session && c.Selection == e.state.Selection && !e.disposed && e.state.Login.Active == false
}
func (e *Engine) available(c Command) bool {
	return e.current(c) && e.state.Status == "ready" && !e.state.Busy && !e.pending && !e.mutationActive && !e.fileBusy
}
func (e *Engine) resetIdentity() {
	e.epoch++
	e.stopJobs()
	e.state.Session = token()
	e.state.Selection++
	e.state.SelectedID = ""
	e.state.Messages = []Message{}
	e.state.Draft = ""
	e.state.Reply = nil
	e.state.Attachment = nil
	e.state.AvatarClear()
	e.drafts = map[string]string{}
	e.replies = map[string]Reply{}
	e.local = map[string]Chat{}
	e.known = map[string]Contact{}
	e.seenRevision = map[string]struct{}{}
	e.revisionOrder = nil
	e.seenMessages = map[string]struct{}{}
	e.messageOrder = nil
	e.noticePending = map[string]time.Time{}
	e.focusID = ""
	e.pending = false
	e.fileBusy = false
	e.products = nil
	e.picturePaths = map[string]string{}
	e.profileAttempted = map[string]bool{}
	e.avatarLoading = map[string]bool{}
	e.avatarFailed = map[string]bool{}
	e.avatarCacheBytes = 0
	e.avatarActive = 0
	e.avatarData = map[string]string{}
	e.stickerAttempted = map[string]bool{}
	e.stickerData = map[string]string{}
	e.stickerActive = 0
	e.stickerGeneration++
}
func (e *Engine) Handle(c Command) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.disposed {
		return
	}
	if e.inflight >= 32 && c.Action != "locale" && c.Action != "mode" && c.Action != "login-cancel" && c.Action != "file-cancel" && c.Action != "preview-close" {
		return
	}
	switch c.Action {
	case "locale":
		if c.Text == "ja" || c.Text == "en" {
			e.locale = c.Text
		}
	case "mode":
		if c.Locale == "ja" || c.Locale == "en" {
			e.locale = c.Locale
		}
		e.mode(c.Mode)
	case "refresh":
		e.refresh(false)
	case "select":
		e.selectChat(c.ID)
	case "start-chat":
		e.startChat(c)
	case "draft":
		e.draft(c)
	case "send":
		e.send(c)
	case "notifications":
		e.state.Notifications = c.Enabled
	case "focus":
		if e.current(c) && c.Focused {
			e.focusID = c.ID
			e.focusSession = c.Session
			e.focusSelection = c.Selection
		} else {
			e.focusID = ""
		}
	case "login", "login-retry":
		e.startLogin(c)
	case "login-confirm":
		e.confirmLogin(c)
	case "login-cancel":
		e.cancelLoginAttempt()
	case "reply", "reply-cancel", "react", "unsend", "attach", "attach-cancel", "download", "preview", "preview-close", "file-cancel":
		e.feature(c)
	case "sticker-open", "sticker-page", "sticker-choose", "sticker-close", "sticker-hide", "sticker-send":
		e.sticker(c)
	}
	e.syncStickerImages()
	e.publish()
}
func (e *Engine) mode(mode string) {
	if mode != "live" && mode != "demo" || mode == e.state.Mode {
		return
	}
	notifications := e.state.Notifications
	e.resetIdentity()
	e.state = initial()
	e.state.Notifications = notifications
	e.state.Mode = mode
	if mode == "demo" {
		e.demo()
	}
}
func (e *Engine) demo() {
	e.state.Status = "ready"
	e.state.StatusText = "架空の会話 · デモ"
	e.state.Account = Account{ID: "demo-self", Name: "自分", DemoFixture: true}
	names := []string{"山田 太郎", "週末の集まり", "佐藤 花子", "家族", "鈴木 健"}
	previews := []string{"駅前のカフェでどう？", "土曜日、楽しみにしてます！", "写真ありがとう！", "了解です", "また来週！"}
	times := []string{"19:40", "19:32", "18:15", "17:08", "昨日"}
	e.demoMessages = map[string][]Message{}
	for i, name := range names {
		id := "demo-" + string(rune('0'+i))
		chat := Chat{DemoFixture: true, ID: id, Name: name, Preview: previews[i], Time: times[i], Unread: 0, Group: i == 1 || i == 3, UpdatedAt: int64(5 - i)}
		if i == 1 {
			chat.Unread = 2
		}
		e.state.Chats = append(e.state.Chats, chat)
		if !chat.Group {
			e.state.Contacts = append(e.state.Contacts, Contact{DemoFixture: true, ID: id, Name: name})
			e.known[id] = Contact{DemoFixture: true, ID: id, Name: name}
		}
		if i > 0 {
			e.demoMessages[id] = []Message{{DemoFixture: true, ID: strconv.Itoa(201 + i), Text: previews[i], Sender: name, SenderID: id, Time: times[i], Day: "今日", Timestamp: 1, Status: "ok"}}
		}
	}
	e.state.Contacts = append(e.state.Contacts, Contact{DemoFixture: true, ID: "demo-new", Name: "高橋 葵"}, Contact{DemoFixture: true, ID: "demo-new-2", Name: "田中 直人"})
	e.known["demo-new"] = Contact{DemoFixture: true, ID: "demo-new", Name: "高橋 葵"}
	e.known["demo-new-2"] = Contact{DemoFixture: true, ID: "demo-new-2", Name: "田中 直人"}
	texts := []string{"明日の待ち合わせ、何時にする？", "19時くらいでどうかな？", "いいね！", "駅前のカフェでどう？", "そこにしよう！"}
	rows := []Message{}
	for i, text := range texts {
		own := i == 1 || i == 4
		sender, id := "山田 太郎", "demo-0"
		if own {
			sender, id = "自分", "demo-self"
		}
		rows = append(rows, Message{DemoFixture: true, ID: strconv.Itoa(101 + i), Text: text, Own: own, Sender: sender, SenderID: id, Time: []string{"19:35", "19:36", "19:38", "19:40", "19:41"}[i], Day: "今日", Timestamp: int64(i), Status: "ok", Reactions: []Reaction{}})
	}
	rows[2].ContentType = 7
	rows[2].Encrypted = boolPtr(false)
	rows[2].Sticker = &Sticker{ID: "1001", PackageID: "1", Version: "1", Alt: "架空のカラースタンプ"}
	rows[2].Reactions = []Reaction{{Name: "love", Count: 2, Own: true}}
	rows[3].Text = "カフェの写真"
	rows[3].ContentType = 1
	rows[3].Downloadable = true
	rows[3].FileName = "cafe.png"
	rows[4].ReplyTo = "104"
	e.demoMessages["demo-0"] = rows
	e.state.SelectedID = "demo-0"
	e.state.Messages = append([]Message(nil), rows...)
	e.state.HistoryStatus = "ready"
	e.state.ContactsStatus = "ready"
	e.state.Avatars = map[string]string{"demo-self": demoBlue, "demo-0": demoSage, "demo-2": demoSand, "demo-new": demoBlue}
}
func boolPtr(v bool) *bool { return &v }

func (e *Engine) draft(c Command) {
	if !e.current(c) || len(c.Text) > 40000 || len(utf16.Encode([]rune(c.Text))) > 10000 || c.Sequence < 0 {
		return
	}
	if c.Text != "" && e.drafts[c.ID] == "" && len(e.drafts) >= 50 {
		return
	}
	e.state.Draft = c.Text
	e.state.DraftVersion++
	e.state.DraftAck = c.Sequence
	if c.Text == "" {
		delete(e.drafts, c.ID)
	} else {
		e.drafts[c.ID] = c.Text
	}
}
func (e *Engine) send(c Command) {
	if !e.available(c) || c.Text != e.state.Draft || c.AttachmentID != "" && e.state.Attachment == nil || e.state.Stickers.Selected != nil {
		return
	}
	reply := e.replies[c.ID]
	if c.ReplyTo != reply.ID {
		return
	}
	if reply.ID != "" && !e.hasMessage(reply.ID) {
		e.state.SendNote = "返信先が現在の履歴にありません。返信を解除してください"
		return
	}
	attachment := e.state.Attachment
	if attachment != nil && (c.AttachmentID != attachment.ID || c.Text != "") {
		return
	}
	if attachment == nil && (strings.TrimSpace(c.Text) == "" || len(utf16.Encode([]rune(c.Text))) > 10000) {
		return
	}
	if e.state.Mode == "live" && !fullID.MatchString(c.ID) {
		return
	}
	e.pending = true
	e.mutationActive = true
	e.state.SendStatus = "pending"
	e.state.SendNote = ""
	epoch := e.epoch
	ctx := e.resourceCtx
	mode := e.state.Mode
	draft := c.Text
	e.publish()
	e.spawn(func() {
		var result SendResult
		var err error
		if mode == "demo" {
			e.mu.Lock()
			e.demoSeq++
			demoID := 900000 + e.demoSeq
			e.mu.Unlock()
			result = SendResult{ID: strconv.Itoa(demoID), ChatID: c.ID, Encrypted: true}
		} else if e.ops.Send != nil {
			result, err = e.ops.Send(ctx, c.ID, c.Text, reply.ID, attachment)
		} else {
			err = errors.New("send unavailable")
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
		now := time.Now()
		item := Message{SenderUnavailable: e.state.Account.NameUnavailable || e.state.Account.DemoFixture, ID: result.ID, Text: draft, Own: true, SenderID: e.state.Account.ID, Sender: e.state.Account.Name, Timestamp: now.UnixMilli(), Time: now.Format("15:04"), Day: now.Format("2006/01/02"), Status: "ok", ReplyTo: reply.ID, ContentType: 0}
		if attachment != nil {
			item.Text = "［ファイル：" + attachment.Name + "］"
			item.GeneratedText = true
			item.FileName = attachment.Name
			item.ContentType = 14
			item.Downloadable = true
		}
		e.sentMessage(c.ID, item)
		if e.drafts[c.ID] == draft {
			delete(e.drafts, c.ID)
			if e.state.SelectedID == c.ID {
				e.state.Draft = ""
				e.state.DraftVersion++
			}
		}
		delete(e.replies, c.ID)
		if e.state.SelectedID == c.ID {
			e.state.Reply = nil
			e.state.Attachment = nil
		}
		e.publish()
	})
}
func (e *Engine) hasMessage(id string) bool {
	for _, m := range e.state.Messages {
		if m.ID == id {
			return true
		}
	}
	return false
}
func (e *Engine) startChat(c Command) {
	if c.ID != e.state.SelectedID || c.Session != e.state.Session || c.Selection != e.state.Selection || !e.availableStart() {
		return
	}
	contact, ok := e.known[c.ContactID]
	if !ok || e.state.Mode == "live" && !strings.HasPrefix(contact.ID, "u") {
		return
	}
	for _, chat := range e.state.Chats {
		if chat.ID == contact.ID {
			e.selectChat(contact.ID)
			return
		}
	}
	if len(e.state.Chats) >= 500 || len(e.local) >= 50 {
		return
	}
	chat := Chat{ID: contact.ID, Name: contact.Name, NameUnavailable: contact.NameUnavailable, DemoFixture: contact.DemoFixture, Preview: "まだメッセージはありません", PreviewKey: "まだメッセージはありません"}
	e.local[chat.ID] = chat
	e.state.Chats = append(e.state.Chats, chat)
	e.selectChat(chat.ID)
}
func (e *Engine) availableStart() bool {
	return e.state.Status == "ready" && e.state.ContactsStatus == "ready" && !e.state.Busy && !e.pending && !e.mutationActive && !e.fileBusy
}
func (e *Engine) selectChat(id string) {
	if e.state.Login.Active || e.pending || !slices.ContainsFunc(e.state.Chats, func(c Chat) bool { return c.ID == id }) {
		return
	}
	changed := e.state.SelectedID != id
	if changed {
		e.fileGeneration++
		if e.fileCancel != nil {
			e.fileCancel()
			e.fileCancel = nil
		}
		e.fileBusy = false
		e.state.Attachment = nil
		e.state.Reply = nil
		e.state.Preview = ""
		if e.stickerCancel != nil {
			e.stickerCancel()
		}
		e.stickerCtx, e.stickerCancel = context.WithCancel(e.resourceCtx)
		e.stickerGeneration++
		e.stickerActive = 0
		e.stickerAttempted = map[string]bool{}
		e.stickerData = map[string]string{}
		e.state.StickerImages = map[string]string{}
		e.state.Stickers = StickerView{Status: "idle", Products: []StickerProduct{}, Items: []Sticker{}}
		e.products = nil
		e.focusID = ""
	}
	e.state.SelectedID = id
	e.state.Selection++
	e.state.Draft = e.drafts[id]
	e.state.DraftVersion++
	e.state.Messages = []Message{}
	e.state.HistoryStatus = "loading"
	e.state.HistoryNote = ""
	e.state.Reply = nil
	if reply, ok := e.replies[id]; ok {
		e.state.Reply = &reply
	}
	if e.historyCancel != nil {
		e.historyCancel()
	}
	if e.state.Mode == "demo" {
		e.state.Messages = append([]Message(nil), e.demoMessages[id]...)
		e.state.HistoryStatus = "ready"
		return
	}
	if e.ops.History == nil {
		e.state.HistoryStatus = "error"
		return
	}
	ctx, cancel := context.WithCancel(e.resourceCtx)
	e.historyCancel = cancel
	e.historyGeneration++
	generation, epoch := e.historyGeneration, e.epoch
	e.publish()
	e.spawn(func() {
		rows, err := e.ops.History(ctx, id)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		if errors.Is(err, ErrUnauthenticated) {
			e.loseAuthentication()
			return
		}
		if e.historyGeneration != generation {
			return
		}
		if err != nil || len(rows) > 100 {
			e.state.HistoryStatus = "error"
			e.state.HistoryNote = "履歴を取得できませんでした。再読み込みしてください"
		} else {
			merged := map[string]Message{}
			for _, m := range rows {
				merged[m.ID] = m
			}
			for _, m := range e.state.Messages {
				merged[m.ID] = m
			}
			e.state.Messages = []Message{}
			for _, m := range merged {
				e.state.Messages = append(e.state.Messages, m)
			}
			slices.SortFunc(e.state.Messages, func(a, b Message) int {
				if a.Timestamp < b.Timestamp {
					return -1
				}
				if a.Timestamp > b.Timestamp {
					return 1
				}
				return strings.Compare(a.ID, b.ID)
			})
			if len(e.state.Messages) > 100 {
				e.state.Messages = e.state.Messages[len(e.state.Messages)-100:]
			}
			e.state.HistoryStatus = "ready"
			e.queueProfiles()
			e.syncAvatars(false)
			e.syncStickerImages()
			for _, m := range e.state.Messages {
				if m.Status != "ok" {
					e.state.HistoryStatus = "partial"
					e.state.HistoryNote = "一部のメッセージを復号できませんでした"
					break
				}
			}
		}
		e.publish()
	})
}
