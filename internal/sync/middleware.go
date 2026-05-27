// middleware.go provides HTTP middleware that rejects mutating requests on
// replica containers. GET/HEAD/OPTIONS pass through; all other methods return
// 503 with a REPLICA_READ_ONLY error directing clients to the primary.
package sync

import (
	"net/http"
)

// RejectWrites returns middleware that blocks mutating requests on replicas.
// Read-only methods (GET, HEAD, OPTIONS) pass through.
func RejectWrites() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":{"code":"REPLICA_READ_ONLY","message":"this replica does not accept writes; send to primary"}}`))
			}
		})
	}
}
