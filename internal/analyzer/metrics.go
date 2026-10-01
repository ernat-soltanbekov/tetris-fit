package analyzer

import (
	"errors"
	"math"

	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

var ErrGrid = errors.New("expected a nonempty square grid of complete labelled tetrominoes")

func GridSize(grid []string) (int, error) {
	size := len(grid)
	if size == 0 {
		return 0, ErrGrid
	}
	for _, row := range grid {
		if len(row) != size {
			return 0, ErrGrid
		}
	}
	return size, nil
}

func TotalCells(size int) (int, error) {
	if size <= 0 || size > math.MaxInt/size {
		return 0, ErrGrid
	}
	return size * size, nil
}

// FilledCells counts labels in the final grid, not # symbols from the input.
func FilledCells(grid []string) (int, error) {
	filled := 0
	for _, row := range grid {
		for i := range row {
			if row[i] >= 'A' && row[i] <= 'Z' {
				filled++
			} else if row[i] != '.' {
				return 0, ErrGrid
			}
		}
	}
	return filled, nil
}

func EmptyCells(grid []string) int {
	empty := 0
	for _, row := range grid {
		for i := range row {
			if row[i] == '.' {
				empty++
			}
		}
	}
	return empty
}

func Compactness(filled, total int) (float64, error) {
	if total <= 0 || filled < 0 || filled > total {
		return 0, ErrGrid
	}
	return float64(filled) / float64(total) * 100, nil
}

func TheoreticalMinimum(pieces int) (int, error) {
	if pieces < 1 || pieces > tetromino.MaxPieces {
		return 0, ErrGrid
	}
	side := 1
	for side*side < pieces*4 {
		side++
	}
	return side, nil
}

func MatchesMinimum(size, minimum int) bool { return size == minimum }
