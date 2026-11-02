package world

import (
	"math"
	"testing"
)

// Redesign: a solo small/medium fish (no neighbours in perception) must
// steer back toward its school instead of cruising solo forever.
func TestSoloSmallFishRegroupsToSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	// Two small fish (type 0) far apart, both cruising east at the
	// same speed: without regroup the gap holds.
	w.Fish = []Fish{
		{X: 20, Y: 12, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
		{X: 40, Y: 12, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
		{X: 41, Y: 12.5, Vx: 3, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	before := math.Hypot(w.Fish[0].X-w.Fish[1].X, w.Fish[0].Y-w.Fish[1].Y)
	for i := 0; i < 240; i++ {
		w.Step(1.0 / 60)
	}
	after := math.Hypot(w.Fish[0].X-w.Fish[1].X, w.Fish[0].Y-w.Fish[1].Y)
	if after >= before-1.0 {
		t.Fatalf("solo small fish did not regroup: gap %.2f -> %.2f", before, after)
	}
}

// A lone schooling predator far from its school abandons the pursuit
// instead of chasing solo: hunger high, prey ahead, but ~30 cells from
// company, well beyond the loner hunt radius.
func TestLonerAbandonsHuntWhenFarFromSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 29, 12)
	c := testPredator(Tuna, 5, 28.5, 12.5)
	a.School, b.School, c.School = 5, 5, 5
	l := testPredator(Tuna, 5, 50, 12)
	l.School = 0
	l.Hunger = 0.9
	l.RetargetAt = 0
	w.Predators = []Predator{a, b, c, l}
	w.Fish = []Fish{
		{X: 56, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	pinCanvasChurn(w)
	// Abandon is instant: one step already clears the lock.
	w.Step(1.0 / 60)
	if w.Predators[3].Target != nil {
		t.Fatal("loner far from school kept hunting instead of regrouping")
	}
	// And the loner rejoins instead of chasing solo (generous horizon:
	// wander and current add noise to the closing speed).
	for i := 0; i < 600; i++ {
		w.Step(1.0 / 60)
	}
	if got := w.Predators[3].School; got != 5 {
		t.Fatalf("loner never rejoined its school (school %d)", got)
	}
	if !w.Fish[0].Alive {
		t.Fatal("lone fish was eaten during the regroup run")
	}
}

// A loner facing away from its school still rejoins fast: aggressive
// regroup means no lingering individuals when nothing is hunting them.
func TestLonerRegroupsAggressively(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 29, 12)
	m := testPredator(Tuna, 5, 28.5, 12.5)
	l := testPredator(Tuna, 5, 38, 12) // heading east, away from the school
	a.School, b.School, m.School = 5, 5, 5
	l.School = 0
	w.Predators = []Predator{a, b, m, l}
	for i := 0; i < 200; i++ {
		w.Step(1.0 / 60)
	}
	if got := w.Predators[3].School; got != 5 {
		t.Fatalf("loner still solo after regrouping (school %d)", got)
	}
}
