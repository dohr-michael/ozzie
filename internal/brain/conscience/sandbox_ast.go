package conscience

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// denylistEntry defines a command and optional flags that make it dangerous.
// If Flags is nil, the command is always blocked.
type denylistEntry struct {
	Flags  []string // nil = always blocked; non-nil = blocked when ANY flag is present
	Reason string
}

// denylist maps binary names to their danger rules.
var denylist = map[string]denylistEntry{
	// Always blocked — privilege escalation / eval
	"sudo":   {Reason: "privilege escalation"},
	"doas":   {Reason: "privilege escalation"},
	"pkexec": {Reason: "privilege escalation"},
	"su":     {Reason: "switch user"},
	"eval":   {Reason: "eval execution"},
	"source": {Reason: "source execution"},
	".":      {Reason: "source execution"},

	// Always blocked — disk/partition
	"mkfs":  {Reason: "filesystem format"},
	"fdisk": {Reason: "partition edit"},

	// Blocked with specific flags
	"rm":    {Flags: []string{"r", "R", "f", "F"}, Reason: "destructive remove"},
	"chmod": {Flags: []string{"R"}, Reason: "recursive chmod"},
	"chown": {Flags: []string{"R"}, Reason: "recursive chown"},
	"find":  {Flags: []string{"delete", "exec", "execdir"}, Reason: "destructive find"},
}

// ddEntry is handled separately since "dd" danger is based on of= arg presence.
const ddReason = "raw disk write (dd)"

// validateCommandAST parses a shell command string into an AST and walks it
// to detect dangerous commands. Returns an error if a dangerous pattern is found.
func validateCommandAST(command string) error {
	prog, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		// Unparseable command — block conservatively
		return fmt.Errorf("unparseable command: %w", err)
	}

	var walkErr error
	syntax.Walk(prog, func(node syntax.Node) bool {
		if walkErr != nil {
			return false
		}
		switch n := node.(type) {
		case *syntax.CallExpr:
			walkErr = checkCallExpr(n)
		case *syntax.Redirect:
			walkErr = checkRedirect(n)
		case *syntax.FuncDecl:
			walkErr = checkForkBomb(n)
		}
		return walkErr == nil
	})
	return walkErr
}

// checkCallExpr checks a simple command (binary + args) against the denylist.
func checkCallExpr(call *syntax.CallExpr) error {
	if len(call.Args) == 0 {
		return nil
	}

	name := resolveWord(call.Args[0])
	if name == "" {
		// Dynamic command ($cmd or similar) — block conservatively
		if containsParamExp(call.Args[0]) {
			return fmt.Errorf("dynamic command (variable expansion) not allowed")
		}
		return nil
	}

	// dd special case: blocked if of=... present
	if name == "dd" {
		for _, arg := range call.Args[1:] {
			w := resolveWord(arg)
			if strings.HasPrefix(w, "of=") {
				return fmt.Errorf("blocked: %s", ddReason)
			}
		}
		return nil
	}

	entry, found := denylist[name]
	if !found {
		// Check prefix match for commands like mkfs.ext4 → mkfs
		if idx := strings.IndexByte(name, '.'); idx > 0 {
			entry, found = denylist[name[:idx]]
		}
	}
	if !found {
		return nil
	}

	// Always blocked (no flag condition)
	if entry.Flags == nil {
		return fmt.Errorf("blocked: %s (%s)", name, entry.Reason)
	}

	// Check flags and arguments
	flags := extractFlags(call.Args[1:])
	args := extractArgValues(call.Args[1:])
	for _, f := range entry.Flags {
		if flags[f] || args["-"+f] {
			return fmt.Errorf("blocked: %s with -%s (%s)", name, f, entry.Reason)
		}
	}

	return nil
}

// checkRedirect detects writes to raw device paths (>/dev/sd*).
func checkRedirect(redir *syntax.Redirect) error {
	if redir.Op != syntax.RdrOut && redir.Op != syntax.AppOut && redir.Op != syntax.RdrAll {
		return nil
	}
	if redir.Word == nil {
		return nil
	}
	target := resolveWord(redir.Word)
	if strings.HasPrefix(target, "/dev/sd") || strings.HasPrefix(target, "/dev/nvme") {
		return fmt.Errorf("blocked: raw device write to %s", target)
	}
	return nil
}

