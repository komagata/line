package gui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
)

type memoryStore struct{ s *session.State }

func (m *memoryStore) Load() (*session.State, error) {
	if m.s == nil {
		return nil, session.ErrNotFound
	}
	copy := *m.s
	return &copy, nil
}
func (m *memoryStore) Save(s *session.State) error { copy := *s; m.s = &copy; return nil }
func (m *memoryStore) Delete() error               { m.s = nil; return nil }

type sendAPI struct {
	session.API
	calls int
	seq   int64
}

func (a *sendAPI) GetBlockedContactIds() ([]string, error) { return nil, nil }
func (a *sendAPI) SendMessage(seq int64, m *line.Message) (*line.Message, error) {
	a.calls++
	a.seq = seq
	return &line.Message{ID: "88"}, nil
}
func TestDirectSendUsesPersistedSequenceAndOneMutation(t *testing.T) {
	account := "u" + strings.Repeat("a", 43)
	peer := "u" + strings.Repeat("b", 43)
	store := &memoryStore{s: &session.State{Version: 1, MID: account, AccessToken: "fictional", NoE2EE: true, LastReqSeq: 41}}
	api := &sendAPI{}
	manager := session.NewManager(store)
	manager.NewClient = func(string) session.API { return api }
	direct := &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
	result, err := direct.Send(context.Background(), peer, "hello", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != "88" || api.calls != 1 || api.seq <= 41 || store.s.LastReqSeq != api.seq {
		t.Fatalf("result=%+v calls=%d seq=%d saved=%d", result, api.calls, api.seq, store.s.LastReqSeq)
	}
}

type contactFailAPI struct {
	session.API
	id string
}

func (a *contactFailAPI) GetProfileContext(context.Context) (*line.Profile, error) {
	return &line.Profile{Mid: a.id, DisplayName: "Me"}, nil
}
func (a *contactFailAPI) GetAllContactIds() ([]string, error) {
	return nil, errors.New("fictional contact timeout")
}
func (a *contactFailAPI) GetMessageBoxes(line.MessageBoxesOptions) (*line.MessageBoxesResponse, error) {
	return &line.MessageBoxesResponse{}, nil
}
func TestDirectSnapshotKeepsChatsWhenContactsFail(t *testing.T) {
	id := "u" + strings.Repeat("f", 43)
	store := &memoryStore{s: &session.State{Version: 1, MID: id, AccessToken: "fictional", NoE2EE: true}}
	api := &contactFailAPI{id: id}
	manager := session.NewManager(store)
	manager.NewClient = func(string) session.API { return api }
	direct := &Direct{Manager: manager, Lock: func() (func(), error) { return func() {}, nil }}
	snapshot, err := direct.Snapshot(context.Background())
	if err != nil || !snapshot.ContactsFailed {
		t.Fatalf("contact failure dropped account: %+v %v", snapshot, err)
	}
}

// Snapshot now verifies blocking before deriving selectable-contact authority.
func (a *contactFailAPI) GetBlockedContactIds() ([]string, error) { return nil, nil }
