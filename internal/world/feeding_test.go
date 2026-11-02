package world

import (
	"math"
	"sort"
	"testing"
)

// A fresh spot gathers calm bait fish: a nearby schooler must move toward
// it within a few seconds when no hunter is around.
func TestFeedingSpotGathersNearbyFish(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{
		{X: 20, Y: 6, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.9, MillDir: 1},
	}
	w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(28, 6, 0.9)}
	before := dist2D(w.Fish[0].X, w.Fish[0].Y, w.FeedingSpots[0].X, w.FeedingSpots[0].Y)
	for i := 0; i < 180; i++ {
		w.Step(1.0 / 60)
		if len(w.FeedingSpots) == 0 {
			t.Fatal("feeding spot vanished before the fish could gather")
		}
	}
	after := dist2D(w.Fish[0].X, w.Fish[0].Y, w.FeedingSpots[0].X, w.FeedingSpots[0].Y)
	if after >= before {
		t.Fatalf("fish did not gather at the spot: dist %.2f -> %.2f", before, after)
	}
}

// Feeding is nibbled away: a crowd on the spot depletes it until it is gone.
func TestFeedingSpotDepletedByCrowd(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{
		{X: 28, Y: 6, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.9, MillDir: 1},
		{X: 28.5, Y: 6.2, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.9, MillDir: 1},
	}
	spot := w.newFeedingSpotAt(28, 6, 0.9)
	spot.Food = 0.5
	w.FeedingSpots = []FeedingSpot{spot}
	born := w.FeedingSpots[0].BornAt
	// Drive depletion directly: background spawns during a full Step
	// would be counted as survivors, so assert on the original spot.
	for i := 0; i < 120; i++ {
		w.Time += 1.0 / 60
		w.updateFeedingSpots(1.0 / 60)
	}
	for _, s := range w.FeedingSpots {
		if s.BornAt == born {
			t.Fatalf("crowded spot was not eaten away: food=%.2f", s.Food)
		}
	}
}

// Any hunter scatters feeders: a shark next to the spot must stop the
// gathering pull so the fish bolts instead of nibbling.
func TestFeedingInterruptedByShark(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	w.Predators = nil // the flight must answer the shark, not a tuna
	w.Fish = []Fish{
		{X: 28, Y: 6, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.9, MillDir: 1},
	}
	w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(29, 6, 0.9)}
	w.Sharks[0].Nodes[0], w.Sharks[0].Nodes[1] = 28.5, 6
	w.Sharks[0].Heading = 0
	w.Sharks[0].Depth = 0.9
	w.Sharks[0].Hunger = 0
	w.Sharks[0].RetargetAt = 1e9
	for i := 0; i < 60; i++ {
		w.Step(1.0 / 60)
	}
	if w.Fish[0].Alarm < 0.25 {
		t.Fatalf("fish kept calm next to a shark at its feeding spot: alarm=%.2f", w.Fish[0].Alarm)
	}
	// A schooling predator scatters feeders the same way: fear answers
	// the hunter's presence, so a satiated non-hunting tuna still
	// interrupts (and cannot eat the test fish mid-phase).
	w.Sharks = nil
	p := testPredator(Mackerel, 5, 28.5, 6)
	p.Hunger = 0
	p.RetargetAt = 1e9
	p.Depth = 0.9
	w.Predators = []Predator{p}
	f := &w.Fish[0]
	f.X, f.Y = 28, 6
	f.Vx, f.Vy = 0, 0
	f.Depth = 0.9
	f.Alarm = 0
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	if w.Fish[0].Alarm < 0.25 {
		t.Fatalf("fish kept calm next to a hunting predator at its feeding spot: alarm=%.2f", w.Fish[0].Alarm)
	}
}

