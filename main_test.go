package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- in-process tests of the CSV writer -------------------------------------

func TestWriteTokens(t *testing.T) {
	tests := []struct {
		name   string
		tokens []Token
		want   string
	}{
		{
			name:   "header only",
			tokens: nil,
			want:   "Type,Value,Pos\n",
		},
		{
			name: "word symbol int",
			tokens: []Token{
				tok("word", "x", 0),
				tok("symbol", "=", 2),
				tok("int", "1", 4),
			},
			want: "Type,Value,Pos\nword,x,0\nsymbol,=,2\nint,1,4\n",
		},
		{
			name:   "comma value is quoted",
			tokens: []Token{tok("symbol", ",", 1)},
			want:   "Type,Value,Pos\nsymbol,\",\",1\n",
		},
		{
			name:   "quote value is escaped",
			tokens: []Token{tok("symbol", `"`, 0)},
			want:   "Type,Value,Pos\nsymbol,\"\"\"\",0\n",
		},
		{
			name:   "newline value is quoted",
			tokens: []Token{tok("symbol", "\n", 0)},
			want:   "Type,Value,Pos\nsymbol,\"\n\",0\n",
		},
		{
			name:   "negative float",
			tokens: []Token{tok("float", "-1.5", 3)},
			want:   "Type,Value,Pos\nfloat,-1.5,3\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeTokens(&buf, tt.tokens); err != nil {
				t.Fatalf("writeTokens() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("writeTokens() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// failingWriter fails every write, standing in for a broken pipe or full disk.
type failingWriter struct{ err error }

func (w failingWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestWriteTokensPropagatesWriteError(t *testing.T) {
	wantErr := errors.New("disk full")
	err := writeTokens(failingWriter{err: wantErr}, []Token{tok("word", "a", 0)})
	if err == nil {
		t.Fatal("writeTokens() error = nil, want a wrapped write error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("writeTokens() error = %v, want it to wrap %v", err, wantErr)
	}
	if !strings.HasPrefix(err.Error(), "write: ") {
		t.Errorf("writeTokens() error = %q, want a %q prefix", err, "write: ")
	}
}

// TestWriteTokensMatchesPipeline pins the full in-process pipeline
// (tokenize -> CSV) against the exact bytes the CLI is expected to emit.
func TestWriteTokensMatchesPipeline(t *testing.T) {
	var buf bytes.Buffer
	if err := writeTokens(&buf, Text("x = 1\r\n").tokenize()); err != nil {
		t.Fatalf("writeTokens() error = %v", err)
	}
	want := "Type,Value,Pos\nword,x,0\nsymbol,=,2\nint,1,4\n"
	if got := buf.String(); got != want {
		t.Errorf("pipeline output =\n%q\nwant\n%q", got, want)
	}
}

// --- end-to-end tests against the built binary ------------------------------

// binPath is the tokenizer binary built once by TestMain.
var binPath string

func TestMain(m *testing.M) {
	os.Exit(runTestMain(m))
}

func runTestMain(m *testing.M) int {
	dir, err := os.MkdirTemp("", "tokenizer-e2e")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup: temp dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)

	name := "tokenizer_test_bin"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binPath = filepath.Join(dir, name)

	cmd := exec.Command(goTool(), "build", "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup: go build: %v\n", err)
		return 1
	}
	return m.Run()
}

func goTool() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}

// runBinary executes the built tokenizer with the given args and stdin,
// returning stdout, stderr, and the exit code.
func runBinary(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running %s %v: %v", binPath, args, err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestBinaryStdinToStdout(t *testing.T) {
	stdout, stderr, code := runBinary(t, "x = 1\r\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	want := "Type,Value,Pos\nword,x,0\nsymbol,=,2\nint,1,4\n"
	if stdout != want {
		t.Errorf("stdout =\n%q\nwant\n%q", stdout, want)
	}
}

func TestBinaryEmptyInput(t *testing.T) {
	stdout, stderr, code := runBinary(t, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if want := "Type,Value,Pos\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBinaryFileFlagsShort(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	outPath := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(inPath, []byte("total += 2.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runBinary(t, "ignored stdin", "-i", inPath, "-o", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty when -o is used", stdout)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "Type,Value,Pos\nword,total,0\nsymbol,+,6\nsymbol,=,7\nfloat,2.5,9\n"
	if string(got) != want {
		t.Errorf("output file =\n%q\nwant\n%q", got, want)
	}
}

func TestBinaryFileFlagsLong(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	outPath := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(inPath, []byte("a,b"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runBinary(t, "", "--inputFile", inPath, "--outputFile", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// The comma token is CSV-quoted.
	want := "Type,Value,Pos\nword,a,0\nsymbol,\",\",1\nword,b,2\n"
	if string(got) != want {
		t.Errorf("output file =\n%q\nwant\n%q", got, want)
	}
}

func TestBinaryOutputFileTruncated(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	outPath := filepath.Join(dir, "out.csv")
	if err := os.WriteFile(inPath, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte(strings.Repeat("stale\n", 100)), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runBinary(t, "", "-i", inPath, "-o", outPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Type,Value,Pos\nword,z,0\n"; string(got) != want {
		t.Errorf("output file =\n%q\nwant\n%q", got, want)
	}
}

func TestBinaryMissingInputFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.txt")
	stdout, stderr, code := runBinary(t, "", "-i", missing)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "tokenizer: open input:") {
		t.Errorf("stderr = %q, want it to mention %q", stderr, "tokenizer: open input:")
	}
}

func TestBinaryUnwritableOutputFile(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(inPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The parent directory does not exist, so os.Create fails.
	outPath := filepath.Join(dir, "missing-dir", "out.csv")

	stdout, stderr, code := runBinary(t, "", "-i", inPath, "-o", outPath)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "tokenizer: ") {
		t.Errorf("stderr = %q, want a %q prefix", stderr, "tokenizer: ")
	}
}

func TestBinaryUnknownFlag(t *testing.T) {
	stdout, stderr, code := runBinary(t, "", "--nope")
	if code == 0 {
		t.Errorf("exit code = 0, want non-zero for an unknown flag")
	}
	if stdout+stderr == "" {
		t.Error("expected usage output for an unknown flag, got nothing")
	}
}

func TestBinaryHelp(t *testing.T) {
	stdout, stderr, code := runBinary(t, "", "--help")
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for --help", code)
	}
	out := stdout + stderr
	for _, want := range []string{"inputFile", "outputFile"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q:\n%s", want, out)
		}
	}
}
