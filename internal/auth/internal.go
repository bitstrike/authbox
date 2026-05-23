package auth

import (
	"crypto/subtle"
	"net/http"
)

const internalTokenHeader = "X-Internal-Token"

// InternalMiddleware authenticates internal (container-to-container) requests
// using a shared secret token.
func InternalMiddleware(sharedSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get(internalTokenHeader)
			if token == "" {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"missing internal token"}}`, http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(sharedSecret)) != 1 {
				http.Error(w, `{"error":{"code":"UNAUTHORIZED","message":"invalid internal token"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
