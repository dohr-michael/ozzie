// Package components provides reusable TUI components.
package components

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

var (
	markdownRenderer     *glamour.TermRenderer
	markdownRendererOnce sync.Once
)

// customStyleConfig returns a glamour style that matches our TUI theme.
// Colors:
//   - Primary (violet): #7C3AED
//   - Secondary (green): #10B981
//   - Error (red): #EF4444
//   - Warning (amber): #F59E0B
//   - Muted (gray): #6B7280
func customStyleConfig() ansi.StyleConfig {
	// Base on dark theme, customize to match our palette
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "",
				BlockSuffix: "",
				Color:       stringPtr("#E5E7EB"), // Light gray for base text
			},
			Margin: uintPtr(0),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#6B7280"), // Muted
				Italic: boolPtr(true),
			},
			Indent:      uintPtr(2),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#7C3AED"), // Primary violet
				Bold:  boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#7C3AED"),
				Bold:   boolPtr(true),
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#7C3AED"),
				Bold:   boolPtr(true),
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#10B981"), // Green for h3+
				Bold:   boolPtr(true),
				Prefix: "### ",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#10B981"),
				Bold:   boolPtr(true),
				Prefix: "#### ",
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#10B981"),
				Prefix: "##### ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  stringPtr("#6B7280"),
				Prefix: "###### ",
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
			Color:  stringPtr("#E5E7EB"),
		},
		Strong: ansi.StylePrimitive{
			Bold:  boolPtr(true),
			Color: stringPtr("#FFFFFF"),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("#374151"),
			Format: "─────────────────────────────────────────",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       stringPtr("#E5E7EB"),
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       stringPtr("#E5E7EB"),
		},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#E5E7EB"),
			},
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("#60A5FA"), // Blue for links
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("#60A5FA"),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr("#F59E0B"), // Amber for images
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr("#F59E0B"),
			Format: "🖼  {{.text}}",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           stringPtr("#F59E0B"), // Amber for inline code
				BackgroundColor: stringPtr("#1F2937"),
				Prefix:          " ",
				Suffix:          " ",
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
				Margin: uintPtr(0),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
				Error: ansi.StylePrimitive{
					Color: stringPtr("#EF4444"),
				},
				Comment: ansi.StylePrimitive{
					Color:  stringPtr("#6B7280"),
					Italic: boolPtr(true),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
					Bold:  boolPtr(true),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
					Bold:  boolPtr(true),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
				},
				KeywordType: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				Operator: ansi.StylePrimitive{
					Color: stringPtr("#F59E0B"),
				},
				Punctuation: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				NameTag: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				NameClass: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
					Bold:  boolPtr(true),
				},
				NameConstant: ansi.StylePrimitive{
					Color: stringPtr("#F59E0B"),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: stringPtr("#F59E0B"),
				},
				NameFunction: ansi.StylePrimitive{
					Color: stringPtr("#60A5FA"),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: stringPtr("#F59E0B"),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: stringPtr("#F59E0B"),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: stringPtr("#EF4444"),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: stringPtr("#10B981"),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: stringPtr("#7C3AED"),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: stringPtr("#1F2937"),
				},
			},
		},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("#E5E7EB"),
				},
			},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionList: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#E5E7EB"),
			},
		},
		DefinitionTerm: ansi.StylePrimitive{
			Color: stringPtr("#7C3AED"),
			Bold:  boolPtr(true),
		},
		DefinitionDescription: ansi.StylePrimitive{
			Color:       stringPtr("#E5E7EB"),
			BlockPrefix: "  ",
		},
	}
}

// Helper functions for pointer creation
func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

// GetMarkdownRenderer returns a singleton markdown renderer.
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	markdownRendererOnce.Do(func() {
		markdownRenderer, _ = glamour.NewTermRenderer(
			glamour.WithStyles(customStyleConfig()),
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
	})
	return markdownRenderer
}

// RenderMarkdown renders markdown content to styled terminal output.
// If rendering fails, returns the original content.
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}

	renderer := GetMarkdownRenderer(width)
	if renderer == nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	// Trim trailing newlines that glamour adds
	return strings.TrimRight(rendered, "\n")
}

// RenderMarkdownWithWidth creates a new renderer with specific width.
// Use this when width changes dynamically.
func RenderMarkdownWithWidth(content string, width int) string {
	if content == "" {
		return ""
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(customStyleConfig()),
		glamour.WithWordWrap(width),
		glamour.WithEmoji(),
	)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(rendered, "\n")
}
