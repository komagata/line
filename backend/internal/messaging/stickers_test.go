package messaging

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kongesque/line-cli/internal/session"
	"github.com/kongesque/line-cli/pkg/line"
)

type stickerAPI struct {
	*fakeAPI
	products []line.StickerProduct
	reads    int
}

func (f *stickerAPI) GetProfile() (*line.Profile, error) { return &line.Profile{RegionCode: "JP"}, nil }
func (f *stickerAPI) OwnedStickerProducts(offset int, country string) (*line.StickerProductPage, error) {
	f.reads++
	return &line.StickerProductPage{Products: f.products, Offset: offset, Total: len(f.products)}, nil
}
func pack() line.StickerProduct {
	var p line.StickerProduct
	_ = json.Unmarshal([]byte(`{"id":"1","name":"Fixture","latestVersion":2,"validUntil":-1,"productTypeSummary":{"stickerSummary":{"stickerResourceType":4,"stickerIdRanges":[{"start":"9007199254740993","size":2}],"stickerHash":"abcd"}}}`), &p)
	return p
}
func stickerSetup(t *testing.T) (*Client, *stickerAPI, *memStore) {
	c, f, _, s := setup(t, false)
	a := &stickerAPI{fakeAPI: f, products: []line.StickerProduct{pack()}}
	c.Session.NewClient = func(string) session.API { return a }
	return c, a, s
}
func TestStickerDecodeProjection(t *testing.T) {
	c, _, _ := stickerSetup(t)
	raw := &line.Message{ID: "1", ContentType: 7, ContentMetadata: map[string]string{"STKID": "123", "STKPKGID": "1", "STKVER": "2", "STKOPT": "AS", "STKHASH": "abcd", "STKTXT": "hi\x00there", "secret": "private"}}
	item := c.Decode("u-peer", raw)
	if item.Sticker == nil || item.Sticker.ID != "123" || item.Sticker.Alt != "hithere" {
		t.Fatalf("missing projection: %+v", item)
	}
	data, _ := json.Marshal(item)
	if strings.Contains(string(data), "private") {
		t.Fatal("raw metadata leaked")
	}
	for _, key := range []string{"STKID", "STKPKGID", "STKVER", "STKOPT", "STKHASH"} {
		old := raw.ContentMetadata[key]
		raw.ContentMetadata[key] = "../evil"
		if c.Decode("u-peer", raw).Sticker != nil {
			t.Fatal("unsafe", key)
		}
		raw.ContentMetadata[key] = old
	}
	delete(raw.ContentMetadata, "STKPKGID")
	if c.Decode("u-peer", raw).Sticker == nil {
		t.Fatal("receive-only sticker missing package")
	}
	raw.Chunks = []string{"encrypted"}
	if c.Decode("u-peer", raw).Sticker != nil {
		t.Fatal("encrypted unsupported metadata rendered")
	}
}
func TestStickerSendOwnedOnce(t *testing.T) {
	c, f, s := stickerSetup(t)
	result, err := c.SendSticker("u-peer", "1", "9007199254740994", "42")
	if err != nil || result.Encrypted || f.sends != 1 || f.reads != 1 || s.state.LastReqSeq != f.sequence {
		t.Fatalf("send: %+v %v", result, err)
	}
	m := f.sent
	if m.ContentType != 7 || !m.HasContent || len(m.Chunks) != 0 || m.Text != "" || m.RelatedMessageID != "42" || m.ContentMetadata["STKOPT"] != "AS" || m.ContentMetadata["STKVER"] != "2" || m.ContentMetadata["STKHASH"] != "abcd" {
		t.Fatalf("wrong sticker: %+v", m)
	}
	f.sendErr = errors.New("timeout secret")
	if _, err = c.SendSticker("u-peer", "1", "9007199254740994", ""); err == nil || f.sends != 2 || !strings.Contains(err.Error(), "may have occurred") {
		t.Fatal("retry/uncertainty violation")
	}
}
func TestStickerSendRejectsUnownedExpiredUnsupportedBlockedAndPersistFailure(t *testing.T) {
	for _, kind := range []string{"unowned", "range", "expired", "dynamic", "blocked", "storage", "bad-range", "overflow"} {
		t.Run(kind, func(t *testing.T) {
			c, f, s := stickerSetup(t)
			id := "9007199254740993"
			p := &f.products[0]
			switch kind {
			case "unowned":
				f.products = nil
			case "range":
				id = "9007199254740995"
			case "expired":
				p.ValidUntil = "12345"
			case "dynamic":
				p.Summary.Sticker.ResourceType = 7
			case "blocked":
				f.blocked = []string{"u-peer"}
			case "storage":
				s.saveErr = errors.New("no save")
			case "bad-range":
				p.Summary.Sticker.Ranges[0].Size = "0"
			case "overflow":
				p.Summary.Sticker.Ranges[0].Start = "18446744073709551615"
			}
			if _, err := c.SendSticker("u-peer", "1", id, ""); err == nil || f.sends != 0 {
				t.Fatal("invalid send")
			}
		})
	}
}

