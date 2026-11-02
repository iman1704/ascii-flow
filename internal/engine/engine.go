// Package engine ports src/engine.ts: wiring the World simulation to
// output with a fixed-timestep loop.
//
// BROWSER → TERMINAL MAPPING:
//   - requestAnimationFrame + STEP accumulator → time.Ticker at --fps with
//     the same STEP (1/60) / MAX_STEPS (4) accumulator.
//   - ResizeObserver + devicePixelRatio → SIGWINCH listener + term.GetSize
//     (cells; no DPR).
//   - IntersectionObserver visibility + prefers-reduced-motion → dropped:
//     a screensaver runs fullscreen until dismissed.
//   - pointermove/pointerdown (attract + splash) → dropped: no pointer
//     in a raw terminal, and timer-driven splashes would ring the water
//     with no creature behind them.
//   - destroy() listener cleanup → deferred terminal restore (raw mode,
//     cursor, alternate screen).
//
// Exit: any keypress (screensaver semantics), SIGINT/SIGTERM via ctx.
package engine

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/stackedlol/ascii-sharks/internal/renderer"
	"github.com/stackedlol/ascii-sharks/internal/world"
)

// STEP and MAX_STEPS mirror engine.ts's fixed-timestep loop.
const (
	STEP      = 1.0 / 60
	MAX_STEPS = 4
)

// Options configures a screensaver run. The ocean is always the natural,
// area-scaled preset; the flags only tune playback speed and which
// hunters stay out of the water.
type Options struct {
	FPS        int
	NoSharks   bool
	NoPelagics bool
}

// Run takes over the terminal until any key is pressed or ctx ends.
func Run(ctx context.Context, opts Options) error {
	fps := opts.FPS
	if fps <= 0 {
		fps = 30
	}
	if fps > 240 {
		// time.Ticker panics on a non-positive interval, and integer
		// division makes one of absurd --fps values; cap the rate instead.
		fps = 240
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Screensaver needs a TTY for raw mode + size.
		// Fall through only for hosted/CI smoke runs.
		return runHeadless(ctx, opts, fps)
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer restore(fd, oldState)

	out := os.Stdout
	// Alternate screen + hide cursor; restored on exit.
	out.WriteString("\x1b[?1049h\x1b[?25l")
	defer out.WriteString("\x1b[?1049l\x1b[?25h")

	w := world.NewWorld(nil, 0)
	w.NoSharks = opts.NoSharks
	w.NoPelagics = opts.NoPelagics
	w.Natural = true
	r := renderer.New()
	resize := func() {
		if cols, rows, err := term.GetSize(fd); err == nil {
			r.Resize(cols, rows)
			w.Resize(cols, rows)
			// Full clear: after a shrink, rows beyond the new grid would
			// otherwise keep showing the previous larger frame.
			out.WriteString("\x1b[2J")
		}
	}
	resize()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)

	// Any keypress dismisses the screensaver.
	key := make(chan struct{})
	go func() {
		var b [1]byte
		_, _ = os.Stdin.Read(b[:])
		close(key)
	}()

	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	var last = time.Now()
	acc := 0.0

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-key:
			return nil
		case <-winch:
			resize()
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			last = now
			if dt > 0.1 {
				dt = 0.1 // clamp tab-switch gaps, as in engine.ts
			}
			acc += dt
			steps := 0
			for acc >= STEP && steps < MAX_STEPS {
				w.Step(STEP)
				acc -= STEP
				steps++
			}
			if steps == MAX_STEPS {
				acc = 0
			}
			out.WriteString(r.Render(w, dt))
		}
	}
}

// runHeadless advances the sim without a terminal so CI can smoke-test the
// loop; it renders nothing.
func runHeadless(ctx context.Context, opts Options, fps int) error {
	w := world.NewWorld(nil, 0)
	w.NoSharks = opts.NoSharks
	w.NoPelagics = opts.NoPelagics
	w.Natural = true
	w.Resize(80, 24)
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.Step(STEP)
			// No renderer here to consume the queue; drop events so uneaten
			// meals cannot accumulate over a long headless run.
			w.Events = w.Events[:0]
		}
	}
}

func restore(fd int, st *term.State) {
	// Best effort: the terminal must be usable after we exit.
	_ = term.Restore(fd, st)
}
