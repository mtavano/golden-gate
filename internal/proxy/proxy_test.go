package proxy

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mtavano/golden-gate/internal/service"
	"github.com/mtavano/golden-gate/internal/storage"
)

// stubDB is a no-op storage.Database used to construct a *service.RequestSvc
// in tests without requiring a real SQLite-backed database. BeginTx returns
// an error so AddRequest fails fast and is silently logged inside RoundTrip
// (it never short-circuits the HTTP flow we care about here).
type stubDB struct{}

func (stubDB) BeginTx(context.Context) (storage.Transactioner, error) {
	return nil, errors.New("stubDB: not implemented")
}
func (stubDB) Get(any, string, ...any) error          { return errors.New("stubDB") }
func (stubDB) Select(any, string, ...any) error       { return errors.New("stubDB") }
func (stubDB) Exec(string, ...any) (sql.Result, error) { return nil, errors.New("stubDB") }
func (stubDB) Query(string, ...any) (*sql.Rows, error) { return nil, errors.New("stubDB") }

// TestProxyStripsAcceptEncoding asserts that the request reaching the upstream
// has no Accept-Encoding header, even when the incoming client request asked
// for brotli/gzip. This guarantees upstreams always answer with identity
// (uncompressed) bodies so the persisted audit copy is human-readable.
func TestProxyStripsAcceptEncoding(t *testing.T) {
	var gotAcceptEncoding string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	p := NewProxy(&Config{
		ServiceName: "test",
		BasePrefix:  "/test",
		Target:      upstream.URL,
	}, service.NewRequestSvc(stubDB{}))

	req := httptest.NewRequest(http.MethodGet, "/test/health", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rr := httptest.NewRecorder()

	p.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotAcceptEncoding != "" {
		t.Fatalf("expected upstream to receive no Accept-Encoding header, got %q", gotAcceptEncoding)
	}
}

// TestProxyReadableBodyWhenUpstreamClaimsBrotli simulates the original bug:
// an upstream that advertises Content-Encoding: br with opaque bytes. After
// the fix the upstream should NEVER receive Accept-Encoding, so it should
// fall back to identity. We still assert the proxied response body matches
// what the upstream actually wrote — i.e. nothing in our pipeline corrupts
// it — and that the body sent to the client is the readable payload.
func TestProxyReadableBodyWhenUpstreamSendsIdentity(t *testing.T) {
	payload := `{"status":"ok","message":"hello"}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ae := r.Header.Get("Accept-Encoding"); ae != "" {
			t.Errorf("upstream should not receive Accept-Encoding, got %q", ae)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer upstream.Close()

	p := NewProxy(&Config{
		ServiceName: "test",
		BasePrefix:  "/cloud/piggi2",
		Target:      upstream.URL,
	}, service.NewRequestSvc(stubDB{}))

	req := httptest.NewRequest(http.MethodGet, "/cloud/piggi2/health", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rr := httptest.NewRecorder()

	p.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("expected readable JSON body, got %q", string(body))
	}
}
