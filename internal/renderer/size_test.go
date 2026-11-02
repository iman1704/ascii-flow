package renderer

import (
	"testing"

	"github.com/iman1704/ascii-flow/internal/world"
)

// Each size class draws distinctly: ',' small and '-' medium take one
// cell, while '=' large spreads over a head cell plus a trailing body cell.
func TestFishSizeGlyphs(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 10, Y: 10, Vx: 1, Species: 0, Alive: true, Depth: 1}, // small
			{X: 20, Y: 10, Vx: 1, Species: 1, Alive: true, Depth: 1}, // medium
			{X: 30, Y: 10, Vx: 1, Species: 2, Alive: true, Depth: 1}, // large
		},
	}
	r.markFish(w)
	r.drawFish()
	for cell, want := range map[int]rune{
		r.cellIndex(10, 10): ',',
		r.cellIndex(20, 10): '-',
		r.cellIndex(30, 10): '=',
		r.cellIndex(29, 10): '=',
	} {
		if got := r.cells[cell].ch; got != want {
			t.Fatalf("fish cell %d drew %q, want %q", cell, got, want)
		}
	}
}

// A large fish at the arena edge falls back to its head cell alone instead
// of writing out of bounds.
func TestLargeFishTailClippedAtEdge(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 0.5, Y: 10, Vx: 1, Species: 2, Alive: true, Depth: 1},
		},
	}
	r.markFish(w)
	r.drawFish()
	if got := r.cells[r.cellIndex(0, 10)].ch; got != '=' {
		t.Fatalf("edge large fish head drew %q, want '='", got)
	}
}
