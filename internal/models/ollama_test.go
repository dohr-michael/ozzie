package models

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaTransport_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"model":"test"}`))
	}))
	defer srv.Close()

	transport := &ollamaTransport{inner: http.DefaultTransport, provider: "ollama"}
	req, _ := http.NewRequest("POST", srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"model":"test"}` {
		t.Errorf("body: got %q, want %q", string(body), `{"model":"test"}`)
	}
}

func TestOllamaTransport_NonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("no available server"))
	}))
	defer srv.Close()

	transport := &ollamaTransport{inner: http.DefaultTransport, provider: "ollama"}
	req, _ := http.NewRequest("POST", srv.URL, nil)
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error for non-JSON response")
	}

	var unavail *ErrModelUnavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("expected ErrModelUnavailable, got %T: %v", err, err)
	}
	if unavail.Provider != "ollama" {
		t.Errorf("provider: got %q, want %q", unavail.Provider, "ollama")
	}
	if !strings.Contains(unavail.Body, "no available server") {
		t.Errorf("body: got %q, want to contain %q", unavail.Body, "no available server")
	}
}

func TestOllamaTransport_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("service unavailable"))
	}))
	defer srv.Close()

	transport := &ollamaTransport{inner: http.DefaultTransport, provider: "ollama"}
	req, _ := http.NewRequest("POST", srv.URL, nil)
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error for 503 response")
	}

	var unavail *ErrModelUnavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("expected ErrModelUnavailable, got %T: %v", err, err)
	}
	if !strings.Contains(unavail.Body, "service unavailable") {
		t.Errorf("body: got %q, want to contain %q", unavail.Body, "service unavailable")
	}
}

func TestOllamaTransport_ConnectionError(t *testing.T) {
	transport := &ollamaTransport{inner: http.DefaultTransport, provider: "ollama"}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:1", nil) // nothing listening
	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected error for connection failure")
	}

	var unavail *ErrModelUnavailable
	if !errors.As(err, &unavail) {
		t.Fatalf("expected ErrModelUnavailable, got %T: %v", err, err)
	}
	if unavail.Provider != "ollama" {
		t.Errorf("provider: got %q, want %q", unavail.Provider, "ollama")
	}
	if unavail.Cause == nil {
		t.Error("expected non-nil Cause for connection error")
	}
}

func TestOllamaTransport_StreamingNDJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(200)
		w.Write([]byte(`{"done":false}` + "\n"))
	}))
	defer srv.Close()

	transport := &ollamaTransport{inner: http.DefaultTransport, provider: "ollama"}
	req, _ := http.NewRequest("POST", srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error for ndjson: %v", err)
	}
	resp.Body.Close()
}
