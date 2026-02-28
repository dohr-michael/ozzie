package plugins

import (
	"testing"
)

func TestExtractText_BasicHTML(t *testing.T) {
	html := `<html><head><title>Test</title></head><body><h1>Hello</h1><p>World</p></body></html>`
	got := extractText(html)
	if got != "Test\nHello\nWorld" {
		t.Errorf("expected 'Test\\nHello\\nWorld', got %q", got)
	}
}

func TestExtractText_ScriptAndStyle(t *testing.T) {
	html := `<p>Before</p><script>var x=1;</script><style>.a{color:red}</style><p>After</p>`
	got := extractText(html)
	if got != "Before\nAfter" {
		t.Errorf("expected 'Before\\nAfter', got %q", got)
	}
}

func TestExtractText_WhitespaceCollapse(t *testing.T) {
	html := `<p>Hello    world   here</p>`
	got := extractText(html)
	if got != "Hello world here" {
		t.Errorf("expected 'Hello world here', got %q", got)
	}
}

func TestExtractText_Entities(t *testing.T) {
	html := `<p>A &amp; B &lt; C</p>`
	got := extractText(html)
	if got != "A & B < C" {
		t.Errorf("expected 'A & B < C', got %q", got)
	}
}

func TestExtractText_PlainText(t *testing.T) {
	text := "Just plain text with no tags"
	got := extractText(text)
	if got != text {
		t.Errorf("expected %q, got %q", text, got)
	}
}

func TestWebFetchManifest_Structure(t *testing.T) {
	m := WebFetchManifest()
	if m.Name != "web_fetch" {
		t.Errorf("expected name 'web_fetch', got %q", m.Name)
	}
	if len(m.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(m.Tools))
	}
	if !m.Tools[0].Dangerous {
		t.Error("web_fetch should be marked dangerous")
	}
}

func TestWebSearchManifest_Structure(t *testing.T) {
	m := WebSearchManifest()
	if m.Name != "web_search" {
		t.Errorf("expected name 'web_search', got %q", m.Name)
	}
	if len(m.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(m.Tools))
	}
	if m.Tools[0].Dangerous {
		t.Error("web_search should NOT be marked dangerous (read-only)")
	}
}
