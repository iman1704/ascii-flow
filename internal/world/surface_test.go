package world

import (
	"testing"
)

// surfaceBand matches the renderer's clamp so sim and drawing agree.
func TestSurfaceBandClamp(t *testing.T) {
	if got := surfaceBand(30); got != 4.2 {
		t.Fatalf("surfaceBand(30) = %v, want 4.2", got)
	}
	if got := surfaceBand(10); got != 3 {
		t.Fatalf("surfaceBand(10) = %v, want 3 (minimum)", got)
	}
	if got := surfaceBand(100); got != 6 {
		t.Fatalf("surfaceBand(100) = %v, want 6 (maximum)", got)
	}
}

func TestNearSurfaceFish(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	shallow := &Fish{X: 4, Y: 3, Depth: 1}
	if !w.nearSurfaceFish(shallow) {
		t.Fatal("shallow fish not detected as near the surface")
	}
	deep := &Fish{X: 4, Y: 8, Depth: 0.35}
	if w.nearSurfaceFish(deep) {
		t.Fatal("deep fish detected as near the surface")
	}
}

// A fish already at the surface bolts upward when chased instead of
// diving: its vertical velocity turns up and its depth stays shallow.
func TestFishSurfaceBoltRisesWhenChased(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Mackerel, 5, 10, 3)}
	w.Predators[0].Hunger = 0
	w.Predators[0].RetargetAt = 1e9
	w.Predators[0].Depth = 0.7
	w.Fish = []Fish{{X: 12, Y: 3, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1}}
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	f := &w.Fish[0]
	if !f.Alive {
		t.Fatal("surface fish was eaten during the bolt test")
	}
	if f.Vy >= -0.5 {
		t.Fatalf("chased surface fish did not bolt upward: vy=%.3f", f.Vy)
	}
	if f.Depth < 0.85 {
		t.Fatalf("chased surface fish dove instead of staying shallow: depth=%.3f", f.Depth)
	}
}

// A schooling predator chased by a deeper shark near the surface rises
// and keeps shallow instead of returning to its patrol depth.
func TestPredatorSurfaceBoltWhenChasedByShark(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 6, 3
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 6 - float64(n)*0.5
		sh.Nodes[n*2+1] = 3
	}
	sh.Heading = 0
	sh.Speed = 0
	sh.Hunger = 0
	sh.RetargetAt = 1e9
	sh.Target = nil
	sh.TargetPred = nil
	sh.Depth = 0.5
	sh.BreachAt = 1e9
	w.Predators = []Predator{testPredator(Tuna, 5, 12, 3)}
	w.Predators[0].Depth = 0.9
	w.Fish = nil

	before := w.Predators[0].Nodes[1]
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	p := &w.Predators[0]
	if !p.Alive {
		t.Fatal("surface predator was eaten during the bolt test")
	}
	if p.Depth < 0.9 {
		t.Fatalf("chased surface predator did not stay shallow: depth=%.3f", p.Depth)
	}
	if p.Nodes[1] >= before {
		t.Fatalf("chased surface predator did not move upward: y %.2f -> %.2f", before, p.Nodes[1])
	}
}

// A calm fish on a voluntary surface break climbs to the top rows and
// rises to the surface depth even with no predator around.
func TestCalmFishSurfaceBreakRises(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{{
		X: 40, Y: 3, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.9,
		MillDir: 1, SurfaceRollAt: 1e9, SurfaceUntil: 100,
	}}
	minY := w.Fish[0].Y
	for i := 0; i < 90; i++ {
		w.Step(1.0 / 60)
		if w.Fish[0].Y < minY {
			minY = w.Fish[0].Y
		}
	}
	f := &w.Fish[0]
	if f.Depth < 0.95 {
		t.Fatalf("surface-breaking fish did not rise to the surface: depth=%.3f", f.Depth)
	}
	if minY >= 2.5 {
		t.Fatalf("surface-breaking fish did not climb toward the top: minY=%.2f", minY)
	}
}

// A calm fish near the surface must not randomly start a voluntary
// surface break: no break may be scheduled over a long calm run.
func TestCalmFishSchedulesNoSurfaceBreak(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{{
		X: 40, Y: 3, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.9,
		MillDir: 1, SurfaceRollAt: 0, SurfaceUntil: 0,
	}}
	for i := 0; i < 3600; i++ {
		f := &w.Fish[0]
		f.X, f.Y = 40, 3
		f.Depth = 0.9
		w.Step(1.0 / 60)
		if w.Fish[0].SurfaceUntil != 0 {
			t.Fatalf("calm fish scheduled a surface break at step %d: SurfaceUntil=%.3f", i, w.Fish[0].SurfaceUntil)
		}
	}
}

// A shark with no prey schedules a surface break, then rises toward the
// top rows and shallow depth while the break lasts.
func TestSharkBreachesSurface(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 40, 16
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 40 - float64(n)*0.5
		sh.Nodes[n*2+1] = 16
	}
	sh.Heading = 0
	sh.Speed = 0
	sh.Hunger = 0
	sh.RetargetAt = 1e9
	sh.Target = nil
	sh.TargetPred = nil
	sh.Depth = 0.5
	sh.Dz = 0
	sh.BreachAt = 0
	sh.BreachUntil = 0
	w.Fish = nil
	w.Predators = nil

	for i := 0; i < 90; i++ {
		w.Step(1.0 / 60)
	}
	if w.Sharks[0].BreachUntil <= 0 {
		t.Fatal("breach was never scheduled")
	}
	if w.Sharks[0].Depth <= 0.8 {
		t.Fatalf("breaching shark did not rise to the surface: depth=%.3f", w.Sharks[0].Depth)
	}
	if w.Sharks[0].Nodes[1] >= 16 {
		t.Fatalf("breaching shark did not climb toward the surface: y=%.2f", w.Sharks[0].Nodes[1])
	}
}

