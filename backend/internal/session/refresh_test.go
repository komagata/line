package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kongesque/line-cli/pkg/line"
)

func TestRefreshTimingUsesServerIssueTime(t *testing.T) {
	now := time.Unix(2000, 0)
	for _, tc := range []struct {
		duration, issued string
		want             time.Time
	}{
		{"3600", "1000", time.Unix(4570, 0)},
		{"3600", "", time.Unix(5570, 0)},
		{"3600", "invalid", time.Unix(5570, 0)},
		{"10", "1000", time.Unix(1005, 0)},
		{"604800", "1000", time.Unix(605770, 0)},
		{"0", "1000", time.Time{}},
		{"9223372036854775807", "1000", time.Time{}},
	} {
		if got := refreshAt(now, &line.TokenV3IssueResult{DurationUntilRefreshSec: tc.duration, TokenIssueTimeEpochSec: tc.issued}); !got.Equal(tc.want) {
			t.Fatalf("duration=%s issued=%s got=%v want=%v", tc.duration, tc.issued, got, tc.want)
		}
	}
}

func TestLoginPersistsCompleteTokenMetadata(t *testing.T) {
	token := &line.TokenV3IssueResult{AccessToken: "new", RefreshToken: "refresh", DurationUntilRefreshSec: "604800", TokenIssueTimeEpochSec: "900", RefreshApiRetryPolicy: &line.RefreshApiRetryPolicy{InitialDelayInMillis: "100", MaxDelayInMillis: "5000", Multiplier: 2, JitterRate: 0.1}}
	s := &memoryStore{}
	f := &fakeAPI{loginResults: []*line.LoginResult{{TokenV3IssueResult: token, NoE2EE: true}}}
	if _, err := testManager(s, f).Login("synthetic", "synthetic", nil); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Load()
	if err != nil || saved.DurationUntilRefreshSec != "604800" || saved.TokenIssueTimeEpochSec != "900" || saved.RefreshApiRetryPolicy == nil || *saved.RefreshApiRetryPolicy != *token.RefreshApiRetryPolicy || !saved.RefreshAt.Equal(time.Unix(605670, 0)) {
		t.Fatal("complete metadata not saved")
	}
}

func TestRestartAcrossSevenDayBoundary(t *testing.T) {
	for _, offset := range []time.Duration{-31 * time.Second, -30 * time.Second, 0, time.Hour, 24 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			issued := time.Unix(1000, 0)
			s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "original-refresh", DurationUntilRefreshSec: "604800", TokenIssueTimeEpochSec: "1000", RefreshAt: issued.Add(8 * 24 * time.Hour)}}
			f := &fakeAPI{refreshResult: &line.TokenV3IssueResult{AccessToken: "new", RefreshToken: "rotated-refresh", DurationUntilRefreshSec: "604800", TokenIssueTimeEpochSec: "605800"}}
			m := testManager(s, f)
			m.Now = func() time.Time { return issued.Add(168*time.Hour + offset) }
			expected := 1
			if offset < -30*time.Second {
				expected = 0
			}
			var used string
			m.NewClient = func(token string) API { used = token; return f }
			if err := m.Do(func(API) error {
				if expected == 1 && (used != "new" || s.state.RefreshToken != "rotated-refresh" || s.state.TokenIssueTimeEpochSec != "605800") {
					t.Fatal("request before complete durable rotation")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if f.refreshCalls != expected || s.state.Invalidated {
				t.Fatal("normal refresh boundary invalidated session")
			}
			// A fresh manager after a process restart reuses the committed token.
			restarted := testManager(s, f)
			restarted.Now = m.Now
			if err := restarted.Do(func(API) error { return nil }); err != nil || f.refreshCalls != expected {
				t.Fatal("restart lost refresh timing", err)
			}
		})
	}
}

