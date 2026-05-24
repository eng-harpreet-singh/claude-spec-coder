# claude-spec-coder

A command-line tool that generates Go code from a Markdown specification
by way of the Claude API. It runs in one of two modes:

| Mode | Description |
| --- | --- |
| `once` | One-shot generation. Read the spec, call Claude, write the output, exit. |
| `refine` | Generate from the spec, then accept refinement instructions on stdin. Each turn replays the full conversation history. |

The two modes share a single underlying API call. The only structural
difference is whether message history is accumulated and resent.

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

Interactive refinement:

```sh
go run . -mode=refine
```

Flags:

```
-mode  string   "once" or "refine" (default "once")
-spec  string   path to the specification file (default "spec.md")
-out   string   path to write generated code   (default "output/generated.go")
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

`main.go` is wiring only: flags, I/O, and the refine loop. All API
logic lives in `internal/generator`, which is independently testable.

## Tests

```sh
go test ./...
```

Tests cover the conversation lifecycle, input validation, and the
response parser. They do not require an API key or a network.

## Notes

- Temperature is fixed at 0. The same prompt produces the same output,
  which makes review tractable.
- The API key is read from `ANTHROPIC_API_KEY`. It is never embedded
  in source.
- The conversation history grows on every refine turn. Long sessions
  will eventually hit the model's context limit; truncation or
  summarisation belongs in a future change.

## License

MIT — see [LICENSE](LICENSE).
