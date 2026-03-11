package connectors

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/pkg/connector"
)

// testReaction records a single AddReaction/RemoveReaction call.
type testReaction struct {
	ChannelID string
	MessageID string
	Reaction  connector.ReactionType
	Added     bool // true = add, false = remove
}

// fakeReactioner extends fakeConnector with Reactioner support.
type fakeReactioner struct {
	fakeConnector
	mu        sync.Mutex
	reactions []testReaction
}

var _ connector.Reactioner = (*fakeReactioner)(nil)

func (f *fakeReactioner) AddReaction(_ context.Context, channelID, messageID string, rt connector.ReactionType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reactions = append(f.reactions, testReaction{channelID, messageID, rt, true})
	return nil
}

func (f *fakeReactioner) RemoveReaction(_ context.Context, channelID, messageID string, rt connector.ReactionType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reactions = append(f.reactions, testReaction{channelID, messageID, rt, false})
	return nil
}

func (f *fakeReactioner) Reactions() []testReaction {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]testReaction, len(f.reactions))
	copy(out, f.reactions)
	return out
}

func setupReactor(t *testing.T, fc connector.Connector) (*ProgressReactor, *events.Bus, *SessionMap) {
	t.Helper()
	bus := events.NewBus(128)
	t.Cleanup(func() { bus.Close() })

	sm := NewSessionMap(t.TempDir())
	sm.SetMessageContext("sess1", fc.Name(), "ch1", "msg1")

	pr := NewProgressReactor(ProgressReactorConfig{
		Bus:        bus,
		Connectors: map[string]connector.Connector{fc.Name(): fc},
		SessionMap: sm,
	})
	t.Cleanup(func() { pr.Stop() })

	return pr, bus, sm
}

func TestProgressReactorLLMThinking(t *testing.T) {
	fc := &fakeReactioner{fakeConnector: fakeConnector{name: "test"}}
	_, bus, _ := setupReactor(t, fc)

	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.LLMCallPayload{
		Phase: "request",
		Model: "test-model",
	}, "sess1"))

	time.Sleep(100 * time.Millisecond)

	reactions := fc.Reactions()
	if len(reactions) == 0 {
		t.Fatal("expected thinking reaction")
	}
	if reactions[0].Reaction != connector.ReactionThinking {
		t.Fatalf("expected ReactionThinking, got %q", reactions[0].Reaction)
	}
	if !reactions[0].Added {
		t.Fatal("expected add, not remove")
	}
	if reactions[0].ChannelID != "ch1" || reactions[0].MessageID != "msg1" {
		t.Fatalf("wrong target: ch=%q msg=%q", reactions[0].ChannelID, reactions[0].MessageID)
	}
}

func TestProgressReactorToolReaction(t *testing.T) {
	fc := &fakeReactioner{fakeConnector: fakeConnector{name: "test"}}
	_, bus, _ := setupReactor(t, fc)

	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.ToolCallPayload{
		Status: events.ToolStatusStarted,
		Name:   "web_search",
	}, "sess1"))

	time.Sleep(100 * time.Millisecond)

	reactions := fc.Reactions()
	if len(reactions) == 0 {
		t.Fatal("expected web reaction")
	}
	if reactions[0].Reaction != connector.ReactionWeb {
		t.Fatalf("expected ReactionWeb, got %q", reactions[0].Reaction)
	}
}

func TestProgressReactorDedup(t *testing.T) {
	fc := &fakeReactioner{fakeConnector: fakeConnector{name: "test"}}
	_, bus, _ := setupReactor(t, fc)

	// Send the same tool twice
	for i := 0; i < 2; i++ {
		bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.ToolCallPayload{
			Status: events.ToolStatusStarted,
			Name:   "web_search",
		}, "sess1"))
	}

	time.Sleep(100 * time.Millisecond)

	reactions := fc.Reactions()
	added := 0
	for _, r := range reactions {
		if r.Added {
			added++
		}
	}
	if added != 1 {
		t.Fatalf("expected 1 add (dedup), got %d", added)
	}
}

func TestProgressReactorClearOnDone(t *testing.T) {
	fc := &fakeReactioner{fakeConnector: fakeConnector{name: "test"}}
	_, bus, _ := setupReactor(t, fc)

	// Add two different reactions
	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.LLMCallPayload{
		Phase: "request",
		Model: "m",
	}, "sess1"))
	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.ToolCallPayload{
		Status: events.ToolStatusStarted,
		Name:   "web_search",
	}, "sess1"))

	time.Sleep(100 * time.Millisecond)

	// Now send assistant message to clear
	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.AssistantMessagePayload{
		Content: "done",
	}, "sess1"))

	time.Sleep(100 * time.Millisecond)

	reactions := fc.Reactions()
	removed := 0
	for _, r := range reactions {
		if !r.Added {
			removed++
		}
	}
	if removed != 2 {
		t.Fatalf("expected 2 removals, got %d (reactions: %+v)", removed, reactions)
	}
}

func TestProgressReactorNonReactioner(t *testing.T) {
	// fakeConnector does NOT implement Reactioner — should not panic
	fc := &fakeConnector{name: "test"}
	_, bus, _ := setupReactor(t, fc)

	bus.Publish(events.NewTypedEventWithSession(events.SourceAgent, events.LLMCallPayload{
		Phase: "request",
		Model: "m",
	}, "sess1"))

	time.Sleep(100 * time.Millisecond)
	// No panic = pass
}

func TestToolReaction(t *testing.T) {
	tests := []struct {
		tool     string
		reaction connector.ReactionType
	}{
		{"web_search", connector.ReactionWeb},
		{"web_fetch", connector.ReactionWeb},
		{"run_command", connector.ReactionCommand},
		{"str_replace_editor", connector.ReactionEdit},
		{"submit_task", connector.ReactionTask},
		{"plan_task", connector.ReactionTask},
		{"store_memory", connector.ReactionMemory},
		{"query_memories", connector.ReactionMemory},
		{"forget_memory", connector.ReactionMemory},
		{"schedule_task", connector.ReactionSchedule},
		{"trigger_schedule", connector.ReactionSchedule},
		{"activate_tools", connector.ReactionActivate},
		{"activate_skill", connector.ReactionActivate},
		{"some_unknown_tool", connector.ReactionTool},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := toolReaction(tt.tool)
			if got != tt.reaction {
				t.Errorf("toolReaction(%q) = %q, want %q", tt.tool, got, tt.reaction)
			}
		})
	}
}
