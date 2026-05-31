// Command claude-spec-coder generates Go code from a Markdown specification
// using the Claude API. It runs in one of two modes — single-shot generation
// from a spec, or an interactive refinement loop that maintains conversation
// history across turns — and supports response streaming in both.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/harpreetsingh/claude-spec-coder/internal/generator"
)

func main() {
	mode := flag.String("mode", "once", "operation mode: 'once' or 'refine'")
	specPath := flag.String("spec", "spec.md", "path to the specification file")
	outPath := flag.String("out", "output/generated.go", "path to write the generated code")
	stream := flag.Bool("stream", false, "print tokens as they arrive instead of waiting for the full response")
	flag.Parse()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}

	spec, err := os.ReadFile(*specPath)
	if err != nil {
		log.Fatalf("read spec %q: %v", *specPath, err)
	}

	gen := generator.New(apiKey)

	switch *mode {
	case "once":
		runOnce(gen, string(spec), *outPath, *stream)
	case "refine":
		runRefine(gen, string(spec), *outPath, *stream)
	default:
		log.Fatalf("unknown mode %q (use 'once' or 'refine')", *mode)
	}
}

func runOnce(gen *generator.Generator, spec, outPath string, stream bool) {
	ctx := context.Background()

	var (
		code string
		err  error
	)
	if stream {
		code, err = gen.GenerateOnceStream(ctx, spec, printToken)
		fmt.Println()
	} else {
		code, err = gen.GenerateOnce(ctx, spec)
	}
	if err != nil {
		log.Fatalf("generation failed: %v", err)
	}

	writeFile(outPath, code)
	log.Printf("wrote %s", outPath)
}

func runRefine(gen *generator.Generator, spec, outPath string, stream bool) {
	ctx := context.Background()
	conv := generator.NewConversation()

	code, _, hasCode, err := sendOnce(ctx, gen, conv, spec, stream)
	if err != nil {
		log.Fatalf("initial generation failed: %v", err)
	}
	if !hasCode {
		log.Fatal("initial generation returned no code")
	}
	writeFile(outPath, code)
	log.Printf("wrote %s (turn %d)", outPath, conv.TurnCount())
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("refine ▸ ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			return
		}

		code, raw, hasCode, err := sendOnce(ctx, gen, conv, input, stream)
		if err != nil {
			log.Printf("send failed: %v", err)
			continue
		}

		if !hasCode {
			if !stream {
				fmt.Println()
				fmt.Println(raw)
			}
			fmt.Println()
			log.Printf("no code returned, %s unchanged (turn %d)", outPath, conv.TurnCount())
			continue
		}

		writeFile(outPath, code)
		log.Printf("wrote %s (turn %d, %d messages in history)", outPath, conv.TurnCount(), conv.MessageCount())
		fmt.Println()
	}
}

// sendOnce dispatches to the streaming or buffered Send depending on the flag.
// Streaming prints tokens to stdout as they arrive; buffered returns the full
// reply at the end.
func sendOnce(ctx context.Context, gen *generator.Generator, conv *generator.Conversation, msg string, stream bool) (code, raw string, hasCode bool, err error) {
	if stream {
		code, raw, hasCode, err = gen.SendStream(ctx, conv, msg, printToken)
		fmt.Println()
		return
	}
	return gen.Send(ctx, conv, msg)
}

// printToken writes a streamed token to stdout and flushes so it appears
// immediately. Without the flush the terminal would buffer until a newline.
func printToken(t string) {
	os.Stdout.WriteString(t)
}

func writeFile(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}