func TestStickerHistoryUsesProjection(t *testing.T) {
	c, f, _ := stickerSetup(t)
	f.history = []*line.Message{{ID: "1", ContentType: 7, ContentMetadata: map[string]string{"STKID": "10", "STKPKGID": "1"}}}
	rows, err := c.History("u-peer", 10)
	if err != nil || len(rows) != 1 || rows[0].Sticker == nil {
		t.Fatal("history lost sticker", err)
	}
}
func TestAllStandardStickerOptionsAndAuthMutationNotReplayed(t *testing.T) {
	for kind, opt := range []string{"", "A", "S", "AS", "P", "PS"} {
		c, f, _ := stickerSetup(t)
		f.products[0].Summary.Sticker.ResourceType = kind + 1
		if _, err := c.SendSticker("u-peer", "1", "9007199254740993", ""); err != nil || f.sent.ContentMetadata["STKOPT"] != opt {
			t.Fatal("wrong standard option", kind, err)
		}
	}
	c, f, s := stickerSetup(t)
	f.sendErr = errors.New("REQUEST_NEED_LOGIN private")
	_, err := c.SendSticker("u-peer", "1", "9007199254740993", "")
	if err == nil || f.sends != 1 || !s.state.Invalidated || strings.Contains(err.Error(), "private") {
		t.Fatal("auth mutation retried or leaked", err)
	}
}
func TestStickerCatalogValidity(t *testing.T) {
	for _, kind := range []string{"overlap", "duplicate", "limit", "zero-version", "invalid-hash", "missing-range", "custom"} {
		t.Run(kind, func(t *testing.T) {
			c, f, _ := stickerSetup(t)
			p := &f.products[0]
			switch kind {
			case "overlap":
				p.Summary.Sticker.Ranges = append(p.Summary.Sticker.Ranges, p.Summary.Sticker.Ranges[0])
			case "duplicate":
				f.products = append(f.products, pack())
			case "limit":
				f.products = make([]line.StickerProduct, 501)
			case "zero-version":
				p.Version = "0"
			case "invalid-hash":
				p.Summary.Sticker.Hash = "../secret"
			case "missing-range":
				p.Summary.Sticker.Ranges = nil
			case "custom":
				p.Summary.Sticker.ResourceType = 8
			}
			products, err := c.OwnedStickers()
			if kind == "custom" {
				if err != nil || products[0].Supported {
					t.Fatal("custom pack sendable")
				}
			} else if err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

type stickerHTTP func(*http.Request) (*http.Response, error)

func (f stickerHTTP) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestStickerRealHTTPSerializationAndOneMutation(t *testing.T) {
	c, _, store := stickerSetup(t)
	requests := []string{}
	writes := 0
	api := line.NewClient("fictional-token")
	api.HTTPClient.Transport = stickerHTTP(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.Path)
		body, _ := io.ReadAll(req.Body)
		response := ""
		switch {
		case strings.HasSuffix(req.URL.Path, "/getProfile"):
			response = `{"code":0,"data":{"mid":"u-self","regionCode":"JP"}}`
		case strings.HasSuffix(req.URL.Path, "/getOwnedProductSummaries"):
			if string(body) != `["stickershop",0,1000,{"country":"JP","language":"ja"}]` {
				t.Fatal("shop request", string(body))
			}
			response = `{"code":0,"data":{"offset":0,"totalSize":1,"productList":[{"id":"1","name":"Fixture","latestVersion":1,"validUntil":-1,"productTypeSummary":{"stickerSummary":{"stickerResourceType":6,"stickerIdRanges":[{"start":10,"size":2}],"stickerHash":"abcd"}}}]}}`
		case strings.HasSuffix(req.URL.Path, "/getBlockedContactIds"):
			response = `{"code":0,"data":[]}`
		case strings.HasSuffix(req.URL.Path, "/sendMessage"):
			writes++
			var args []json.RawMessage
			if json.Unmarshal(body, &args) != nil || len(args) != 2 {
				t.Fatal("send argument shape")
			}
			var seq int64
			_ = json.Unmarshal(args[0], &seq)
			var msg line.Message
			_ = json.Unmarshal(args[1], &msg)
			if seq != store.state.LastReqSeq || msg.ContentType != 7 || !msg.HasContent || msg.To != "u-peer" || msg.Text != "" || len(msg.Chunks) != 0 || msg.ContentMetadata["STKOPT"] != "PS" || msg.ContentMetadata["STKID"] != "10" || msg.ContentMetadata["STKPKGID"] != "1" || msg.RelatedMessageID != "42" {
				t.Fatal("wire sticker contract")
			}
			response = `{"code":0,"data":{"id":"123"}}`
		default:
			t.Fatal("unexpected API", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
	})
	c.Session.NewClient = func(string) session.API { return api }
	sent, err := c.SendSticker("u-peer", "1", "10", "42")
	if err != nil || writes != 1 || sent.Encrypted || len(requests) != 4 {
		t.Fatal("send transaction", err, requests)
	}
}

type expiringStickerAPI struct {
	*stickerAPI
	beforeBlocked func()
}

func (f *expiringStickerAPI) GetBlockedContactIds() ([]string, error) {
	f.beforeBlocked()
	return nil, nil
}
func TestStickerExpiryRecheckedAfterBlockedContactRead(t *testing.T) {
	c, f, _ := stickerSetup(t)
	f.products[0].ValidUntil = "20000"
	api := &expiringStickerAPI{stickerAPI: f, beforeBlocked: func() { c.Session.Now = func() time.Time { return time.UnixMilli(20001) } }}
	c.Session.NewClient = func(string) session.API { return api }
	if _, err := c.SendSticker("u-peer", "1", "9007199254740993", ""); err == nil || f.sends != 0 {
		t.Fatal("sticker expired during read was sent")
	}
}
