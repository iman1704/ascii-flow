package world

import (
	"math"
	"testing"
)

// Schooling predators seed as two live schools, much smaller than sharks.
func TestPredatorsSeededSmallerThanSharks(t *testing.T) {
	w := NewWorld(nil, 0)
	w.Resize(80, 24)
	if len(w.Predators) < 14 || len(w.Predators) > 19 {
		t.Fatalf("expected tuna+mackerel schools (14..19), got %d", len(w.Predators))
	}
	var tuna, mackerel int
	for _, p := range w.Predators {
		if !p.Alive {
			t.Fatal("seeded predators should start alive")
		}
		if !p.Kind.Valid() {
			t.Fatalf("seeded predator has invalid kind %q", p.Kind)
		}
		switch p.Kind {
		case Tuna:
			tuna++
		case Mackerel:
			mackerel++
		}
		if len(p.Nodes) != SHARK_NODES*2 {
			t.Fatalf("predator spine has %d floats, want %d", len(p.Nodes), SHARK_NODES*2)
		}
	}
	if tuna == 0 || mackerel == 0 {
		t.Fatalf("expected both schools, got %d tuna %d mackerel", tuna, mackerel)
	}
	tunaSchools, mackerelSchools := map[int]bool{}, map[int]bool{}
	for _, p := range w.Predators {
		if p.School == 0 {
			t.Fatalf("seeded %s swims lone (school 0)", p.Kind)
		}
		if p.Kind == Tuna {
			tunaSchools[p.School] = true
		} else {
			mackerelSchools[p.School] = true
		}
	}
	if len(tunaSchools) != 1 || len(mackerelSchools) != 1 {
		t.Fatalf("seed schools fragmented: tuna=%v mackerel=%v", tunaSchools, mackerelSchools)
	}
	for s := range tunaSchools {
		if mackerelSchools[s] {
			t.Fatalf("tuna and mackerel share school %d", s)
		}
	}
	for _, p := range w.Predators {
		for _, sh := range w.Sharks {
			if p.Length >= sh.Length {
				t.Fatalf("predator length %.2f not smaller than shark %.2f", p.Length, sh.Length)
			}
		}
	}
}

// A hungry predator locks onto a fish ahead of it and accelerates.
func TestPredatorPursuesPrey(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Tuna, 5, 10, 12)}
	p := &w.Predators[0]
	p.Hunger = 0.9
	p.RetargetAt = 0
	p.Speed = 2
	w.Fish = []Fish{
		{X: 16, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	w.Step(1.0 / 60)
	if p.Target == nil {
		t.Fatal("hungry predator did not lock onto the fish ahead of it")
	}
	for i := 0; i < 60; i++ {
		w.Step(1.0 / 60)
	}
	if p.Speed <= 2 {
		t.Fatalf("hunting predator did not accelerate: speed=%.2f", p.Speed)
	}
}

// A predator near a shark gains distance from it.
func TestPredatorFleesShark(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) == 0 {
		t.Fatal("expected a shark from Seed")
	}
	sh := &w.Sharks[0]
	sh.Nodes[0], sh.Nodes[1] = 40, 12
	sh.Heading = math.Pi
	sh.Speed = 1
	sh.Hunger = 0
	sh.RetargetAt = 1e9
	sh.Target = nil
	sh.TargetPred = nil
	w.Predators = []Predator{testPredator(Tuna, 5, 43, 12)}
	w.Fish = nil
	before := math.Hypot(43-sh.Nodes[0], 12-sh.Nodes[1])
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	p := &w.Predators[0]
	if !p.Alive {
		t.Fatal("fleeing predator was eaten during the flight test")
	}
	after := math.Hypot(p.Nodes[0]-sh.Nodes[0], p.Nodes[1]-sh.Nodes[1])
	if after <= before {
		t.Fatalf("predator did not flee: dist %.2f -> %.2f", before, after)
	}
}

