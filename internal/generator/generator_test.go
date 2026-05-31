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

func TestGenerateOnceStream_RejectsEmptyPrompt(t *testing.T) {
	g := New("test-key")
	for _, in := range []string{"", "   "} {
		if _, err := g.GenerateOnceStream(context.Background(), in, nil); err == nil {
			t.Errorf("GenerateOnceStream(%q) returned nil error", in)
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

func TestSendStream_RejectsEmptyMessage(t *testing.T) {
	g := New("test-key")
	c := NewConversation()
	if _, _, _, err := g.SendStream(context.Background(), c, "   ", nil); err == nil {
		t.Fatal("SendStream with empty input returned nil error")
	}
	if c.MessageCount() != 0 {
		t.Errorf("history grew on rejected input: %d messages", c.MessageCount())
	}
}

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		hasCode bool
	}{
		{
			name:    "raw package declaration",
			in:      "package main\n\nfunc f() {}",
			want:    "package main\n\nfunc f() {}\n",
			hasCode: true,
		},
		{
			name:    "fenced with language tag",
			in:      "```go\npackage main\n\nfunc f() {}\n```",
			want:    "package main\n\nfunc f() {}\n",
			hasCode: true,
		},
		{
			name:    "fenced without language tag",
			in:      "```\npackage main\nfunc f() {}\n```",
			want:    "package main\nfunc f() {}\n",
			hasCode: true,
		},
		{
			name:    "prose preamble plus fenced code",
			in:      "No changes needed. Here is the file:\n\n```go\npackage main\nfunc f() {}\n```",
			want:    "package main\nfunc f() {}\n",
			hasCode: true,
		},
		{
			name:    "pure prose",
			in:      "The code is already correct.",
			hasCode: false,
		},
		{
			name:    "empty",
			in:      "   \n\t  ",
			hasCode: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractCode(tc.in)
			if ok != tc.hasCode {
				t.Fatalf("hasCode = %v, want %v (got %q)", ok, tc.hasCode, got)
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
