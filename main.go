package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

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

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func scanNumber(s string, pos int) (int, bool) {
	i := pos
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	intDigits := 0
	for i < len(s) && isDigit(s[i]) {
		i++
		intDigits++
	}
	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		// Treat as a decimal point only if followed by a digit (".5", "3.14")
		// or if there are already integer digits (trailing dot "5.").
		if (i+1 < len(s) && isDigit(s[i+1])) || intDigits > 0 {
			i++
			for i < len(s) && isDigit(s[i]) {
				i++
				fracDigits++
			}
		}
	}
	if intDigits+fracDigits == 0 {
		return 0, false
	}
	// Glued to a following identifier char -> not a standalone number.
	if i < len(s) && isLetter(s[i]) {
		return 0, false
	}
	return i - pos, true
}

type Text string

func (s Text) tokenize() []Token {
	raw := string(s)
	var tokens []Token
	i := 0
	for i < len(raw) {
		c := raw[i]
		switch {
		case c == ' ' || c == '	' || c == '\n' || c == '\r':
			i++
		case isLetter(c):
			// Word: letters, digits, and dots (so "2.5x", "1e3", "a.b" group).
			start := i
			for i < len(raw) && (isLetter(raw[i]) || isDigit(raw[i]) || raw[i] == '.') {
				i++
			}
			tokens = append(tokens, Token{Type: "word", Value: raw[start:i], Pos: start})
		case isDigit(c) || ((c == '+' || c == '-') && i+1 < len(raw) && (isDigit(raw[i+1]) || raw[i+1] == '.')) ||
			(c == '.' && i+1 < len(raw) && isDigit(raw[i+1])):
			// Number (possibly signed or leading-dot).
			n, ok := scanNumber(raw, i)
			if ok {
				typ := "int"
				for j := i; j < i+n; j++ {
					if raw[j] == '.' {
						typ = "float"
						break
					}
				}
				tokens = append(tokens, Token{Type: typ, Value: raw[i : i+n], Pos: i})
				i += n
			} else if isDigit(c) {
				// Digit-led run that is not a standalone number (glued to a letter, e.g. "2.5x" / "1e3") -> treat as a word.
				start := i
				for i < len(raw) && (isLetter(raw[i]) || isDigit(raw[i]) || raw[i] == '.') {
					i++
				}
				tokens = append(tokens, Token{Type: "word", Value: raw[start:i], Pos: start})
			} else {
				// Sign or dot alone (no number follows) -> a symbol.
				tokens = append(tokens, Token{Type: "symbol", Value: string(c), Pos: i})
				i++
			}
		default:
			// Every other character is a special symbol on its own.
			tokens = append(tokens, Token{Type: "symbol", Value: string(c), Pos: i})
			i++
		}
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
