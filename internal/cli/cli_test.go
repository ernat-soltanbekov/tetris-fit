package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArguments(t *testing.T) {
	if c, err := arguments([]string{"file"}, false); err != nil || !c.rotate {
		t.Fatal("rotations must be enabled by default")
	}
	if c, err := arguments([]string{"file", "--fixed"}, false); err != nil || c.rotate {
		t.Fatal("fixed orientation option")
	}
	for _, args := range [][]string{{"file", "--rotate", "--fixed"}, {"file", "--fixed", "--rotate"}} {
		if _, err := arguments(args, false); err == nil {
			t.Fatal("conflicting modes accepted")
		}
	}
	for _, args := range [][]string{
		{"file", "--analyze", "--rotate", "--visualize", "--timeout=3s", "--max-attempts=50"},
		{"--analyze", "--timeout", "3s", "--max-attempts", "50", "file", "--rotate", "--visualize"},
	} {
		c, err := arguments(args, false)
		if err != nil || c.path != "file" || !c.analyze || !c.rotate || !c.visualize || c.maxAttempts != 50 {
			t.Fatalf("%+v %v", c, err)
		}
	}
	if c, err := arguments([]string{"--", "-file"}, false); err != nil || c.path != "-file" {
		t.Fatal(c, err)
	}
	for _, args := range [][]string{
		nil, {""}, {"a", "b"}, {"--analyze"}, {"--wat", "file"}, {"--analyze=true", "file"},
		{"file", "--analyze", "--analyze"}, {"file", "--timeout"}, {"file", "--timeout=0"},
		{"file", "--timeout=6m"}, {"file", "--timeout=oops"}, {"file", "--max-attempts=-1"},
		{"file", "--max-attempts=18446744073709551616"}, {"file", "--max-attempts"},
	} {
		if _, err := arguments(args, false); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestExactErrorOutput(t *testing.T) {
	cases := [][]string{nil, {"missing-file"}, {"../"}, {"../../testdata/goodexample00.txt", "--unknown"}}
	files, err := filepath.Glob("../../testdata/bad*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		cases = append(cases, []string{path}, []string{path, "--analyze"})
	}
	for _, args := range cases {
		var out, diagnostics bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostics, false); code == 0 || out.String() != "ERROR\n" || diagnostics.Len() != 0 {
			t.Fatalf("%v: code=%d out=%q stderr=%q", args, code, out.String(), diagnostics.String())
		}
	}
}

func TestSuccessfulModes(t *testing.T) {
	for _, args := range [][]string{{"../../testdata/goodexample00.txt"}, {"--analyze", "../../testdata/goodexample00.txt"}, {"../../testdata/goodexample00.txt", "--analyze"}} {
		var out, diagnostics bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostics, false); code != 0 || diagnostics.Len() != 0 {
			t.Fatal(code, diagnostics.String())
		}
		if !strings.HasPrefix(out.String(), "AA\nAA\n") {
			t.Fatal(out.String())
		}
		if len(args) == 1 && out.String() != "AA\nAA\n" {
			t.Fatal("basic output polluted")
		}
		if len(args) > 1 && !strings.Contains(out.String(), "Compactness: 100.00%") {
			t.Fatal(out.String())
		}
	}
	for _, defaultMode := range []bool{false, true} {
		args := []string{"../../testdata/rotation-demo.txt", "--analyze", "--fixed"}
		if !defaultMode {
			args = append(args, "--visualize")
		}
		var out, diagnostics bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostics, defaultMode); code != 0 {
			t.Fatal(code)
		}
		for _, want := range []string{"backtrack piece A", "Final solution reached.", "Backtracks: 2", "Grid size: 4x4"} {
			if !strings.Contains(out.String(), want) {
				t.Fatal("missing", want)
			}
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "two words Казак.txt")
	if err := os.WriteFile(path, []byte("##..\n##..\n....\n....\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if code := Run(context.Background(), []string{path}, &out, &diagnostics, false); code != 0 {
		t.Fatal(code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("writer closed") }

func TestLimitsHelpAndOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"../../testdata/rotation-demo.txt", "--analyze", "--max-attempts=1"}, {"../../testdata/hardexam.txt", "--timeout=1ns"}} {
		var out, diagnostics bytes.Buffer
		if code := Run(context.Background(), args, &out, &diagnostics, false); code != 124 || out.String() != "ERROR\n" || !strings.Contains(diagnostics.String(), "no minimum solution is claimed") {
			t.Fatal(code, out.String(), diagnostics.String())
		}
	}
	var out, diagnostics bytes.Buffer
	if code := Run(context.Background(), []string{"--help"}, &out, &diagnostics, false); code != 0 || !strings.Contains(out.String(), "Usage:") {
		t.Fatal(code)
	}
	for _, args := range [][]string{{"--help"}, {"../../testdata/goodexample00.txt"}, {"../../testdata/goodexample00.txt", "--visualize"}} {
		if code := Run(context.Background(), args, failingWriter{}, &diagnostics, false); code == 0 {
			t.Fatal("write failure ignored")
		}
	}
	if code := Run(nil, []string{"x"}, &out, &diagnostics, false); code != 2 {
		t.Fatal(code)
	}
	if code := Run(context.Background(), nil, nil, &diagnostics, false); code != 2 {
		t.Fatal(code)
	}
}
