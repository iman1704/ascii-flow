package world

import (
	"math"
	"testing"
)

// Calm schools should work the water column, not just screen left and
// right: in predator-free water the average vertical speed is a real
// share of the swim. Without the vertical drive schools contour level
// (mean|Vy| well under 1.8); with it they tour at ≈ 2.2 and up.
func TestCalmSchoolsTravelVerticallyToo(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.NoSharks = true
	w.NoPelagics = true
	w.Resize(120, 40)
	w.Sharks = nil
	w.Predators = nil
	pinCanvasChurn(w)
	var sum float64
	var n int
	for s := 0; s < 3600; s++ {
		w.Step(1.0 / 60)
		w.FeedingSpots = nil
		if s < 600 { // let the seeded schools settle into the cruise
			continue
		}
		for i := range w.Fish {
			if f := &w.Fish[i]; f.Alive {
				v := f.Vy
				if v < 0 {
					v = -v
				}
				sum += v
				n++
			}
		}
	}
	if got := sum / float64(n); got < 1.8 {
		t.Fatalf("calm schools barely move vertically: mean|Vy|=%.2f, want >= 1.8", got)
	}
}

// School depth should read as a spread water column, not a flat layer:
// the cross-school depth standard deviation must clear a floor. Before
// the rework every fish relaxed to the same baseline (std ≈ 0.05).
func TestSchoolDepthsVary(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 60)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	pinCanvasChurn(w)
	var stdSum float64
	var samples int
	for s := 0; s < 3600; s++ {
		w.Step(1.0 / 60)
		w.FeedingSpots = nil
		if s%60 != 0 {
			continue
		}
		var mean float64
		var n int
		for i := range w.Fish {
			if w.Fish[i].Alive {
				mean += w.Fish[i].Depth
				n++
			}
		}
		if n == 0 {
			continue
		}
		mean /= float64(n)
		var v float64
		for i := range w.Fish {
			if w.Fish[i].Alive {
				d := w.Fish[i].Depth - mean
				v += d * d
			}
		}
		stdSum += math.Sqrt(v / float64(n))
		samples++
	}
	if got := stdSum / float64(samples); got < 0.08 {
		t.Fatalf("school depths nearly flat: std=%.3f, want >= 0.08", got)
	}
}

// Natural bait-fish movement: a slow fish caught by a strong lateral threat
// must carve a turn like a real fish (speed-dependent turn rate), not snap
// sideways in a single step. Before the rework a slow fish under lateral
// flee turned ~0.9 rad in one 1/60s step (~54 rad/s); the limit holds it to
// a believable dart.
func TestFishTurnRateLimitedUnderLoad(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(240, 100)
	w.Predators = nil
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 60, 47
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 60 - float64(n)*0.5
		sh.Nodes[n*2+1] = 47
	}
	sh.Heading = 0
	sh.Speed = 0
	sh.Hunger = 0
	sh.RetargetAt = 1e9
	sh.Target = nil
	sh.Depth = 0.8
	sh.Dz = 0
	sh.BreachAt = 1e9
	w.Fish = []Fish{{
		X: 60, Y: 50, Vx: 0.5, Vy: 0, Species: 0,
		Alive: true, Depth: 0.8, MillDir: 1,
	}}
	w.FeedingSpots = nil
	before := math.Atan2(w.Fish[0].Vy, w.Fish[0].Vx)
	w.Step(1.0 / 60)
	after := math.Atan2(w.Fish[0].Vy, w.Fish[0].Vx)
	d := after - before
	for d > math.Pi {
		d -= 2 * math.Pi
	}
	for d < -math.Pi {
		d += 2 * math.Pi
	}
	if math.Abs(d) > 0.35 {
		t.Fatalf("slow fish snapped %.3f rad in one step, want a carved turn <= 0.35", math.Abs(d))
	}
}

// Same encounter, longitudinal axis: one step must not teleport the speed.
// Before the rework the same lateral scare changed speed by ~2.25 cells/s in
// a single step (~135 cells/s²); the acceleration cap holds the dart to a
// muscular but believable surge.
func TestFishAccelerationLimitedUnderLoad(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(240, 100)
	w.Predators = nil
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 60, 47
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 60 - float64(n)*0.5
		sh.Nodes[n*2+1] = 47
	}
	sh.Heading = 0
	sh.Speed = 0
	sh.Hunger = 0
	sh.RetargetAt = 1e9
	sh.Target = nil
	sh.Depth = 0.8
	sh.Dz = 0
	sh.BreachAt = 1e9
	w.Fish = []Fish{{
		X: 60, Y: 50, Vx: 0.5, Vy: 0, Species: 0,
		Alive: true, Depth: 0.8, MillDir: 1,
	}}
	w.FeedingSpots = nil
	before := math.Hypot(w.Fish[0].Vx, w.Fish[0].Vy)
	w.Step(1.0 / 60)
	after := math.Hypot(w.Fish[0].Vx, w.Fish[0].Vy)
	if dv := math.Abs(after - before); dv > 1.2 {
		t.Fatalf("fish speed jumped %.3f cells/s in one step, want a surge <= 1.2", dv)
	}
}
