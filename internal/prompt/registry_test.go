package prompt

import "testing"

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register("test.one", "Test One", "hello")

	got := r.Get("test.one")
	if got == nil {
		t.Fatal("expected template, got nil")
	}
	if got.Text != "hello" {
		t.Errorf("text = %q, want %q", got.Text, "hello")
	}
	if got.Name != "Test One" {
		t.Errorf("name = %q, want %q", got.Name, "Test One")
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := NewRegistry()
	if got := r.Get("nope"); got != nil {
		t.Errorf("expected nil for missing ID, got %+v", got)
	}
}

func TestRegistry_MustGet_Panics(t *testing.T) {
	r := NewRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing template")
		}
	}()
	r.MustGet("nope")
}

func TestRegistry_All_Sorted(t *testing.T) {
	r := NewRegistry()
	r.Register("b.second", "B", "two")
	r.Register("a.first", "A", "one")
	r.Register("c.third", "C", "three")

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	if all[0].ID != "a.first" || all[1].ID != "b.second" || all[2].ID != "c.third" {
		t.Errorf("not sorted: %s, %s, %s", all[0].ID, all[1].ID, all[2].ID)
	}
}

func TestRegistry_Overwrite(t *testing.T) {
	r := NewRegistry()
	r.Register("x", "X", "old")
	r.Register("x", "X updated", "new")

	got := r.Get("x")
	if got.Text != "new" {
		t.Errorf("expected overwritten text, got %q", got.Text)
	}
}
