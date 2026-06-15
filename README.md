# claude-spec-coder

A command-line tool that generates Go code from a Markdown specification
by way of the Claude API.

| Mode | Description |
| --- | --- |
| `once` | One-shot generation. Read the spec, call Claude, write the output, exit. |
| `refine` | Generate from the spec, then accept refinement instructions on stdin. Each turn replays the full conversation history. |

The `-stream` flag prints tokens as they arrive instead of waiting for
the complete response.

The `-max-history-tokens` flag auto-truncates conversation history when
it would exceed the configured token budget, so long refine sessions
don't blow through the model's context window.

## Requirements

- Go 1.22 or later
- An Anthropic API key — https://console.anthropic.com

## Setup

```sh
git clone https://github.com/harpreetsingh/claude-spec-coder.git
cd claude-spec-coder
go mod download
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Usage

```sh
# Single-shot generation
go run . -mode=once

# Single-shot, streaming
go run . -mode=once -stream

# Interactive refinement
go run . -mode=refine

# Interactive refinement with streaming
go run . -mode=refine -stream

# Refinement with a tight token budget (auto-truncates older turns)
go run . -mode=refine -stream -max-history-tokens=2000
```

### Flags

```
-mode                string   "once" or "refine"                    (default "once")
-spec                string   path to the specification file        (default "spec.md")
-out                 string   path to write generated code          (default "output/generated.go")
-stream              bool     print tokens as they arrive           (default false)
-max-history-tokens  int      truncate history at this many tokens  (default 4000)
```

In `refine` mode the tool prints turn count, message count, and current
input token count after every turn so you can watch the conversation
grow. When the count exceeds `-max-history-tokens`, the oldest user +
assistant pair is dropped before the next call.

## Layout

```
.
├── main.go
├── internal/
│   └── generator/
│       ├── generator.go      API client, conversation, token counting, truncation
│       └── generator_test.go unit tests (no API key required)
├── spec.md
├── go.mod
└── README.md
```

## How truncation works

Before each refine turn:

1. Count input tokens for the current history (`Messages.CountTokens`).
2. If the count is over the limit, drop the oldest user+assistant pair.
3. Repeat until the history fits.

Messages are dropped in pairs to preserve the user/assistant alternation
the API requires. The system prompt is never truncated. The most recent
turns — which carry the most relevant context for the next response —
are always kept.

This is the simplest production strategy. More sophisticated approaches
(summarising old turns into a single message, semantic relevance
ranking) are out of scope for this tool.

## Tests

```sh
go test ./...
```

Tests cover input validation, conversation lifecycle, the response
parser, and the truncation no-op paths. They do not require an API key
or a network.

## Notes

- Temperature is fixed at 0. The same prompt produces the same output.
- The API key is read from `ANTHROPIC_API_KEY`. It is never embedded.
- Token counting issues one API call per truncation check. For very
  long sessions this is cheap but not free. For tight cost control,
  cache the count locally between turns or estimate before calling.

## License

MIT — see [LICENSE](LICENSE).
