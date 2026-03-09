package htmltext_test

import (
	"strings"
	"testing"

	"github.com/dohr-michael/ozzie/pkg/htmltext"
)

func TestExtract_Basic(t *testing.T) {
	got := htmltext.Extract("<p>Hello <b>world</b></p>")
	if got != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", got)
	}
}

func TestExtract_BlockTags(t *testing.T) {
	got := htmltext.Extract("<div>one</div><div>two</div>")
	if !strings.Contains(got, "one\ntwo") {
		t.Fatalf("expected newline between blocks, got %q", got)
	}
}

func TestExtract_Headings(t *testing.T) {
	got := htmltext.Extract("<h1>Title</h1><p>Body</p>")
	if !strings.Contains(got, "Title\nBody") {
		t.Fatalf("expected newline between heading and body, got %q", got)
	}
}

func TestExtract_ScriptRemoved(t *testing.T) {
	got := htmltext.Extract("<p>before</p><script>var x=1;</script><p>after</p>")
	if strings.Contains(got, "var") {
		t.Fatalf("script content should be removed, got %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("expected 'before' and 'after', got %q", got)
	}
}

func TestExtract_StyleRemoved(t *testing.T) {
	got := htmltext.Extract("<p>before</p><style>.x{color:red}</style><p>after</p>")
	if strings.Contains(got, "color") {
		t.Fatalf("style content should be removed, got %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("expected 'before' and 'after', got %q", got)
	}
}

func TestExtract_Entities(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"a&nbsp;b", "a b"},
		{"a&#160;b", "a b"},
		{"&apos;", "'"},
		{"&#39;", "'"},
	}
	for _, tt := range tests {
		got := htmltext.Extract(tt.input)
		if got != tt.want {
			t.Errorf("Extract(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtract_WhitespaceCollapse(t *testing.T) {
	got := htmltext.Extract("hello   \t\n  world")
	if got != "hello world" {
		t.Fatalf("expected collapsed whitespace, got %q", got)
	}
}

func TestExtract_BrTag(t *testing.T) {
	got := htmltext.Extract("line1<br>line2<br/>line3<br />line4")
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), got)
	}
}

func TestExtract_ListItems(t *testing.T) {
	got := htmltext.Extract("<ul><li>a</li><li>b</li><li>c</li></ul>")
	if strings.Count(got, "\n") < 2 {
		t.Fatalf("expected newlines between list items, got %q", got)
	}
}

func TestExtract_NestedTags(t *testing.T) {
	got := htmltext.Extract("<div><p>nested <em>emphasis</em> here</p></div>")
	if !strings.Contains(got, "nested emphasis here") {
		t.Fatalf("expected nested content extracted, got %q", got)
	}
}

func TestExtract_Empty(t *testing.T) {
	got := htmltext.Extract("")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtract_PlainText(t *testing.T) {
	got := htmltext.Extract("no html here")
	if got != "no html here" {
		t.Fatalf("expected pass-through, got %q", got)
	}
}

func TestExtract_UTF8(t *testing.T) {
	got := htmltext.Extract("<p>café ☕ naïve</p>")
	if got != "café ☕ naïve" {
		t.Fatalf("expected UTF-8 preserved, got %q", got)
	}
}

func TestExtract_ClosingBlockTags(t *testing.T) {
	got := htmltext.Extract("text</p>more</div>end")
	// Closing block tags should also trigger newlines
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected newlines at closing block tags, got %q", got)
	}
}
