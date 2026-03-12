package skills

import "fmt"

// DAG represents a directed acyclic graph of workflow steps.
type DAG struct {
	steps map[string]*Step
	order []string // topological order
}

// NewDAG builds a DAG from steps using Kahn's algorithm.
// Returns an error if the graph contains a cycle or references unknown steps.
func NewDAG(steps []Step) (*DAG, error) {
	d := &DAG{
		steps: make(map[string]*Step, len(steps)),
	}

	// Index steps
	for i := range steps {
		d.steps[steps[i].ID] = &steps[i]
	}

	// Build adjacency and in-degree
	inDegree := make(map[string]int, len(steps))
	dependents := make(map[string][]string, len(steps)) // step → steps that depend on it

	for _, s := range steps {
		inDegree[s.ID] += 0 // ensure entry exists
		for _, need := range s.Needs {
			if _, ok := d.steps[need]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", s.ID, need)
			}
			inDegree[s.ID]++
			dependents[need] = append(dependents[need], s.ID)
		}
	}

	// Kahn's algorithm
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)

		for _, dep := range dependents[id] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(order) != len(steps) {
		return nil, fmt.Errorf("cycle detected in workflow steps")
	}

	d.order = order
	return d, nil
}

// TopologicalOrder returns the step IDs in topological order.
func (d *DAG) TopologicalOrder() []string {
	result := make([]string, len(d.order))
	copy(result, d.order)
	return result
}

// ReadySteps returns step IDs whose dependencies are all in the completed set.
func (d *DAG) ReadySteps(completed map[string]bool) []string {
	var ready []string
	for _, id := range d.order {
		if completed[id] {
			continue
		}
		step := d.steps[id]
		allDone := true
		for _, need := range step.Needs {
			if !completed[need] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, id)
		}
	}
	return ready
}

// Step returns the step with the given ID, or nil.
func (d *DAG) Step(id string) *Step {
	return d.steps[id]
}

// Len returns the number of steps in the DAG.
func (d *DAG) Len() int {
	return len(d.steps)
}
