package main

import (
	"strings"
	"testing"
)

func TestIsDigit(t *testing.T) {
	for c := range 256 {
		want := c >= '0' && c <= '9'
		if got := isDigit(byte(c)); got != want {
			t.Errorf("isDigit(%q) = %v, want %v", rune(c), got, want)
		}
	}
}

func TestIsLetter(t *testing.T) {
	for _, c := range "aZ_9" {
		want := c == '_' || c == 'a' || c == 'Z'
		if got := isLetter(byte(c)); got != want {
			t.Errorf("isLetter(%q) = %v, want %v", c, got, want)
		}
	}
	// Everything outside a-z, A-Z, _ is not a letter.
	for c := range 256 {
		want := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if got := isLetter(byte(c)); got != want {
			t.Errorf("isLetter(%q) = %v, want %v", rune(c), got, want)
		}
	}
}

func TestScanNumber(t *testing.T) {
	tests := []struct {
		name string
		s    string
		pos  int
		want int
		ok   bool
	}{
		{"plain int", "42", 0, 2, true},
		{"negative int", "-42", 0, 3, true},
		{"positive int", "+7", 0, 2, true},
		{"decimal", "3.14", 0, 4, true},
		{"trailing dot", "5.", 0, 2, true},
		{"leading dot", ".5", 0, 2, true},
		{"signed leading dot", "-.5", 0, 3, true},
		{"stops at space", "12 ", 0, 2, true},
		{"stops at symbol", "12+3", 0, 2, true},
		{"only one dot", "1.2.3", 0, 3, true},
		{"mid-string offset", "x42", 1, 2, true},
		{"exponent glue", "1e3", 0, 0, false},
		{"unit suffix glue", "2.5x", 0, 0, false},
		{"underscore glue", "12_", 0, 0, false},
		{"sign only", "-", 0, 0, false},
		{"sign and dot only", "-.", 0, 0, false},
		{"dot only", ".", 0, 0, false},
		{"letters", "abc", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, ok := scanNumber(tt.s, tt.pos)
			if n != tt.want || ok != tt.ok {
				t.Errorf("scanNumber(%q, %d) = (%d, %v), want (%d, %v)",
					tt.s, tt.pos, n, ok, tt.want, tt.ok)
			}
		})
	}
}

