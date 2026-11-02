package renderer

import (
	"math"
	"testing"

	"github.com/stackedlol/ascii-sharks/internal/world"
)

// Only creature-caused events ring: a catch near the surface spawns a
// ripple, while ambient splashes only feed the wake field.
func TestRipplesNeedCreatureCause(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{Width: 60, Height: 30}
	w.Events = append(w.Events, world.WorldEvent{Kind: "eat", X: 30, Y: 1})
	r.consumeEvents(w, 10)
	if len(r.ripples) != 1 {
		t.Fatalf("surface catch spawned %d ripples, want 1", len(r.ripples))
	}

	r2 := New()
	r2.Resize(60, 30)
	w2 := &world.World{Width: 60, Height: 30}
	w2.Events = append(w2.Events, world.WorldEvent{Kind: "splash", X: 30, Y: 1})
	r2.consumeEvents(w2, 10)
	if len(r2.ripples) != 0 {
		t.Fatalf("ambient splash spawned %d ripples, want 0", len(r2.ripples))
	}

	r3 := New()
	r3.Resize(60, 30)
	w3 := &world.World{Width: 60, Height: 30}
	w3.Events = append(w3.Events, world.WorldEvent{Kind: "eat", X: 30, Y: 20})
	r3.consumeEvents(w3, 10)
	if len(r3.ripples) != 0 {
		t.Fatalf("deep catch spawned %d ripples, want 0", len(r3.ripples))
	}
}

// A feeding school's ring must read as a ring, not background churn:
// each shallow feed event spawns one wide, strong ripple (wider and
// stronger than a catch ripple), while a deep one stays ring-free.
func TestFeedEventSpawnsRingRipple(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{Width: 60, Height: 30}
	w.Events = append(w.Events, world.WorldEvent{Kind: "feed", X: 30, Y: 1})
	r.consumeEvents(w, 10)
	if len(r.ripples) != 1 {
		t.Fatalf("surface feed spawned %d ripples, want 1", len(r.ripples))
	}
	if got := r.ripples[0].scale; got != 1.4 {
		t.Fatalf("feed ripple scale = %v, want 1.4 (wider than a catch ripple)", got)
	}
	if got := r.ripples[0].strength; got != 0.9 {
		t.Fatalf("feed ripple strength = %v, want 0.9 (stronger than a catch ripple)", got)
	}

	r2 := New()
	r2.Resize(60, 30)
	w2 := &world.World{Width: 60, Height: 30}
	w2.Events = append(w2.Events, world.WorldEvent{Kind: "feed", X: 30, Y: 20})
	r2.consumeEvents(w2, 10)
	if len(r2.ripples) != 0 {
		t.Fatalf("deep feed spawned %d ripples, want 0", len(r2.ripples))
	}
}

// A calm fish near the surface rings only while moving up toward it;
// level, diving, and deep fish stay ring-free.
func TestCalmFishNearSurfaceEmitsRipple(t *testing.T) {
	slam := func(y, vy float64) int {
		r := New()
		r.Resize(60, 30)
		w := &world.World{
			Width: 60, Height: 30,
			Fish: []world.Fish{
				{X: 20, Y: y, Vx: 1, Vy: vy, Species: 0, Alive: true, Depth: 1},
			},
		}
		for i := 0; i < 100; i++ {
			w.Time += 1.0 / 30
			r.Render(w, 1.0/30)
		}
		return len(r.ripples)
	}
	if n := slam(1, -1); n == 0 {
		t.Fatal("rising surface fish emitted no ripples")
	}
	if n := slam(1, 0); n != 0 {
		t.Fatalf("level fish emitted %d ripples, want 0", n)
	}
	if n := slam(1, 1); n != 0 {
		t.Fatalf("diving fish emitted %d ripples, want 0", n)
	}
	if n := slam(20, -1); n != 0 {
		t.Fatalf("deep fish emitted %d ripples, want 0", n)
	}
}

