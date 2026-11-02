package world

import "testing"

// Sharks ignore small/medium fish: a small fish at the mouth survives and is
// never targeted, while a large fish at the same spot is eaten.
func TestSharkIgnoresSmallFish(t *testing.T) {
	newSetup := func(species int) *World {
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
		sh.RetargetAt = 0
		sh.Target = nil
		sh.TargetPred = nil
		sh.Depth = 0.7
		sh.Dz = 0
		w.Predators = nil
		w.Fish = []Fish{
			{X: 10.5, Y: 12, Vx: 0, Vy: 0, Species: species, Alive: true, Depth: 0.7, MillDir: 1},
		}
		return w
	}

	small := newSetup(0) // small type, inedible
	small.Step(1.0 / 60)
	if !small.Fish[0].Alive {
		t.Fatal("shark ate a small fish; only large fish are edible")
	}
	if small.Sharks[0].Target != nil {
		t.Fatal("shark targeted a small fish; only large fish are prey")
	}

	large := newSetup(2) // large type, edible
	large.Step(1.0 / 60)
	if large.Fish[0].Alive {
		t.Fatal("shark did not eat the large fish at its mouth")
	}
	if large.Sharks[0].Target == nil {
		t.Fatal("shark did not target the large fish ahead of it")
	}
}

// Schools spawn at the type minimum with no maximum: spawnSchools adds
// exactly n fish and every fish carries a valid size-only type.
func TestSpawnSchoolsExactCount(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = nil
	w.spawnSchools(50)
	if len(w.Fish) != 50 {
		t.Fatalf("spawnSchools(50) added %d fish, want 50", len(w.Fish))
	}
	for i, f := range w.Fish {
		if f.Species < 0 || f.Species > 2 {
			t.Fatalf("fish %d has type %d, want 0..2", i, f.Species)
		}
		if !f.Alive {
			t.Fatalf("spawned fish %d should start alive", i)
		}
	}
}

// Respawn has no maximum: a large due batch revives as one school instead
// of being capped at an old per-species max.
func TestRespawnNoMaxCap(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = make([]Fish, 0, 30)
	for k := 0; k < 30; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 20, Y: 12, Vx: 1, Vy: 0, Species: 2,
			Alive: false, RespawnAt: 0, Depth: 0.8, MillDir: 1,
		})
	}
	w.lastSpawn = -100
	w.Time = 100
	w.respawn()
	revived := 0
	for _, f := range w.Fish {
		if f.Alive {
			revived++
		}
	}
	if revived != 30 {
		t.Fatalf("respawn revived %d of 30 due fish, want all 30 (no max)", revived)
	}
}

// Close schools of the same size group into a single larger school,
// while different sizes hold apart.
func TestSameSizeSchoolsMerge(t *testing.T) {
	centroid := func(w *World, lo, hi int) (float64, float64) {
		var x, y float64
		for _, f := range w.Fish[lo:hi] {
			x += f.X
			y += f.Y
		}
		n := float64(hi - lo)
		return x / n, y / n
	}
	dist := func(ax, ay, bx, by float64) float64 {
		dx, dy := ax-bx, ay-by
		return dx*dx + dy*dy
	}

	newCalmWorld := func() *World {
		w := NewWorld([]SharkSpec{}, 0)
		w.Resize(80, 24)
		w.Sharks = nil
		w.Predators = nil
		w.Fish = nil
		pinCanvasChurn(w)
		return w
	}

	// Same size: two close medium-type clusters must converge. Medium
	// has the strongest cohesion, so neighbouring schools fuse.
	w := newCalmWorld()
	for k := 0; k < 5; k++ {
		w.Fish = append(w.Fish,
			Fish{X: 20 + float64(k)*0.3, Y: 12, Vx: 1, Vy: 0, Species: 1, Alive: true, Depth: 0.8, MillDir: 1})
	}
	for k := 0; k < 5; k++ {
		w.Fish = append(w.Fish,
			Fish{X: 24 + float64(k)*0.3, Y: 12, Vx: 1, Vy: 0, Species: 1, Alive: true, Depth: 0.8, MillDir: 1})
	}
	fx0, fy0 := centroid(w, 0, 5)
	sx0, sy0 := centroid(w, 5, 10)
	before := dist(fx0, fy0, sx0, sy0)
	for i := 0; i < 600; i++ {
		w.Step(1.0 / 60)
	}
	fx1, fy1 := centroid(w, 0, 5)
	sx1, sy1 := centroid(w, 5, 10)
	if after := dist(fx1, fy1, sx1, sy1); after >= before {
		t.Fatalf("same-size schools did not merge: centroid d2 %.2f -> %.2f", before, after)
	}

	// Different sizes: a small-type and a large-type cluster hold apart.
	w2 := newCalmWorld()
	for k := 0; k < 5; k++ {
		w2.Fish = append(w2.Fish,
			Fish{X: 20 + float64(k)*0.3, Y: 12, Vx: 1, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1})
	}
	for k := 0; k < 5; k++ {
		w2.Fish = append(w2.Fish,
			Fish{X: 24 + float64(k)*0.3, Y: 12, Vx: 1, Vy: 0, Species: 2, Alive: true, Depth: 0.8, MillDir: 1})
	}
	gx0, gy0 := centroid(w2, 0, 5)
	hx0, hy0 := centroid(w2, 5, 10)
	mixedBefore := dist(gx0, gy0, hx0, hy0)
	for i := 0; i < 600; i++ {
		w2.Step(1.0 / 60)
	}
	gx1, gy1 := centroid(w2, 0, 5)
	hx1, hy1 := centroid(w2, 5, 10)
	if after := dist(gx1, gy1, hx1, hy1); after < mixedBefore*0.25 {
		t.Fatalf("different sizes fused: centroid d2 %.2f -> %.2f", mixedBefore, after)
	}
}

// Predator individuals vary from half-grown to 15% over the roster base, at
// spawn and on respawn.
func TestPredatorSizeVariation(t *testing.T) {
	w := NewWorld(nil, 60)
	w.Resize(80, 24)
	if len(w.Predators) == 0 {
		t.Fatal("expected seeded predators")
	}
	bases := map[PredatorKind]float64{Tuna: 0.32, Mackerel: 0.27}
	for i, p := range w.Predators {
		base, ok := bases[p.Kind]
		if !ok {
			t.Fatalf("predator %d has unknown kind %q", i, p.Kind)
		}
		if got := p.Size / base; got < 0.5-1e-9 || got > 1.15+1e-9 {
			t.Fatalf("predator %d size %.3f outside 0.5x-1.15x of base %.2f", i, p.Size, base)
		}
		if want := w.baseLength * p.Size; p.Length != want {
			t.Fatalf("predator %d length %.3f != baseLength*size %.3f", i, p.Length, want)
		}
	}

	// Revival rolls a fresh size in the same band.
	for i := range w.Predators {
		w.Predators[i].Alive = false
		w.Predators[i].RespawnAt = 0
	}
	w.lastPredSpawn = -100
	w.Time = 100
	w.respawnPredators()
	revived := 0
	for i, p := range w.Predators {
		if !p.Alive {
			continue
		}
		revived++
		if got := p.Size / bases[p.Kind]; got < 0.5-1e-9 || got > 1.15+1e-9 {
			t.Fatalf("revived predator %d size %.3f outside 0.5x-1.15x", i, p.Size)
		}
	}
	if revived == 0 {
		t.Fatal("no predators revived for size check")
	}
}
