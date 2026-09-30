package gui

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

func (e *Engine) refresh(resync bool) {
	if e.state.Mode != "live" || e.state.Login.Active || e.state.Busy || e.pending || e.ops.Snapshot == nil {
		return
	}
	e.state.Busy = true
	e.state.Status = "checking"
	e.state.StatusText = "LINEを確認しています…"
	e.state.ContactsStatus = "loading"
	epoch := e.epoch
	ctx := e.resourceCtx
	e.publish()
	e.spawn(func() {
		snapshot, err := e.ops.Snapshot(ctx)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		e.state.Busy = false
		if err != nil {
			e.state.ContactsStatus = "error"
			if errors.Is(err, ErrUnauthenticated) {
				e.loseAuthentication()
			} else {
				e.state.Status = "error"
				e.state.StatusText = "LINEを読み込めませんでした。再読み込みしてください"
			}
			e.publish()
			return
		}
		if !fullID.MatchString(snapshot.Account.ID) || len(snapshot.Chats) > 500 || len(snapshot.Contacts) > 500 {
			e.state.Status = "error"
			e.state.StatusText = "LINEの応答を確認できませんでした"
			e.publish()
			return
		}
		if e.state.Account.ID != "" && e.state.Account.ID != snapshot.Account.ID {
			e.resetIdentity()
			e.state.Chats = nil
		}
		e.state.Account = snapshot.Account
		e.picturePaths = snapshot.Pictures
		if !resync {
			e.profileAttempted = map[string]bool{}
		}
		e.state.Avatars = map[string]string{}
		serverIDs := map[string]bool{}
		for _, c := range snapshot.Chats {
			serverIDs[c.ID] = true
		}
		local := []Chat{}
		for id, c := range e.local {
			if !serverIDs[id] {
				local = append(local, c)
			}
		}
		if len(snapshot.Chats)+len(local) > 500 {
			e.state.ContactNote = "読み込めるトークは最大500件です。作成中のトークと下書きを保つため、一覧の更新を保留しました"
			e.state.Status = "ready"
			e.state.Busy = false
			e.startWatch()
			e.publish()
			return
		}
		for id := range serverIDs {
			delete(e.local, id)
		}
		e.state.ContactNote = ""
		for i := range snapshot.Chats {
			for _, old := range e.state.Chats {
				if old.ID == snapshot.Chats[i].ID {
					snapshot.Chats[i].Preview = old.Preview
					if old.UpdatedAt > snapshot.Chats[i].UpdatedAt {
						snapshot.Chats[i].UpdatedAt = old.UpdatedAt
						snapshot.Chats[i].Time = old.Time
					}
					break
				}
			}
		}
		e.state.Chats = append(append([]Chat{}, snapshot.Chats...), local...)
		sort.SliceStable(e.state.Chats, func(i, j int) bool { return e.state.Chats[i].UpdatedAt > e.state.Chats[j].UpdatedAt })
		e.state.Contacts = snapshot.Contacts
		e.state.NamesPartial = snapshot.NamesPartial
		e.known = map[string]Contact{}
		for _, c := range snapshot.Contacts {
			e.known[c.ID] = c
		}
		if snapshot.ContactsFailed {
			e.state.ContactsStatus = "error"
		} else {
			e.state.ContactsStatus = "ready"
		}
		e.state.Status = "ready"
		e.state.StatusText = "接続中"
		if e.state.SelectedID != "" {
			found := false
			for _, c := range e.state.Chats {
				if c.ID == e.state.SelectedID {
					found = true
					break
				}
			}
			if found {
				e.selectChat(e.state.SelectedID)
			} else {
				e.state.SelectedID = ""
				e.state.Messages = []Message{}
				e.state.Draft = ""
				e.state.Reply = nil
				e.state.Attachment = nil
			}
		}
		e.startWatch()
		e.syncAvatars(!resync)
		e.publish()
	})
}

var ErrUnauthenticated = errors.New("session unavailable")

func (e *Engine) startWatch() {
	if e.ops.Watch == nil || e.watchCancel != nil || e.state.Mode != "live" {
		return
	}
	ctx, cancel := context.WithCancel(e.resourceCtx)
	e.watchCancel = cancel
	epoch := e.epoch
	e.state.Watching = true
	e.spawn(func() {
		err := e.ops.Watch(ctx, func(event WatchEvent) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.disposed || e.epoch != epoch || e.state.Mode != "live" || e.state.Login.Active {
				return
			}
			e.watchEvent(event)
			e.publish()
		})
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		e.watchCancel = nil
		e.state.Watching = false
		if err != nil && !errors.Is(err, context.Canceled) {
			if errors.Is(err, ErrUnauthenticated) {
				e.loseAuthentication()
			} else {
				e.state.Status = "offline"
				e.state.StatusText = "受信が停止しました。設定から再読み込みしてください"
			}
		}
		e.publish()
	})
}

