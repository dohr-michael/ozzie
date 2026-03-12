package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/dohr-michael/ozzie/internal/sessions"
)

func TestEnrichQueryWithSession(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		sess     *sessions.Session
		expected string
	}{
		{
			name:     "empty session",
			query:    "how to test",
			sess:     &sessions.Session{},
			expected: "how to test",
		},
		{
			name:  "with title and root dir",
			query: "how to test",
			sess: &sessions.Session{
				Title:   "Debug the API",
				RootDir: "/home/user/projects/myapp",
			},
			expected: "how to test Debug the API myapp",
		},
		{
			name:  "with title only",
			query: "how to test",
			sess: &sessions.Session{
				Title: "Debug the API",
			},
			expected: "how to test Debug the API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichQueryWithSession(tt.query, tt.sess)
			if got != tt.expected {
				t.Errorf("enrichQueryWithSession() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractSessionTags(t *testing.T) {
	tests := []struct {
		name     string
		sess     *sessions.Session
		expected []string
	}{
		{
			name:     "empty session",
			sess:     &sessions.Session{},
			expected: nil,
		},
		{
			name: "with language",
			sess: &sessions.Session{
				Language: "Go",
			},
			expected: []string{"go"},
		},
		{
			name: "with language and project",
			sess: &sessions.Session{
				Language: "Go",
				Metadata: map[string]string{"project": "Ozzie"},
			},
			expected: []string{"go", "ozzie"},
		},
		{
			name: "with project only",
			sess: &sessions.Session{
				Metadata: map[string]string{"project": "MyApp"},
			},
			expected: []string{"myapp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionTags(tt.sess)
			if len(got) != len(tt.expected) {
				t.Fatalf("extractSessionTags() returned %d tags, want %d", len(got), len(tt.expected))
			}
			for i, tag := range got {
				if tag != tt.expected[i] {
					t.Errorf("tag[%d] = %q, want %q", i, tag, tt.expected[i])
				}
			}
		})
	}
}

func TestRecentUserContext(t *testing.T) {
	tests := []struct {
		name     string
		messages []*schema.Message
		maxN     int
		expected string
	}{
		{
			name:     "no messages",
			messages: nil,
			maxN:     2,
			expected: "",
		},
		{
			name: "single user message (skipped as last)",
			messages: []*schema.Message{
				{Role: schema.User, Content: "hello"},
			},
			maxN:     2,
			expected: "",
		},
		{
			name: "two user messages",
			messages: []*schema.Message{
				{Role: schema.User, Content: "first question"},
				{Role: schema.Assistant, Content: "answer"},
				{Role: schema.User, Content: "second question"},
			},
			maxN:     2,
			expected: "first question",
		},
		{
			name: "truncates long messages",
			messages: []*schema.Message{
				{Role: schema.User, Content: string(make([]byte, 300))},
				{Role: schema.User, Content: "last"},
			},
			maxN:     2,
			expected: string(make([]byte, 200)),
		},
		{
			name: "respects maxN",
			messages: []*schema.Message{
				{Role: schema.User, Content: "a"},
				{Role: schema.User, Content: "b"},
				{Role: schema.User, Content: "c"},
				{Role: schema.User, Content: "last"},
			},
			maxN:     1,
			expected: "c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recentUserContext(tt.messages, tt.maxN)
			if got != tt.expected {
				t.Errorf("recentUserContext() = %q, want %q", got, tt.expected)
			}
		})
	}
}