// checkForkBomb detects function declarations that call themselves (basic fork bomb pattern).
func checkForkBomb(fn *syntax.FuncDecl) error {
	fnName := fn.Name.Value
	var found bool
	syntax.Walk(fn.Body, func(node syntax.Node) bool {
		if found {
			return false
		}
		if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			if resolveWord(call.Args[0]) == fnName {
				found = true
			}
		}
		return !found
	})
	if found {
		return fmt.Errorf("blocked: fork bomb (self-recursive function %q)", fnName)
	}
	return nil
}

// extractCommandPathsAST parses a command and returns all file-path-like arguments.
// Used by constraint validation to check path restrictions.
func extractCommandPathsAST(command string) []string {
	prog, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}

	var paths []string
	syntax.Walk(prog, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		// Skip the binary itself (first arg), collect path-like args
		for i, arg := range call.Args {
			if i == 0 {
				continue
			}
			w := resolveWord(arg)
			if looksLikePath(w) {
				paths = append(paths, w)
			}
		}
		return true
	})

	// Also collect redirect targets from Stmts
	syntax.Walk(prog, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		for _, redir := range stmt.Redirs {
			if redir.Word != nil {
				w := resolveWord(redir.Word)
				if looksLikePath(w) {
					paths = append(paths, w)
				}
			}
		}
		return true
	})
	return paths
}

// extractAllBinariesAST parses a command and returns the binary name from each
// simple command, descending into subshells and command substitutions.
func extractAllBinariesAST(command string) []string {
	prog, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}

	var binaries []string
	syntax.Walk(prog, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		name := resolveWord(call.Args[0])
		if name != "" {
			binaries = append(binaries, name)
		}
		return true
	})
	return binaries
}

// containsSubshellAST returns true if the command contains subshell, command
// substitution, or process substitution syntax.
func containsSubshellAST(command string) bool {
	prog, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return true // Conservative: unparseable → assume subshell
	}

	var found bool
	syntax.Walk(prog, func(node syntax.Node) bool {
		if found {
			return false
		}
		switch node.(type) {
		case *syntax.CmdSubst: // $(...) and backticks
			found = true
		case *syntax.Subshell: // (...)
			found = true
		case *syntax.ProcSubst: // <(...) and >(...)
			found = true
		}
		return !found
	})
	return found
}

// --- helpers ---

// resolveWord extracts the literal string from a shell Word.
// Returns "" if the word contains unexpandable parts (variables, etc).
func resolveWord(w *syntax.Word) string {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			// Only resolve if all parts are literals
			for _, dp := range p.Parts {
				lit, ok := dp.(*syntax.Lit)
				if !ok {
					return ""
				}
				sb.WriteString(lit.Value)
			}
		default:
			return "" // ParamExp, CmdSubst, etc — can't resolve statically
		}
	}
	return sb.String()
}

// containsParamExp returns true if a Word contains parameter expansion ($var).
func containsParamExp(w *syntax.Word) bool {
	for _, part := range w.Parts {
		switch part.(type) {
		case *syntax.ParamExp:
			return true
		case *syntax.CmdSubst:
			return true
		}
	}
	return false
}

// extractFlags collects single-char and long flags from argument words.
// Handles: -rf (→ {r, f}), --recursive (→ {recursive}), -R (→ {R}).
func extractFlags(args []*syntax.Word) map[string]bool {
	flags := make(map[string]bool)
	for _, arg := range args {
		w := resolveWord(arg)
		if w == "" || w == "-" || w == "--" {
			continue
		}
		if strings.HasPrefix(w, "--") {
			// Long flag: --recursive → "recursive"
			flags[strings.TrimPrefix(w, "--")] = true
		} else if strings.HasPrefix(w, "-") {
			// Short flags: -rf → "r", "f"
			for _, c := range w[1:] {
				flags[string(c)] = true
			}
		}
	}
	return flags
}

// extractArgValues collects the literal values of arguments as a set.
// Used for matching find-style actions like -delete, -exec, -execdir.
func extractArgValues(args []*syntax.Word) map[string]bool {
	vals := make(map[string]bool)
	for _, arg := range args {
		w := resolveWord(arg)
		if w != "" {
			vals[w] = true
		}
	}
	return vals
}

// looksLikePath returns true if a string looks like a filesystem path.
func looksLikePath(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "/") ||
		strings.HasPrefix(s, "~/") ||
		strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, "./")
}
