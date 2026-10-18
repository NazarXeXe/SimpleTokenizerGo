package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alexflint/go-arg"
)

var args struct {
	InputPath  string `arg:"-i,--inputFile" help:"input file (default: stdin)"`
	OutputPath string `arg:"-o,--outputFile" help:"output file (default: stdout)"`
}

type Token struct {
	Type  string
	Value string
	Pos   int
}

func tokenize(s string) []Token {
	var tokens []Token
	pos := 0
	for i := 0; i < len(s); {
		if s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' {
			i++
			pos++
			continue
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			i++
		}
		tokens = append(tokens, Token{Type: "word", Value: strings.TrimSpace(s[start:i]), Pos: pos})
		pos += i - start
	}
	return tokens
}

func main() {
	arg.MustParse(&args)

	in, err := openInput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tokenizer: %v\n", err)
		os.Exit(1)
	}
	defer in.Close()

	src, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tokenizer: read: %v\n", err)
		os.Exit(1)
	}

	out, err := openOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tokenizer: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	tokens := tokenize(string(src))
	for _, t := range tokens {
		if _, err := fmt.Fprintf(out, "%-5s @%-3d %q\n", t.Type, t.Pos, t.Value); err != nil {
			fmt.Fprintf(os.Stderr, "tokenizer: write: %v\n", err)
			os.Exit(1)
		}
	}
}

func openInput() (io.ReadCloser, error) {
	if args.InputPath == "" {
		return os.Stdin, nil
	}
	f, err := os.Open(args.InputPath)
	if err != nil {
		return nil, fmt.Errorf("open input: %w", err)
	}
	return f, nil
}

func openOutput() (io.WriteCloser, error) {
	if args.OutputPath == "" {
		return os.Stdout, nil
	}
	return os.Create(args.OutputPath)
}
