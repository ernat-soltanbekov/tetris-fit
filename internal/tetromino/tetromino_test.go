package tetromino

import (
	"errors"
	"testing"
)

func TestNormalizeAndRotations(t *testing.T) {
	cases := []struct {
		cells                []Point
		width, height, turns int
	}{
		{[]Point{{2, 2}, {3, 2}, {2, 3}, {3, 3}}, 2, 2, 1},
		{[]Point{{0, 3}, {1, 3}, {2, 3}, {3, 3}}, 4, 1, 2},
		{[]Point{{2, 3}, {1, 3}, {1, 2}, {1, 1}}, 2, 3, 4},
	}
	for _, tc := range cases {
		p, err := New(tc.cells)
		if err != nil || p.Width() != tc.width || p.Height() != tc.height {
			t.Fatalf("shape: %+v %v", p, err)
		}
		turns := p.Rotations()
		if len(turns) != tc.turns {
			t.Fatalf("rotations: %d", len(turns))
		}
		for _, turn := range turns {
			validated, err := New(turn.Cells[:])
			if err != nil || validated != turn {
				t.Fatalf("invalid rotation: %+v", turn)
			}
		}
	}
	left, _ := New([]Point{{0, 0}, {0, 1}, {0, 2}, {1, 2}})
	right, _ := New([]Point{{1, 0}, {1, 1}, {1, 2}, {0, 2}})
	for _, turn := range left.Rotations() {
		if turn == right {
			t.Fatal("rotation must not reflect")
		}
	}
}

func TestRejectInvalid(t *testing.T) {
	for _, cells := range [][]Point{
		nil, {{0, 0}}, {{0, 0}, {0, 0}, {0, 1}, {0, 2}},
		{{0, 0}, {1, 1}, {2, 2}, {3, 3}}, {{0, 0}, {0, 1}, {3, 2}, {3, 3}},
		{{-1, 0}, {0, 0}, {1, 0}, {2, 0}}, {{0, 0}, {1, 0}, {2, 0}, {4, 0}},
	} {
		if _, err := New(cells); !errors.Is(err, ErrInvalid) {
			t.Fatalf("accepted %v", cells)
		}
	}
}

func TestAllFixedOrientations(t *testing.T) {
	shapes := make(map[uint16]bool)
	for a := 0; a < 16; a++ {
		for b := a + 1; b < 16; b++ {
			for c := b + 1; c < 16; c++ {
				for d := c + 1; d < 16; d++ {
					cells := []Point{{a % 4, a / 4}, {b % 4, b / 4}, {c % 4, c / 4}, {d % 4, d / 4}}
					if p, err := New(cells); err == nil {
						shapes[p.Key()] = true
					}
				}
			}
		}
	}
	if len(shapes) != 19 {
		t.Fatalf("got %d fixed orientations, want 19", len(shapes))
	}
}
