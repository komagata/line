package gui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/kongesque/line-cli/internal/messaging"
	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
)

type repairMetadataAPI struct {
	*incompleteContactAPI
	blockedErr, detailErr error
}

func (a *repairMetadataAPI) GetBlockedContactIds() ([]string, error) { return nil, a.blockedErr }
func (a *repairMetadataAPI) GetContactsV2(ids []string) (*line.ContactsResponse, error) {
	if a.detailErr != nil {
		return nil, a.detailErr
	}
	return a.incompleteContactAPI.GetContactsV2(ids)
}
func TestRepairMetadataFailureAuthority(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	for _, auth := range []bool{false, true} {
		cause := errors.New("fixture unavailable")
		if auth {
			cause = session.ErrSessionInvalidated
		}
		api := &repairMetadataAPI{incompleteContactAPI: &incompleteContactAPI{self: self, peer: peer, includeDetails: true}, blockedErr: cause}
		d := auditDirect(api, self)
		v, err := d.Snapshot(context.Background())
		if auth {
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("auth lost: %v", err)
			}
		} else if err != nil || !v.ContactsFailed || len(v.Contacts) != 0 {
			t.Fatalf("failed blocked lookup granted authority: %+v %v", v, err)
		}
		api.blockedErr = nil
		api.detailErr = cause
		_, err = d.Profiles(context.Background(), []string{peer})
		if auth && !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("profile auth lost: %v", err)
		}
		if !auth && (err == nil || errors.Is(err, ErrUnauthenticated)) {
			t.Fatalf("ordinary metadata failure classified as logout: %v", err)
		}
	}
}
func TestRepairPreflightAuthRevokesSession(t *testing.T) {
	self, peer := "u"+strings.Repeat("a", 43), "u"+strings.Repeat("b", 43)
	api := &repairMetadataAPI{incompleteContactAPI: &incompleteContactAPI{self: self, peer: peer}, blockedErr: session.ErrSessionInvalidated}
	_, err := auditDirect(api, self).Send(context.Background(), peer, "fictional", "", nil)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("blocked preflight auth classification lost: %v", err)
	}
}
func TestRepairProfileAuthClearsIdentityAfterSelectionChange(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	e, id := readyEngine(Operations{Profiles: func(context.Context, []string) (map[string]ProfileInfo, error) {
		close(entered)
		<-release
		return nil, ErrUnauthenticated
	}})
	defer e.Close()
	e.mu.Lock()
	e.state.Messages = []Message{{ID: "1", SenderID: "u" + strings.Repeat("e", 43)}}
	e.drafts[id] = "private"
	e.state.Draft = "private"
	e.state.Avatars = map[string]string{id: "private"}
	e.queueProfiles()
	e.mu.Unlock()
	<-entered
	e.mu.Lock()
	e.state.Selection++
	e.mu.Unlock()
	close(release)
	e.Wait()
	v := e.View()
	if v.Status != "unauthenticated" || v.SelectedID != "" || v.Account.ID != "" || v.Draft != "" || len(v.Messages) != 0 || len(v.Avatars) != 0 {
		t.Fatal("profile logout retained identity/editor/images")
	}
}
func TestRepairLabelUTF16Limit(t *testing.T) {
	if got := label("一\n二\r三\t四", 7); got != "一 二 三 四" {
		t.Fatalf("label separators %q", got)
	}
	if got := cleanText("😀😀x", 4); got != "😀😀" {
		t.Fatalf("UTF16 limit %q", got)
	}
}

func TestRepairCatalogAndDownloadAuthRevokeIdentity(t *testing.T) {
	for _, action := range []string{"sticker-open", "preview"} {
		t.Run(action, func(t *testing.T) {
			e, id := readyEngine(Operations{Catalog: func(context.Context) ([]OwnedProduct, error) { return nil, ErrUnauthenticated }, Download: func(context.Context, string, string) ([]byte, error) { return nil, ErrUnauthenticated }})
			defer e.Close()
			e.mu.Lock()
			e.state.Messages = []Message{{ID: "1", ContentType: 1, Downloadable: true}}
			e.mu.Unlock()
			v := e.View()
			e.Handle(Command{Action: action, ID: id, Session: v.Session, Selection: v.Selection, MessageID: "1"})
			e.Wait()
			if e.View().Status != "unauthenticated" {
				t.Fatal("metadata auth loss did not revoke identity")
			}
		})
	}
}

