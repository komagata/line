package gui

import (
	"errors"
	"strings"
)

func (e *Engine) queueProfiles() {
	if e.ops.Profiles == nil || e.state.Mode != "live" || e.state.Status != "ready" || e.disposed {
		return
	}
	ids := []string{}
	for _, m := range e.state.Messages {
		if fullID.MatchString(m.SenderID) && strings.EqualFold(m.SenderID[:1], "u") && e.picturePaths[m.SenderID] == "" && !e.profileAttempted[m.SenderID] {
			ids = append(ids, m.SenderID)
			e.profileAttempted[m.SenderID] = true
			if len(ids) >= 100 || len(e.profileAttempted) >= 500 {
				break
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	epoch := e.epoch
	ctx := e.resourceCtx
	selection := e.state.Selection
	lookup := e.ops.Profiles
	e.spawn(func() {
		profiles, err := lookup(ctx, ids)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.disposed || e.epoch != epoch {
			return
		}
		if errors.Is(err, ErrUnauthenticated) {
			e.loseAuthentication()
			return
		}
		if err != nil || e.state.Selection != selection {
			return
		}
		for _, id := range ids {
			p, ok := profiles[id]
			if !ok {
				continue
			}
			if path := avatarPath(p.Path); path != "" {
				e.picturePaths[id] = path
			}
			if p.Name != "" {
				for i := range e.state.Messages {
					if e.state.Messages[i].SenderID == id && !e.state.Messages[i].Own {
						e.state.Messages[i].Sender = p.Name
						e.state.Messages[i].SenderUnavailable = false
					}
				}
			}
		}
		e.syncAvatars(false)
		e.publish()
	})
}
