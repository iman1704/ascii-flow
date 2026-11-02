package world

import (
	"math"
	"testing"
)

// Seed fills the arena with the default roster and area-scaled schools of
// live fish, so Step has real schools and sharks to move.
func TestSeedSpawnsSchools(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Resize(80, 24)
	if len(w.Sharks) != len(DefaultSharks()) {
		t.Fatalf("expected %d sharks, got %d", len(DefaultSharks()), len(w.Sharks))
	}
	if len(w.Fish) < 12 {
		t.Fatalf("expected area-scaled schools, got %d fish", len(w.Fish))
	}
	for _, f := range w.Fish {
		if !f.Alive {
			t.Fatal("seeded fish should start alive")
		}
	}
	for _, sh := range w.Sharks {
		if sh.Speed <= 0 {
			t.Fatal("seeded sharks should start moving")
		}
	}
}

// Task 2: schools move together (shared heading carries them forward and
// neighbours keep them grouped) instead of sitting static.
func TestFishSchoolSwimsTogether(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil // isolate schooling from predation, like sharks above
	w.Fish = []Fish{
		{X: 20, Y: 12, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 22, Y: 12, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
		{X: 21, Y: 13, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	x0 := w.Fish[0].X
	w.Step(1.0 / 60)
	if w.Fish[0].X == x0 && w.Fish[0].Y == 12 {
		t.Fatalf("fish did not move: still at (%v, %v)", w.Fish[0].X, w.Fish[0].Y)
	}
	d01 := math.Hypot(w.Fish[0].X-w.Fish[1].X, w.Fish[0].Y-w.Fish[1].Y)
	if d01 > 6 {
		t.Fatalf("school split apart: neighbours %.2f cells apart", d01)
	}
}

// Task 2: fish near a shark panic and gain distance from it.
func TestFishFleesShark(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 40, 12
	sh.Heading = 0
	w.Predators = nil // the flight must answer the shark, not a tuna
	w.Fish = []Fish{
		{X: 43, Y: 12, Vx: 1, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	before := math.Hypot(43-sh.Nodes[0], 12-sh.Nodes[1])
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	after := math.Hypot(w.Fish[0].X-sh.Nodes[0], w.Fish[0].Y-sh.Nodes[1])
	if w.Fish[0].Alarm <= 0 {
		t.Fatalf("fish near shark never alarmed (alarm=%v)", w.Fish[0].Alarm)
	}
	if after <= before {
		t.Fatalf("fish did not flee: dist %.2f -> %.2f", before, after)
	}
}

// Task 3: a hungry shark locks onto nearby fish and accelerates toward them.
func TestSharkPursuesPrey(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 10, 12
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 10 - float64(n)*0.5
		sh.Nodes[n*2+1] = 12
	}
	sh.Heading = 0
	sh.Speed = 2
	sh.Hunger = 0.9
	sh.RetargetAt = 0
	w.Predators = nil // the lock-on must answer the fish, not a tuna
	w.Fish = []Fish{
		{X: 16, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 1, MillDir: 1},
	}
	w.Step(1.0 / 60)
	if sh.Target == nil {
		t.Fatal("hungry shark did not lock onto the fish ahead of it")
	}
	for i := 0; i < 60; i++ {
		w.Step(1.0 / 60)
	}
	if sh.Speed <= 2 {
		t.Fatalf("hunting shark did not accelerate: speed=%.2f", sh.Speed)
	}
}

// Hunting sharks burst clearly above cruise: with live prey held ahead,
// the shark must settle well above patrol speed, so hunts read as faster
// and close on fleeing fish.
func TestSharkHuntingBurstsFaster(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 10, 12
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 10 - float64(n)*0.5
		sh.Nodes[n*2+1] = 12
	}
	sh.Heading = 0
	sh.Speed = 2
	sh.Hunger = 1
	sh.RetargetAt = 0
	w.Predators = nil
	w.Fish = []Fish{
		{X: 16, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 1, MillDir: 1},
	}
	cruise := 45/pxCell + sh.Length*0.06
	for i := 0; i < 180; i++ {
		// Hold live prey ahead of the snout so the lock never drops to
		// a catch and the burst can settle.
		f := &w.Fish[0]
		f.Alive = true
		f.X = sh.Nodes[0] + 6
		f.Y = sh.Nodes[1]
		f.Depth = sh.Depth
		f.Vx, f.Vy = 0, 0
		sh.Hunger = 1
		w.Step(1.0 / 60)
		if sh.Target == nil {
			t.Fatal("hungry shark did not lock onto the fish ahead of it")
		}
	}
	if sh.Speed <= cruise*1.8 {
		t.Fatalf("hunting shark not clearly faster than cruise: speed=%.2f cruise=%.2f", sh.Speed, cruise)
	}
}

// Task 4: eaten fish come back later as a new school swimming in from the
// edges, so the ocean stays populated.
func TestRespawnRepopulates(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	if len(w.Fish) < 8 {
		t.Fatalf("expected a seeded school, got %d fish", len(w.Fish))
	}
	// Eight due at once bypasses the small-wave gate, so the school
	// respawns on the next step.
	for i := 0; i < 8; i++ {
		w.Fish[i].Alive = false
		w.Fish[i].RespawnAt = w.Time
	}
	w.Step(1.0 / 60)
	for i := 0; i < 8; i++ {
		if !w.Fish[i].Alive {
			t.Fatalf("dead fish %d never respawned", i)
		}
		if w.Fish[i].Vx == 0 && w.Fish[i].Vy == 0 {
			t.Fatalf("respawned fish %d drifts motionless", i)
		}
	}
}

// Task 4: a trickle of dead fish waits for the next wave instead of
// respawning one by one.
func TestRespawnWaitsForWave(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish[0].Alive = false
	w.Fish[0].RespawnAt = w.Time
	w.Fish[1].Alive = false
	w.Fish[1].RespawnAt = w.Time
	w.lastSpawn = w.Time
	w.Step(1.0 / 60)
	if w.Fish[0].Alive || w.Fish[1].Alive {
		t.Fatal("small recent wave respawned instead of waiting")
	}
}

// An explicit fish count overrides the area-scaled population exactly,
// while 0 scales with terminal area.
func TestFishCountOption(t *testing.T) {
	w := NewWorld(nil, 25)
	w.Resize(80, 24)
	if len(w.Fish) != 25 {
		t.Fatalf("explicit count 25 seeded %d fish", len(w.Fish))
	}
	w = NewWorld(nil, 0)
	w.Resize(80, 24)
	if len(w.Fish) < 85 || len(w.Fish) > 340 {
		t.Fatalf("default population %d outside scaled 85..340 range", len(w.Fish))
	}
}

// Task 7: shrinking the arena keeps every fish and shark (clamped into the
// new bounds) instead of clearing or reseeding the ocean.
func TestResizePreservesLife(t *testing.T) {
	w := NewWorld(nil, 60)
	w.Resize(80, 24)
	for i := 0; i < 10; i++ {
		w.Step(1.0 / 60)
	}
	nFish, nShark := len(w.Fish), len(w.Sharks)
	if nFish == 0 || nShark == 0 {
		t.Fatalf("expected seeded life, got %d fish %d sharks", nFish, nShark)
	}
	w.Resize(40, 10)
	if len(w.Fish) != nFish || len(w.Sharks) != nShark {
		t.Fatalf("resize cleared life: %d->%d fish, %d->%d sharks",
			nFish, len(w.Fish), nShark, len(w.Sharks))
	}
	for i, f := range w.Fish {
		if f.X < 0 || f.X > 40 || f.Y < 0 || f.Y > 10 {
			t.Fatalf("fish %d left outside shrunk arena (%v, %v)", i, f.X, f.Y)
		}
	}
	for si, sh := range w.Sharks {
		for k := 0; k < SHARK_NODES; k++ {
			x, y := sh.Nodes[k*2], sh.Nodes[k*2+1]
			if x < 0 || x > 40 || y < 0 || y > 10 {
				t.Fatalf("shark %d node %d outside shrunk arena (%v, %v)", si, k, x, y)
			}
		}
	}
	// The preserved ocean must still simulate.
	alive := -1
	for i, f := range w.Fish {
		if f.Alive {
			alive = i
			break
		}
	}
	if alive < 0 {
		t.Fatal("no live fish left to observe after resize")
	}
	x0 := w.Fish[alive].X
	w.Step(1.0 / 60)
	if w.Fish[alive].X == x0 {
		t.Fatal("preserved fish did not move after resize")
	}
}

// Live LOD: growing the arena rescales shark lengths toward a fresh seed
// at the new size, and shrinking brings them back down.
func TestResizeRescalesSharkLength(t *testing.T) {
	w := NewWorld(nil, 60)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected seeded sharks")
	}
	small := make([]float64, len(w.Sharks))
	for i, sh := range w.Sharks {
		small[i] = sh.Length
	}
	w.Resize(160, 48)
	for i, sh := range w.Sharks {
		if sh.Length <= small[i] {
			t.Fatalf("shark %d did not grow on enlarge: %.2f -> %.2f", i, small[i], sh.Length)
		}
		if sh.Length > 430.0/12.0+1e-9 {
			t.Fatalf("shark %d exceeded max clamp: %.2f", i, sh.Length)
		}
	}
	// A fresh world seeded large should match the rescaled lengths.
	fresh := NewWorld(nil, 60)
	fresh.Resize(160, 48)
	if len(fresh.Sharks) != len(w.Sharks) {
		t.Fatalf("shark count changed on resize: %d -> %d", len(small), len(w.Sharks))
	}
	for i := range w.Sharks {
		if diff := w.Sharks[i].Length - fresh.Sharks[i].Length; diff < -1e-9 || diff > 1e-9 {
			t.Fatalf("rescaled shark %d length %.2f != fresh seed %.2f", i, w.Sharks[i].Length, fresh.Sharks[i].Length)
		}
	}
	w.Resize(80, 24)
	for i, sh := range w.Sharks {
		if diff := sh.Length - small[i]; diff < -1e-9 || diff > 1e-9 {
			t.Fatalf("shark %d did not shrink back: want %.2f got %.2f", i, small[i], sh.Length)
		}
	}
}

// Live LOD: the default area-scaled population tracks the arena, while an
// explicit count stays pinned across resizes.
func TestResizeAdjustsAutoFishPopulation(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Resize(80, 24)
	start := len(w.Fish)
	w.Resize(160, 48)
	if len(w.Fish) <= start {
		t.Fatalf("auto population did not grow on enlarge: %d -> %d", start, len(w.Fish))
	}
	if len(w.Fish) > 340 {
		t.Fatalf("auto population exceeded max clamp: %d", len(w.Fish))
	}
	w.Resize(40, 10)
	if len(w.Fish) >= 340 {
		t.Fatalf("auto population did not shrink: %d", len(w.Fish))
	}

	pinned := NewWorld(nil, 25)
	pinned.Resize(80, 24)
	pinned.Resize(160, 48)
	if len(pinned.Fish) != 25 {
		t.Fatalf("explicit fish count changed on resize: %d", len(pinned.Fish))
	}
}

// Task 5: a splash startles nearby fish outward and marks them alarmed.
func TestSplashStartlesFish(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = []Fish{
		{X: 43, Y: 12, Vx: 1, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	w.Splash(40, 12)
	if w.Fish[0].Alarm != 1 {
		t.Fatalf("splashed fish not alarmed (alarm=%v)", w.Fish[0].Alarm)
	}
	if w.Fish[0].Vx <= 1 {
		t.Fatalf("splashed fish not pushed outward (vx=%v)", w.Fish[0].Vx)
	}
	if len(w.Events) == 0 || w.Events[len(w.Events)-1].Kind != "splash" {
		t.Fatalf("expected splash event, got %+v", w.Events)
	}
}

// Depth: an alarmed fish dives below its calm baseline.
func TestFishDivesWhenAlarmed(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil // depth mechanics need a predator-free arena
	w.Fish = []Fish{
		{X: 20, Y: 12, Vx: 2, Vy: 0, Species: 0, Alive: true, Depth: 0.9, MillDir: 1, Alarm: 1},
		{X: 50, Y: 12, Vx: 2, Vy: 0, Species: 0, Alive: true, Depth: 0.9, MillDir: 1, Alarm: 0},
	}
	for i := 0; i < 120; i++ {
		w.Step(1.0 / 60)
	}
	alarmed, calm := w.Fish[0].Depth, w.Fish[1].Depth
	if alarmed >= 0.9 {
		t.Fatalf("alarmed fish never dived: depth=%.3f", alarmed)
	}
	if alarmed >= calm {
		t.Fatalf("alarmed fish not deeper than calm: alarmed=%.3f calm=%.3f", alarmed, calm)
	}
	if alarmed < fishDepthMin-1e-9 || alarmed > fishDepthMax+1e-9 {
		t.Fatalf("fish depth out of bounds: %.3f", alarmed)
	}
}

// Depth: a hunting shark steers its depth toward its prey.
func TestSharkDivesTowardPrey(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 10, 12
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 10 - float64(n)*0.5
		sh.Nodes[n*2+1] = 12
	}
	sh.Heading = 0
	sh.Speed = 2
	sh.Hunger = 0.9
	sh.RetargetAt = 0
	sh.Depth = 0.2
	sh.Dz = 0
	w.Predators = nil // the dive must answer the fish, not a tuna
	w.Fish = []Fish{
		{X: 16, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.9, MillDir: 1},
	}
	for i := 0; i < 120; i++ {
		w.Step(1.0 / 60)
	}
	if w.Sharks[0].Depth <= 0.2 {
		t.Fatalf("hunting shark never dived toward prey: depth=%.3f", w.Sharks[0].Depth)
	}
}

// Depth: a fish at the mouth x/y but a full layer away in depth escapes.
func TestDeepFishEscapesEat(t *testing.T) {
	newSetup := func(fishDepth, sharkDepth float64) *World {
		w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
		w.Resize(80, 24)
		sh := &w.Sharks[0]
		sh.Nodes[0], sh.Nodes[1] = 10, 12
		for n := 1; n < SHARK_NODES; n++ {
			sh.Nodes[n*2] = 10 - float64(n)*0.5
			sh.Nodes[n*2+1] = 12
		}
		sh.Heading = 0
		sh.Speed = 0
		sh.Hunger = 0.9
		sh.RetargetAt = 1e9
		sh.Target = nil
		sh.Depth = sharkDepth
		sh.Dz = 0
		w.Predators = nil // depth-gated catch must not meet a passing tuna
		w.Fish = []Fish{
			{X: 10.5, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: fishDepth, MillDir: 1},
		}
		return w
	}
	deep := newSetup(0.9, 0.2)
	deep.Step(1.0 / 60)
	if !deep.Fish[0].Alive {
		t.Fatal("deep fish at mouth x/y was eaten despite depth gap")
	}
	aligned := newSetup(0.4, 0.4)
	aligned.Step(1.0 / 60)
	if aligned.Fish[0].Alive {
		t.Fatal("aligned-depth fish at mouth was not eaten")
	}
}

// Task 3: a shark catches fish at its mouth, hunger drops on a catch, and
// hunger builds back up over time.
func TestSharkEatsAndHungers(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 10, 12
	for n := 1; n < SHARK_NODES; n++ {
		sh.Nodes[n*2] = 10 - float64(n)*0.5
		sh.Nodes[n*2+1] = 12
	}
	sh.Heading = 0
	sh.Speed = 2
	sh.Hunger = 0.9
	sh.RetargetAt = 0
	// Inside the mouth radius (L*0.05+8px), so the catch is deterministic.
	// Species 2 (large) is the edible fish.
	w.Fish = []Fish{
		{X: 10.5, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 1, MillDir: 1},
	}
	// The hunger-build phase needs a prey-free arena; schooling predators
	// are live prey too, so clear them alongside the eaten fish's school.
	w.Predators = nil
	w.Step(1.0 / 60)
	if w.Fish[0].Alive {
		t.Fatal("shark did not catch the fish at its mouth")
	}
	if len(w.Events) == 0 || w.Events[0].Kind != "eat" {
		t.Fatalf("expected eat event, got %+v", w.Events)
	}
	if sh.Hunger >= 0.9 {
		t.Fatalf("hunger did not drop after eating: %.2f", sh.Hunger)
	}
	hungry := sh.Hunger
	for i := 0; i < 120; i++ {
		w.Step(1.0 / 60)
	}
	if sh.Hunger <= hungry {
		t.Fatalf("hunger did not build over time: %.2f -> %.2f", hungry, sh.Hunger)
	}
}