func tok(typ, value string, pos int) Token { return Token{Type: typ, Value: value, Pos: pos} }

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Token
	}{
		{"empty input", "", nil},
		{"only whitespace", " \t\r\n  ", nil},
		{"two words", "hello world", []Token{
			tok("word", "hello", 0),
			tok("word", "world", 6),
		}},
		{"whitespace positions", "  x\ty\r\nz ", []Token{
			tok("word", "x", 2),
			tok("word", "y", 4),
			tok("word", "z", 7),
		}},
		{"dots stay in a word", "a.b.c", []Token{tok("word", "a.b.c", 0)}},
		{"underscore and digits in a word", "_x1", []Token{tok("word", "_x1", 0)}},
		{"word with trailing digits", "x2y", []Token{tok("word", "x2y", 0)}},
		{"int", "42", []Token{tok("int", "42", 0)}},
		{"negative int", "-42", []Token{tok("int", "-42", 0)}},
		{"positive int", "+7", []Token{tok("int", "+7", 0)}},
		{"float", "3.14", []Token{tok("float", "3.14", 0)}},
		{"float with trailing dot", "5.", []Token{tok("float", "5.", 0)}},
		{"float with leading dot", ".5", []Token{tok("float", ".5", 0)}},
		{"float signed leading dot", "-.5", []Token{tok("float", "-.5", 0)}},
		{"second dot starts a new number", "1.2.3", []Token{
			tok("float", "1.2", 0),
			tok("float", ".3", 3),
		}},
		{"two numbers", "3.14 2", []Token{
			tok("float", "3.14", 0),
			tok("int", "2", 5),
		}},
		// A digit run glued to an identifier char is a word, not a number.
		{"exponent is a word", "1e3", []Token{tok("word", "1e3", 0)}},
		{"unit suffix is a word", "2.5x", []Token{tok("word", "2.5x", 0)}},
		{"hex literal is a word", "0x1F", []Token{tok("word", "0x1F", 0)}},
		{"digit underscore glue is a word", "12_ab", []Token{tok("word", "12_ab", 0)}},
		{"letter-led exponent is a word", "e10", []Token{tok("word", "e10", 0)}},
		// Sign with no number after it is a symbol on its own.
		{"lone minus", "-", []Token{tok("symbol", "-", 0)}},
		{"minus before a word", "-a", []Token{
			tok("symbol", "-", 0),
			tok("word", "a", 1),
		}},
		{"minus dot", "-.", []Token{
			tok("symbol", "-", 0),
			tok("symbol", ".", 1),
		}},
		{"plus between words", "a+b", []Token{
			tok("word", "a", 0),
			tok("symbol", "+", 1),
			tok("word", "b", 2),
		}},
		{"punctuation is one symbol each", "a,b", []Token{
			tok("word", "a", 0),
			tok("symbol", ",", 1),
			tok("word", "b", 2),
		}},
		{"quoted words", `x = "a b"`, []Token{
			tok("word", "x", 0),
			tok("symbol", "=", 2),
			tok("symbol", `"`, 4),
			tok("word", "a", 5),
			tok("word", "b", 7),
			tok("symbol", `"`, 8),
		}},
		{"braces and operators", "{a[0]>=1.5;}", []Token{
			tok("symbol", "{", 0),
			tok("word", "a", 1),
			tok("symbol", "[", 2),
			tok("int", "0", 3),
			tok("symbol", "]", 4),
			tok("symbol", ">", 5),
			tok("symbol", "=", 6),
			tok("float", "1.5", 7),
			tok("symbol", ";", 10),
			tok("symbol", "}", 11),
		}},
		// A sign followed by a word: the sign fails the number scan, so it is a
		// symbol and the rest of the glue re-scans as a word.
		{"signed exponent", "+1.5e-3", []Token{
			tok("symbol", "+", 0),
			tok("word", "1.5e", 1),
			tok("int", "-3", 5),
		}},
		// Non-ASCII input is handled a byte at a time, so each UTF-8 byte of a
		// rune becomes its own symbol. Positions are byte offsets, and the
		// value is the raw source byte (0xCF, 0x80 for "π").
		{"multibyte rune is per-byte", "π", []Token{
			tok("symbol", "\xcf", 0),
			tok("symbol", "\x80", 1),
		}},
		{"multibyte between words", "π a", []Token{
			tok("symbol", "\xcf", 0),
			tok("symbol", "\x80", 1),
			tok("word", "a", 3),
		}},
		{"latin1 byte", "ü", []Token{
			tok("symbol", "\xc3", 0),
			tok("symbol", "\xbc", 1),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Text(tt.input).tokenize()
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q) returned %d tokens %v, want %d tokens %v",
					tt.input, len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize(%q)[%d] = %+v, want %+v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestTokenizeInvariants checks the properties that must hold for every input:
// tokens are ordered and non-overlapping, each token's value is the exact
// source substring at its position, and the only bytes left untokenized are
// whitespace.
func TestTokenizeInvariants(t *testing.T) {
	inputs := []string{
		"",
		" ",
		"hello, world!",
		"a.b.c 1e3 2.5x 3.14 -42 +.5 5.",
		"x=-1.5e-3*(a_1+2);",
		"tabs\tand\nnewlines\r\nmixed",
		"π + 1",
		"ünïcödé 42",
		"\x00\x01binary\xff",
		"....",
		"---+++",
		"1.2.3.4",
		strings.Repeat("ab 12.5 ", 20),
	}
	for _, in := range inputs {
		toks := Text(in).tokenize()
		covered := make([]bool, len(in))
		prevEnd := 0
		for i, tk := range toks {
			switch tk.Type {
			case "word", "int", "float", "symbol":
			default:
				t.Errorf("tokenize(%q)[%d]: unknown type %q", in, i, tk.Type)
			}
			if tk.Value == "" {
				t.Errorf("tokenize(%q)[%d]: empty value", in, i)
			}
			if tk.Pos < prevEnd {
				t.Errorf("tokenize(%q)[%d]: position %d overlaps previous token ending at %d",
					in, i, tk.Pos, prevEnd)
			}
			end := tk.Pos + len(tk.Value)
			if tk.Pos < 0 || end > len(in) {
				t.Errorf("tokenize(%q)[%d]: range [%d,%d) outside input of length %d",
					in, i, tk.Pos, end, len(in))
				continue
			}
			if got := in[tk.Pos:end]; got != tk.Value {
				t.Errorf("tokenize(%q)[%d]: value %q does not match source %q at position %d",
					in, i, tk.Value, got, tk.Pos)
			}
			for k := tk.Pos; k < end; k++ {
				if covered[k] {
					t.Errorf("tokenize(%q)[%d]: byte %d covered twice", in, i, k)
				}
				covered[k] = true
			}
			prevEnd = end
		}
		for k := 0; k < len(in); k++ {
			if !covered[k] && !isSpaceByte(in[k]) {
				t.Errorf("tokenize(%q): byte %d (%q) was not tokenized", in, k, in[k])
			}
		}
	}
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
