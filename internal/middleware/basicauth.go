// Package middleware provides small, dependency-free HTTP middlewares
// used across Golden Gate's internal HTTP surfaces (e.g. the dashboard
// editor). Each middleware is a plain net/http.Handler wrapper.
package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

// BasicAuth wraps next with HTTP Basic Auth. Realm is shown in the browser
// prompt. user/pass are compared in constant time. If user OR pass is empty,
// the middleware responds 503 to every request — we refuse to serve an
// "auth-protected" endpoint with no real credentials.
func BasicAuth(next http.Handler, user, pass, realm string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Refuse to operate without credentials configured. Returning 503
		// makes the misconfiguration obvious instead of silently allowing
		// (or worse, silently denying) traffic.
		if user == "" || pass == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.Error(w, "Editor disabled: credentials not configured", http.StatusServiceUnavailable)
			return
		}

		gotUser, gotPass, ok := r.BasicAuth()
		if !ok {
			unauthorized(w, realm)
			return
		}

		// Compare lengths first to short-circuit obvious mismatches; subtle
		// still returns 0 on length mismatch but being explicit keeps intent
		// clear and avoids any chance of slice surprises.
		userOK := len(gotUser) == len(user) &&
			subtle.ConstantTimeCompare([]byte(gotUser), []byte(user)) == 1
		passOK := len(gotPass) == len(pass) &&
			subtle.ConstantTimeCompare([]byte(gotPass), []byte(pass)) == 1

		if !userOK || !passOK {
			unauthorized(w, realm)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