// Prey fish panic and gain distance from a nearby schooling predator.
func TestFishFleesPredator(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Mackerel, 5, 40, 12)}
	w.Predators[0].Hunger = 0
	w.Predators[0].RetargetAt = 1e9
	w.Fish = []Fish{
		{X: 43, Y: 12, Vx: 1, Vy: 0, Species: 0, Alive: true, Depth: 1, MillDir: 1},
	}
	before := math.Hypot(43-40, 0)
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	if w.Fish[0].Alarm <= 0 {
		t.Fatalf("fish near predator never alarmed (alarm=%v)", w.Fish[0].Alarm)
	}
	after := math.Hypot(w.Fish[0].X-w.Predators[0].Nodes[0], w.Fish[0].Y-w.Predators[0].Nodes[1])
	if after <= before {
		t.Fatalf("fish did not flee predator: dist %.2f -> %.2f", before, after)
	}
}

// A shark catches a predator at its mouth: the predator dies, shark hunger
// drops, and an eat event fires.
func TestSharkEatsPredator(t *testing.T) {
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
	sh.TargetPred = nil
	sh.Depth = 0.7
	w.Predators = []Predator{testPredator(Tuna, 5, 10.5, 12)}
	w.Predators[0].Depth = 0.7
	w.Fish = nil
	w.Step(1.0 / 60)
	if w.Predators[0].Alive {
		t.Fatal("predator at the shark's mouth was not eaten")
	}
	if sh.Hunger >= 0.9 {
		t.Fatalf("shark hunger did not drop after eating predator: %.2f", sh.Hunger)
	}
	if len(w.Events) == 0 || w.Events[len(w.Events)-1].Kind != "eat" {
		t.Fatalf("expected eat event, got %+v", w.Events)
	}
}

// A predator eats a fish at its mouth and its hunger drops.
func TestPredatorEatsFish(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Predators = []Predator{testPredator(Tuna, 5, 10, 12)}
	w.Predators[0].Hunger = 0.9
	w.Predators[0].RetargetAt = 1e9
	w.Predators[0].Depth = 0.7
	w.Fish = []Fish{
		{X: 10.4, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.7, MillDir: 1},
	}
	w.Step(1.0 / 60)
	if w.Fish[0].Alive {
		t.Fatal("fish at the predator's mouth was not eaten")
	}
	if w.Predators[0].Hunger >= 0.9 {
		t.Fatalf("predator hunger did not drop after eating: %.2f", w.Predators[0].Hunger)
	}
}

// Shrinking the arena keeps every predator spine clamped in bounds.
func TestResizePreservesPredators(t *testing.T) {
	w := NewWorld(nil, 60)
	w.Resize(80, 24)
	n := len(w.Predators)
	if n == 0 {
		t.Fatal("expected seeded predators")
	}
	w.Resize(40, 10)
	if len(w.Predators) != n {
		t.Fatalf("resize cleared predators: %d -> %d", n, len(w.Predators))
	}
	for i, p := range w.Predators {
		for k := 0; k < SHARK_NODES; k++ {
			x, y := p.Nodes[k*2], p.Nodes[k*2+1]
			if x < 0 || x > 40 || y < 0 || y > 10 {
				t.Fatalf("predator %d node %d outside shrunk arena (%v, %v)", i, k, x, y)
			}
		}
	}
}

// A calm school holds together: no solo drift without hunting or sharks.
func TestCalmSchoolHoldsTogether(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 30, 12)
	c := testPredator(Tuna, 5, 29, 13)
	a.School, b.School, c.School = 5, 5, 5
	w.Predators = []Predator{a, b, c}
	for i := 0; i < 120; i++ {
		w.Step(1.0 / 60)
	}
	for i := range w.Predators {
		if w.Predators[i].School != 5 {
			t.Fatalf("calm member %d left school 5 (now %d)", i, w.Predators[i].School)
		}
	}
	maxD := 0.0
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			d := math.Hypot(
				w.Predators[i].Nodes[0]-w.Predators[j].Nodes[0],
				w.Predators[i].Nodes[1]-w.Predators[j].Nodes[1])
			maxD = max(maxD, d)
		}
	}
	if maxD >= predFissionGap {
		t.Fatalf("calm school spread to %.2f cells", maxD)
	}
}

