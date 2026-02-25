package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// EmbedJob represents a single embedding task.
type EmbedJob struct {
	ID      string
	Content string
	Meta    map[string]string
	Delete  bool
}

// Pipeline processes embedding jobs asynchronously via a single worker goroutine.
type Pipeline struct {
	vector *VectorStore
	jobs   chan EmbedJob
	wg     sync.WaitGroup
}

// NewPipeline creates a new embedding pipeline.
// queueSize controls the buffered channel capacity (default: 100).
func NewPipeline(vector *VectorStore, queueSize int) *Pipeline {
	if queueSize <= 0 {
		queueSize = 100
	}
	return &Pipeline{
		vector: vector,
		jobs:   make(chan EmbedJob, queueSize),
	}
}

// Start launches the worker goroutine that processes jobs sequentially.
func (p *Pipeline) Start(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for job := range p.jobs {
			if err := ctx.Err(); err != nil {
				return
			}
			p.processJob(ctx, job)
		}
	}()
}

// Enqueue adds a job to the pipeline. Non-blocking: drops the job if the queue is full.
func (p *Pipeline) Enqueue(job EmbedJob) {
	select {
	case p.jobs <- job:
	default:
		slog.Warn("embedding pipeline queue full, dropping job", "id", job.ID)
	}
}

// Stop closes the job channel, drains remaining jobs, and waits for the worker to finish.
func (p *Pipeline) Stop() {
	close(p.jobs)
	p.wg.Wait()
}

func (p *Pipeline) processJob(ctx context.Context, job EmbedJob) {
	if job.Delete {
		if err := p.vector.Delete(ctx, job.ID); err != nil {
			slog.Warn("embedding pipeline: delete failed", "id", job.ID, "error", err)
		}
		return
	}

	if err := p.vector.Upsert(ctx, job.ID, job.Content, job.Meta); err != nil {
		slog.Warn("embedding pipeline: upsert failed", "id", job.ID, "error", err)
	}
}

// BuildEmbedText formats a memory entry for embedding.
// Format: "Title [tag1, tag2]\ncontent"
func BuildEmbedText(entry *MemoryEntry, content string) string {
	var sb strings.Builder
	sb.WriteString(entry.Title)
	if len(entry.Tags) > 0 {
		sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(entry.Tags, ", ")))
	}
	sb.WriteString("\n")
	sb.WriteString(content)
	return sb.String()
}

// BuildEmbedMeta extracts metadata from a memory entry for vector storage.
func BuildEmbedMeta(entry *MemoryEntry) map[string]string {
	return map[string]string{
		"type":   string(entry.Type),
		"source": entry.Source,
		"title":  entry.Title,
	}
}
