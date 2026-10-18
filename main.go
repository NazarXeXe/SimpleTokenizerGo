package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
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

func tokenType(raw string) string {
	i := 0
	// Optional sign.
	if i < len(raw) && (raw[i] == '+' || raw[i] == '-') {
		i++
	}
	intDigits := 0
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		i++
		intDigits++
	}
	fracDigits := 0
	isFloat := false
	if i < len(raw) && raw[i] == '.' {
		isFloat = true
		i++
		for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
			i++
			fracDigits++
		}
	}

	if i != len(raw) || (intDigits+fracDigits) == 0 {
		return "word"
	}
	if isFloat {
		return "float"
	}
	return "int"
}

type Text string

func (s Text) tokenize() []Token {
	raw := string(s)
	var tokens []Token
	pos := 0
	for i := 0; i < len(raw); {
		if raw[i] == ' ' || raw[i] == '	' || raw[i] == '\n' || raw[i] == '\r' {
			i++
			pos++
			continue
		}
		start := i
		for i < len(raw) && raw[i] != ' ' && raw[i] != '	' && raw[i] != '\n' && raw[i] != '\r' {
			i++
		}
		tokens = append(tokens, Token{Type: tokenType(raw[start:i]), Value: strings.TrimSpace(raw[start:i]), Pos: pos})
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

	tokens := Text(src).tokenize()
	w := csv.NewWriter(out)
	if err := w.Write([]string{"Type", "Value", "Pos"}); err != nil {
		fmt.Fprintf(os.Stderr, "tokenizer: write: %v\n", err)
		os.Exit(1)
	}
	for _, t := range tokens {
		if err := w.Write([]string{t.Type, t.Value, strconv.Itoa(t.Pos)}); err != nil {
			fmt.Fprintf(os.Stderr, "tokenizer: write: %v\n", err)
			os.Exit(1)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "tokenizer: write: %v\n", err)
		os.Exit(1)
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