func loginView(stage, attempt, request, image, pin, reason string) Login {
	labels := map[string]string{"idle": "QRコードでログイン", "preparing": "QRコードを準備しています…", "replace-confirmation": "この端末に保存済みのLINEセッションを置き換えますか？", "scan": "スマートフォンのLINEでQRコードを読み取ってください", "phone": "スマートフォンのLINEでログインを承認してください", "saving": "承認結果を確認し、この端末へ安全に保存しています…", "success": "ログイン情報を保存しました。トークを読み込みます", "cancelled": "ログインを中止しました", "expired": "QRコードの有効期限が切れました。もう一度お試しください", "error": "ログインできませんでした。もう一度お試しください", "uncertain": "ログイン結果を確認できません。スマートフォンの表示を確認し、再読み込みで保存状態を確認してください"}
	active := stage == "preparing" || stage == "replace-confirmation" || stage == "scan" || stage == "phone" || stage == "saving"
	v := Login{Stage: stage, Attempt: attempt, Request: request, Active: active, Label: "QRコードでログイン", StatusText: labels[stage], CanConfirm: stage == "replace-confirmation", CanCancel: active, CanRetry: stage == "cancelled" || stage == "expired" || stage == "error"}
	if stage == "scan" {
		v.Image = image
	}
	if stage == "phone" {
		v.PIN = pin
	}
	if reason != "" {
		v.StatusText = reason
	}
	return v
}
func (e *Engine) startLogin(c Command) {
	if e.state.Mode != "live" || e.state.Busy || e.pending || e.mutationActive || e.state.Login.Active || e.ops.Login == nil || c.Action == "login-retry" && !e.state.Login.CanRetry {
		return
	}
	e.resetIdentity()
	e.state.Chats = []Chat{}
	e.state.Contacts = []Contact{}
	e.state.Account = Account{Name: "自分"}
	e.state.Busy = true
	attempt := token()
	request := ""
	if strings.HasPrefix(c.Request, "ui-") && len(c.Request) <= 15 {
		request = c.Request
	}
	e.state.Login = loginView("preparing", attempt, request, "", "", "")
	epoch := e.epoch
	ctx, cancel := context.WithTimeout(e.resourceCtx, 610*time.Second)
	e.loginCancel = cancel
	e.loginConfirm = make(chan struct{})
	confirm := e.loginConfirm
	e.publish()
	e.spawn(func() {
		err := e.ops.Login(ctx, confirm, func(ev LoginEvent) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.disposed || e.epoch != epoch || e.state.Login.Attempt != attempt {
				return context.Canceled
			}
			e.state.Login = loginView(ev.Stage, attempt, request, ev.Image, ev.PIN, ev.Reason)
			e.publish()
			return nil
		})
		cancel()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		e.state.Busy = false
		e.loginCancel = nil
		e.loginConfirm = nil
		stage := "success"
		reason := ""
		if err != nil {
			stage = "error"
			reason = "ログインできませんでした。保存状態を確認して再試行してください"
			if errors.Is(err, context.Canceled) {
				stage = "cancelled"
				reason = ""
			}
			if errors.Is(err, ErrLoginUncertain) {
				stage = "uncertain"
				reason = ""
			}
		}
		e.state.Login = loginView(stage, attempt, request, "", "", reason)
		e.state.Session = token()
		e.state.Selection++
		e.state.Status = "idle"
		e.publish()
		if stage == "success" {
			e.refresh(false)
		}
	})
}

var ErrLoginUncertain = errors.New("login outcome uncertain")

func (e *Engine) confirmLogin(c Command) {
	if !e.state.Login.CanConfirm || c.Attempt != e.state.Login.Attempt || e.loginConfirm == nil {
		return
	}
	close(e.loginConfirm)
	e.loginConfirm = nil
	e.state.Login = loginView("preparing", e.state.Login.Attempt, e.state.Login.Request, "", "", "")
}
func (e *Engine) cancelLoginAttempt() {
	if !e.state.Login.Active {
		return
	}
	stage := "cancelled"
	if e.state.Login.Stage == "phone" || e.state.Login.Stage == "saving" {
		stage = "uncertain"
	}
	attempt, request := e.state.Login.Attempt, e.state.Login.Request
	e.epoch++
	if e.loginCancel != nil {
		e.loginCancel()
		e.loginCancel = nil
	}
	e.loginConfirm = nil
	e.state.Login = loginView(stage, attempt, request, "", "", "")
	e.state.Busy = false
	e.state.Status = "idle"
}
