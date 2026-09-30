package line

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type stickerRoundTrip func(*http.Request) (*http.Response, error)

func (f stickerRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOwnedStickerContract(t *testing.T) {
	c := NewClient("fictional-token")
	calls := 0
	c.HTTPClient.Transport = stickerRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.String() != ShopBaseURL+"/ShopService/getOwnedProductSummaries" || string(b) != `["stickershop",0,1000,{"country":"JP","language":"ja"}]` {
			t.Fatalf("unexpected request: %s %s %s", r.Method, r.URL, b)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"offset":0,"totalSize":1,"productList":[{"id":"1","name":"Fixture","latestVersion":1,"validUntil":-1,"productTypeSummary":{"stickerSummary":{"stickerResourceType":1,"stickerIdRanges":[{"start":100,"size":2}]}}}]}}`)), Header: make(http.Header)}, nil
	})
	page, err := c.OwnedStickerProducts(0, "JP")
	if err != nil || len(page.Products) != 1 {
		t.Fatalf("catalog: %v %v", page, err)
	}
	if _, err = c.OwnedStickerProducts(0, "JP/evil"); err == nil || calls != 1 {
		t.Fatal("invalid country reached transport")
	}
	for _, body := range []string{`{}`, `{"code":1,"message":"private"}`, `{"code":0,"data":{}}`, strings.Repeat("x", 2*1024*1024+1)} {
		c.HTTPClient.Transport = stickerRoundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, err = c.OwnedStickerProducts(0, "JP"); err == nil || strings.Contains(err.Error(), "private") {
			t.Fatal("malformed wrapper was accepted or leaked")
		}
	}
}