func TestConcurrentCallsRefreshOnlyOnce(t *testing.T) {
	s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "refresh", RefreshAt: time.Unix(1, 0)}}
	f := &fakeAPI{refreshResult: &line.TokenV3IssueResult{AccessToken: "new", RefreshToken: "rotated", DurationUntilRefreshSec: "3600"}}
	m := testManager(s, f)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Go(func() {
			if err := m.Do(func(API) error { return nil }); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if f.refreshCalls != 1 || s.saves != 1 {
		t.Fatal("concurrent calls duplicated refresh")
	}
}

type retryAPI struct {
	*fakeAPI
	attempts []int
	tokens   []string
	fail     int
}

func (f *retryAPI) RefreshAccessTokenContext(ctx context.Context, token string, attempt int) (*line.TokenV3IssueResult, error) {
	f.attempts = append(f.attempts, attempt)
	f.tokens = append(f.tokens, token)
	if len(f.attempts) <= f.fail {
		return nil, &line.ResponseError{Status: 503}
	}
	return &line.TokenV3IssueResult{AccessToken: "new", RefreshToken: "rotated", DurationUntilRefreshSec: "3600"}, nil
}

func TestRefreshBackoffAndRetryCount(t *testing.T) {
	for _, fail := range []int{2, 10} {
		s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "refresh", RefreshAt: time.Unix(1, 0), RefreshApiRetryPolicy: &line.RefreshApiRetryPolicy{InitialDelayInMillis: "100", MaxDelayInMillis: "250", Multiplier: 2, JitterRate: 0}}}
		f := &retryAPI{fakeAPI: &fakeAPI{}, fail: fail}
		m := NewManager(s)
		m.NewClient = func(string) API { return f }
		var delays []time.Duration
		m.Wait = func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }
		calls := 0
		err := m.Do(func(API) error { calls++; return nil })
		want := min(fail+1, 4)
		if len(f.attempts) != want || len(delays) != want-1 {
			t.Fatal("unbounded attempts")
		}
		for i, attempt := range f.attempts {
			if attempt != i || f.tokens[i] != "refresh" {
				t.Fatal("incorrect retry request")
			}
		}
		for i, d := range delays {
			if d != time.Duration(min(100*(1<<i), 250))*time.Millisecond {
				t.Fatal("policy ignored", d)
			}
		}
		if fail < 4 {
			if err != nil || calls != 1 || s.state.RefreshToken != "rotated" {
				t.Fatal("did not recover", err)
			}
		} else if err == nil || calls != 0 || s.state.Invalidated || s.saves != 0 {
			t.Fatal("failed refresh altered session")
		}
	}
}

func TestRefreshBackoffCancellationAndBounds(t *testing.T) {
	p := &line.RefreshApiRetryPolicy{InitialDelayInMillis: "1000", MaxDelayInMillis: "5000", Multiplier: 2, JitterRate: 0.5}
	if refreshDelay(p, 0, 0) != 500*time.Millisecond || refreshDelay(p, 0, 1) != 1500*time.Millisecond || refreshDelay(p, 10, 1) != 5*time.Second {
		t.Fatal("jitter/cap incorrect")
	}
	p.InitialDelayInMillis = "9223372036854775807"
	p.MaxDelayInMillis = "9223372036854775807"
	p.Multiplier = 1e100
	if d := refreshDelay(p, 100, 1); d > 30*time.Second || d <= 0 {
		t.Fatal("unbounded delay")
	}
	s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "refresh", RefreshAt: time.Unix(1, 0)}}
	f := &retryAPI{fakeAPI: &fakeAPI{}, fail: 10}
	m := NewManager(s)
	m.NewClient = func(string) API { return f }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Wait = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	err := m.DoContext(ctx, func(API) error { t.Fatal("request after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) || len(f.attempts) != 1 || s.saves != 0 {
		t.Fatal("cancelled refresh retried", err)
	}
}

func TestRefreshForcedLogoutAndMalformedResponse(t *testing.T) {
	for _, cause := range []error{errors.New("V3_TOKEN_CLIENT_LOGGED_OUT"), nil} {
		s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "refresh", RefreshAt: time.Unix(1, 0)}}
		f := &fakeAPI{refreshErr: cause}
		err := testManager(s, f).Do(func(API) error { t.Fatal("request after failed refresh"); return nil })
		if err == nil || f.refreshCalls != 1 || s.state.Invalidated != (cause != nil) {
			t.Fatal("incorrect failed refresh classification")
		}
	}
}