// Sharks and schooling predators ring from the snout at the surface,
// but only while moving up toward it.
func TestHunterNearSurfaceEmitsRipple(t *testing.T) {
	nodesAt := func(y float64) []float64 {
		n := make([]float64, world.SHARK_NODES*2)
		for k := 0; k < world.SHARK_NODES; k++ {
			n[k*2] = 30 - float64(k)*0.5
			n[k*2+1] = y
		}
		return n
	}
	slam := func(heading float64) int {
		r := New()
		r.Resize(60, 30)
		w := &world.World{
			Width: 60, Height: 30,
			Sharks: []world.Shark{
				{ID: 1, Kind: world.Classic, Nodes: nodesAt(1), Speed: 2.5, Depth: 1, Heading: heading},
			},
			Predators: []world.Predator{
				{ID: 2, Nodes: nodesAt(1), Speed: 2.5, Depth: 1, Alive: true, Heading: heading},
			},
		}
		for i := 0; i < 100; i++ {
			w.Time += 1.0 / 30
			r.Render(w, 1.0/30)
		}
		return len(r.ripples)
	}
	if n := slam(-math.Pi / 2); n == 0 {
		t.Fatal("rising surface hunters emitted no ripples")
	}
	if n := slam(math.Pi / 2); n != 0 {
		t.Fatalf("diving hunters emitted %d ripples, want 0", n)
	}
}

// A calm fish on a voluntary surface break rings even when cruising level
// at the surface, not just while moving up.
func TestSurfaceBreakFishEmitsRippleWhenLevel(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 20, Y: 1, Vx: 1, Vy: 0, Species: 0, Alive: true, Depth: 1, SurfaceUntil: 10},
		},
	}
	for i := 0; i < 100; i++ {
		w.Time += 1.0 / 30
		r.Render(w, 1.0/30)
	}
	if len(r.ripples) == 0 {
		t.Fatal("surface-breaking level fish emitted no ripples")
	}
}

// Bigger, faster, harder action rings wider: a fleeing large fish out-rings
// a cruising small one, and a hunting shark out-rings a patrolling one.
func TestRippleScaleGrowsWithSizeAndSpeed(t *testing.T) {
	small := &world.Fish{Species: 0, Alarm: 0}
	large := &world.Fish{Species: 2, Alarm: 1}
	calmSmall := fishRippleScale(small, 46.0/12)
	calmLarge := fishRippleScale(&world.Fish{Species: 2}, 32.0/12)
	fleeLarge := fishRippleScale(large, 60.0/12)
	if calmLarge <= calmSmall {
		t.Fatalf("cruising large fish scale %v not bigger than cruising small %v", calmLarge, calmSmall)
	}
	if fleeLarge <= calmLarge {
		t.Fatalf("fleeing large fish scale %v not bigger than cruising %v", fleeLarge, calmLarge)
	}
	patrol := hunterRippleScale(20, 2.5)
	hunt := hunterRippleScale(20, 8)
	grown := hunterRippleScale(30, 2.5)
	if hunt <= patrol {
		t.Fatalf("hunting shark scale %v not bigger than patrol %v", hunt, patrol)
	}
	if grown <= patrol {
		t.Fatalf("long shark scale %v not bigger than short %v", grown, patrol)
	}
}

// Small rings die young: after the small ring's life only the large one
// remains, and both are gone after the large ring's life.
func TestRipplesPrunedAfterLife(t *testing.T) {
	_, smallLife := rippleExtent(0.5)
	_, largeLife := rippleExtent(2.0)
	if smallLife >= largeLife {
		t.Fatalf("small ring life %v not shorter than large %v", smallLife, largeLife)
	}
	r := New()
	r.Resize(60, 30)
	r.spawnRipple(30, 2, 0, 1, 0.5)
	r.spawnRipple(30, 2, 0, 1, 2.0)
	r.pruneRipples(smallLife + 0.1)
	if len(r.ripples) != 1 {
		t.Fatalf("expected only the large ripple left, got %d", len(r.ripples))
	}
	r.pruneRipples(largeLife + 0.1)
	if len(r.ripples) != 0 {
		t.Fatalf("expired ripples not pruned: %d left", len(r.ripples))
	}
}

// An empty ocean draws no water decoration on its own.
func TestEmptyFrameHasNoWaterDots(t *testing.T) {
	r := New()
	r.Resize(40, 20)
	w := &world.World{Width: 40, Height: 20}
	frame := r.Render(w, 1.0/30)
	for _, ch := range frame {
		if ch == '·' {
			t.Fatal("empty frame has water dots; no ambient water should draw")
		}
	}
}

// The surface line was removed: no '~' cells may appear.
func TestNoSurfaceLine(t *testing.T) {
	r := New()
	r.Resize(40, 20)
	w := world.NewWorld([]world.SharkSpec{{Kind: world.Classic, Size: 1}}, 10)
	w.Resize(40, 20)
	frame := r.Render(w, 1.0/30)
	for _, ch := range frame {
		if ch == '~' {
			t.Fatal("frame has a '~' surface line cell; the surface line should be gone")
		}
	}
}
