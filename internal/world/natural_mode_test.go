package world

import "testing"

// Natural mode's caps are area-scaled maxima: fish keep the existing
// density, predators grow from a small-pane floor of 12 to 28, and sharks
// run from 1 to 4.
func TestNaturalCapsScaleWithArea(t *testing.T) {
	cases := []struct {
		cols, rows int
		fish       int
		predators  int
		sharks     int
	}{
		{80, 24, 91, 12, 1},
		{100, 30, 142, 12, 2},
		{120, 40, 228, 16, 3},
		{160, 50, 340, 27, 4},
		{200, 60, 340, 28, 4},
	}
	for _, c := range cases {
		if got := targetFishCountFor(c.cols, c.rows); got != c.fish {
			t.Errorf("fish cap at %dx%d = %d, want %d", c.cols, c.rows, got, c.fish)
		}
		if got := targetPredatorCountFor(c.cols, c.rows); got != c.predators {
			t.Errorf("predator cap at %dx%d = %d, want %d", c.cols, c.rows, got, c.predators)
		}
		if got := targetSharkCountFor(c.cols, c.rows); got != c.sharks {
			t.Errorf("shark cap at %dx%d = %d, want %d", c.cols, c.rows, got, c.sharks)
		}
	}
}

// The first natural seed fills every cap exactly once, with four predator
// schools split between tuna and mackerel and random valid shark kinds.
func TestNaturalSeedFillsCaps(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Natural = true
	w.Resize(120, 40)

	if got, want := len(w.Fish), targetFishCountFor(120, 40); got != want {
		t.Fatalf("seeded %d fish, want %d", got, want)
	}
	if got, want := len(w.Predators), targetPredatorCountFor(120, 40); got != want {
		t.Fatalf("seeded %d predators, want %d", got, want)
	}
	if got, want := len(w.Sharks), targetSharkCountFor(120, 40); got != want {
		t.Fatalf("seeded %d sharks, want %d", got, want)
	}

	schools := map[int]bool{}
	tunaSchools := map[int]bool{}
	mackerelSchools := map[int]bool{}
	for i := range w.Predators {
		p := &w.Predators[i]
		schools[p.School] = true
		base := predatorBaseSize(p.Kind)
		if p.Size < base*0.5-1e-9 || p.Size > base*1.15+1e-9 {
			t.Fatalf("predator size %.3f outside 0.5x-1.15x of %.3f", p.Size, base)
		}
		switch p.Kind {
		case Tuna:
			tunaSchools[p.School] = true
		case Mackerel:
			mackerelSchools[p.School] = true
		default:
			t.Fatalf("unexpected predator kind %q", p.Kind)
		}
	}
	if len(schools) != naturalPredatorSchools {
		t.Fatalf("seeded %d predator schools, want %d", len(schools), naturalPredatorSchools)
	}
	if len(tunaSchools) != 2 || len(mackerelSchools) != 2 {
		t.Fatalf("school kinds = %d tuna / %d mackerel, want 2 / 2", len(tunaSchools), len(mackerelSchools))
	}

	for i := range w.Sharks {
		sh := &w.Sharks[i]
		if !sh.Kind.Valid() {
			t.Fatalf("natural shark has invalid kind %q", sh.Kind)
		}
		if sh.Size < 0.5-1e-9 || sh.Size > 1.15+1e-9 {
			t.Fatalf("natural shark size %.3f outside 0.5x-1.15x", sh.Size)
		}
	}
}

// Growing the pane raises the caps only; natural mode does not immediately
// spawn more creatures to maintain a minimum population.
func TestNaturalGrowthDoesNotForceSpawn(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Natural = true
	w.Resize(80, 24)
	fish, preds, sharks := len(w.Fish), len(w.Predators), len(w.Sharks)

	if targetSharkCountFor(200, 60) <= targetSharkCountFor(80, 24) {
		t.Fatal("test expects the larger pane to raise the shark cap")
	}
	w.Resize(200, 60)
	if got := len(w.Fish); got != fish {
		t.Errorf("growth spawned fish: %d -> %d", fish, got)
	}
	if got := len(w.Predators); got != preds {
		t.Errorf("growth spawned predators: %d -> %d", preds, got)
	}
	if got := len(w.Sharks); got != sharks {
		t.Errorf("growth spawned sharks: %d -> %d", sharks, got)
	}
}

// Shrinking keeps every creature on screen instead of deleting it:
// the live count is unchanged in that frame and everything is clamped
// into the new bounds, so the extra creatures swim out by movement alone.
func TestNaturalShrinkMarksExcessAsLeaving(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Natural = true
	w.Resize(200, 60)
	totalFish, totalPreds, totalSharks := len(w.Fish), len(w.Predators), len(w.Sharks)

	w.Resize(80, 24)
	if len(w.Fish) != totalFish || len(w.Predators) != totalPreds || len(w.Sharks) != totalSharks {
		t.Fatalf("shrink deleted creatures immediately: fish %d->%d preds %d->%d sharks %d->%d",
			totalFish, len(w.Fish), totalPreds, len(w.Predators), totalSharks, len(w.Sharks))
	}

	for i := range w.Fish {
		if w.Fish[i].X < 0 || w.Fish[i].X > 80 || w.Fish[i].Y < 0 || w.Fish[i].Y > 24 {
			t.Fatalf("fish %d outside shrunken bounds after resize", i)
		}
	}
}

// Replacement timers use the default caller timing in all modes; natural
// mode no longer has a wide 6-20s window.
func TestNaturalRespawnDeadline(t *testing.T) {
	w := &World{Natural: true, Time: 10}
	for i := 0; i < 200; i++ {
		if got := w.respawnDeadline(3, 8); got < 13 || got > 18 {
			t.Fatalf("natural respawn deadline %.3f outside [13,18]", got)
		}
	}
	w.Natural = false
	for i := 0; i < 200; i++ {
		if got := w.respawnDeadline(3, 8); got < 13 || got > 18 {
			t.Fatalf("normal respawn deadline %.3f outside [13,18]", got)
		}
	}
}
