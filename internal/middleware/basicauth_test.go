package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testUser  = "admin"
	testPass  = "supersecurepassword"
	testRealm = "Golden Gate Editor"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func doRequest(t *testing.T, h http.Handler, user, pass string, sendAuth bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if sendAuth {
		req.SetBasicAuth(user, pass)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	h := BasicAuth(okHandler(), testUser, testPass, testRealm)
	rr := doRequest(t, h, testUser, testPass, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", body)
	}
}

func TestBasicAuth_WrongPassword(t *testing.T) {
	h := BasicAuth(okHandler(), testUser, testPass, testRealm)
	rr := doRequest(t, h, testUser, "wrong-password", true)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Basic realm=") {
		t.Fatalf("expected WWW-Authenticate Basic challenge, got %q", got)
	}
}

func TestBasicAuth_WrongUser(t *testing.T) {
	h := BasicAuth(okHandler(), testUser, testPass, testRealm)
	rr := doRequest(t, h, "not-admin", testPass, true)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestBasicAuth_EmptyConfig(t *testing.T) {
	cases := []struct {
		name string
		user string
		pass string
	}{
		{"both empty", "", ""},
		{"empty user", "", testPass},
		{"empty pass", testUser, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := BasicAuth(okHandler(), tc.user, tc.pass, testRealm)
			rr := doRequest(t, h, testUser, testPass, true)

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d", rr.Code)
			}
			if !strings.Contains(rr.Body.String(), "Editor disabled") {
				t.Fatalf("expected disabled message in body, got %q", rr.Body.String())
			}
		})
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	h := BasicAuth(okHandler(), testUser, testPass, testRealm)
	rr := doRequest(t, h, "", "", false)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	got := rr.Header().Get("WWW-Authenticate")
	if got == "" {
		t.Fatal("expected WWW-Authenticate header to be set")
	}
	if !strings.HasPrefix(got, "Basic realm=") {
		t.Fatalf("expected Basic realm challenge, got %q", got)
	}
	if !strings.Contains(got, testRealm) {
		t.Fatalf("expected realm %q in challenge, got %q", testRealm, got)
	}
	if !strings.Contains(got, `charset="UTF-8"`) {
		t.Fatalf("expected charset=\"UTF-8\" in challenge, got %q", got)
	}
}
