package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
)

// TaskOutputReader reads the output of a completed task.
// tasks.Store satisfies this interface via its ReadOutput method.
type TaskOutputReader interface {
	ReadOutput(taskID string) (string, error)
}

// LLMSummarizer generates a text summary from a prompt.
type LLMSummarizer interface {
	Summarize(ctx context.Context, prompt string) (string, error)
}

// DedupRetriever checks for existing similar memories before storing.
type DedupRetriever interface {
	Retrieve(query string, tags []string, limit int) ([]RetrievedMemory, error)
}

// ExtractorConfig configures the cross-task learning extractor.
type ExtractorConfig struct {
	Store      Store
	Pipeline   *Pipeline
	TaskReader TaskOutputReader
	Summarizer LLMSummarizer
	Bus        *events.Bus
	Retriever  DedupRetriever // optional: dedup check before storing (nil = no dedup)
}

// Extractor listens for task.completed events and extracts reusable lessons
// from task output via an LLM, storing them as memories.
type Extractor struct {
	store       Store
	pipeline    *Pipeline
	taskReader  TaskOutputReader
	summarizer  LLMSummarizer
	bus         *events.Bus
	retriever   DedupRetriever
	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// NewExtractor creates a new cross-task learning extractor.
func NewExtractor(cfg ExtractorConfig) *Extractor {
	return &Extractor{
		store:      cfg.Store,
		pipeline:   cfg.Pipeline,
		taskReader: cfg.TaskReader,
		summarizer: cfg.Summarizer,
		bus:        cfg.Bus,
		retriever:  cfg.Retriever,
	}
}

// Start subscribes to task.completed events and begins extracting lessons.
func (e *Extractor) Start() {
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.unsubscribe = e.bus.Subscribe(e.handleEvent, events.EventTaskCompleted)
	slog.Info("memory extractor started")
}

// Stop cancels pending extractions and unsubscribes from events.
func (e *Extractor) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.unsubscribe != nil {
		e.unsubscribe()
	}
	slog.Info("memory extractor stopped")
}

func (e *Extractor) handleEvent(ev events.Event) {
	payload, ok := events.GetTaskCompletedPayload(ev)
	if !ok {
		return
	}
	go e.extractLessons(payload.TaskID, payload.Title)
}

const maxOutputLen = 4000

const extractionPrompt = `Extract 0-3 reusable lessons from this task output.
Each lesson should be something useful across future sessions (patterns, conventions, gotchas, decisions).
Return a JSON array: [{"title":"...", "content":"...", "tags":["..."]}]
If no reusable lessons, return [].

Task: %s

Output (truncated):
%s`

func (e *Extractor) extractLessons(taskID, title string) {
	output, err := e.taskReader.ReadOutput(taskID)
	if err != nil || output == "" {
		return
	}

	if len(output) > maxOutputLen {
		output = output[:maxOutputLen]
	}

	prompt := fmt.Sprintf(extractionPrompt, title, output)

	resp, err := e.summarizer.Summarize(e.ctx, prompt)
	if err != nil {
		slog.Debug("memory extractor: summarize failed", "task_id", taskID, "error", err)
		return
	}

	lessons := parseLessons(resp)
	if len(lessons) == 0 {
		return
	}

	now := time.Now()
	for _, lesson := range lessons {
		if e.isDuplicate(lesson.Title, lesson.Content) {
			continue
		}
		entry := &MemoryEntry{
			ID:         generateMemoryID(),
			Title:      lesson.Title,
			Type:       MemoryProcedure,
			Source:     "task:" + taskID,
			Tags:       lesson.Tags,
			CreatedAt:  now,
			UpdatedAt:  now,
			LastUsedAt: now,
			Confidence: 0.6,
		}
		if err := e.store.Create(entry, lesson.Content); err != nil {
			slog.Debug("memory extractor: store failed", "task_id", taskID, "title", lesson.Title, "error", err)
			continue
		}
		if e.pipeline != nil {
			e.pipeline.Enqueue(EmbedJob{
				ID:      entry.ID,
				Content: BuildEmbedText(entry, lesson.Content),
				Meta:    BuildEmbedMeta(entry),
			})
		}
		slog.Info("memory extractor: stored lesson", "task_id", taskID, "title", lesson.Title)
	}
}

const dedupScoreThreshold = 0.65

func (e *Extractor) isDuplicate(title, content string) bool {
	if e.retriever == nil {
		return false
	}
	query := title + " " + content
	if runes := []rune(query); len(runes) > 300 {
		query = string(runes[:300])
	}
	results, err := e.retriever.Retrieve(query, nil, 1)
	if err != nil || len(results) == 0 {
		return false
	}
	if results[0].Score >= dedupScoreThreshold {
		slog.Debug("memory extractor: skipping duplicate",
			"title", title,
			"existing", results[0].Entry.Title,
			"score", results[0].Score,
		)
		return true
	}
	return false
}

type extractedLesson struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// parseLessons extracts lessons from LLM response, handling raw JSON and markdown fences.
func parseLessons(resp string) []extractedLesson {
	resp = strings.TrimSpace(resp)

	// Strip markdown code fences if present
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		// Remove first and last lines (``` markers)
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			resp = strings.Join(lines, "\n")
		}
	}

	resp = strings.TrimSpace(resp)

	var lessons []extractedLesson
	if err := json.Unmarshal([]byte(resp), &lessons); err != nil {
		return nil
	}

	// Filter out empty lessons and cap at 3
	var valid []extractedLesson
	for _, l := range lessons {
		if l.Title == "" || l.Content == "" {
			continue
		}
		valid = append(valid, l)
		if len(valid) >= 3 {
			break
		}
	}
	return valid
}
