package session

import "context"

// Context is local to this facade. Resolving on each call preserves the storage
// identity, encrypted format, permission checks and locked-session behavior.
func (s KeychainStore) WithContext(ctx context.Context) Store {
	return contextStore{KeychainStore: s, ctx: ctx}
}

type contextStore struct {
	KeychainStore
	ctx context.Context
}

func (s contextStore) resolve() (linuxStorage, error) {
	if err := s.ctx.Err(); err != nil {
		return linuxStorage{}, err
	}
	store, err := resolvedLinuxStorage()
	store.context = s.ctx
	store.native.secrets.run = func(args []string, input []byte) secretResult {
		return runSecretToolContext(s.ctx, "/usr/bin/secret-tool", args, input)
	}
	return store, err
}
func (s contextStore) Load() (*State, error) {
	store, err := s.resolve()
	if err != nil {
		return nil, err
	}
	return store.Load()
}
func (s contextStore) Save(state *State) error {
	store, err := s.resolve()
	if err != nil {
		return err
	}
	return store.Save(state)
}
func (s contextStore) Delete() error {
	store, err := s.resolve()
	if err != nil {
		return err
	}
	return store.Delete()
}
func (s contextStore) Prepare() error {
	store, err := s.resolve()
	if err != nil {
		return err
	}
	return store.Prepare()
}
func (s contextStore) StorageIdentity() (string, error) {
	store, err := s.resolve()
	if err != nil {
		return "", err
	}
	return store.StorageIdentity()
}
