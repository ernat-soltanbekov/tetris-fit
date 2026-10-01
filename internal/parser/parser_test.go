package parser

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ernat-soltanbekov/tetris-fit/internal/tetromino"
)

const square = "##..\n##..\n....\n...."

func TestOfficialFiles(t *testing.T) {
	files, err := filepath.Glob("../../testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			pieces, err := ReadFile(path)
			bad := strings.HasPrefix(filepath.Base(path), "bad")
			if bad && !errors.Is(err, ErrFormat) || !bad && (err != nil || len(pieces) == 0) {
				t.Fatalf("pieces=%d error=%v", len(pieces), err)
			}
		})
	}
}

func TestFormatBoundaries(t *testing.T) {
	for _, text := range []string{square, square + "\n", strings.ReplaceAll(square+"\n", "\n", "\r\n"), strings.Repeat(square+"\n\n", 25) + square} {
		if _, err := Parse(strings.NewReader(text)); err != nil {
			t.Fatalf("valid input rejected: %v", err)
		}
	}
	for _, text := range []string{
		"", "\n" + square, square + "\n\n", square + " ", strings.Replace(square, "#", "x", 1),
		strings.Replace(square, "#", "\x00", 1), "\xef\xbb\xbf" + square, strings.Replace(square, ".", "é", 1),
		strings.ReplaceAll(square, "\n", "\r"), strings.Replace(square, "##..", "###..", 1),
		strings.Replace(square, "##..", "#..", 1), square + "\n....", square + "\n\n\n" + square,
		strings.Repeat(square+"\n\n", 26) + square, strings.Repeat(".", MaxInputBytes+1),
		"#...\n.#..\n..#.\n...#", "##..\n....\n..##\n....",
		"###.\n##..\n....\n....", "#...\n##..\n....\n....",
	} {
		if _, err := Parse(strings.NewReader(text)); !errors.Is(err, ErrFormat) {
			t.Errorf("accepted invalid %q: %v", text, err)
		}
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

type endlessReader struct{ read int }

func (r *endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '.'
	}
	r.read += len(p)
	return len(p), nil
}

func TestReadFailuresAndBoundedInput(t *testing.T) {
	if _, err := Parse(nil); !errors.Is(err, ErrFormat) {
		t.Fatal(err)
	}
	failure := errors.New("read failed")
	if _, err := Parse(failingReader{failure}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	r := new(endlessReader)
	if _, err := Parse(r); !errors.Is(err, ErrFormat) || r.read != MaxInputBytes+1 {
		t.Fatalf("read %d: %v", r.read, err)
	}
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "missing")} {
		if _, err := ReadFile(path); err == nil {
			t.Fatalf("accepted %s", path)
		}
	}
	path := filepath.Join(dir, "oversized")
	if err := os.WriteFile(path, []byte(strings.Repeat(".", MaxInputBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); !errors.Is(err, ErrFormat) {
		t.Fatal(err)
	}
}

func encode(pieces []tetromino.Piece) string {
	var blocks []string
	for _, p := range pieces {
		rows := []string{"....", "....", "....", "...."}
		for _, c := range p.Cells {
			row := []byte(rows[c.Y])
			row[c.X] = '#'
			rows[c.Y] = string(row)
		}
		blocks = append(blocks, strings.Join(rows, "\n"))
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(square))
	f.Add([]byte(square + "\n\n" + square))
	f.Add([]byte("ERROR"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		pieces, err := Parse(strings.NewReader(string(data)))
		if err != nil {
			return
		}
		if len(pieces) < 1 || len(pieces) > 26 {
			t.Fatalf("piece count %d", len(pieces))
		}
		copy, err := Parse(strings.NewReader(encode(pieces)))
		if err != nil || !reflect.DeepEqual(copy, pieces) {
			t.Fatalf("round trip: %v", err)
		}
	})
}

var _ io.Reader = (*endlessReader)(nil)
