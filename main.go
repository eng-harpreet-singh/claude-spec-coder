// Command claude-spec-coder generates Go code from a Markdown specification
// using the Claude API. It runs in one of two modes: a single-shot generation
// from a spec, or an interactive refinement loop that maintains conversation
// history across turns.
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
		runOnce(gen, string(spec), *outPath)
	case "refine":
		runRefine(gen, string(spec), *outPath)
	default:
		log.Fatalf("unknown mode %q (use 'once' or 'refine')", *mode)
	}
}

func runOnce(gen *generator.Generator, spec, outPath string) {
	code, err := gen.GenerateOnce(context.Background(), spec)
	if err != nil {
		log.Fatalf("generation failed: %v", err)
	}
	writeFile(outPath, code)
	log.Printf("wrote %s", outPath)
}

func runRefine(gen *generator.Generator, spec, outPath string) {
	ctx := context.Background()
	conv := generator.NewConversation()

	code, _, hasCode, err := gen.Send(ctx, conv, spec)
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

		code, raw, hasCode, err := gen.Send(ctx, conv, input)
		if err != nil {
			log.Printf("send failed: %v", err)
			continue
		}

		if !hasCode {
			fmt.Println()
			fmt.Println(raw)
			fmt.Println()
			log.Printf("no code returned, %s unchanged (turn %d)", outPath, conv.TurnCount())
			continue
		}

		writeFile(outPath, code)
		log.Printf("wrote %s (turn %d, %d messages in history)", outPath, conv.TurnCount(), conv.MessageCount())
		fmt.Println()
	}
}

func writeFile(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}