func TestStaleClientCannotInvalidateOrRefreshNewCredentials(t *testing.T) {
	s := &memoryStore{state: &State{Generation: "same-login", AccessToken: "old", RefreshToken: "refresh"}}
	f := &fakeAPI{}
	m := testManager(s, f)
	calls := 0
	if err := m.Do(func(API) error {
		calls++
		if calls == 1 {
			s.state.AccessToken = "new"
			s.state.RefreshToken = "new-refresh"
			return errors.New("V3_TOKEN_CLIENT_LOGGED_OUT")
		}
		return nil
	}); err != nil || calls != 2 || f.refreshCalls != 0 || s.state.Invalidated {
		t.Fatal("stale response corrupted current session", err)
	}
	if err := m.RecoverStream(context.Background(), "same-login", "old", errors.New(`{"code":119}`)); err != nil || f.refreshCalls != 0 {
		t.Fatal("stale stream triggered refresh", err)
	}
	if err := m.RecoverStream(context.Background(), "old-login", "new", errors.New("V3_TOKEN_CLIENT_LOGGED_OUT")); !errors.Is(err, ErrSessionChanged) || s.state.Invalidated {
		t.Fatal("different login invalidated")
	}
}

func TestRefreshCommitsNewPolicyAndPreservesOmittedRefreshToken(t *testing.T) {
	s := &memoryStore{state: &State{Generation: "login", AccessToken: "old", RefreshToken: "keep-refresh", RefreshAt: time.Unix(1, 0), LastReqSeq: 123, RefreshApiRetryPolicy: &line.RefreshApiRetryPolicy{InitialDelayInMillis: "1"}}}
	policy := &line.RefreshApiRetryPolicy{InitialDelayInMillis: "500", MaxDelayInMillis: "5000", Multiplier: 3, JitterRate: 0.1}
	f := &fakeAPI{refreshResult: &line.TokenV3IssueResult{AccessToken: "new", DurationUntilRefreshSec: "604800", TokenIssueTimeEpochSec: "1000", RefreshApiRetryPolicy: policy}}
	m := testManager(s, f)
	if err := m.Do(func(API) error {
		saved, err := s.Load()
		if err != nil || saved.RefreshToken != "keep-refresh" || saved.Generation != "login" || saved.LastReqSeq != 123 || saved.RefreshApiRetryPolicy == nil || *saved.RefreshApiRetryPolicy != *policy || saved.TokenIssueTimeEpochSec != "1000" || saved.DurationUntilRefreshSec != "604800" {
			t.Fatal("refresh commit lost state")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestReactiveRefreshCommitFailureNeverPublishesOrReplays(t *testing.T) {
	for _, failure := range []error{errors.New("storage failed"), ErrDurabilityUncertain} {
		s := &memoryStore{state: &State{AccessToken: "old", RefreshToken: "refresh"}, saveErr: failure}
		f := &fakeAPI{refreshResult: &line.TokenV3IssueResult{AccessToken: "new", RefreshToken: "rotated"}}
		m := testManager(s, f)
		m.NewClient = func(token string) API {
			if token != "old" {
				t.Fatal("uncommitted client published")
			}
			return f
		}
		calls := 0
		err := m.Do(func(API) error { calls++; return errors.New(`{"code":119}`) })
		if !errors.Is(err, failure) || calls != 1 || f.refreshCalls != 1 || s.state.AccessToken != "old" {
			t.Fatal("storage failure retried refresh/read", err)
		}
	}
}