// A calm school keeps individuals distinct: neighbours settle with enough
// room between snouts that bodies do not pile onto each other.
func TestCalmSchoolKeepsDistinctSpacing(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 30, 12)
	c := testPredator(Tuna, 5, 29, 13)
	a.School, b.School, c.School = 5, 5, 5
	a.Hunger, b.Hunger, c.Hunger = 0, 0, 0
	a.RetargetAt, b.RetargetAt, c.RetargetAt = 1e9, 1e9, 1e9
	w.Predators = []Predator{a, b, c}
	pinCanvasChurn(w)
	for i := 0; i < 600; i++ {
		w.Step(1.0 / 60)
	}
	minD := math.Inf(1)
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			d := math.Hypot(
				w.Predators[i].Nodes[0]-w.Predators[j].Nodes[0],
				w.Predators[i].Nodes[1]-w.Predators[j].Nodes[1])
			minD = min(minD, d)
		}
	}
	if minD < 2.0 {
		t.Fatalf("calm school crowds individuals: min snout gap %.2f cells", minD)
	}
}

// Widely separated clumps of one school split into distinct schools.
func TestPredatorFissionSplitsFarClumps(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	ps := []Predator{
		testPredator(Tuna, 5, 20, 12),
		testPredator(Tuna, 5, 21, 12),
		testPredator(Tuna, 5, 22, 12),
		testPredator(Tuna, 5, 40, 12),
		testPredator(Tuna, 5, 41, 12),
		testPredator(Tuna, 5, 42, 12),
	}
	for i := range ps {
		ps[i].School = 5
	}
	w.Predators = ps
	w.Step(1.0 / 60)
	s := func(i int) int { return w.Predators[i].School }
	if s(0) == 0 || s(0) != s(1) || s(0) != s(2) {
		t.Fatalf("near triple broke school: %d vs %d vs %d", s(0), s(1), s(2))
	}
	if s(3) == 0 || s(3) != s(4) || s(3) != s(5) {
		t.Fatalf("far triple broke school: %d vs %d vs %d", s(3), s(4), s(5))
	}
	if s(0) == s(3) {
		t.Fatal("clumps 20 cells apart kept one school ID")
	}
}

// Three loners meeting count as a school; two alone stay loners.
func TestLonersFormSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 20, 12)
	b := testPredator(Tuna, 5, 21.5, 12)
	a.School, b.School = 0, 0
	w.Predators = []Predator{a, b}
	w.Step(1.0 / 60)
	if s0, s1 := w.Predators[0].School, w.Predators[1].School; s0 != 0 || s1 != 0 {
		t.Fatalf("pair of loners counted as a school: %d vs %d", s0, s1)
	}
	c := testPredator(Tuna, 5, 23, 12)
	c.School = 0
	w.Predators = append(w.Predators, c)
	w.Step(1.0 / 60)
	s := func(i int) int { return w.Predators[i].School }
	if s(0) == 0 || s(0) != s(1) || s(0) != s(2) {
		t.Fatalf("meeting loners did not form a school: %d vs %d vs %d", s(0), s(1), s(2))
	}
}

// A loner within reach joins the nearest school.
func TestLonerJoinsSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 30, 12)
	b := testPredator(Tuna, 5, 31, 12)
	c := testPredator(Tuna, 5, 30.5, 12.5)
	l := testPredator(Tuna, 5, 35.5, 12)
	a.School, b.School, c.School, l.School = 5, 5, 5, 0
	w.Predators = []Predator{a, b, c, l}
	w.Step(1.0 / 60)
	if w.Predators[3].School != 5 {
		t.Fatalf("loner did not join school 5 (now %d)", w.Predators[3].School)
	}
}

