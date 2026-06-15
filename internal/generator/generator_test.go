package generator

import (
	"context"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestNewConversation_StartsEmpty(t *testing.T) {
	c := NewConversation()
	if got := c.TurnCount(); got != 0 {
		t.Errorf("TurnCount = %d, want 0", got)
	}
	if got := c.MessageCount(); got != 0 {
		t.Errorf("MessageCount = %d, want 0", got)
	}
}

func TestGenerateOnce_RejectsEmptyPrompt(t *testing.T) {
	g := New("test-key")
	for _, in := range []string{"", "   ", "\n\t\n"} {
		if _, err := g.GenerateOnce(context.Background(), in); err == nil {
			t.Errorf("GenerateOnce(%q) returned nil error", in)
		}
	}
}

func TestSend_RejectsEmptyMessage(t *testing.T) {
	g := New("test-key")
	c := NewConversation()
	if _, _, _, err := g.Send(context.Background(), c, "   "); err == nil {
		t.Fatal("Send with empty input returned nil error")
	}
	if c.MessageCount() != 0 {
		t.Errorf("history grew on rejected input: %d messages", c.MessageCount())
	}
}

// TestTruncateIfNeeded_NoOpWhenEmpty verifies that truncation on an empty
// conversation does nothing.
func TestTruncateIfNeeded_NoOpWhenEmpty(t *testing.T) {
	g := New("test-key")
	c := NewConversation()
	dropped, err := g.TruncateIfNeeded(context.Background(), c, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
}

// TestTruncateIfNeeded_DisabledWhenMaxZero verifies that a zero or negative
// limit disables truncation entirely.
func TestTruncateIfNeeded_DisabledWhenMaxZero(t *testing.T) {
	g := New("test-key")
	c := &Conversation{
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("world")),
		},
	}
	dropped, err := g.TruncateIfNeeded(context.Background(), c, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	if c.MessageCount() != 2 {
		t.Errorf("messages dropped despite disabled limit")
	}
}

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		hasCode bool
	}{
		{"raw package", "package main\nfunc f() {}", "package main\nfunc f() {}\n", true},
		{"fenced go", "```go\npackage main\nfunc f() {}\n```", "package main\nfunc f() {}\n", true},
		{"fenced no lang", "```\npackage main\n```", "package main\n", true},
		{"prose plus fenced", "No changes.\n\n```go\npackage main\n```", "package main\n", true},
		{"pure prose", "Already correct.", "", false},
		{"empty", "   ", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractCode(tc.in)
			if ok != tc.hasCode {
				t.Fatalf("hasCode = %v, want %v", ok, tc.hasCode)
			}
			if ok && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractFencedBlock(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{"with tag", "```go\npackage main\n```", "package main\n", true},
		{"no tag", "```\npackage main\n```", "package main\n", true},
		{"no fence", "package main", "", false},
		{"unclosed", "```go\npackage main", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractFencedBlock(tc.in)
			if ok != tc.found {
				t.Fatalf("found = %v, want %v", ok, tc.found)
			}
			if ok && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
