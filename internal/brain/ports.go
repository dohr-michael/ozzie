package brain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/pkg/memory"
)

// ---- Tool Ports ----

// Tool is the domain interface for any invokable tool.
type Tool interface {
	Info(ctx context.Context) (*ToolInfo, error)
	Run(ctx context.Context, argumentsInJSON string) (string, error)
}

// ToolInfo describes a tool's metadata at the domain level.
type ToolInfo struct {
	Name        string
	Description string
}

// ToolLookup resolves tools by name from the registry.
type ToolLookup interface {
	ToolsByNames(names []string) []Tool
	ToolNames() []string
}

// ---- Message ----

// Message is the domain-level chat message.
type Message struct {
	Role    string
	Content string
	Ts      time.Time // zero when not applicable (e.g. LLM prompts)
}

// Standard message roles.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)

// ---- Runner Ports ----

// Runner executes an agent turn.
type Runner interface {
	Run(ctx context.Context, messages []Message) (string, error)
}

// RunnerFactory creates agent runners for a given model + tools.
type RunnerFactory interface {
	CreateRunner(ctx context.Context, model string, instruction string, tools []Tool, opts ...RunnerOption) (Runner, error)
}

// RunnerOption configures optional Runner behavior.
type RunnerOption func(*RunnerOpts)

// RunnerOpts holds optional configuration for CreateRunner.
type RunnerOpts struct {
	MaxIterations   int
	Middlewares      []any       // opaque adapter-specific middlewares
	PreemptionCheck func() bool // returns true when preemption is requested
}

// ApplyRunnerOpts processes variadic options into RunnerOpts.
func ApplyRunnerOpts(opts []RunnerOption) RunnerOpts {
	var o RunnerOpts
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithMaxIterations sets the maximum number of ReAct iterations.
func WithMaxIterations(n int) RunnerOption {
	return func(o *RunnerOpts) { o.MaxIterations = n }
}

// WithMiddlewares passes opaque adapter-specific middlewares to the runner.
func WithMiddlewares(mws []any) RunnerOption {
	return func(o *RunnerOpts) { o.Middlewares = mws }
}

// WithPreemptionCheck sets a callback that signals cooperative preemption.
func WithPreemptionCheck(fn func() bool) RunnerOption {
	return func(o *RunnerOpts) { o.PreemptionCheck = fn }
}

// ErrRunnerPreempted is returned by Runner.Run when preemption is triggered.
var ErrRunnerPreempted = errors.New("runner preempted")

// SummarizeFunc performs a non-streaming LLM call.
type SummarizeFunc func(ctx context.Context, prompt string) (string, error)

// ---- Model Errors ----

// ErrModelUnavailable indicates a model provider is temporarily unavailable.
type ErrModelUnavailable struct {
	Provider string
	Cause    error
}

func (e *ErrModelUnavailable) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("model unavailable (%s): %v", e.Provider, e.Cause)
	}
	return fmt.Sprintf("model unavailable (%s)", e.Provider)
}

func (e *ErrModelUnavailable) Unwrap() error { return e.Cause }

// ---- Tier Resolution ----

// TierResolver maps provider names to model tiers.
type TierResolver interface {
	ProviderTier(name string) ModelTier
}

// ---- Memory Port ----

// MemoryRetriever retrieves relevant memories for context injection.
type MemoryRetriever interface {
	Retrieve(ctx context.Context, query string, tags []string, limit int) ([]memory.RetrievedMemory, error)
}

// ---- Capacity Ports ----

// CapacitySlot represents an acquired LLM capacity slot.
type CapacitySlot interface{}

// CapacityPool acquires and releases LLM capacity slots.
type CapacityPool interface {
	AcquireInteractive(providerName string) (CapacitySlot, error)
	Release(slot CapacitySlot)
}

// ---- Task Ports ----

// TaskStore is the persistence interface for tasks.
type TaskStore interface {
	Create(t *Task) error
	Get(id string) (*Task, error)
	List(filter ListFilter) ([]*Task, error)
	Update(t *Task) error
	Delete(id string) error
	AppendCheckpoint(taskID string, cp Checkpoint) error
	LoadCheckpoints(taskID string) ([]Checkpoint, error)
	WriteOutput(taskID string, content string) error
	ReadOutput(taskID string) (string, error)
}

// SkillExecutor runs a skill by name.
type SkillExecutor interface {
	RunSkill(ctx context.Context, skillName string, vars map[string]string) (string, error)
}

// ToolPermissionsSeeder can seed per-session tool permissions.
type ToolPermissionsSeeder interface {
	AllowForSession(sessionID, toolName string)
}

// TaskExecutor runs a single task to completion.
type TaskExecutor interface {
	Run(ctx context.Context) error
}

// TaskExecutorFactory creates a TaskExecutor for a given task.
type TaskExecutorFactory func(task *Task, cfg TaskExecutorConfig) TaskExecutor

// TaskExecutorConfig holds dependencies for task execution.
type TaskExecutorConfig struct {
	Store           TaskStore
	Bus             events.EventBus
	RunnerFactory   RunnerFactory
	ModelName       string
	ToolLookup      ToolLookup
	SkillRunner     SkillExecutor
	PreemptionCheck func() bool
	Middlewares     []any
	Retriever       MemoryRetriever
	Tier            ModelTier
	PromptPrefix    string
	Perms           ToolPermissionsSeeder
	ClientFacing    bool
	Persona         string
}

// TaskSubmitter is the interface for submitting and managing tasks.
type TaskSubmitter interface {
	Submit(t *Task) error
	Cancel(taskID string, reason string) error
	Store() TaskStore
	AvailableActors() []ActorInfo
}

// InlineExecutor can execute tasks synchronously when the pool has 1 actor.
type InlineExecutor interface {
	ShouldInline() bool
	ExecuteInline(ctx context.Context, t *Task) (output string, err error)
}
