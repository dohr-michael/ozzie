package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestMiddleware_NilAuth(t *testing.T) {
	handler := Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	localAuth, err := NewLocalAuth(filepath.Join(dir, ".local_token"), identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	var gotDeviceID string
	handler := Middleware(localAuth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDeviceID = DeviceIDFromContext(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+localAuth.token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotDeviceID != DeviceLocal {
		t.Fatalf("expected device ID %q, got %q", DeviceLocal, gotDeviceID)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	localAuth, err := NewLocalAuth(filepath.Join(dir, ".local_token"), identity.Recipient())
	if err != nil {
		t.Fatal(err)
	}

	handler := Middleware(localAuth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