// A far loner swims back to its school instead of staying solo.
func TestLonerRegroupsOverTime(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 29, 12)
	m := testPredator(Tuna, 5, 28.5, 12.5)
	l := testPredator(Tuna, 5, 38, 12)
	a.School, b.School, m.School = 5, 5, 5
	l.School = 0
	l.Heading = math.Pi // already facing the school
	w.Predators = []Predator{a, b, m, l}
	for i := 0; i < 150; i++ {
		w.Step(1.0 / 60)
	}
	if w.Predators[3].School != w.Predators[0].School || w.Predators[3].School == 0 {
		t.Fatalf("loner never regrouped: schools %d, %d, %d, %d",
			w.Predators[0].School, w.Predators[1].School, w.Predators[2].School, w.Predators[3].School)
	}
}

// Close same-kind schools either merge or refuse-with-cooldown: never a
// silent overlap, and the refusal is recorded for that encounter.
func TestSchoolsMergeOrRefuse(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	ps := []Predator{
		testPredator(Tuna, 5, 28, 12),
		testPredator(Tuna, 5, 29, 12),
		testPredator(Tuna, 5, 28.5, 12.5),
		testPredator(Tuna, 5, 33, 12),
		testPredator(Tuna, 5, 34, 12),
		testPredator(Tuna, 5, 33.5, 12.5),
	}
	ps[0].School, ps[1].School, ps[2].School = 5, 5, 5
	ps[3].School, ps[4].School, ps[5].School = 6, 6, 6
	w.Predators = ps
	w.Step(1.0 / 60)
	s := func(i int) int { return w.Predators[i].School }
	merged := s(0) != 0 && s(0) == s(1) && s(0) == s(2) && s(0) == s(3) && s(0) == s(4) && s(0) == s(5)
	refused := w.predAvoid[predPairKey(5, 6)] > w.Time
	if !merged && !refused {
		t.Fatalf("encounter neither merged nor recorded refusal: %v", w.Predators)
	}
}

// Cross-kind schools never merge, however close.
func TestCrossKindSchoolsNeverMerge(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 29, 12)
	e := testPredator(Tuna, 5, 28.5, 12.5)
	c := testPredator(Mackerel, 5, 32, 12)
	d := testPredator(Mackerel, 5, 33, 12)
	f := testPredator(Mackerel, 5, 32.5, 12.5)
	a.School, b.School, e.School = 5, 5, 5
	c.School, d.School, f.School = 6, 6, 6
	w.Predators = []Predator{a, b, e, c, d, f}
	for i := 0; i < 5; i++ {
		w.Step(1.0 / 60)
	}
	if w.Predators[0].School != 5 || w.Predators[1].School != 5 || w.Predators[2].School != 5 {
		t.Fatal("tuna school changed across kinds")
	}
	if w.Predators[3].School != 6 || w.Predators[4].School != 6 || w.Predators[5].School != 6 {
		t.Fatal("mackerel school changed across kinds")
	}
}

// testPredator builds a straight horizontal predator spine of the given
// length centred near (x, y), heading east.
func testPredator(kind PredatorKind, length, x, y float64) Predator {
	nodes := make([]float64, SHARK_NODES*2)
	seg := length / float64(SHARK_NODES-1)
	for k := 0; k < SHARK_NODES; k++ {
		nodes[k*2] = x - seg*float64(k)
		nodes[k*2+1] = y
	}
	// Nonzero school IDs: 0 means a lone individual under the dynamic
	// school rules, so test predators start as members.
	school := 1
	if kind == Mackerel {
		school = 2
	}
	return Predator{
		ID: 1, Kind: kind, School: school, Size: 0.3, Length: length,
		Nodes: nodes, Heading: 0, Speed: 42 / pxCell, Depth: 0.8, Alive: true,
	}
}
