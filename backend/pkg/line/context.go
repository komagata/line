package line

import (
	"context"
	"io"
	"net/http"
)

// WithContext returns a fresh operation-local client. No live client's mutex,
// cache, HTTP client or transport is modified. Both RPC and OBS legacy methods
// inherit cancellation, including while the response body is being consumed.
func (c *Client) WithContext(ctx context.Context) *Client {
	bind := func(client *http.Client) *http.Client {
		copy := http.Client{}
		if client != nil {
			copy = *client
		}
		base := copy.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		copy.Transport = contextTransport{parent: ctx, base: base}
		return &copy
	}
	return &Client{AccessToken: c.AccessToken, HTTPClient: bind(c.HTTPClient), OBSClient: bind(c.obsHTTPClient())}
}

type contextTransport struct {
	parent context.Context
	base   http.RoundTripper
}

func (t contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.parent.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.parent, cancel)
	cleanup := func() { stop(); cancel() }
	if err := t.parent.Err(); err != nil {
		cleanup()
		return nil, err
	}
	response, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		cleanup()
		return response, err
	}
	response.Body = &contextBody{ReadCloser: response.Body, cleanup: cleanup}
	return response, nil
}

type contextBody struct {
	io.ReadCloser
	cleanup func()
}

func (b *contextBody) Close() error { defer b.cleanup(); return b.ReadCloser.Close() }
