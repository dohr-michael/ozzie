package skills

import (
	"strings"
	"testing"
)

func TestDAG_Linear(t *testing.T) {
	steps := []Step{
		{ID: "a", Instruction: "do A"},
		{ID: "b", Instruction: "do B", Needs: []string{"a"}},
		{ID: "c", Instruction: "do C", Needs: []string{"b"}},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := dag.TopologicalOrder()
	if len(order) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(order))
	}

	// a must come before b, b before c
	idx := indexMap(order)
	if idx["a"] > idx["b"] || idx["b"] > idx["c"] {
		t.Errorf("unexpected order: %v", order)
	}

	// Ready from empty
	ready := dag.ReadySteps(map[string]bool{})
	if len(ready) != 1 || ready[0] != "a" {
		t.Errorf("expected [a], got %v", ready)
	}

	// Ready after a
	ready = dag.ReadySteps(map[string]bool{"a": true})
	if len(ready) != 1 || ready[0] != "b" {
		t.Errorf("expected [b], got %v", ready)
	}

	// Ready after a, b
	ready = dag.ReadySteps(map[string]bool{"a": true, "b": true})
	if len(ready) != 1 || ready[0] != "c" {
		t.Errorf("expected [c], got %v", ready)
	}
}

func TestDAG_FanOut(t *testing.T) {
	// A → B, A → C (B and C are parallel after A)
	steps := []Step{
		{ID: "a", Instruction: "do A"},
		{ID: "b", Instruction: "do B", Needs: []string{"a"}},
		{ID: "c", Instruction: "do C", Needs: []string{"a"}},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := dag.TopologicalOrder()
	idx := indexMap(order)
	if idx["a"] > idx["b"] || idx["a"] > idx["c"] {
		t.Errorf("a should come before b and c: %v", order)
	}

	// After a, both b and c are ready
	ready := dag.ReadySteps(map[string]bool{"a": true})
	if len(ready) != 2 {
		t.Errorf("expected 2 ready steps, got %v", ready)
	}
}

func TestDAG_FanIn(t *testing.T) {
	// B, C → D
	steps := []Step{
		{ID: "b", Instruction: "do B"},
		{ID: "c", Instruction: "do C"},
		{ID: "d", Instruction: "do D", Needs: []string{"b", "c"}},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both b and c are ready from the start
	ready := dag.ReadySteps(map[string]bool{})
	if len(ready) != 2 {
		t.Errorf("expected 2 ready steps, got %v", ready)
	}

	// Only b done — d not ready
	ready = dag.ReadySteps(map[string]bool{"b": true})
	if len(ready) != 1 || ready[0] != "c" {
		t.Errorf("expected [c], got %v", ready)
	}

	// Both done — d is ready
	ready = dag.ReadySteps(map[string]bool{"b": true, "c": true})
	if len(ready) != 1 || ready[0] != "d" {
		t.Errorf("expected [d], got %v", ready)
	}
}

func TestDAG_Diamond(t *testing.T) {
	// A → B, A → C, B+C → D
	steps := []Step{
		{ID: "a", Instruction: "do A"},
		{ID: "b", Instruction: "do B", Needs: []string{"a"}},
		{ID: "c", Instruction: "do C", Needs: []string{"a"}},
		{ID: "d", Instruction: "do D", Needs: []string{"b", "c"}},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := dag.TopologicalOrder()
	idx := indexMap(order)
	if idx["a"] > idx["b"] || idx["a"] > idx["c"] {
		t.Errorf("a should come before b and c")
	}
	if idx["b"] > idx["d"] || idx["c"] > idx["d"] {
		t.Errorf("b and c should come before d")
	}
}

func TestDAG_CycleDetection(t *testing.T) {
	steps := []Step{
		{ID: "a", Instruction: "do A", Needs: []string{"b"}},
		{ID: "b", Instruction: "do B", Needs: []string{"a"}},
	}

	_, err := NewDAG(steps)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestDAG_UnknownDep(t *testing.T) {
	steps := []Step{
		{ID: "a", Instruction: "do A", Needs: []string{"missing"}},
	}

	_, err := NewDAG(steps)
	if err == nil {
		t.Fatal("expected unknown dep error")
	}
	if !strings.Contains(err.Error(), "unknown step") {
		t.Errorf("expected unknown step error, got: %v", err)
	}
}

func TestDAG_SingleStep(t *testing.T) {
	steps := []Step{
		{ID: "only", Instruction: "do it"},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dag.Len() != 1 {
		t.Errorf("expected 1 step, got %d", dag.Len())
	}

	ready := dag.ReadySteps(map[string]bool{})
	if len(ready) != 1 || ready[0] != "only" {
		t.Errorf("expected [only], got %v", ready)
	}
}

func TestDAG_Empty(t *testing.T) {
	dag, err := NewDAG([]Step{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dag.Len() != 0 {
		t.Errorf("expected 0 steps, got %d", dag.Len())
	}
	ready := dag.ReadySteps(map[string]bool{})
	if len(ready) != 0 {
		t.Errorf("expected no ready steps, got %v", ready)
	}
}

func TestDAG_StepLookup(t *testing.T) {
	steps := []Step{
		{ID: "a", Instruction: "do A", Title: "Step A"},
	}

	dag, err := NewDAG(steps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := dag.Step("a")
	if s == nil {
		t.Fatal("expected step a")
	}
	if s.Title != "Step A" {
		t.Errorf("expected title 'Step A', got %q", s.Title)
	}

	if dag.Step("missing") != nil {
		t.Error("expected nil for missing step")
	}
}

func indexMap(order []string) map[string]int {
	m := make(map[string]int, len(order))
	for i, id := range order {
		m[id] = i
	}
	return m
}
