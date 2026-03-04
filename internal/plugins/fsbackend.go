package plugins

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/google/uuid"

	"github.com/dohr-michael/ozzie/internal/events"
)

// skipDirs are directories to skip during search and traversal.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".hg":          true,
}

// isBinary checks if a file appears to be binary by looking for null bytes
// in the first 512 bytes.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// writeConfirmTimeout is how long the backend waits for user confirmation
// before denying an out-of-sandbox write in interactive mode.
const writeConfirmTimeout = 60 * time.Second

// OzzieBackend implements filesystem.Backend using the local filesystem.
// It resolves paths relative to the WorkDir from the context.
// writeAllowedPaths lists additional directories (e.g. $OZZIE_HOME/tmp)
// where writes are permitted without confirmation.
// readRestrictedPaths lists directories that the agent cannot read (e.g. .age/).
type OzzieBackend struct {
	writeAllowedPaths   []string
	readRestrictedPaths []string
	bus                 *events.Bus      // nil = no interactive confirmation (tests)
	perms               *ToolPermissions // nil = no accept-all bypass
}

// OzzieBackendOption configures an OzzieBackend.
type OzzieBackendOption func(*OzzieBackend)

// WithWriteAllowedPaths adds paths where writes are permitted without confirmation.
func WithWriteAllowedPaths(paths ...string) OzzieBackendOption {
	return func(b *OzzieBackend) { b.writeAllowedPaths = append(b.writeAllowedPaths, paths...) }
}

// WithReadRestrictedPaths adds paths that the agent cannot read.
func WithReadRestrictedPaths(paths ...string) OzzieBackendOption {
	return func(b *OzzieBackend) { b.readRestrictedPaths = append(b.readRestrictedPaths, paths...) }
}

// NewOzzieBackend creates a new filesystem backend.
// Use functional options for write-allowed and read-restricted paths.
func NewOzzieBackend(bus *events.Bus, perms *ToolPermissions, opts ...OzzieBackendOption) *OzzieBackend {
	b := &OzzieBackend{bus: bus, perms: perms}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// validateReadPath checks if a path is inside a restricted directory.
func (b *OzzieBackend) validateReadPath(ctx context.Context, path string) error {
	absPath, err := filepath.Abs(b.resolvePath(ctx, path))
	if err != nil {
		return fmt.Errorf("read guard: resolve path: %w", err)
	}
	if real, err := evalSymlinksExisting(absPath); err == nil {
		absPath = real
	}

	for _, restricted := range b.readRestrictedPaths {
		clean := filepath.Clean(restricted)
		if real, err := filepath.EvalSymlinks(clean); err == nil {
			clean = real
		}
		if isUnder(absPath, clean) {
			return fmt.Errorf("access denied: path %q is inside a restricted directory", path)
		}
	}
	return nil
}

// isInsideSandbox checks whether absPath falls inside the working directory
// or any of the allowed paths.
func (b *OzzieBackend) isInsideSandbox(ctx context.Context, absPath string) bool {
	if wd := events.WorkDirFromContext(ctx); wd != "" {
		cleanWD := filepath.Clean(wd)
		if real, err := filepath.EvalSymlinks(cleanWD); err == nil {
			cleanWD = real
		}
		if isUnder(absPath, cleanWD) {
			return true
		}
	}
	for _, ap := range b.writeAllowedPaths {
		clean := filepath.Clean(ap)
		if real, err := filepath.EvalSymlinks(clean); err == nil {
			clean = real
		}
		if isUnder(absPath, clean) {
			return true
		}
	}
	return false
}

// validateWritePath enforces write-path restrictions:
//   - Inside sandbox (workDir + allowed paths): always allowed.
//   - Outside sandbox + autonomous mode: hard block (error).
//   - Outside sandbox + interactive mode: prompt user for confirmation.
func (b *OzzieBackend) validateWritePath(ctx context.Context, path string) error {
	absPath, err := filepath.Abs(b.resolvePath(ctx, path))
	if err != nil {
		return fmt.Errorf("write guard: resolve path: %w", err)
	}
	if real, err := evalSymlinksExisting(absPath); err == nil {
		absPath = real
	}

	if b.isInsideSandbox(ctx, absPath) {
		return nil
	}

	// Outside sandbox — autonomous mode: hard block
	if events.IsAutonomousContext(ctx) {
		return fmt.Errorf("write blocked: path %q is outside allowed write directories", path)
	}

	// Accept-all mode (e.g. -y flag) — skip confirmation
	if b.perms != nil {
		if sid := events.SessionIDFromContext(ctx); sid != "" && b.perms.IsSessionAcceptAll(sid) {
			return nil
		}
	}

	// Outside sandbox — interactive mode: ask for confirmation
	return b.confirmWrite(ctx, path)
}

// confirmWrite prompts the user via the event bus and blocks until they
// approve or deny. Returns nil on approval, error on denial or timeout.
func (b *OzzieBackend) confirmWrite(ctx context.Context, path string) error {
	if b.bus == nil {
		// No bus (e.g. unit tests with no bus wired) — deny by default
		return fmt.Errorf("write blocked: path %q is outside allowed write directories (no confirmation bus)", path)
	}

	token := uuid.New().String()

	b.bus.Publish(events.NewTypedEvent(events.SourcePlugin, events.PromptRequestPayload{
		Type:  events.PromptTypeConfirm,
		Label: fmt.Sprintf("Write outside sandbox: %q — allow?", path),
		Token: token,
	}))

	ch, unsub := b.bus.SubscribeChan(1, events.EventPromptResponse)
	defer unsub()

	ctx, cancel := context.WithTimeout(ctx, writeConfirmTimeout)
	defer cancel()

	for {
		select {
		case event := <-ch:
			payload, ok := events.GetPromptResponsePayload(event)
			if !ok || payload.Token != token {
				continue
			}
			if payload.Cancelled {
				return fmt.Errorf("write denied by user: path %q is outside sandbox", path)
			}
			return nil // user approved
		case <-ctx.Done():
			return fmt.Errorf("write confirmation timed out for path %q", path)
		}
	}
}

func (b *OzzieBackend) resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if wd := events.WorkDirFromContext(ctx); wd != "" {
		return filepath.Join(wd, path)
	}
	return path
}

