package session

import (
	"context"
	"sync"

	"github.com/kongesque/line-cli/pkg/line"
)

// WithContext creates an operation-local facade, never changing the manager or
// factory used by concurrent watch/commands. Storage and request sequences still
// use the original manager's mutex and the caller's process lock.
func (m *Manager) WithContext(ctx context.Context) *Manager {
	store := m.Store
	storage := m.Storage
	if scoped, ok := store.(interface{ WithContext(context.Context) Store }); ok {
		store = scoped.WithContext(ctx)
		if preparer, ok := store.(StoragePreparer); ok {
			storage = preparer
		}
	}
	return &Manager{parent: m, context: ctx, Store: store, Storage: storage,
		Wait: m.Wait, Now: m.Now, ExportKeys: m.ExportKeys, beginQRKey: m.beginQRKey,
		NewClient: func(token string) API {
			api := m.NewClient(token)
			if client, ok := api.(*line.Client); ok {
				return client.WithContext(ctx)
			}
			return api
		},
	}
}
func (m *Manager) mutex() *sync.Mutex {
	if m.parent != nil {
		return m.parent.mutex()
	}
	return &m.mu
}
func (m *Manager) ctx() context.Context {
	if m.context != nil {
		return m.context
	}
	return context.Background()
}
