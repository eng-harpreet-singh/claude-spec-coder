# claude-spec-coder

A command-line tool that generates Go code from a Markdown specification
by way of the Claude API. It runs in one of two modes — single-shot
generation, or interactive refinement — and supports response streaming
in both.

| Mode | Description |
| --- | --- |
| `once` | One-shot generation. Read the spec, call Claude, write the output, exit. |
| `refine` | Generate from the spec, then accept refinement instructions on stdin. Each turn replays the full conversation history. |

The `-stream` flag prints tokens as they arrive, instead of waiting for
the complete response.

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

Single-shot generation from the included example spec:

```sh
go run . -mode=once
```

Single-shot generation, streaming the response to the terminal:

```sh
go run . -mode=once -stream
```

Interactive refinement:

```sh
go run . -mode=refine
```

Interactive refinement with streaming (recommended — gives faster
feedback as the model writes):

```sh
go run . -mode=refine -stream
```

Flags:

```
-mode    string   "once" or "refine"                 (default "once")
-spec    string   path to the specification file     (default "spec.md")
-out     string   path to write generated code       (default "output/generated.go")
-stream  bool     print tokens as they arrive        (default false)
```

In refine mode, type instructions at the `refine ▸` prompt. Type `exit`
to quit. When a refinement requires no change, Claude responds with a
short explanation and the output file is left as-is.

## Layout

```
.
├── main.go
├── internal/
│   └── generator/
│       ├── generator.go
│       └── generator_test.go
├── spec.md
├── go.mod
└── README.md
```

`main.go` is wiring only: flags, I/O, and the refine loop. All API logic
lives in `internal/generator`, which is independently testable.

## Tests

```sh
go test ./...
```

Tests cover input validation, the conversation lifecycle, and the
response parser. They do not require an API key or a network.

## Notes

- Temperature is fixed at 0. The same prompt produces the same output,
  which makes review tractable.
- The API key is read from `ANTHROPIC_API_KEY`. It is never embedded
  in source.
- Streaming responses reduces perceived latency, especially for longer
  outputs. The full response is still parsed at the end before the file
  is written.
- The conversation history grows on every refine turn. Long sessions
  will eventually hit the model's context limit; truncation or
  summarisation belongs in a future change.

## License

MIT — see [LICENSE](LICENSE).
