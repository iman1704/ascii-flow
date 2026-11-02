package world

import "testing"

// pinCanvasChurn parks the entry waves far in the future so long-horizon
// mechanics tests measure a closed tank. It changes no assertion: like the
// existing w.Sharks = nil / w.FeedingSpots = nil isolation lines, it only
// holds an orthogonal process still while the behavior under test is
// measured.
func pinCanvasChurn(w *World) {
	w.nextFishEnterAt = 1e9
	w.nextPredEnterAt = 1e9
	w.nextSharkEnterAt = 1e9
}

// Canvas behaviors: anything past a grace zone despawns by movement alone,
// and new ones enter later, capped at the maxima. Fish past the grace zone
// despawn; fish inside stay.
func TestCanvasFishDespawnPastGrace(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 5)
	w.Resize(80, 24)
	mk := func(x float64) Fish {
		return Fish{X: x, Y: 12, Vx: 3, Vy: 0, Species: 2, Alive: true,
			Depth: 0.8, MillDir: 1}
	}
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{mk(85), mk(40)}
	pinCanvasChurn(w)
	w.Step(1.0 / 60)
	if len(w.Fish) != 1 {
		t.Fatalf("expected fish past grace to despawn, got %d fish", len(w.Fish))
	}
	if w.Fish[0].X > 80 {
		t.Fatal("fish past grace was kept instead of the inside one")
	}
}

// Entries refill the tank but never exceed the max: forcing the entry timer
// with room below the cap appends an edge-entry school, and a full tank
// stays put.
func TestCanvasFishEntryCappedAtMax(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 5)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{{
		X: 40, Y: 12, Vx: 3, Vy: 0, Species: 2, Alive: true,
		Depth: 0.8, MillDir: 1,
	}}
	w.nextFishEnterAt = -1
	w.Step(1.0 / 60)
	if len(w.Fish) <= 1 || len(w.Fish) > 5 {
		t.Fatalf("entry should refill toward max 5, got %d fish", len(w.Fish))
	}
	w.nextFishEnterAt = -1
	w.Step(1.0 / 60)
	if len(w.Fish) > 5 {
		t.Fatalf("entries exceeded max fish 5: got %d", len(w.Fish))
	}
}

// Sharks leave too: a shark fully past the grace zone is removed,
// and a later entry swims a fresh shark in while below the roster max.
func TestCanvasSharkExitAndEnter(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) != 1 {
		t.Fatalf("expected 1 seeded shark, got %d", len(w.Sharks))
	}
	nodes := make([]float64, SHARK_NODES*2)
	for k := 0; k < SHARK_NODES; k++ {
		nodes[k*2] = 86 + float64(k)*0.1
		nodes[k*2+1] = 12
	}
	w.Sharks[0].Nodes = nodes
	pinCanvasChurn(w)
	w.Step(1.0 / 60)
	if len(w.Sharks) != 0 {
		t.Fatal("shark fully past grace should despawn")
	}
	w.nextSharkEnterAt = -1
	w.Step(1.0 / 60)
	if len(w.Sharks) != 1 {
		t.Fatalf("expected 1 entered shark, got %d", len(w.Sharks))
	}
}

// Predator entries roll fresh sizes in the roster's young-to-grown band and
// arrive as one school.
func TestCanvasPredatorEntryFreshSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(120, 40)
	w.Sharks = nil
	w.Fish = nil
	w.Predators = nil
	w.nextPredEnterAt = -1
	w.Step(1.0 / 60)
	if len(w.Predators) < 3 {
		t.Fatalf("expected an entering predator school, got %d", len(w.Predators))
	}
	school := w.Predators[0].School
	if school == 0 {
		t.Fatal("entered predators should share a school ID")
	}
	for _, p := range w.Predators {
		if p.School != school {
			t.Fatal("entered predators split across schools")
		}
		base := predatorBaseSize(p.Kind)
		lo, hi := base*0.5, base*1.15+1e-9
		if w.baseLength <= 0 || p.Size < lo || p.Size > hi {
			t.Fatalf("entered predator size %.3f outside fresh band [%.3f, %.3f]",
				p.Size, base*0.5, base*1.15)
		}
		if !p.Alive {
			t.Fatal("entered predators should arrive alive")
		}
	}
}
