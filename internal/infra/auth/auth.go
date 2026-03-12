// Package auth provides authentication for the Ozzie gateway.
package auth

import (
	"errors"
	"net/http"
)

// ErrUnauthorized is returned when authentication fails.
var ErrUnauthorized = errors.New("unauthorized")

// DeviceLocal is the device ID for local token auth.
const DeviceLocal = "local"

// Authenticator validates incoming connections.
type Authenticator interface {
	// AuthenticateHTTP checks an HTTP request. Returns device ID or error.
	AuthenticateHTTP(r *http.Request) (string, error)
	// AuthenticateWS checks a WS upgrade request. Returns device ID or error.
	AuthenticateWS(r *http.Request) (string, error)
}

type contextKey int

const deviceIDKey contextKey = iota

// DeviceIDFromContext extracts the device ID set by the auth middleware.
func DeviceIDFromContext(r *http.Request) string {
	v, _ := r.Context().Value(deviceIDKey).(string)
	return v
}
