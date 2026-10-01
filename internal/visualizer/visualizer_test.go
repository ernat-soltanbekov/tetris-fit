package visualizer

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/tetris-fit/internal/solver"
)

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRenderEventsAndErrors(t *testing.T) {
	var out bytes.Buffer
	observe := New(&out)
	for i, kind := range []string{"start", "place", "backtrack", "solution"} {
		if err := observe(solver.Event{Kind: kind, Piece: 'A', Grid: []string{"AA", "AA"}, Stats: solver.Stats{Attempts: uint64(i), Backtracks: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"Trying 2x2 square", "Step 1: place piece A", "Step 2: backtrack piece A", "Final solution reached.", "attempts: 3, backtracks: 1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want)
		}
	}
	if strings.Count(out.String(), "AA\nAA\n") != 4 {
		t.Fatal("missing snapshots")
	}
	if err := observe(solver.Event{Kind: "invalid"}); err == nil {
		t.Fatal("unknown event accepted")
	}
	if err := New(brokenWriter{})(solver.Event{Kind: "solution"}); err == nil {
		t.Fatal("lost writer error")
	}
}
