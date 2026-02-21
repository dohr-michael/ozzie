package models

// Selector stub types for future model selection logic.

// SelectOp represents a selection operation.
type SelectOp string

const (
	SelectByTag  SelectOp = "tag"
	SelectByName SelectOp = "name"
)

// Constraint represents a model selection constraint.
type Constraint struct {
	Op    SelectOp `json:"op"`
	Value string   `json:"value"`
}

// Selector selects a model based on constraints.
type Selector struct {
	Constraints []Constraint
}