// Feeding near the surface rings the water: a calm crowd on a shallow spot
// must emit feed events for the renderer to turn into ripples.
func TestFeedingEmitsRipplesNearSurface(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{
		{X: 28, Y: 2, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 28.5, Y: 2.2, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 27.6, Y: 2.3, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(28, 2, 1)}
	feeds := 0
	for i := 0; i < 300; i++ {
		w.Step(1.0 / 60)
		for _, e := range w.Events {
			if e.Kind == "feed" {
				feeds++
			}
		}
		w.Events = w.Events[:0]
		if len(w.FeedingSpots) == 0 {
			break
		}
	}
	if feeds == 0 {
		t.Fatal("feeding crowd near the surface emitted no feed events")
	}
}

// A gathered ball orbits at a few cells out, not on top of the spot:
// calm fish at ball range must still nibble and ring, or visible
// bundles never produce ripples.
func TestFeedingBallRangeNibbles(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{
		{X: 33.5, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 26.5, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 30, Y: 7.5, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(30, 4, 1)}
	before := w.FeedingSpots[0].Food
	feeds := 0
	for i := 0; i < 60; i++ {
		w.updateFeedingSpots(1.0 / 60)
		for _, e := range w.Events {
			if e.Kind == "feed" {
				feeds++
			}
		}
		w.Events = w.Events[:0]
	}
	if w.FeedingSpots[0].Food >= before {
		t.Fatalf("ball-range fish did not nibble: food=%.2f", w.FeedingSpots[0].Food)
	}
	if feeds == 0 {
		t.Fatal("ball-range feeding emitted no feed events for ripples")
	}
}

// Ripples ring the ball's rim, not its centre: feed events must land
// away from the spot and scatter around it instead of stacking.
func TestFeedingRipplesRingTheEdge(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{
		{X: 33, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 27, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 30, Y: 7, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 30, Y: 1, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(30, 4, 1)}
	sx, sy := w.FeedingSpots[0].X, w.FeedingSpots[0].Y
	var xs, ys []float64
	for i := 0; i < 300 && len(xs) < 6; i++ {
		w.Time += 1.0 / 60
		w.updateFeedingSpots(1.0 / 60)
		for _, e := range w.Events {
			if e.Kind == "feed" {
				xs = append(xs, e.X)
				ys = append(ys, e.Y)
			}
		}
		w.Events = w.Events[:0]
	}
	if len(xs) < 4 {
		t.Fatalf("ball edge emitted %d feed events, want at least 4", len(xs))
	}
	minX, maxX, minY, maxY := xs[0], xs[0], ys[0], ys[0]
	for i := range xs {
		if d := dist2D(xs[i], ys[i], sx, sy); d < 1.5*1.5 {
			t.Fatalf("feed event too close to the centre: dist²=%.2f", d)
		}
		minX, maxX = min(minX, xs[i]), max(maxX, xs[i])
		minY, maxY = min(minY, ys[i]), max(maxY, ys[i])
	}
	if maxX-minX < 1 && maxY-minY < 1 {
		t.Fatal("feed events stacked in one place instead of ringing the edge")
	}
}

// One ripple tick rings the whole school at once: a small ball emits 3
// feed events and a large ball 5, spaced around the circumference so the
// ripples encircle the school instead of pulsing per fish.
func TestFeedingRingEncirclesSchool(t *testing.T) {
	ring := func(fish []Fish, want int) {
		w := NewWorld([]SharkSpec{}, 0)
		w.Resize(80, 24)
		w.Sharks = nil
		w.Predators = nil
		w.Fish = fish
		w.FeedingSpots = []FeedingSpot{w.newFeedingSpotAt(30, 4, 1)}
		w.Time += 1.0 / 60
		w.updateFeedingSpots(1.0 / 60)
		var xs, ys []float64
		for _, e := range w.Events {
			if e.Kind == "feed" {
				xs = append(xs, e.X)
				ys = append(ys, e.Y)
			}
		}
		if len(xs) != want {
			t.Fatalf("one tick emitted %d feed events, want %d", len(xs), want)
		}
		angles := make([]float64, len(xs))
		for i := range xs {
			dx, dy := xs[i]-30, ys[i]-4
			if dx*dx+dy*dy < 1.5*1.5 {
				t.Fatalf("ring event %d too close to the centre", i)
			}
			angles[i] = math.Atan2(dy, dx)
		}
		sort.Float64s(angles)
		maxGap := 0.0
		for i := range angles {
			next := angles[(i+1)%len(angles)]
			gap := next - angles[i]
			if gap < 0 {
				gap += 2 * math.Pi
			}
			maxGap = math.Max(maxGap, gap)
		}
		if maxGap > math.Pi {
			t.Fatalf("ring does not encircle the school: max angular gap %.2f rad", maxGap)
		}
	}
	small := []Fish{
		{X: 33, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 27, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 30, Y: 7, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 30, Y: 1, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	ring(small, 3)
	large := append(append([]Fish{}, small...),
		Fish{X: 32, Y: 5, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		Fish{X: 28, Y: 3, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		Fish{X: 31, Y: 6, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		Fish{X: 29, Y: 2, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		Fish{X: 30, Y: 4, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	)
	ring(large, 5)
}

func dist2D(x1, y1, x2, y2 float64) float64 {
	dx, dy := x1-x2, y1-y2
	return dx*dx + dy*dy
}
