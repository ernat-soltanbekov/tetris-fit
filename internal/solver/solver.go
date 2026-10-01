// Package solver searches square sizes in ascending order and proves feasibility
// by backtracking. It never reports an interrupted search as an optimal solution.
package solver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

const (
	MaxSide   = 24 // A 6x6 arrangement of 4x4 boxes always holds 26 pieces.
	memoLimit = 50000
)

var (
	ErrInput        = errors.New("expected 1 to 26 valid tetrominoes")
	ErrAttemptLimit = errors.New("placement attempt limit reached")
)

type Stats struct {
	Attempts, Backtracks             uint64
	Nodes, MemoHits, ComponentPrunes uint64
	SquaresTried                     int
	Elapsed                          time.Duration
}

type Result struct {
	Grid  []string
	Stats Stats
}

type Event struct {
	Kind  string // start, place, backtrack, or solution
	Piece byte
	Grid  []string // A fresh snapshot: observers cannot change the search board.
	Stats Stats
}

type Options struct {
	Rotate      bool
	MaxAttempts uint64 // Zero means no count limit; context still bounds work.
	Observe     func(Event) error
}

type group struct {
	labels     []byte
	shapes     []tetromino.Piece
	placements []placement
	used       int
	next       int // Identical pieces use increasing placement indices.
}

type placement struct {
	row, height int
	masks       [4]uint32
	cells       [4]int
}

// Solve owns all mutable state, so separate calls can run concurrently.
func Solve(ctx context.Context, pieces []tetromino.Piece, options Options) (result Result, err error) {
	started := time.Now()
	defer func() { result.Stats.Elapsed = time.Since(started) }()
	if ctx == nil || len(pieces) == 0 || len(pieces) > tetromino.MaxPieces {
		return result, ErrInput
	}
	groups := make([]group, 0, len(pieces))
	byShape := make(map[uint16]int)
	minimum := MinimumSide(len(pieces))
	for index, input := range pieces {
		piece, validation := tetromino.New(input.Cells[:])
		if validation != nil {
			return result, fmt.Errorf("%w: piece %d", ErrInput, index+1)
		}
		minimum = max(minimum, max(piece.Width(), piece.Height()))
		shapes := []tetromino.Piece{piece}
		if options.Rotate {
			shapes = piece.Rotations()
		}
		key := shapes[0].Key()
		if existing, ok := byShape[key]; ok {
			groups[existing].labels = append(groups[existing].labels, byte('A'+index))
		} else {
			byShape[key] = len(groups)
			groups = append(groups, group{labels: []byte{byte('A' + index)}, shapes: shapes})
		}
	}
	for side := minimum; side <= MaxSide; side++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if !rectangleGroupsFit(groups, side) {
			continue
		}
		state := search{ctx: ctx, side: side, holes: side*side - 4*len(pieces), groups: groups,
			stats: &result.Stats, options: options, failed: make(map[stateKey]struct{}),
			board: make([]byte, side*side)}
		for i := range state.board {
			state.board[i] = '.'
		}
		for i := range groups {
			groups[i].used, groups[i].next = 0, 0
			groups[i].placements = positions(groups[i].shapes, side)
		}
		result.Stats.SquaresTried++
		if err := state.emit("start", 0); err != nil {
			return result, err
		}
		found, err := state.visit(len(pieces))
		if err != nil {
			return result, err
		}
		if found {
			result.Grid = state.snapshot()
			return result, nil
		}
	}
	return result, errors.New("internal error: exhausted the constructive square bound")
}

// Fixed bars have a per-row/per-column capacity. Each 2x2 square consumes one
// cell of each (row parity, column parity) colour, so at most floor(N/2)^2 fit.
// These bounds are exact for an isolated rectangular group, even if other
// groups are present. Rotatable bars need a different bound and are skipped.
func rectangleGroupsFit(groups []group, side int) bool {
	for _, g := range groups {
		if len(g.shapes) != 1 {
			continue
		}
		shape := g.shapes[0]
		width, height := shape.Width(), shape.Height()
		if width*height == 4 && (side/width)*(side/height) < len(g.labels) {
			return false
		}
	}
	return true
}

// MinimumSide is the exact integer ceil(sqrt(4*pieces)), without float rounding.
func MinimumSide(pieces int) int {
	side := 0
	for side*side < 4*pieces {
		side++
	}
	return side
}

func positions(shapes []tetromino.Piece, side int) []placement {
	result := make([]placement, 0, side*side*len(shapes))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			for _, shape := range shapes {
				if x+shape.Width() > side || y+shape.Height() > side {
					continue
				}
				p := placement{row: y, height: shape.Height()}
				for i, cell := range shape.Cells {
					p.masks[cell.Y] |= 1 << uint(x+cell.X)
					p.cells[i] = (y+cell.Y)*side + x + cell.X
				}
				result = append(result, p)
			}
		}
	}
	return result
}
