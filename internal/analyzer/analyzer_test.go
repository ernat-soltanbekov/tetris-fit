package analyzer

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestMetricHelpers(t *testing.T) {
	grid := []string{"AA.", "AA.", "..."}
	if n, err := GridSize(grid); n != 3 || err != nil {
		t.Fatal(n, err)
	}
	for _, grid := range [][]string{nil, {"..", "..."}} {
		if _, err := GridSize(grid); err == nil {
			t.Fatal("accepted nonsquare")
		}
	}
	if n, err := TotalCells(3); n != 9 || err != nil {
		t.Fatal(n, err)
	}
	for _, n := range []int{0, -1, math.MaxInt} {
		if _, err := TotalCells(n); err == nil {
			t.Fatal("accepted invalid size", n)
		}
	}
	if n, err := FilledCells(grid); n != 4 || err != nil {
		t.Fatal(n, err)
	}
	if _, err := FilledCells([]string{"#"}); err == nil {
		t.Fatal("accepted input instead of solved labels")
	}
	if n := EmptyCells(grid); n != 5 {
		t.Fatal(n)
	}
	if value, err := Compactness(4, 9); err != nil || math.Abs(value-400.0/9) > 1e-9 {
		t.Fatal(value, err)
	}
	for _, pair := range [][2]int{{-1, 9}, {10, 9}, {0, 0}, {1, -1}} {
		if _, err := Compactness(pair[0], pair[1]); err == nil {
			t.Fatal(pair)
		}
	}
	for count, want := range map[int]int{1: 2, 2: 3, 4: 4, 5: 5, 26: 11} {
		if n, err := TheoreticalMinimum(count); n != want || err != nil {
			t.Fatal(n, err)
		}
	}
	for _, n := range []int{0, -1, 27, math.MaxInt} {
		if _, err := TheoreticalMinimum(n); err == nil {
			t.Fatal(n)
		}
	}
	if !MatchesMinimum(4, 4) || MatchesMinimum(5, 4) {
		t.Fatal("minimum comparison")
	}
}

func TestQuality(t *testing.T) {
	q, err := Quality([]string{"AABB", "AABB", "CCDD", "CCDD"})
	if err != nil || q.GridSize != 4 || q.TotalCells != 16 || q.Tetrominoes != 4 || q.FilledCells != 16 || q.EmptyCells != 0 || q.Compactness != 100 || q.TheoreticalMinimum != 4 || !q.MatchesMinimum {
		t.Fatalf("%+v %v", q, err)
	}
	q, err = Quality([]string{"AA.", "AA.", "..."})
	if err != nil || q.EmptyCells != 5 || q.MatchesMinimum {
		t.Fatalf("%+v %v", q, err)
	}
	for _, grid := range [][]string{
		nil, {"ERROR"}, {"..", ".."}, {"AA", "A."}, {"BB", "BB"},
		{"A.A", "...", "A.A"}, {"AAA", "AAA", "AA."}, {"AA#", "AA.", "..."},
		{"AA...", ".....", ".....", ".....", "...AA"},
	} {
		if _, err := Quality(grid); !errors.Is(err, ErrGrid) {
			t.Fatalf("accepted %v: %v", grid, err)
		}
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }

func TestPerformanceAndReport(t *testing.T) {
	p, err := Performance(23, 7, 2*time.Millisecond)
	if err != nil || p.PlacementAttempts != 23 || p.Backtracks != 7 || p.ExecutionTime != 2*time.Millisecond {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := Performance(0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Performance(0, 1, 0); err == nil {
		t.Fatal("invalid counters accepted")
	}
	if _, err := Performance(1, 0, -1); err == nil {
		t.Fatal("negative duration accepted")
	}
	q, _ := Quality([]string{"AA", "AA"})
	var out bytes.Buffer
	if err := WriteReport(&out, "file\nname", q, p); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2x2 (4 cells)", "Filled cells: 4", "Empty cells: 0", "100.00%", "2x2 (matches)", "Execution time: 2ms", "Placement attempts: 23", "Backtracks: 7", `"file\nname"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want)
		}
	}
	if err := WriteReport(brokenWriter{}, "x", q, p); err == nil {
		t.Fatal("lost write error")
	}
}
