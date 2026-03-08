package auth

import (
	"context"
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
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), deviceIDKey, deviceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
