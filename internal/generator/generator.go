// Package generator wraps the Claude Messages API to produce Go code from
// a natural-language specification. It exposes two patterns: GenerateOnce
// for one-shot calls, and Conversation+Send for multi-turn refinement.
package generator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const defaultModel = "claude-sonnet-4-6"

const maxTokens = 2048

// systemInstruction constrains the model to return raw Go code so the
// output can be written directly to a .go file. The "no changes" clause
// is what lets callers detect a refinement that didn't require an edit.
const systemInstruction = `You are a senior Go engineer.
Produce production-quality Go code that satisfies the user's request.

Rules:
- Return ONLY valid, compilable Go code with brief comments.
- Do not wrap the code in Markdown fences.
- Do not include explanation outside the code.
- Use the standard library unless the user explicitly allows otherwise.
- When asked to change existing code, return the full updated file.
- If no change is needed, respond with a short plain-text explanation
  and no code.`

// Generator issues requests against the Claude Messages API.
type Generator struct {
	client anthropic.Client
	model  anthropic.Model
}

// New returns a Generator configured with the given API key.
func New(apiKey string) *Generator {
	return &Generator{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  anthropic.Model(defaultModel),
	}
}

// Conversation holds the message history for a multi-turn exchange.
// The Claude API is stateless; history is replayed on every call.
type Conversation struct {
	messages []anthropic.MessageParam
}

// NewConversation returns an empty Conversation.
func NewConversation() *Conversation { return &Conversation{} }

// TurnCount returns the number of user turns sent so far.
func (c *Conversation) TurnCount() int {
	n := 0
	for _, m := range c.messages {
		if m.Role == anthropic.MessageParamRoleUser {
			n++
		}
	}
	return n
}

// MessageCount returns the total number of messages in the history,
// counting user and assistant messages together.
func (c *Conversation) MessageCount() int { return len(c.messages) }

// GenerateOnce sends a single message to Claude and returns the generated
// code. It does not retain any history. An error is returned if Claude
// responds without code.
func (g *Generator) GenerateOnce(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is empty")
	}

	resp, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:       g.model,
		MaxTokens:   maxTokens,
		Temperature: anthropic.Float(0),
		System:      []anthropic.TextBlockParam{{Text: systemInstruction}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude messages: %w", err)
	}

	code, _, ok := extractCode(resp)
	if !ok {
		return "", errors.New("claude returned no code")
	}
	return code, nil
}

// Send appends userMessage to conv, calls Claude with the full history,
// and appends the reply to conv. It returns the extracted code, the raw
// reply text, and a flag indicating whether any code was found.
//
// When hasCode is false, the caller should display raw rather than write
// to disk. The assistant message is appended to history either way so
// subsequent turns have full context.
func (g *Generator) Send(ctx context.Context, conv *Conversation, userMessage string) (code, raw string, hasCode bool, err error) {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return "", "", false, errors.New("user message is empty")
	}

	conv.messages = append(conv.messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	)

	resp, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:       g.model,
		MaxTokens:   maxTokens,
		Temperature: anthropic.Float(0),
		System:      []anthropic.TextBlockParam{{Text: systemInstruction}},
		Messages:    conv.messages,
	})
	if err != nil {
		// Roll back so a retry does not double-append.
		conv.messages = conv.messages[:len(conv.messages)-1]
		return "", "", false, fmt.Errorf("claude messages: %w", err)
	}

	code, raw, hasCode = extractCode(resp)
	conv.messages = append(conv.messages,
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(raw)),
	)
	return code, raw, hasCode, nil
}

// extractCode pulls Go source out of a Claude response. The system prompt
// asks for raw code, but the model occasionally returns prose with a
// fenced block ("no changes needed, here's the file"), so both shapes
// are handled.
func extractCode(resp *anthropic.Message) (code, raw string, ok bool) {
	if resp == nil || len(resp.Content) == 0 {
		return "", "", false
	}

	var b strings.Builder
	for _, block := range resp.Content {
		b.WriteString(block.Text)
	}
	raw = b.String()

	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "package "):
		return stripTrailingFence(trimmed) + "\n", raw, true
	case strings.Contains(trimmed, "```"):
		if fenced, found := extractFencedBlock(trimmed); found {
			fenced = strings.TrimSpace(fenced)
			if strings.HasPrefix(fenced, "package ") {
				return fenced + "\n", raw, true
			}
		}
	}
	return "", raw, false
}

func extractFencedBlock(s string) (string, bool) {
	start := strings.Index(s, "```")
	if start < 0 {
		return "", false
	}
	rest := s[start+3:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return "", false
	}
	rest = rest[nl+1:]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func stripTrailingFence(s string) string {
	if i := strings.LastIndex(s, "\n```"); i > 0 && strings.TrimSpace(s[i:]) == "```" {
		return strings.TrimSpace(s[:i])
	}
	return s
}
