package generator

import (
	"context"
	"testing"
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

func TestExtractFencedBlock(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{"with language tag", "```go\npackage main\n```", "package main\n", true},
		{"without language tag", "```\npackage main\n```", "package main\n", true},
		{"no fence", "package main", "", false},
		{"unclosed fence", "```go\npackage main", "", false},
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

func TestStripTrailingFence(t *testing.T) {
	tests := []struct{ in, want string }{
		{"package main", "package main"},
		{"package main\n```", "package main"},
		{"package main\nfunc f() {}\n```", "package main\nfunc f() {}"},
	}
	for _, tc := range tests {
		if got := stripTrailingFence(tc.in); got != tc.want {
			t.Errorf("stripTrailingFence(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractCode_NilResponseReturnsFalse(t *testing.T) {
	if _, _, ok := extractCode(nil); ok {
		t.Error("extractCode(nil) returned ok=true")
	}
}
