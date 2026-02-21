package main

import (
	"encoding/json"
	"strings"

	"github.com/extism/go-pdk"
)

type crawlerInput struct {
	URL string `json:"url"`
}

type crawlerOutput struct {
	URL     string `json:"url"`
	Content string `json:"content"`
	Length  int    `json:"length"`
}

type crawlerError struct {
	Error string `json:"error"`
}

//export handle
func handle() int32 {
	input := pdk.Input()

	var req crawlerInput
	if err := json.Unmarshal(input, &req); err != nil {
		return outputError("invalid input: " + err.Error())
	}

	if req.URL == "" {
		return outputError("url is required")
	}

	// Use Extism HTTP to fetch the page
	httpReq := pdk.NewHTTPRequest(pdk.MethodGet, req.URL)
	resp := httpReq.Send()

	if resp.Status() >= 400 {
		return outputError("HTTP error: status " + statusText(resp.Status()))
	}

	body := string(resp.Body())

	// Strip HTML tags to extract text
	text := stripHTML(body)
	text = collapseWhitespace(text)

	// Truncate to reasonable size
	if len(text) > 10000 {
		text = text[:10000] + "\n... (truncated)"
	}

	out, _ := json.Marshal(crawlerOutput{
		URL:     req.URL,
		Content: text,
		Length:  len(text),
	})
	pdk.Output(out)
	return 0
}

func outputError(msg string) int32 {
	out, _ := json.Marshal(crawlerError{Error: msg})
	pdk.Output(out)
	return 1
}

// stripHTML removes HTML tags using a simple state machine.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// collapseWhitespace reduces multiple whitespace to single spaces and trims.
func collapseWhitespace(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range s {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if isSpace {
			if !prev {
				b.WriteRune(' ')
			}
			prev = true
		} else {
			b.WriteRune(r)
			prev = false
		}
	}
	return strings.TrimSpace(b.String())
}

func statusText(code uint16) string {
	switch code {
	case 400:
		return "400 Bad Request"
	case 401:
		return "401 Unauthorized"
	case 403:
		return "403 Forbidden"
	case 404:
		return "404 Not Found"
	case 500:
		return "500 Internal Server Error"
	default:
		return string(rune('0'+code/100)) + "xx"
	}
}

func main() {}
