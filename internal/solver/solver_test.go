package solver_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ernat-soltanbekov/tetris-fit/internal/parser"
	"github.com/ernat-soltanbekov/tetris-fit/internal/solver"
	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

func catalogue() []tetromino.Piece {
	unique := make(map[uint16]tetromino.Piece)
	for a := 0; a < 16; a++ {
		for b := a + 1; b < 16; b++ {
			for c := b + 1; c < 16; c++ {
				for d := c + 1; d < 16; d++ {
					p, err := tetromino.New([]tetromino.Point{{X: a % 4, Y: a / 4}, {X: b % 4, Y: b / 4}, {X: c % 4, Y: c / 4}, {X: d % 4, Y: d / 4}})
					if err == nil {
						unique[p.Key()] = p
					}
				}
			}
		}
	}
	var result []tetromino.Piece
	for _, p := range unique {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}

// oracle deliberately uses a plain boolean board, input order, and no pruning,
// memoization, grouping, row masks or candidate selection from the real solver.
func oracle(pieces []tetromino.Piece, rotate bool) int {
	variants := make([][]tetromino.Piece, len(pieces))
	for i, p := range pieces {
		variants[i] = []tetromino.Piece{p}
		if rotate {
			variants[i] = p.Rotations()
		}
	}
	for side := 1; ; side++ {
		if side*side < len(pieces)*4 {
			continue
		}
		board := make([]bool, side*side)
		var place func(int) bool
		place = func(index int) bool {
			if index == len(pieces) {
				return true
			}
			for _, p := range variants[index] {
				for y := 0; y < side; y++ {
					for x := 0; x < side; x++ {
						cells := [4]int{}
						fits := true
						for j, c := range p.Cells {
							xx, yy := x+c.X, y+c.Y
							if xx >= side || yy >= side {
								fits = false
								break
							}
							cells[j] = yy*side + xx
							if board[cells[j]] {
								fits = false
								break
							}
						}
						if !fits {
							continue
						}
						for _, c := range cells {
							board[c] = true
						}
						if place(index + 1) {
							return true
						}
						for _, c := range cells {
							board[c] = false
						}
					}
				}
			}
			return false
		}
		if place(0) {
			return side
		}
	}
}

func verifyGrid(t testing.TB, grid []string, pieces []tetromino.Piece, rotate bool) {
	t.Helper()
	side := len(grid)
	if side == 0 {
		t.Fatal("empty solution")
	}
	actual := make([][]tetromino.Point, len(pieces))
	for y, row := range grid {
		if len(row) != side {
			t.Fatal("not square")
		}
		for x := range row {
			if row[x] == '.' {
				continue
			}
			index := int(row[x]) - 'A'
			if index < 0 || index >= len(pieces) {
				t.Fatalf("unexpected label %c", row[x])
			}
			actual[index] = append(actual[index], tetromino.Point{X: x, Y: y})
		}
	}
	for i, cells := range actual {
		if len(cells) != 4 {
			t.Fatalf("piece %d has %d cells", i, len(cells))
		}
		minX, minY := side, side
		for _, c := range cells {
			minX = min(minX, c.X)
			minY = min(minY, c.Y)
		}
		for j := range cells {
			cells[j].X -= minX
			cells[j].Y -= minY
		}
		variants := []tetromino.Piece{pieces[i]}
		if rotate {
			variants = pieces[i].Rotations()
		}
		match := false
		for _, p := range variants {
			if reflect.DeepEqual(cells, p.Cells[:]) {
				match = true
			}
		}
		if !match {
			t.Fatalf("piece %d changed shape/orientation: %v", i, cells)
		}
	}
}

func read(t testing.TB, name string) []tetromino.Piece {
	t.Helper()
	p, err := parser.ReadFile("../../testdata/" + name + ".txt")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOfficialAndMaximumInputs(t *testing.T) {
	for name, holes := range map[string]int{"goodexample00": 0, "goodexample01": 9, "goodexample02": 4, "goodexample03": 5, "hardexam": 1, "max-squares": 40, "max-bars": 40} {
		t.Run(name, func(t *testing.T) {
			pieces := read(t, name)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rotate := name != "max-bars"
			result, err := solver.Solve(ctx, pieces, solver.Options{Rotate: rotate})
			if err != nil {
				t.Fatal(err)
			}
			verifyGrid(t, result.Grid, pieces, rotate)
			if got := strings.Count(strings.Join(result.Grid, ""), "."); got != holes {
				t.Fatalf("holes %d want %d", got, holes)
			}
			if result.Stats.Attempts == 0 || result.Stats.Elapsed <= 0 || result.Stats.Elapsed > 10*time.Second {
				t.Fatalf("stats %+v", result.Stats)
			}
		})
	}
}

func TestAgainstIndependentOracle(t *testing.T) {
	shapes := catalogue()
	if len(shapes) != 19 {
		t.Fatal(len(shapes))
	}
	check := func(pieces []tetromino.Piece, rotate bool) {
		result, err := solver.Solve(context.Background(), pieces, solver.Options{Rotate: rotate})
		if err != nil {
			t.Fatal(err)
		}
		verifyGrid(t, result.Grid, pieces, rotate)
		if want := oracle(pieces, rotate); len(result.Grid) != want {
			t.Fatalf("rotate=%v pieces=%v got=%d oracle=%d", rotate, pieces, len(result.Grid), want)
		}
	}
	for _, a := range shapes {
		for _, b := range shapes {
			check([]tetromino.Piece{a, b}, false)
			check([]tetromino.Piece{a, b}, true)
		}
	}
	random := rand.New(rand.NewSource(2008))
	for i := 0; i < 120; i++ {
		pieces := make([]tetromino.Piece, 3+i%2)
		for j := range pieces {
			pieces[j] = shapes[random.Intn(len(shapes))]
		}
		check(pieces, i%3 == 0)
	}
}

func TestOrientationModes(t *testing.T) {
	pieces := read(t, "rotation-demo")
	for _, tc := range []struct {
		rotate bool
		side   int
	}{{false, 4}, {true, 3}} {
		result, err := solver.Solve(context.Background(), pieces, solver.Options{Rotate: tc.rotate})
		if err != nil || len(result.Grid) != tc.side {
			t.Fatalf("%+v %v", result, err)
		}
		verifyGrid(t, result.Grid, pieces, tc.rotate)
	}
}

func TestErrorsNeverReturnASolution(t *testing.T) {
	pieces := read(t, "rotation-demo")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tc := range []struct {
		ctx     context.Context
		pieces  []tetromino.Piece
		options solver.Options
		want    error
	}{
		{nil, pieces, solver.Options{}, solver.ErrInput},
		{context.Background(), nil, solver.Options{}, solver.ErrInput},
		{context.Background(), make([]tetromino.Piece, 27), solver.Options{}, solver.ErrInput},
		{context.Background(), []tetromino.Piece{{}}, solver.Options{}, solver.ErrInput},
		{canceled, pieces, solver.Options{}, context.Canceled},
		{deadline, pieces, solver.Options{}, context.DeadlineExceeded},
		{context.Background(), pieces, solver.Options{MaxAttempts: 1}, solver.ErrAttemptLimit},
	} {
		result, err := solver.Solve(tc.ctx, tc.pieces, tc.options)
		if !errors.Is(err, tc.want) || result.Grid != nil {
			t.Fatalf("result=%+v error=%v want=%v", result, err, tc.want)
		}
	}
	for _, stage := range []string{"start", "place", "backtrack", "solution"} {
		failure := errors.New("observer disconnected")
		result, err := solver.Solve(context.Background(), pieces, solver.Options{Observe: func(e solver.Event) error {
			if e.Kind == stage {
				return failure
			}
			return nil
		}})
		if !errors.Is(err, failure) || result.Grid != nil {
			t.Fatalf("stage=%s result=%v error=%v", stage, result.Grid, err)
		}
	}
}

func TestVisualizerEventsAreReal(t *testing.T) {
	pieces := read(t, "rotation-demo")
	var previous []string
	var attempts, backtracks uint64
	places, removals, solutions := 0, 0, 0
	result, err := solver.Solve(context.Background(), pieces, solver.Options{Observe: func(e solver.Event) error {
		if e.Stats.Attempts < attempts || e.Stats.Backtracks < backtracks {
			t.Fatal("counters decreased")
		}
		if e.Kind == "place" || e.Kind == "backtrack" {
			changed := 0
			for y, row := range e.Grid {
				for x := range row {
					if row[x] == previous[y][x] {
						continue
					}
					changed++
					before, after := byte('.'), e.Piece
					if e.Kind == "backtrack" {
						before, after = after, before
					}
					if previous[y][x] != before || row[x] != after {
						t.Fatal("event did not match board change")
					}
				}
			}
			if changed != 4 {
				t.Fatal("changed", changed)
			}
			if e.Kind == "place" {
				places++
			} else {
				removals++
				if e.Stats.Backtracks != backtracks+1 {
					t.Fatal("backtrack counter")
				}
			}
		}
		if e.Kind == "solution" {
			solutions++
		}
		previous = append([]string(nil), e.Grid...)
		attempts = e.Stats.Attempts
		backtracks = e.Stats.Backtracks
		// An observer may overwrite its snapshot without corrupting solver state.
		e.Grid[0] = "tampered"
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if removals == 0 || uint64(removals) != result.Stats.Backtracks || places-removals != len(pieces) || solutions != 1 || !reflect.DeepEqual(previous, result.Grid) {
		t.Fatalf("places=%d removes=%d result=%+v", places, removals, result)
	}
	verifyGrid(t, result.Grid, pieces, false)
}

func TestCancelDuringSearch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := solver.Solve(ctx, read(t, "rotation-demo"), solver.Options{Observe: func(e solver.Event) error {
		if e.Kind == "place" {
			cancel()
		}
		return nil
	}})
	if !errors.Is(err, context.Canceled) || result.Grid != nil {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestConcurrentDeterministicSolves(t *testing.T) {
	pieces := read(t, "goodexample01")
	want, err := solver.Solve(context.Background(), pieces, solver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			got, err := solver.Solve(context.Background(), pieces, solver.Options{})
			if err != nil || !reflect.DeepEqual(got.Grid, want.Grid) || got.Stats.Attempts != want.Stats.Attempts || got.Stats.Backtracks != want.Stats.Backtracks {
				t.Fatalf("nondeterministic: %+v %v", got, err)
			}
		})
	}
}

func TestBoundedMixedStress(t *testing.T) {
	shapes := catalogue()
	random := rand.New(rand.NewSource(1989))
	for n := 8; n <= 26; n += 3 {
		pieces := make([]tetromino.Piece, n)
		for i := range pieces {
			pieces[i] = shapes[random.Intn(len(shapes))]
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := solver.Solve(ctx, pieces, solver.Options{MaxAttempts: 50000})
		cancel()
		if err == nil {
			verifyGrid(t, result.Grid, pieces, false)
		} else if !errors.Is(err, solver.ErrAttemptLimit) || result.Grid != nil || result.Stats.Attempts != 50000 {
			t.Fatalf("n=%d %+v %v", n, result, err)
		}
	}
}

func FuzzTwoPieces(f *testing.F) {
	shapes := catalogue()
	f.Add(uint8(0), uint8(18), false)
	f.Add(uint8(6), uint8(6), true)
	f.Fuzz(func(t *testing.T, a, b uint8, rotate bool) {
		pieces := []tetromino.Piece{shapes[int(a)%len(shapes)], shapes[int(b)%len(shapes)]}
		result, err := solver.Solve(context.Background(), pieces, solver.Options{Rotate: rotate})
		if err != nil {
			t.Fatal(err)
		}
		verifyGrid(t, result.Grid, pieces, rotate)
		if len(result.Grid) != oracle(pieces, rotate) {
			t.Fatal("nonminimal solution")
		}
	})
}

func BenchmarkSolver(b *testing.B) {
	for _, name := range []string{"hardexam", "goodexample01", "max-squares", "max-bars"} {
		b.Run(name, func(b *testing.B) {
			pieces := read(b, name)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := solver.Solve(context.Background(), pieces, solver.Options{Rotate: name != "max-bars"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
