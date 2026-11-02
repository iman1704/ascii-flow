package world

import (
	"math"
	"testing"
)

// A distant hunter startles without panicking: alarm must fall off
// quadratically with distance, so a shark at 0.7 fear radii leaves the
// fish calm enough to keep schooling while a shark on top of it still
// causes full panic. The escape reflex itself (dodge, bolt) keeps its
// wider reach; only the sustained alarm state is gated.
func TestDistantHunterStartlesWithoutPanicking(t *testing.T) {
	setup := func(fishX float64) *World {
		w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
		w.Resize(80, 24)
		sh := &w.Sharks[0]
		sh.Nodes[0], sh.Nodes[1] = 30, 12
		for n := 1; n < SHARK_NODES; n++ {
			sh.Nodes[n*2] = 30 - float64(n)*0.5
			sh.Nodes[n*2+1] = 12
		}
		sh.Heading = 0
		sh.Speed = 0
		sh.Hunger = 0
		sh.RetargetAt = 1e9
		sh.Target = nil
		sh.Depth = 0.9
		sh.Dz = 0
		sh.BreachAt = 1e9
		w.Predators = nil
		w.Fish = []Fish{{
			X: fishX, Y: 12, Vx: 0, Vy: 0, Species: 0,
			Alive: true, Depth: 0.9, MillDir: 1,
		}}
		return w
	}
	// Far: 10 cells from the snout (~0.7 fear radii). Must stay calm.
	far := setup(40)
	for i := 0; i < 30; i++ {
		far.Step(1.0 / 60)
	}
	if got := far.Fish[0].Alarm; got >= 0.15 {
		t.Fatalf("distant shark panicked the fish: alarm=%.3f, want < 0.15", got)
	}
	// Near: 1 cell from the snout. Must still cause full panic.
	near := setup(31)
	for i := 0; i < 30; i++ {
		near.Step(1.0 / 60)
	}
	if got := near.Fish[0].Alarm; got < 0.5 {
		t.Fatalf("close shark did not panic the fish: alarm=%.3f, want >= 0.5", got)
	}
}

// In busy water (default sharks and predators, plankton cleared for
// determinism) schools must spend most of their time cruising level:
// total horizontal travel beats vertical travel and most fish-time is
// calm, instead of the school boiling in place.
func TestBusyWaterSwimsLevel(t *testing.T) {
	w := NewWorld(nil, 60)
	w.Resize(80, 24)
	var sumDx, sumDy float64
	var calm, total int64
	prev := make([][2]float64, len(w.Fish))
	alivePrev := make([]bool, len(w.Fish))
	for i := range w.Fish {
		prev[i] = [2]float64{w.Fish[i].X, w.Fish[i].Y}
		alivePrev[i] = true
	}
	for s := 0; s < 5400; s++ {
		w.Step(1.0 / 60)
		w.FeedingSpots = nil
		for i := range w.Fish {
			f := &w.Fish[i]
			if !f.Alive {
				alivePrev[i] = false
				continue
			}
			if alivePrev[i] {
				dx := math.Abs(f.X - prev[i][0])
				dy := math.Abs(f.Y - prev[i][1])
				if dx+dy < 4 { // skip respawn teleports to a new edge
					sumDx += dx
					sumDy += dy
				}
			}
			prev[i] = [2]float64{f.X, f.Y}
			alivePrev[i] = true
			total++
			if f.Alarm <= 0.15 {
				calm++
			}
		}
	}
	if sumDx <= 1.15*sumDy {
		t.Fatalf("busy water moved vertically more than horizontally: dx=%.0f dy=%.0f", sumDx, sumDy)
	}
	if calmPct := float64(calm) / float64(total); calmPct <= 0.30 {
		t.Fatalf("schools almost never calm: %.0f%% calm fish-time", 100*calmPct)
	}
}
