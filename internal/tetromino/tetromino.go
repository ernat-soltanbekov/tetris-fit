// Package tetromino describes four connected squares, independently of a board.
package tetromino

import (
	"errors"
	"sort"
)

const MaxPieces = 26 // Each input piece needs a distinct label from A through Z.

type Point struct{ X, Y int }

// Piece is normalized to the top-left corner and sorted in reading order.
// Callers can construct this exported value; solver entry points validate it.
type Piece struct{ Cells [4]Point }

var ErrInvalid = errors.New("a tetromino must have four distinct edge-connected cells in a 4x4 box")

func New(cells []Point) (Piece, error) {
	if len(cells) != 4 {
		return Piece{}, ErrInvalid
	}
	var piece Piece
	copy(piece.Cells[:], cells)
	minX, minY := 4, 4
	for index, cell := range piece.Cells {
		if cell.X < 0 || cell.X >= 4 || cell.Y < 0 || cell.Y >= 4 {
			return Piece{}, ErrInvalid
		}
		for _, previous := range piece.Cells[:index] {
			if cell == previous {
				return Piece{}, ErrInvalid
			}
		}
		minX, minY = min(minX, cell.X), min(minY, cell.Y)
	}
	// A flood fill, rather than diagonal contact, establishes connectivity.
	visited := [4]bool{true}
	for pass := 0; pass < 4; pass++ {
		for i, from := range piece.Cells {
			if !visited[i] {
				continue
			}
			for j, to := range piece.Cells {
				if abs(from.X-to.X)+abs(from.Y-to.Y) == 1 {
					visited[j] = true
				}
			}
		}
	}
	for _, seen := range visited {
		if !seen {
			return Piece{}, ErrInvalid
		}
	}
	for i := range piece.Cells {
		piece.Cells[i].X -= minX
		piece.Cells[i].Y -= minY
	}
	sort.Slice(piece.Cells[:], func(i, j int) bool {
		a, b := piece.Cells[i], piece.Cells[j]
		return a.Y < b.Y || a.Y == b.Y && a.X < b.X
	})
	return piece, nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (p Piece) Width() int {
	width := 0
	for _, cell := range p.Cells {
		width = max(width, cell.X+1)
	}
	return width
}

func (p Piece) Height() int {
	height := 0
	for _, cell := range p.Cells {
		height = max(height, cell.Y+1)
	}
	return height
}

// Key identifies a normalized orientation; translation does not affect it.
func (p Piece) Key() uint16 {
	var key uint16
	for _, cell := range p.Cells {
		key |= 1 << uint(cell.Y*4+cell.X)
	}
	return key
}

// Rotations returns unique quarter-turns, without reflecting the piece.
func (p Piece) Rotations() []Piece {
	result := make([]Piece, 0, 4)
	seen := make(map[uint16]bool)
	for turn := 0; turn < 4; turn++ {
		if !seen[p.Key()] {
			seen[p.Key()] = true
			result = append(result, p)
		}
		var cells [4]Point
		for i, cell := range p.Cells {
			cells[i] = Point{X: 3 - cell.Y, Y: cell.X}
		}
		p, _ = New(cells[:]) // A rotation preserves the validated shape.
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}
