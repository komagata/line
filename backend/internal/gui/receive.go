package gui

import (
	"context"
	"regexp"
	"time"
)

var revisionPattern = regexp.MustCompile(`^[0-9]{1,24}$`)

func (e *Engine) scheduleResync() {
	if e.resyncTimer != nil || e.disposed || e.state.Mode != "live" || e.state.Login.Active {
		return
	}
	wait := e.resyncDelay
	if e.lastResync.IsZero() {
		wait = 200 * time.Millisecond
	}
	if e.resyncDelay < wait {
		wait = e.resyncDelay
	}
	epoch := e.epoch
	e.resyncTimer = time.AfterFunc(wait, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.epoch != epoch || e.disposed {
			return
		}
		e.resyncTimer = nil
		if e.state.Busy || e.pending || e.fileBusy {
			e.scheduleResync()
			return
		}
		e.lastResync = time.Now()
		e.refresh(true)
	})
}
func (e *Engine) notice(m Message, chat string) {
	now := time.Now()
	when := time.UnixMilli(m.Timestamp)
	if !e.state.Notifications || m.Own || when.Before(e.startedAt.Add(-2*time.Second)) || when.Before(now.Add(-30*time.Second)) || when.After(now.Add(5*time.Second)) || e.focusID == chat && e.focusSession == e.state.Session && e.focusSelection == e.state.Selection {
		return
	}
	if len(e.noticePending) >= 500 {
		return
	}
	e.noticePending[chat] = when
	if e.noticeTimer != nil {
		return
	}
	wait := e.noticeDelay
	if since := time.Until(e.lastNotice.Add(5 * time.Second)); since > wait {
		wait = since
	}
	epoch := e.epoch
	ctx := e.resourceCtx
	e.noticeTimer = time.AfterFunc(wait, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		e.noticeTimer = nil
		eligible := false
		for id, at := range e.noticePending {
			if time.Since(at) > 30*time.Second {
				continue
			}
			if e.focusID == id && e.focusSession == e.state.Session && e.focusSelection == e.state.Selection {
				continue
			}
			eligible = true
			break
		}
		e.noticePending = map[string]time.Time{}
		if !eligible || !e.state.Notifications || e.state.Mode != "live" {
			return
		}
		e.lastNotice = time.Now()
		notify := e.ops.Notify
		locale := e.locale
		if e.ops.NotifyLocalized != nil {
			notify = func(ctx context.Context) bool { return e.ops.NotifyLocalized(ctx, locale) }
		}
		if notify == nil {
			e.state.NotificationNote = "通知を表示できません。notify-send の導入を確認してください"
			e.publish()
			return
		}
		e.spawn(func() {
			delivered := notify(ctx)
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.disposed || e.epoch != epoch {
				return
			}
			if !delivered {
				e.state.NotificationNote = "通知を表示できません。notify-send の導入を確認してください"
				e.publish()
			}
		})
	})
}
func (e *Engine) watchEvent(event WatchEvent) {
	if !revisionPattern.MatchString(event.Revision) {
		return
	}
	if _, seen := e.seenRevision[event.Revision]; seen {
		return
	}
	e.seenRevision[event.Revision] = struct{}{}
	e.revisionOrder = append(e.revisionOrder, event.Revision)
	if len(e.revisionOrder) > 4096 {
		delete(e.seenRevision, e.revisionOrder[0])
		e.revisionOrder = e.revisionOrder[1:]
	}
	if event.Kind == "resync_required" || event.Kind == "operation" {
		e.scheduleResync()
		return
	}
	if event.Kind != "message" || !fullID.MatchString(event.ChatID) || event.Message == nil {
		return
	}
	m := *event.Message
	key := event.ChatID + ":" + m.ID
	if _, seen := e.seenMessages[key]; seen {
		return
	}
	if !numericID.MatchString(m.ID) {
		return
	}
	e.seenMessages[key] = struct{}{}
	e.messageOrder = append(e.messageOrder, key)
	if len(e.messageOrder) > 4096 {
		delete(e.seenMessages, e.messageOrder[0])
		e.messageOrder = e.messageOrder[1:]
	}
	if e.state.SelectedID == event.ChatID {
		found := false
		for i, existing := range e.state.Messages {
			if existing.ID == m.ID {
				e.state.Messages[i] = m
				found = true
				break
			}
		}
		if !found {
			e.state.Messages = append(e.state.Messages, m)
		}
		if len(e.state.Messages) > 100 {
			e.state.Messages = e.state.Messages[len(e.state.Messages)-100:]
		}
	}
	found := false
	for i := range e.state.Chats {
		if e.state.Chats[i].ID == event.ChatID {
			found = true
			e.state.Chats[i].Preview = label(m.Text, 100)
			copy := m
			e.state.Chats[i].PreviewMessage = &copy
			e.state.Chats[i].PreviewKey = ""
			e.state.Chats[i].Time = m.Time
			e.state.Chats[i].UpdatedAt = m.Timestamp
			if !m.Own {
				e.state.Chats[i].Unread = min(9999, e.state.Chats[i].Unread+1)
			}
			break
		}
	}
	if !found {
		e.scheduleResync()
	}
	e.notice(m, event.ChatID)
	e.queueProfiles()
	e.syncAvatars(false)
	e.syncStickerImages()
}