// LsInfo lists file information under the given path.
func (b *OzzieBackend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	dir := b.resolvePath(ctx, req.Path)
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ls: %w", err)
	}

	result := make([]filesystem.FileInfo, 0, len(entries))
	for _, e := range entries {
		entryPath := filepath.Join(dir, e.Name())
		if b.isRestrictedPath(entryPath) {
			continue
		}
		result = append(result, filesystem.FileInfo{
			Path: entryPath,
		})
	}
	return result, nil
}

// Read reads file content with support for line-based offset and limit.
func (b *OzzieBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (string, error) {
	if err := b.validateReadPath(ctx, req.FilePath); err != nil {
		return "", err
	}
	path := b.resolvePath(ctx, req.FilePath)

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("read: %s is a directory, not a file — use ls to list its contents", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	lines := bytes.Split(data, []byte("\n"))

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}

	if offset >= len(lines) {
		return "", nil
	}

	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	parts := make([]string, 0, end-offset)
	for _, l := range lines[offset:end] {
		parts = append(parts, string(l))
	}

	return strings.Join(parts, "\n"), nil
}

// GrepRaw searches for content matching the specified pattern in files.
// Pattern is treated as a literal string (not regex) per the Eino contract.
func (b *OzzieBackend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	searchPath := b.resolvePath(ctx, req.Path)
	if searchPath == "" {
		searchPath = "."
	}

	const maxMatches = 100
	var matches []filesystem.GrepMatch

	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Skip restricted directories
			if b.isRestrictedDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		// Apply glob filter
		if req.Glob != "" {
			matched, _ := filepath.Match(req.Glob, d.Name())
			if !matched {
				return nil
			}
		}

		// Skip binary files
		if isBinary(path) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(line, req.Pattern) {
				matches = append(matches, filesystem.GrepMatch{
					Path:    path,
					Line:    lineNum,
					Content: line,
				})
				if len(matches) >= maxMatches {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	return matches, nil
}

// GlobInfo returns file information matching the glob pattern.
// Uses doublestar for recursive ** glob support (e.g. "**/*.go").
func (b *OzzieBackend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	basePath := b.resolvePath(ctx, req.Path)
	if basePath == "" {
		basePath = "."
	}

	pattern := filepath.Join(basePath, req.Pattern)
	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	result := make([]filesystem.FileInfo, 0, len(matches))
	for _, m := range matches {
		if b.isRestrictedPath(m) {
			continue
		}
		result = append(result, filesystem.FileInfo{Path: m})
	}
	return result, nil
}

// Write creates or updates file content.
func (b *OzzieBackend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	if err := b.validateWritePath(ctx, req.FilePath); err != nil {
		return err
	}

	path := b.resolvePath(ctx, req.FilePath)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("write: resolve path: %w", err)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("write: create dirs: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(req.Content), 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// Edit replaces string occurrences in a file.
func (b *OzzieBackend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	if err := b.validateWritePath(ctx, req.FilePath); err != nil {
		return err
	}

	path := b.resolvePath(ctx, req.FilePath)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}

	content := string(data)

	if req.OldString == "" {
		return fmt.Errorf("edit: old_string is required")
	}

	count := strings.Count(content, req.OldString)
	if count == 0 {
		return fmt.Errorf("edit: old_string not found in file")
	}
	if !req.ReplaceAll && count > 1 {
		return fmt.Errorf("edit: old_string appears %d times (use replace_all=true)", count)
	}

	if req.ReplaceAll {
		content = strings.ReplaceAll(content, req.OldString, req.NewString)
	} else {
		content = strings.Replace(content, req.OldString, req.NewString, 1)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("edit: write: %w", err)
	}
	return nil
}

// isRestrictedDir checks if a directory path falls inside a restricted path.
func (b *OzzieBackend) isRestrictedDir(dirPath string) bool {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return false
	}
	for _, restricted := range b.readRestrictedPaths {
		clean := filepath.Clean(restricted)
		if isUnder(absPath, clean) {
			return true
		}
	}
	return false
}

// isRestrictedPath checks if a file/dir path falls inside a restricted path.
func (b *OzzieBackend) isRestrictedPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, restricted := range b.readRestrictedPaths {
		clean := filepath.Clean(restricted)
		if isUnder(absPath, clean) {
			return true
		}
	}
	return false
}

var _ filesystem.Backend = (*OzzieBackend)(nil)
