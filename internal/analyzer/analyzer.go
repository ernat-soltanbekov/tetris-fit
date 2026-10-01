// Package analyzer reports measured grid properties and measured search costs.
package analyzer

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

type QualityMetrics struct {
	GridSize, TotalCells, FilledCells, EmptyCells int
	Tetrominoes, TheoreticalMinimum               int
	Compactness                                   float64
	MatchesMinimum                                bool
}

type PerformanceMetrics struct {
	PlacementAttempts, Backtracks uint64
	ExecutionTime                 time.Duration
}

func Quality(grid []string) (QualityMetrics, error) {
	var q QualityMetrics
	size, err := GridSize(grid)
	if err != nil {
		return q, err
	}
	total, err := TotalCells(size)
	if err != nil {
		return q, err
	}
	filled, err := FilledCells(grid)
	if err != nil {
		return q, err
	}
	if filled == 0 || filled%4 != 0 || filled/4 > tetromino.MaxPieces {
		return q, ErrGrid
	}
	// Reject misleading reports for missing, fragmented or oversized labels.
	var cells [26][]tetromino.Point
	for y, row := range grid {
		for x := range row {
			if row[x] != '.' {
				label := row[x] - 'A'
				cells[label] = append(cells[label], tetromino.Point{X: x, Y: y})
			}
		}
	}
	pieces := filled / 4
	for label, points := range cells {
		if label >= pieces {
			if len(points) != 0 {
				return q, ErrGrid
			}
			continue
		}
		if len(points) != 4 {
			return q, ErrGrid
		}
		minX, minY := size, size
		for _, point := range points {
			minX, minY = min(minX, point.X), min(minY, point.Y)
		}
		for i := range points {
			points[i].X -= minX
			points[i].Y -= minY
		}
		if _, err := tetromino.New(points); err != nil {
			return q, ErrGrid
		}
	}
	compactness, _ := Compactness(filled, total)
	minimum, _ := TheoreticalMinimum(pieces)
	q = QualityMetrics{GridSize: size, TotalCells: total, FilledCells: filled,
		EmptyCells: EmptyCells(grid), Tetrominoes: pieces, Compactness: compactness,
		TheoreticalMinimum: minimum, MatchesMinimum: MatchesMinimum(size, minimum)}
	return q, nil
}

func Performance(attempts, backtracks uint64, elapsed time.Duration) (PerformanceMetrics, error) {
	if elapsed < 0 || backtracks > attempts {
		return PerformanceMetrics{}, errors.New("invalid measured performance counters")
	}
	return PerformanceMetrics{PlacementAttempts: attempts, Backtracks: backtracks, ExecutionTime: elapsed}, nil
}

func WriteReport(w io.Writer, input string, q QualityMetrics, p PerformanceMetrics) error {
	match := "does not match; geometric constraints require a larger square"
	if q.MatchesMinimum {
		match = "matches"
	}
	_, err := fmt.Fprintf(w, `
=== Solution Analysis ===

Input file: %q
Tetrominoes: %d

Solution quality:
  Grid size: %dx%d (%d cells)
  Filled cells: %d
  Empty cells: %d
  Compactness: %.2f%%
  Theoretical minimum: %dx%d (%s)

Performance:
  Execution time: %s
  Placement attempts: %d
  Backtracks: %d
`, input, q.Tetrominoes, q.GridSize, q.GridSize, q.TotalCells,
		q.FilledCells, q.EmptyCells, q.Compactness, q.TheoreticalMinimum,
		q.TheoreticalMinimum, match, p.ExecutionTime, p.PlacementAttempts, p.Backtracks)
	return err
}
