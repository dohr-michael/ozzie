package membridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dohr-michael/ozzie/internal/core/events"
	"github.com/dohr-michael/ozzie/pkg/llmutil"
	"github.com/dohr-michael/ozzie/internal/core/prompt"
	"github.com/dohr-michael/ozzie/pkg/memory"
)

// TaskOutputReader reads the output of a completed task.
// tasks.Store satisfies this interface via its ReadOutput method.
type TaskOutputReader interface {
	ReadOutput(taskID string) (string, error)
}

// ExtractorConfig configures the cross-task learning extractor.
type ExtractorConfig struct {
	Store      memory.Store
	Pipeline   *memory.Pipeline
	TaskReader TaskOutputReader
	Summarizer memory.LLMSummarizer
	Bus        events.EventBus
	Retriever  memory.MemoryRetriever // optional: dedup check before storing (nil = no dedup)
}

// Extractor listens for task.completed events and extracts reusable lessons
// from task output via an LLM, storing them as memories.
type Extractor struct {
	store       memory.Store
	pipeline    *memory.Pipeline
	taskReader  TaskOutputReader
	summarizer  memory.LLMSummarizer
	bus         events.EventBus
	retriever   memory.MemoryRetriever
	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
	wg          sync.WaitGroup
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

// Stop cancels pending extractions, waits for in-flight goroutines, and unsubscribes.
func (e *Extractor) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.unsubscribe != nil {
		e.unsubscribe()
	}
	e.wg.Wait()
	slog.Info("memory extractor stopped")
}

func (e *Extractor) handleEvent(ev events.Event) {
	payload, ok := events.GetTaskCompletedPayload(ev)
	if !ok {
		return
	}
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.extractLessons(payload.TaskID, payload.Title)
	}()
}

const maxOutputLen = 4000


func (e *Extractor) extractLessons(taskID, title string) {
	output, err := e.taskReader.ReadOutput(taskID)
	if err != nil || output == "" {
		return
	}

	if len(output) > maxOutputLen {
		output = output[:maxOutputLen]
	}

	extractionPrompt := fmt.Sprintf(prompt.ExtractionLessonsPrompt, title, output)

	resp, err := e.summarizer.Summarize(e.ctx, extractionPrompt)
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
		if e.isDuplicate(e.ctx, lesson.Title, lesson.Content) {
			continue
		}
		entry := &memory.MemoryEntry{
			Title:      lesson.Title,
			Type:       memory.MemoryProcedure,
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
			e.pipeline.Enqueue(memory.EmbedJob{
				ID:      entry.ID,
				Content: memory.BuildEmbedText(entry, lesson.Content),
				Meta:    memory.BuildEmbedMeta(entry),
			})
		}
		slog.Info("memory extractor: stored lesson", "task_id", taskID, "title", lesson.Title)
	}
}

const dedupScoreThreshold = 0.65

func (e *Extractor) isDuplicate(ctx context.Context, title, content string) bool {
	if e.retriever == nil {
		return false
	}
	query := title + " " + content
	if runes := []rune(query); len(runes) > 300 {
		query = string(runes[:300])
	}
	results, err := e.retriever.Retrieve(ctx, query, nil, 1)
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
	resp = llmutil.StripCodeFences(resp)

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
