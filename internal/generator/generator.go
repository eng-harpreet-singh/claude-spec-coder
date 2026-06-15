// Package generator wraps the Claude Messages API to produce Go code from
// a natural-language specification. It exposes one-shot and multi-turn
// patterns (with optional streaming) and provides token-aware history
// truncation for long conversations.
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
// output can be written directly to a .go file.
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

// TokenHandler receives each text delta as the model streams its reply.
type TokenHandler func(text string)

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

// MessageCount returns the total number of messages in the history.
func (c *Conversation) MessageCount() int { return len(c.messages) }

// TokenCount asks the API how many input tokens the current conversation
// would consume. Returns the exact billing-equivalent number.
func (g *Generator) TokenCount(ctx context.Context, conv *Conversation) (int, error) {
	if len(conv.messages) == 0 {
		return 0, nil
	}
	res, err := g.client.Messages.CountTokens(ctx, anthropic.MessageCountTokensParams{
		Model:    g.model,
		Messages: conv.messages,
	})
	if err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	return int(res.InputTokens), nil
}

// TruncateIfNeeded drops the oldest user+assistant pairs until the
// conversation's input token count is at or below maxInputTokens. Returns
// the number of messages that were dropped (0 if no truncation was needed).
//
// We always drop in pairs to keep the user/assistant alternation valid.
// The API rejects histories that start with an assistant message or have
// two consecutive messages of the same role.
func (g *Generator) TruncateIfNeeded(ctx context.Context, conv *Conversation, maxInputTokens int) (int, error) {
	if maxInputTokens <= 0 || len(conv.messages) == 0 {
		return 0, nil
	}

	dropped := 0
	for {
		tokens, err := g.TokenCount(ctx, conv)
		if err != nil {
			return dropped, err
		}
		if tokens <= maxInputTokens {
			return dropped, nil
		}

		// Drop the oldest two messages (one user, one assistant). If only
		// one message remains we cannot truncate further without leaving
		// an invalid history; bail out.
		if len(conv.messages) < 2 {
			return dropped, nil
		}
		conv.messages = conv.messages[2:]
		dropped += 2
	}
}

// GenerateOnce sends a single message and returns the generated code.
func (g *Generator) GenerateOnce(ctx context.Context, prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is empty")
	}

	resp, err := g.client.Messages.New(ctx, g.params([]anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}))
	if err != nil {
		return "", fmt.Errorf("claude messages: %w", err)
	}

	code, _, ok := extractFromMessage(resp)
	if !ok {
		return "", errors.New("claude returned no code")
	}
	return code, nil
}

// GenerateOnceStream is the streaming variant of GenerateOnce.
func (g *Generator) GenerateOnceStream(ctx context.Context, prompt string, onToken TokenHandler) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt is empty")
	}

	raw, err := g.stream(ctx, []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
	}, onToken)
	if err != nil {
		return "", err
	}

	code, ok := extractCode(raw)
	if !ok {
		return "", errors.New("claude returned no code")
	}
	return code, nil
}

// Send appends userMessage to conv, calls Claude with the full history,
// and appends the reply to conv.
func (g *Generator) Send(ctx context.Context, conv *Conversation, userMessage string) (code, raw string, hasCode bool, err error) {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return "", "", false, errors.New("user message is empty")
	}

	conv.messages = append(conv.messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	)

	resp, err := g.client.Messages.New(ctx, g.params(conv.messages))
	if err != nil {
		conv.messages = conv.messages[:len(conv.messages)-1]
		return "", "", false, fmt.Errorf("claude messages: %w", err)
	}

	code, raw, hasCode = extractFromMessage(resp)
	conv.messages = append(conv.messages,
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(raw)),
	)
	return code, raw, hasCode, nil
}

// SendStream is the streaming variant of Send.
func (g *Generator) SendStream(ctx context.Context, conv *Conversation, userMessage string, onToken TokenHandler) (code, raw string, hasCode bool, err error) {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		return "", "", false, errors.New("user message is empty")
	}

	conv.messages = append(conv.messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	)

	raw, err = g.stream(ctx, conv.messages, onToken)
	if err != nil {
		conv.messages = conv.messages[:len(conv.messages)-1]
		return "", "", false, err
	}

	code, hasCode = extractCode(raw)
	conv.messages = append(conv.messages,
		anthropic.NewAssistantMessage(anthropic.NewTextBlock(raw)),
	)
	return code, raw, hasCode, nil
}

func (g *Generator) params(messages []anthropic.MessageParam) anthropic.MessageNewParams {
	return anthropic.MessageNewParams{
		Model:       g.model,
		MaxTokens:   maxTokens,
		Temperature: anthropic.Float(0),
		System:      []anthropic.TextBlockParam{{Text: systemInstruction}},
		Messages:    messages,
	}
}

func (g *Generator) stream(ctx context.Context, messages []anthropic.MessageParam, onToken TokenHandler) (string, error) {
	stream := g.client.Messages.NewStreaming(ctx, g.params(messages))

	var b strings.Builder
	for stream.Next() {
		event := stream.Current()
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if td, ok := delta.Delta.AsAny().(anthropic.TextDelta); ok {
				b.WriteString(td.Text)
				if onToken != nil {
					onToken(td.Text)
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return "", fmt.Errorf("claude streaming: %w", err)
	}
	return b.String(), nil
}

func extractFromMessage(resp *anthropic.Message) (code, raw string, ok bool) {
	if resp == nil || len(resp.Content) == 0 {
		return "", "", false
	}
	var b strings.Builder
	for _, block := range resp.Content {
		b.WriteString(block.Text)
	}
	raw = b.String()
	code, ok = extractCode(raw)
	return code, raw, ok
}

func extractCode(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	switch {
	case strings.HasPrefix(trimmed, "package "):
		return stripTrailingFence(trimmed) + "\n", true
	case strings.Contains(trimmed, "```"):
		if fenced, found := extractFencedBlock(trimmed); found {
			fenced = strings.TrimSpace(fenced)
			if strings.HasPrefix(fenced, "package ") {
				return fenced + "\n", true
			}
		}
	}
	return "", false
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
