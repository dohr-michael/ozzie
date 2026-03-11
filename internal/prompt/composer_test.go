package prompt

import (
	"strings"
	"testing"
)

func TestComposer_UseTemplate(t *testing.T) {
	c := NewComposer()
	c.UseTemplate(&Template{ID: "test.id", Name: "Test", Text: "hello world"})

	sections := c.Sections()
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].TemplateID != "test.id" {
		t.Errorf("template ID = %q", sections[0].TemplateID)
	}
	if sections[0].Label != "Test" {
		t.Errorf("label = %q", sections[0].Label)
	}
	if c.String() != "hello world" {
		t.Errorf("string = %q", c.String())
	}
}

func TestComposer_AddSection_SkipsEmpty(t *testing.T) {
	c := NewComposer()
	c.AddSection("Empty", "")
	c.AddSection("Full", "content")

	sections := c.Sections()
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (empty skipped), got %d", len(sections))
	}
	if sections[0].Label != "Full" {
		t.Errorf("label = %q", sections[0].Label)
	}
	if sections[0].TemplateID != "" {
		t.Errorf("expected empty template ID for free-form section")
	}
}

func TestComposer_String_JoinsSections(t *testing.T) {
	c := NewComposer()
	c.AddSection("A", "first")
	c.AddSection("B", "second")
	c.AddSection("C", "third")

	got := c.String()
	if got != "first\n\nsecond\n\nthird" {
		t.Errorf("string = %q", got)
	}
}

func TestComposer_Chaining(t *testing.T) {
	c := NewComposer().
		UseTemplate(&Template{ID: "x", Name: "X", Text: "one"}).
		AddSection("Y", "two")

	if len(c.Sections()) != 2 {
		t.Errorf("expected 2 sections after chaining")
	}
	if !strings.Contains(c.String(), "one") || !strings.Contains(c.String(), "two") {
		t.Errorf("missing expected content")
	}
}

func TestComposer_SectionsReturnsACopy(t *testing.T) {
	c := NewComposer().AddSection("A", "one")
	sections := c.Sections()
	sections[0].Label = "modified"

	if c.Sections()[0].Label != "A" {
		t.Error("Sections() should return a copy")
	}
}
