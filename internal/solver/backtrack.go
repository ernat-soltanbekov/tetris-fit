package solver

import "context"

// Counts and next positions matter: occupancy alone is not a complete state.
type stateKey struct {
	rows [MaxSide]uint32
	used [26]uint8
	next [26]uint16
}

type search struct {
	ctx         context.Context
	side, holes int
	groups      []group
	board       []byte
	rows        [MaxSide]uint32
	stats       *Stats
	options     Options
	failed      map[stateKey]struct{}
}

func (s *search) visit(remaining int) (bool, error) {
	if err := s.ctx.Err(); err != nil {
		return false, err
	}
	s.stats.Nodes++
	if remaining == 0 {
		return true, s.emit("solution", 0)
	}
	key := stateKey{rows: s.rows}
	for i, g := range s.groups {
		key.used[i], key.next[i] = uint8(g.used), uint16(g.next)
	}
	if _, known := s.failed[key]; known {
		s.stats.MemoHits++
		return false, nil
	}
	// Connected pieces cannot cross separate free regions. Each region must leave
	// at least size%4 holes. Skip this cost when there is ample spare space.
	if s.holes <= 8 && !s.regionsCanFit() {
		s.stats.ComponentPrunes++
		return false, nil
	}
	groupIndex, candidates, err := s.mostConstrained()
	if err != nil {
		return false, err
	}
	if len(candidates) == 0 {
		s.remember(key)
		return false, nil
	}
	g := &s.groups[groupIndex]
	label := g.labels[g.used]
	oldNext := g.next
	for _, index := range candidates {
		p := g.placements[index]
		s.place(p, label)
		g.used++
		g.next = index + 1
		if err := s.emit("place", label); err != nil {
			return false, err
		}
		found, err := s.visit(remaining - 1)
		if err != nil || found {
			return found, err
		}
		s.remove(p)
		g.used--
		g.next = oldNext
		s.stats.Backtracks++
		if err := s.emit("backtrack", label); err != nil {
			return false, err
		}
	}
	s.remember(key)
	return false, nil
}

// An attempt is ONE collision test of a piece/orientation/anchor candidate,
// including tests used to choose the most constrained group. Cached candidates
// are then placed without repeating the test or counting the same test twice.
func (s *search) mostConstrained() (int, []int, error) {
	chosen := -1
	var best []int
	for i := range s.groups {
		g := &s.groups[i]
		if g.used == len(g.labels) {
			continue
		}
		valid := make([]int, 0, len(g.placements)-g.next)
		for index := g.next; index < len(g.placements); index++ {
			if s.options.MaxAttempts != 0 && s.stats.Attempts >= s.options.MaxAttempts {
				return -1, nil, ErrAttemptLimit
			}
			if s.stats.Attempts%256 == 0 {
				if err := s.ctx.Err(); err != nil {
					return -1, nil, err
				}
			}
			s.stats.Attempts++
			if s.fits(g.placements[index]) {
				valid = append(valid, index)
			}
		}
		if len(valid) < len(g.labels)-g.used {
			return i, nil, nil
		}
		if chosen == -1 || len(valid) < len(best) {
			chosen, best = i, valid
		}
	}
	return chosen, best, nil
}

func (s *search) fits(p placement) bool {
	for row := 0; row < p.height; row++ {
		if s.rows[p.row+row]&p.masks[row] != 0 {
			return false
		}
	}
	return true
}

func (s *search) place(p placement, label byte) {
	for row := 0; row < p.height; row++ {
		s.rows[p.row+row] |= p.masks[row]
	}
	for _, cell := range p.cells {
		s.board[cell] = label
	}
}

func (s *search) remove(p placement) {
	for row := 0; row < p.height; row++ {
		s.rows[p.row+row] &^= p.masks[row]
	}
	for _, cell := range p.cells {
		s.board[cell] = '.'
	}
}

func (s *search) remember(key stateKey) {
	if len(s.failed) < memoLimit {
		s.failed[key] = struct{}{}
	}
}

func (s *search) snapshot() []string {
	grid := make([]string, s.side)
	for row := range grid {
		grid[row] = string(s.board[row*s.side : (row+1)*s.side])
	}
	return grid
}

func (s *search) emit(kind string, piece byte) error {
	if s.options.Observe == nil {
		return nil
	}
	return s.options.Observe(Event{Kind: kind, Piece: piece, Grid: s.snapshot(), Stats: *s.stats})
}

func (s *search) regionsCanFit() bool {
	var seen [MaxSide * MaxSide]bool
	var stack [MaxSide * MaxSide]int
	requiredHoles := 0
	for start, cell := range s.board {
		if cell != '.' || seen[start] {
			continue
		}
		count, pending := 0, 1
		stack[0], seen[start] = start, true
		for pending > 0 {
			pending--
			current := stack[pending]
			count++
			x, y := current%s.side, current/s.side
			neighbors := [4]int{-1, -1, -1, -1}
			if x > 0 {
				neighbors[0] = current - 1
			}
			if x+1 < s.side {
				neighbors[1] = current + 1
			}
			if y > 0 {
				neighbors[2] = current - s.side
			}
			if y+1 < s.side {
				neighbors[3] = current + s.side
			}
			for _, neighbor := range neighbors {
				if neighbor >= 0 && !seen[neighbor] && s.board[neighbor] == '.' {
					seen[neighbor] = true
					stack[pending] = neighbor
					pending++
				}
			}
		}
		requiredHoles += count % 4
		if requiredHoles > s.holes {
			return false
		}
	}
	return true
}