// A mid-water fish dives deeper when its hunter attacks from a shallower
// depth: depth drops and it pushes downward, away from the hunter.
func TestFishDivesWhenHunterShallower(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Mackerel, 5, 10, 12)}
	w.Predators[0].Hunger = 0
	w.Predators[0].RetargetAt = 1e9
	w.Predators[0].Depth = 0.95
	w.Fish = []Fish{{X: 12, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.85, MillDir: 1}}
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	f := &w.Fish[0]
	if !f.Alive {
		t.Fatal("fish was eaten during the dive test")
	}
	if f.Depth >= 0.85 {
		t.Fatalf("chased fish did not dive from shallower hunter: depth=%.3f", f.Depth)
	}
	if f.Vy <= 0.5 {
		t.Fatalf("chased fish did not push downward: vy=%.3f", f.Vy)
	}
}

// A mid-water fish rises toward the surface when its hunter attacks from
// a deeper depth: depth climbs and it dashes upward.
func TestFishRisesWhenHunterDeeper(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Mackerel, 5, 10, 12)}
	w.Predators[0].Hunger = 0
	w.Predators[0].RetargetAt = 1e9
	w.Predators[0].Depth = 0.35
	w.Fish = []Fish{{X: 12, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.6, MillDir: 1}}
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	f := &w.Fish[0]
	if !f.Alive {
		t.Fatal("fish was eaten during the rise test")
	}
	if f.Depth <= 0.6 {
		t.Fatalf("chased fish did not rise from deeper hunter: depth=%.3f", f.Depth)
	}
	if f.Vy >= -0.5 {
		t.Fatalf("chased fish did not dash upward: vy=%.3f", f.Vy)
	}
}

// A level attack always rises: prey flees upward toward the surface
// when the hunter attacks from the same depth.
func TestFishLevelAttackSplitsEvenly(t *testing.T) {
	for trial := 0; trial < 50; trial++ {
		w := NewWorld([]SharkSpec{}, 0)
		w.Resize(80, 24)
		w.Sharks = nil
		w.Predators = []Predator{testPredator(Mackerel, 5, 10, 12)}
		w.Predators[0].Hunger = 0
		w.Predators[0].RetargetAt = 1e9
		w.Predators[0].Depth = 0.6
		w.Fish = []Fish{{X: 12, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.6, MillDir: 1}}
		w.Step(1.0 / 60)
		if got := w.Fish[0].FleeDir; got != 1 {
			t.Fatalf("trial %d: level attack FleeDir = %d, want 1 (rise)", trial, got)
		}
	}
}

// A schooling predator dives when its shark attacks from a shallower
// depth and rises when the shark attacks from deeper.
func TestPredatorDivesAndRisesWithSharkDepth(t *testing.T) {
	diveCase := func(sharkDepth, predDepth float64) *World {
		w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
		w.Resize(80, 24)
		sh := &w.Sharks[0]
		sh.Nodes[0], sh.Nodes[1] = 6, 12
		for n := 1; n < SHARK_NODES; n++ {
			sh.Nodes[n*2] = 6 - float64(n)*0.5
			sh.Nodes[n*2+1] = 12
		}
		sh.Heading = 0
		sh.Speed = 0
		sh.Hunger = 0
		sh.RetargetAt = 1e9
		sh.Target = nil
		sh.TargetPred = nil
		sh.Depth = sharkDepth
		sh.BreachAt = 1e9
		w.Predators = []Predator{testPredator(Tuna, 5, 12, 12)}
		w.Predators[0].Depth = predDepth
		w.Fish = nil
		return w
	}
	w := diveCase(0.9, 0.6)
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	if got := w.Predators[0].Depth; got >= 0.6 {
		t.Fatalf("predator did not dive from shallower shark: depth=%.3f", got)
	}
	w = diveCase(0.3, 0.6)
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	if got := w.Predators[0].Depth; got <= 0.6 {
		t.Fatalf("predator did not rise from deeper shark: depth=%.3f", got)
	}
}

// A level shark attack always rises: schooling predators flee upward
// toward the surface when the shark attacks from the same depth.
func TestPredatorLevelAttackSplitsEvenly(t *testing.T) {
	for trial := 0; trial < 50; trial++ {
		w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
		w.Resize(80, 24)
		sh := &w.Sharks[0]
		sh.Nodes[0], sh.Nodes[1] = 6, 12
		for n := 1; n < SHARK_NODES; n++ {
			sh.Nodes[n*2] = 6 - float64(n)*0.5
			sh.Nodes[n*2+1] = 12
		}
		sh.Heading = 0
		sh.Speed = 0
		sh.Hunger = 0
		sh.RetargetAt = 1e9
		sh.Target = nil
		sh.TargetPred = nil
		sh.Depth = 0.6
		sh.BreachAt = 1e9
		w.Predators = []Predator{testPredator(Tuna, 5, 12, 12)}
		w.Predators[0].Depth = 0.6
		w.Fish = nil
		w.Step(1.0 / 60)
		if got := w.Predators[0].FleeDir; got != 1 {
			t.Fatalf("trial %d: level attack predator FleeDir = %d, want 1 (rise)", trial, got)
		}
	}
}
