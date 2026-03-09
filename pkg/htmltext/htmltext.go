// Package htmltext converts HTML to clean plain text suitable for LLM
// consumption. It strips tags, collapses whitespace, decodes common HTML
// entities, and inserts newlines at block-level boundaries.
//
// No external dependencies — uses a simple state-machine parser.
package htmltext

import (
	"strings"
	"unicode/utf8"
)

// blockTags lists opening/closing tag prefixes that trigger a newline.
var blockTags = []string{
	"p>", "p ", "div>", "div ", "br>", "br/>", "br />",
	"h1>", "h1 ", "h2>", "h2 ", "h3>", "h3 ", "h4>", "h4 ",
	"h5>", "h5 ", "h6>", "h6 ",
	"li>", "li ", "tr>", "tr ", "td>", "td ",
	"blockquote>", "blockquote ",
	"pre>", "pre ", "hr>", "hr/>", "hr />",
	"header>", "header ", "footer>", "footer ",
	"section>", "section ", "article>", "article ",
	"nav>", "nav ", "aside>", "aside ",
	"figcaption>", "figcaption ",
	"dt>", "dt ", "dd>", "dd ",
}

// Extract strips HTML tags and returns clean plain text.
//
// Behaviour:
//   - <script> and <style> blocks are removed entirely.
//   - Block-level tags (p, div, br, h1–h6, li, tr, td, …) insert newlines.
//   - Common HTML entities (&nbsp; &amp; &lt; &gt; &quot;) are decoded.
//   - Consecutive whitespace is collapsed to a single space.
func Extract(html string) string {
	var sb strings.Builder
	sb.Grow(len(html) / 2)

	inTag := false
	inScript := false
	inStyle := false
	lastSpace := true

	lower := strings.ToLower(html)

	for i := 0; i < len(html); {
		r, size := utf8.DecodeRuneInString(html[i:])

		// Skip <script>…</script>
		if inScript {
			if i+9 <= len(lower) && lower[i:i+9] == "</script>" {
				inScript = false
				i += 9
				continue
			}
			i += size
			continue
		}

		// Skip <style>…</style>
		if inStyle {
			if i+8 <= len(lower) && lower[i:i+8] == "</style>" {
				inStyle = false
				i += 8
				continue
			}
			i += size
			continue
		}

		// Tag opening
		if r == '<' {
			rest := lower[i:]
			if strings.HasPrefix(rest, "<script") {
				inScript = true
				inTag = true
			} else if strings.HasPrefix(rest, "<style") {
				inStyle = true
				inTag = true
			} else {
				inTag = true
			}

			// Block-level tags → newline
			if len(rest) > 1 {
				tag := rest[1:]
				for _, bt := range blockTags {
					if strings.HasPrefix(tag, bt) || strings.HasPrefix(tag, "/"+bt[:len(bt)-1]) {
						if !lastSpace {
							sb.WriteByte('\n')
							lastSpace = true
						}
						break
					}
				}
			}

			i += size
			continue
		}

		// Tag closing
		if r == '>' {
			inTag = false
			i += size
			continue
		}

		// Inside a tag — skip content
		if inTag {
			i += size
			continue
		}

		// HTML entities
		if r == '&' {
			end := strings.IndexByte(html[i:], ';')
			if end > 0 && end < 10 {
				entity := html[i : i+end+1]
				switch entity {
				case "&nbsp;", "&#160;":
					sb.WriteByte(' ')
				case "&amp;":
					sb.WriteByte('&')
				case "&lt;":
					sb.WriteByte('<')
				case "&gt;":
					sb.WriteByte('>')
				case "&quot;":
					sb.WriteByte('"')
				case "&apos;", "&#39;":
					sb.WriteByte('\'')
				default:
					sb.WriteString(entity)
				}
				lastSpace = false
				i += end + 1
				continue
			}
		}

		// Collapse whitespace
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !lastSpace {
				sb.WriteByte(' ')
				lastSpace = true
			}
			i += size
			continue
		}

		sb.WriteRune(r)
		lastSpace = false
		i += size
	}

	return strings.TrimSpace(sb.String())
}