func TestRepairWatchEnrichesEarlierReceipt(t *testing.T) {
	e, id := readyEngine(Operations{})
	defer e.Close()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state.Messages = nil
	e.sentMessage(id, Message{ID: "88", Text: "receipt", Timestamp: 1})
	e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "88", Text: "server\nbody", Timestamp: 2, Time: "12:34", Status: "ok"}})
	if len(e.state.Messages) != 1 || e.state.Messages[0].Text != "server\nbody" || e.state.Chats[0].Time != "12:34" {
		t.Fatal("watch echo did not enrich receipt")
	}
}
func TestRepairBodyUsesOriginalUTF16Bound(t *testing.T) {
	got := projectMessage(messaging.Message{ID: "1", Text: strings.Repeat("あ", 10001)}, Account{}, nil)
	if len([]rune(got.Text)) != 10000 {
		t.Fatal("body limit differs from original 10000 UTF16 units")
	}
}
func TestRepairMutationDrainsAcrossModeReset(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	e, id := readyEngine(Operations{Send: func(context.Context, string, string, string, *Attachment) (SendResult, error) {
		close(entered)
		<-release
		return SendResult{}, context.Canceled
	}})
	v := e.View()
	e.Handle(Command{Action: "draft", ID: id, Session: v.Session, Selection: v.Selection, Text: "first"})
	e.Handle(Command{Action: "send", ID: id, Session: v.Session, Selection: v.Selection, Text: "first"})
	<-entered
	e.Handle(Command{Action: "mode", Mode: "demo"})
	v = e.View()
	e.mu.Lock()
	available := e.available(Command{ID: v.SelectedID, Session: v.Session, Selection: v.Selection})
	e.mu.Unlock()
	close(release)
	e.Wait()
	e.Close()
	if available {
		t.Fatal("new mutation allowed while previous identity mutation was still draining")
	}
}

func TestRepairStickerEchoBoundAndSelection(t *testing.T) {
	for _, otherChat := range []bool{false, true} {
		t.Run(fmt.Sprint(otherChat), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			e, id := readyEngine(Operations{SendSticker: func(context.Context, string, string, string, string) (SendResult, error) {
				close(entered)
				<-release
				return SendResult{ID: "101", ChatID: "u" + strings.Repeat("c", 43)}, nil
			}})
			defer e.Close()
			e.mu.Lock()
			e.state.Messages = nil
			for n := 1; n <= 100; n++ {
				e.state.Messages = append(e.state.Messages, Message{ID: strconv.Itoa(n)})
			}
			e.products = []OwnedProduct{{ID: "1", Supported: true, ValidUntil: "-1", Ranges: []OwnedRange{{Start: "1001", Size: 1}}}}
			e.state.Stickers.Selected = &Sticker{ID: "1001", PackageID: "1"}
			e.mu.Unlock()
			v := e.View()
			e.Handle(Command{Action: "sticker-send", ID: id, Session: v.Session, Selection: v.Selection, PackageID: "1", StickerID: "1001"})
			<-entered
			e.mu.Lock()
			if otherChat {
				e.state.SelectedID = "u" + strings.Repeat("f", 43)
				e.state.Messages = []Message{{ID: "999"}}
			} else {
				e.watchEvent(WatchEvent{Kind: "message", Revision: "1", ChatID: id, Message: &Message{ID: "101", Text: "server sticker", ContentType: 7, Timestamp: 1}})
			}
			e.mu.Unlock()
			close(release)
			e.Wait()
			v = e.View()
			if otherChat {
				if len(v.Messages) != 1 || v.Messages[0].ID != "999" {
					t.Fatal("receipt entered another chat")
				}
			} else {
				count := 0
				for _, m := range v.Messages {
					if m.ID == "101" {
						count++
						if m.Text != "server sticker" {
							t.Fatal("receipt replaced watch content")
						}
					}
				}
				if len(v.Messages) != 100 || count != 1 {
					t.Fatal("sticker echo duplication or bound failure")
				}
			}
		})
	}
}

func TestRepairRefreshDoesNotCarryPreviewAcrossAccounts(t *testing.T) {
	id := "u" + strings.Repeat("c", 43)
	e, _ := readyEngine(Operations{Snapshot: func(context.Context) (Snapshot, error) {
		return Snapshot{Account: Account{ID: "u" + strings.Repeat("f", 43)}, Chats: []Chat{{ID: id, Preview: "トークを開く"}}}, nil
	}})
	defer e.Close()
	e.mu.Lock()
	e.state.Chats[0].Preview = "previous account secret"
	e.state.SelectedID = ""
	e.mu.Unlock()
	e.Handle(Command{Action: "refresh"})
	e.Wait()
	if e.View().Chats[0].Preview != "トークを開く" {
		t.Fatal("preview leaked across account identity")
	}
}
