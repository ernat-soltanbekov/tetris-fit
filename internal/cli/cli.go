// Package cli is shared by the optimizer and the step-by-step visualizer.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ernat-soltanbekov/tetris-fit/internal/analyzer"
	"github.com/ernat-soltanbekov/tetris-fit/internal/parser"
	"github.com/ernat-soltanbekov/tetris-fit/internal/solver"
	"github.com/ernat-soltanbekov/tetris-fit/internal/visualizer"
)

const help = `Usage: tetris-optimizer FILE [--analyze] [--visualize] [--fixed]
       tetris-visualizer FILE [--analyze] [--fixed]

Flags work before or after FILE. Use -- before a filename starting with -.
  --analyze          Report measured quality and performance.
  --visualize        Print each placement and each backtrack.
  --rotate           Allow quarter-turns (the default); never reflect pieces.
  --fixed            Preserve input orientations instead.
  --timeout DURATION Search deadline (default 2m, maximum 5m).
  --max-attempts N   Stop after N collision tests; 0 means no count limit.
  --help            Show this message.
`

type config struct {
	path                             string
	analyze, visualize, rotate, help bool
	timeout                          time.Duration
	maxAttempts                      uint64
}

func arguments(args []string, defaultVisualize bool) (config, error) {
	c := config{visualize: defaultVisualize, rotate: true, timeout: 2 * time.Minute}
	seen := make(map[string]bool)
	literal := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !literal && arg == "--" {
			literal = true
			continue
		}
		if literal || !strings.HasPrefix(arg, "-") {
			if c.path != "" || arg == "" {
				return c, errors.New("expected one file")
			}
			c.path = arg
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		if seen[name] {
			return c, errors.New("duplicate flag")
		}
		seen[name] = true
		switch name {
		case "--help", "--analyze", "--visualize", "--rotate", "--fixed":
			if hasValue {
				return c, errors.New("boolean flag does not take a value")
			}
			switch name {
			case "--help":
				c.help = true
			case "--analyze":
				c.analyze = true
			case "--visualize":
				c.visualize = true
			case "--rotate":
				if seen["--fixed"] {
					return c, errors.New("conflicting orientation flags")
				}
				c.rotate = true
			case "--fixed":
				if seen["--rotate"] {
					return c, errors.New("conflicting orientation flags")
				}
				c.rotate = false
			}
		case "--timeout", "--max-attempts":
			if !hasValue {
				i++
				if i == len(args) {
					return c, errors.New("missing flag value")
				}
				value = args[i]
			}
			var err error
			if name == "--timeout" {
				c.timeout, err = time.ParseDuration(value)
				if err != nil || c.timeout <= 0 || c.timeout > 5*time.Minute {
					return c, errors.New("timeout must be positive and at most 5m")
				}
			} else {
				c.maxAttempts, err = strconv.ParseUint(value, 10, 64)
				if err != nil {
					return c, err
				}
			}
		default:
			return c, errors.New("unknown flag")
		}
	}
	if c.path == "" && !c.help {
		return c, errors.New("missing file")
	}
	return c, nil
}

func Run(parent context.Context, args []string, out, diagnostics io.Writer, defaultVisualize bool) int {
	if out == nil || diagnostics == nil {
		return 2
	}
	fail := func(code int) int {
		fmt.Fprintln(out, "ERROR")
		return code
	}
	c, err := arguments(args, defaultVisualize)
	if err != nil || parent == nil {
		return fail(2)
	}
	if c.help {
		if _, err := io.WriteString(out, help); err != nil {
			return 1
		}
		return 0
	}
	pieces, err := parser.ReadFile(c.path)
	if err != nil {
		return fail(1)
	}
	ctx, cancel := context.WithTimeout(parent, c.timeout)
	defer cancel()
	options := solver.Options{Rotate: c.rotate, MaxAttempts: c.maxAttempts}
	if c.visualize {
		options.Observe = visualizer.New(out)
	}
	result, err := solver.Solve(ctx, pieces, options)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, solver.ErrAttemptLimit) {
			fmt.Fprintf(diagnostics, "Search interrupted: %v; no minimum solution is claimed.\n", err)
			return fail(124)
		}
		return fail(1)
	}
	quality, err := analyzer.Quality(result.Grid)
	if err != nil || quality.Tetrominoes != len(pieces) {
		return fail(1)
	}
	if !c.visualize {
		if _, err := io.WriteString(out, strings.Join(result.Grid, "\n")+"\n"); err != nil {
			return 1
		}
	}
	if c.analyze {
		performance, err := analyzer.Performance(result.Stats.Attempts, result.Stats.Backtracks, result.Stats.Elapsed)
		if err != nil {
			return fail(1)
		}
		if err := analyzer.WriteReport(out, c.path, quality, performance); err != nil {
			return 1
		}
	}
	return 0
}

func Main(defaultVisualize bool) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, os.Args[1:], os.Stdout, os.Stderr, defaultVisualize)
}
