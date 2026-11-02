package world

import (
	"math"
	"testing"
)

// A below-minimum group must migrate toward the closest school of the same
// size: 3 small fish (min 20) start 30 cells west of a 20-strong small
// school, all cruising east in a wide arena so the walls stay out of it.
func TestSmallGroupMigratesToClosestSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(120, 40)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = nil
	for k := 0; k < 3; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 30 + float64(k)*0.3, Y: 20, Vx: 1, Vy: 0,
			Species: 0, Alive: true, Depth: 0.8, MillDir: 1,
		})
	}
	for k := 0; k < 20; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 60 + float64(k%10)*0.3, Y: 20 + float64(k/10)*0.3, Vx: 1, Vy: 0,
			Species: 0, Alive: true, Depth: 0.8, MillDir: 1,
		})
	}
	centroid := func(lo, hi int) (float64, float64) {
		var x, y float64
		for _, f := range w.Fish[lo:hi] {
			x += f.X
			y += f.Y
		}
		n := float64(hi - lo)
		return x / n, y / n
	}
	gx0, gy0 := centroid(0, 3)
	sx0, sy0 := centroid(3, 23)
	before := math.Hypot(gx0-sx0, gy0-sy0)
	for i := 0; i < 240; i++ {
		w.Step(1.0 / 60)
	}
	gx1, gy1 := centroid(0, 3)
	sx1, sy1 := centroid(3, 23)
	after := math.Hypot(gx1-sx1, gy1-sy1)
	if after >= before-5.0 {
		t.Fatalf("small group did not migrate to closest school: gap %.2f -> %.2f", before, after)
	}
}

// Two full schools of the same size merge when close, even beyond the
// perception radius: two groups of 8 large fish (min 7) 12 cells apart.
func TestFullSchoolsMergeWhenClose(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(120, 40)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = nil
	for k := 0; k < 8; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 40 + float64(k)*0.3, Y: 20, Vx: 1, Vy: 0,
			Species: 2, Alive: true, Depth: 0.8, MillDir: 1,
		})
	}
	for k := 0; k < 8; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 52 + float64(k)*0.3, Y: 20, Vx: 1, Vy: 0,
			Species: 2, Alive: true, Depth: 0.8, MillDir: 1,
		})
	}
	centroid := func(lo, hi int) (float64, float64) {
		var x, y float64
		for _, f := range w.Fish[lo:hi] {
			x += f.X
			y += f.Y
		}
		n := float64(hi - lo)
		return x / n, y / n
	}
	fx0, fy0 := centroid(0, 8)
	sx0, sy0 := centroid(8, 16)
	before := math.Hypot(fx0-sx0, fy0-sy0)
	for i := 0; i < 240; i++ {
		w.Step(1.0 / 60)
	}
	fx1, fy1 := centroid(0, 8)
	sx1, sy1 := centroid(8, 16)
	after := math.Hypot(fx1-sx1, fy1-sy1)
	if after >= before-1.0 {
		t.Fatalf("same-size schools did not merge when close: gap %.2f -> %.2f", before, after)
	}
}
