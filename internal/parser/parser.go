// Package parser reads the exact four-row, four-column tetromino format.
package parser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

// Even 26 pieces with CRLF line endings fit below this bound.
const MaxInputBytes = 1024

var ErrFormat = errors.New("invalid tetromino file")

func ReadFile(path string) ([]tetromino.Piece, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxInputBytes {
		return nil, ErrFormat
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

func Parse(reader io.Reader) ([]tetromino.Piece, error) {
	if reader == nil {
		return nil, ErrFormat
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if len(data) == 0 || len(data) > MaxInputBytes {
		return nil, ErrFormat
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	// A final newline is optional; extra blank lines and whitespace are not.
	text = strings.TrimSuffix(text, "\n")
	blocks := strings.Split(text, "\n\n")
	if len(blocks) == 0 || len(blocks) > tetromino.MaxPieces {
		return nil, ErrFormat
	}
	pieces := make([]tetromino.Piece, 0, len(blocks))
	for index, block := range blocks {
		rows := strings.Split(block, "\n")
		if len(rows) != 4 {
			return nil, fmt.Errorf("%w: piece %d needs four rows", ErrFormat, index+1)
		}
		cells := make([]tetromino.Point, 0, 4)
		for y, row := range rows {
			if len(row) != 4 {
				return nil, fmt.Errorf("%w: row width", ErrFormat)
			}
			for x := range row {
				switch row[x] {
				case '#':
					cells = append(cells, tetromino.Point{X: x, Y: y})
				case '.':
				default:
					return nil, fmt.Errorf("%w: unexpected character", ErrFormat)
				}
			}
		}
		piece, err := tetromino.New(cells)
		if err != nil {
			return nil, fmt.Errorf("%w: piece %d: %v", ErrFormat, index+1, err)
		}
		pieces = append(pieces, piece)
	}
	return pieces, nil
}
