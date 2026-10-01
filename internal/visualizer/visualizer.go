// Package visualizer renders actual solver events, including every removal.
package visualizer

import (
	"fmt"
	"io"
	"strings"

	"github.com/ernat-soltanbekov/tetris-fit/internal/solver"
)

// New creates an observer for one solve. No sleeping, colour escapes or network.
func New(writer io.Writer) func(solver.Event) error {
	step := 0
	return func(event solver.Event) error {
		var text strings.Builder
		switch event.Kind {
		case "start":
			fmt.Fprintf(&text, "Trying %dx%d square\n", len(event.Grid), len(event.Grid))
		case "place", "backtrack":
			step++
			fmt.Fprintf(&text, "Step %d: %s piece %c\n", step, event.Kind, event.Piece)
		case "solution":
			fmt.Fprintln(&text, "Final solution reached.")
		default:
			return fmt.Errorf("unknown solver event %q", event.Kind)
		}
		for _, row := range event.Grid {
			fmt.Fprintln(&text, row)
		}
		fmt.Fprintf(&text, "attempts: %d, backtracks: %d\n\n", event.Stats.Attempts, event.Stats.Backtracks)
		_, err := io.WriteString(writer, text.String())
		return err
	}
}
