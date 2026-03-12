package auth

import (
	"context"
	"fmt"
	"net/http"
)

// Middleware returns a chi middleware that enforces authentication.
// If auth is nil, all requests pass (insecure mode).
func Middleware(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth == nil {
				next.ServeHTTP(w, r)
				return
			}
			deviceID, err := auth.AuthenticateHTTP(r)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":"unauthorized","hint":"provide Authorization: Bearer <token> header (token file: $OZZIE_PATH/.local_token)"}`)
				return
			}
			ctx := context.WithValue(r.Context(), deviceIDKey, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
