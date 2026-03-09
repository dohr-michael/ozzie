// Package editor provides a rich file editing library with undo support,
// line-based insert, str_replace with uniqueness check, and directory listing.
// Inspired by the StrReplaceEditor pattern from eino-ext/commandline.
package editor

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Backend abstracts filesystem operations so the editor can work with
// local files, sandboxed environments, or test doubles.
type Backend interface {
	ReadFile(ctx context.Context, path string) (string, error)
	WriteFile(ctx context.Context, path string, content string) error
	IsDirectory(ctx context.Context, path string) (bool, error)
	Exists(ctx context.Context, path string) (bool, error)
	ListDir(ctx context.Context, path string, maxDepth int) ([]string, error)
}

// Editor provides file viewing, creation, replacement, insertion, and undo.
type Editor struct {
	backend     Backend
	fileHistory map[string][]string // path → stack of previous contents
	mu          sync.Mutex
}

// New creates an Editor backed by the given Backend.
func New(backend Backend) *Editor {
	return &Editor{
		backend:     backend,
		fileHistory: make(map[string][]string),
	}
}

const (
	maxOutputLines = 300
	snippetRadius  = 4 // lines of context around edits
)

// View returns the content of a file (with line numbers) or a directory listing.
// viewRange is optional: [startLine] or [startLine, endLine] (1-based inclusive).
func (e *Editor) View(ctx context.Context, path string, viewRange []int) (string, error) {
	isDir, err := e.backend.IsDirectory(ctx, path)
	if err != nil {
		return "", fmt.Errorf("view: %w", err)
	}
	if isDir {
		entries, err := e.backend.ListDir(ctx, path, 2)
		if err != nil {
			return "", fmt.Errorf("view: list directory: %w", err)
		}
		if len(entries) == 0 {
			return "Directory is empty.", nil
		}
		return strings.Join(entries, "\n"), nil
	}

	content, err := e.backend.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("view: %w", err)
	}

	lines := strings.Split(content, "\n")

	startLine := 1
	endLine := len(lines)

	if len(viewRange) >= 1 {
		startLine = viewRange[0]
		if startLine < 1 {
			startLine = 1
		}
	}
	if len(viewRange) >= 2 {
		endLine = viewRange[1]
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return "", fmt.Errorf("view: invalid range [%d, %d]", startLine, endLine)
	}

	return makeOutput(lines[startLine-1:endLine], path, startLine), nil
}

// Create writes a new file. Fails if the file already exists.
func (e *Editor) Create(ctx context.Context, path, content string) (string, error) {
	exists, err := e.backend.Exists(ctx, path)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	if exists {
		return "", fmt.Errorf("create: file %q already exists — use str_replace or insert to edit it", path)
	}

	if err := e.backend.WriteFile(ctx, path, content); err != nil {
		return "", fmt.Errorf("create: %w", err)
	}

	lines := strings.Split(content, "\n")
	return makeOutput(lines, path, 1), nil
}

// StrReplace replaces a unique occurrence of oldStr with newStr in the file.
// Pushes the previous content to the undo stack.
func (e *Editor) StrReplace(ctx context.Context, path, oldStr, newStr string) (string, error) {
	content, err := e.backend.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("str_replace: %w", err)
	}

	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", fmt.Errorf("str_replace: old_str not found in %s", path)
	}
	if count > 1 {
		return "", fmt.Errorf("str_replace: old_str appears %d times in %s — must be unique", count, path)
	}

	e.pushHistory(path, content)

	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := e.backend.WriteFile(ctx, path, newContent); err != nil {
		return "", fmt.Errorf("str_replace: %w", err)
	}

	return e.snippetAround(newContent, newStr, path), nil
}

// Insert inserts text after the given line number (1-based).
// Line 0 inserts at the beginning of the file.
// Pushes the previous content to the undo stack.
func (e *Editor) Insert(ctx context.Context, path string, line int, text string) (string, error) {
	content, err := e.backend.ReadFile(ctx, path)
	if err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	lines := strings.Split(content, "\n")

	if line < 0 || line > len(lines) {
		return "", fmt.Errorf("insert: line %d out of range [0, %d]", line, len(lines))
	}

	e.pushHistory(path, content)

	insertLines := strings.Split(text, "\n")
	newLines := make([]string, 0, len(lines)+len(insertLines))
	newLines = append(newLines, lines[:line]...)
	newLines = append(newLines, insertLines...)
	newLines = append(newLines, lines[line:]...)

	newContent := strings.Join(newLines, "\n")
	if err := e.backend.WriteFile(ctx, path, newContent); err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	// Show snippet around insertion point
	snippetStart := line + 1 // 1-based, first inserted line
	snippetEnd := line + len(insertLines)
	return e.snippetRange(newLines, path, snippetStart, snippetEnd), nil
}

// UndoEdit restores the last saved version of the file.
func (e *Editor) UndoEdit(ctx context.Context, path string) (string, error) {
	e.mu.Lock()
	stack := e.fileHistory[path]
	if len(stack) == 0 {
		e.mu.Unlock()
		return "", fmt.Errorf("undo_edit: no edit history for %s", path)
	}
	prev := stack[len(stack)-1]
	e.fileHistory[path] = stack[:len(stack)-1]
	e.mu.Unlock()

	if err := e.backend.WriteFile(ctx, path, prev); err != nil {
		return "", fmt.Errorf("undo_edit: %w", err)
	}

	lines := strings.Split(prev, "\n")
	return makeOutput(lines, path, 1), nil
}

// pushHistory saves the current content to the undo stack.
func (e *Editor) pushHistory(path, content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fileHistory[path] = append(e.fileHistory[path], content)
}

// makeOutput formats lines with line numbers.
func makeOutput(lines []string, label string, startLine int) string {
	var b strings.Builder
	if len(lines) > maxOutputLines {
		lines = lines[:maxOutputLines]
		fmt.Fprintf(&b, "[Showing first %d lines of %s]\n", maxOutputLines, label)
	}
	for i, line := range lines {
		fmt.Fprintf(&b, "%6d\t%s\n", startLine+i, line)
	}
	return b.String()
}

// snippetAround returns a few lines of context around the first occurrence of needle.
func (e *Editor) snippetAround(content, needle, label string) string {
	lines := strings.Split(content, "\n")
	// Find the line containing the start of needle
	idx := strings.Index(content, needle)
	if idx < 0 {
		return makeOutput(lines, label, 1)
	}
	lineNum := strings.Count(content[:idx], "\n")

	needleLines := strings.Count(needle, "\n") + 1
	start := lineNum - snippetRadius
	if start < 0 {
		start = 0
	}
	end := lineNum + needleLines + snippetRadius
	if end > len(lines) {
		end = len(lines)
	}

	return makeOutput(lines[start:end], label, start+1)
}

// snippetRange returns lines around the given 1-based inclusive range.
func (e *Editor) snippetRange(lines []string, label string, startLine, endLine int) string {
	start := startLine - snippetRadius - 1
	if start < 0 {
		start = 0
	}
	end := endLine + snippetRadius
	if end > len(lines) {
		end = len(lines)
	}
	return makeOutput(lines[start:end], label, start+1)
}
