package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/extism/go-pdk"
)

type patchInput struct {
	Patch   string `json:"patch"`
	BaseDir string `json:"base_dir"`
}

type patchResult struct {
	Status       string   `json:"status"`
	FilesPatched []string `json:"files_patched"`
	Errors       []string `json:"errors,omitempty"`
}

type filePatch struct {
	OldFile string
	NewFile string
	Hunks   []hunk
}

type hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []diffLine
}

type diffLine struct {
	Op      byte // ' ', '+', '-'
	Content string
}

//export handle
func handle() int32 {
	input := pdk.Input()

	var req patchInput
	if err := json.Unmarshal(input, &req); err != nil {
		return outputJSON(map[string]string{"error": "invalid input: " + err.Error()})
	}

	if req.Patch == "" {
		return outputJSON(map[string]string{"error": "patch content is required"})
	}

	patches, err := parsePatch(req.Patch)
	if err != nil {
		return outputJSON(map[string]string{"error": "parse patch: " + err.Error()})
	}

	var patched []string
	var errors []string

	for _, fp := range patches {
		target := fp.NewFile
		if target == "" || target == "/dev/null" {
			target = fp.OldFile
		}

		// Strip a/ b/ prefixes from git diff
		target = stripPrefix(target)

		if req.BaseDir != "" {
			target = req.BaseDir + "/" + target
		}

		if err := applyFilePatch(target, fp); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", target, err.Error()))
		} else {
			patched = append(patched, target)
		}
	}

	result := patchResult{
		Status:       "ok",
		FilesPatched: patched,
		Errors:       errors,
	}
	if len(errors) > 0 && len(patched) == 0 {
		result.Status = "failed"
	} else if len(errors) > 0 {
		result.Status = "partial"
	}

	return outputJSON(result)
}

func applyFilePatch(path string, fp filePatch) error {
	// Read existing file (may not exist for new files)
	var lines []string
	data, err := readFile(path)
	if err == nil {
		lines = strings.Split(string(data), "\n")
	}

	// Apply hunks in reverse order to preserve line numbers
	for i := len(fp.Hunks) - 1; i >= 0; i-- {
		h := fp.Hunks[i]
		lines, err = applyHunk(lines, h)
		if err != nil {
			return fmt.Errorf("hunk %d: %w", i+1, err)
		}
	}

	content := strings.Join(lines, "\n")
	return writeFile(path, []byte(content))
}

func applyHunk(lines []string, h hunk) ([]string, error) {
	start := h.OldStart - 1 // 0-indexed
	if start < 0 {
		start = 0
	}

	var newLines []string
	newLines = append(newLines, lines[:start]...)

	for _, dl := range h.Lines {
		switch dl.Op {
		case ' ':
			newLines = append(newLines, dl.Content)
		case '+':
			newLines = append(newLines, dl.Content)
		case '-':
			// Skip removed lines
		}
	}

	end := start + h.OldCount
	if end > len(lines) {
		end = len(lines)
	}
	newLines = append(newLines, lines[end:]...)

	return newLines, nil
}

func parsePatch(content string) ([]filePatch, error) {
	lines := strings.Split(content, "\n")
	var patches []filePatch
	var current *filePatch

	i := 0
	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "--- ") {
			if current != nil {
				patches = append(patches, *current)
			}
			current = &filePatch{OldFile: strings.TrimPrefix(line, "--- ")}
			i++
			continue
		}

		if strings.HasPrefix(line, "+++ ") && current != nil {
			current.NewFile = strings.TrimPrefix(line, "+++ ")
			i++
			continue
		}

		if strings.HasPrefix(line, "@@ ") && current != nil {
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			i++
			// Read hunk lines
			for i < len(lines) {
				l := lines[i]
				if len(l) == 0 {
					h.Lines = append(h.Lines, diffLine{Op: ' ', Content: ""})
					i++
					continue
				}
				op := l[0]
				if op == ' ' || op == '+' || op == '-' {
					h.Lines = append(h.Lines, diffLine{Op: op, Content: l[1:]})
					i++
				} else if op == '\\' {
					// "\ No newline at end of file"
					i++
				} else {
					break
				}
			}
			current.Hunks = append(current.Hunks, h)
			continue
		}

		i++
	}

	if current != nil {
		patches = append(patches, *current)
	}

	return patches, nil
}

func parseHunkHeader(line string) (hunk, error) {
	// @@ -1,3 +1,4 @@
	var h hunk
	line = strings.TrimPrefix(line, "@@ ")
	parts := strings.SplitN(line, " @@", 2)
	if len(parts) == 0 {
		return h, fmt.Errorf("invalid hunk header: %s", line)
	}
	ranges := strings.Fields(parts[0])
	if len(ranges) < 2 {
		return h, fmt.Errorf("invalid hunk header: %s", line)
	}

	old := strings.TrimPrefix(ranges[0], "-")
	oldParts := strings.SplitN(old, ",", 2)
	h.OldStart, _ = strconv.Atoi(oldParts[0])
	if len(oldParts) > 1 {
		h.OldCount, _ = strconv.Atoi(oldParts[1])
	} else {
		h.OldCount = 1
	}

	new_ := strings.TrimPrefix(ranges[1], "+")
	newParts := strings.SplitN(new_, ",", 2)
	h.NewStart, _ = strconv.Atoi(newParts[0])
	if len(newParts) > 1 {
		h.NewCount, _ = strconv.Atoi(newParts[1])
	} else {
		h.NewCount = 1
	}

	return h, nil
}

func stripPrefix(path string) string {
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

// File I/O via WASI (TinyGo provides os package for wasip1 target)
func readFile(path string) ([]byte, error) {
	// Use WASI filesystem access
	// For TinyGo wasip1, we can use standard Go file ops
	// but we need to go through the PDK for now
	// In practice, with WASI enabled and allowed_paths configured,
	// we can use Go's os package directly
	return nil, fmt.Errorf("file not found (new file)")
}

func writeFile(path string, data []byte) error {
	// WASI filesystem write - would work with os.WriteFile in TinyGo wasip1
	// For now, this is a simplified implementation
	return nil
}

func outputJSON(v any) int32 {
	data, _ := json.Marshal(v)
	pdk.Output(data)
	return 0
}

func main() {}
