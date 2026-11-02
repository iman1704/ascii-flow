package world

import (
	"math"
	"testing"
)

// A school-wide trigger sets every live member bursting together: shared
// end time, shared 10s cooldown, and one shared lead fish.
func TestPredSchoolFrenzyTriggersWholeSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = []Fish{
		{X: 30, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 30, 12)
	c := testPredator(Tuna, 5, 29, 13)
	a.School, b.School, c.School = 5, 5, 5
	w.Predators = []Predator{a, b, c}

	w.triggerPredSchoolFrenzy(5)

	until := w.Predators[0].FrenzyUntil
	if until <= w.Time+frenzyDurationMin-1e-9 || until >= w.Time+frenzyDurationMax+1e-9 {
		t.Fatalf("frenzy duration out of few-seconds bounds: until=%.2f now=%.2f", until, w.Time)
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if p.FrenzyUntil != until {
			t.Fatalf("member %d not synced into burst: %.2f vs %.2f", i, p.FrenzyUntil, until)
		}
		if p.NextFrenzyAt != until+frenzyCooldown {
			t.Fatalf("member %d cooldown != until+10s: %.2f vs %.2f", i, p.NextFrenzyAt, until+frenzyCooldown)
		}
		if !w.inPredFrenzy(p) {
			t.Fatalf("member %d not reporting frenzy", i)
		}
	}
	if w.Predators[0].Target == nil {
		t.Fatal("trigger did not assign a shared lead fish")
	}
	for i := 1; i < 3; i++ {
		if w.Predators[i].Target != w.Predators[0].Target {
			t.Fatal("school members did not share one lead fish")
		}
	}
}

// Joiners adopt the school's burst: syncing pushes the max timers to all.
func TestPredSchoolFrenzySyncUnifies(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 28, 12)
	b := testPredator(Tuna, 5, 29, 12)
	a.School, b.School = 5, 5
	a.FrenzyUntil, a.NextFrenzyAt = 12, 22
	b.FrenzyUntil, b.NextFrenzyAt = 0, 0
	w.Predators = []Predator{a, b}

	w.syncPredSchoolFrenzy()

	for i := range w.Predators {
		if w.Predators[i].FrenzyUntil != 12 || w.Predators[i].NextFrenzyAt != 22 {
			t.Fatalf("member %d did not adopt school timers: %+v", i, w.Predators[i])
		}
	}
}

// Only schools (2+ members) are eligible: loners never appear in the roll set.
func TestPredSchoolFrenzyNeedsSchool(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = nil
	a := testPredator(Tuna, 5, 20, 12)
	a.School = 0
	b := testPredator(Tuna, 5, 30, 12)
	c := testPredator(Tuna, 5, 31, 12)
	b.School, c.School = 7, 7
	w.Predators = []Predator{a, b, c}

	ids := w.predSchoolIDs(2)
	if len(ids) != 1 || ids[0] != 7 {
		t.Fatalf("expected only school 7 eligible, got %v", ids)
	}
}

// A satiated predator still feeds at the mouth while frenzying; calm and
// satiated it would let the meal pass.
func TestPredFrenzyFeedsWhenSatiated(t *testing.T) {
	newSetup := func(frenzy bool) *World {
		w := NewWorld([]SharkSpec{}, 0)
		w.Resize(80, 24)
		w.Sharks = nil
		p := testPredator(Tuna, 5, 10, 12)
		p.Hunger = 0
		p.RetargetAt = 1e9
		p.Depth = 0.7
		if frenzy {
			p.FrenzyUntil = 5
			p.NextFrenzyAt = 15
		}
		w.Predators = []Predator{p}
		w.Fish = []Fish{
			{X: 10.4, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.7, MillDir: 1},
		}
		return w
	}
	calm := newSetup(false)
	calm.Step(1.0 / 60)
	if !calm.Fish[0].Alive {
		t.Fatal("calm satiated predator ate, gate broken")
	}
	burst := newSetup(true)
	burst.Step(1.0 / 60)
	if burst.Fish[0].Alive {
		t.Fatal("frenzying satiated predator did not feed")
	}
}

// Sharks burst individually: triggering one leaves the other calm.
func TestSharkFrenzyIsIndividual(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}, {Kind: Classic, Size: 0.7}}, 0)
	w.Resize(80, 24)
	if len(w.Sharks) < 2 {
		t.Fatal("expected two sharks")
	}
	w.Fish = nil
	w.Predators = nil
	w.Sharks[0].NextFrenzyAt = 0
	w.Sharks[1].NextFrenzyAt = 0

	w.triggerSharkFrenzy(&w.Sharks[0])

	if !w.inSharkFrenzy(&w.Sharks[0]) {
		t.Fatal("triggered shark not frenzying")
	}
	if w.inSharkFrenzy(&w.Sharks[1]) {
		t.Fatal("shark frenzy spread to another individual")
	}
	if w.Sharks[0].NextFrenzyAt != w.Sharks[0].FrenzyUntil+frenzyCooldown {
		t.Fatalf("shark cooldown != until+10s: %.2f vs %.2f",
			w.Sharks[0].NextFrenzyAt, w.Sharks[0].FrenzyUntil+frenzyCooldown)
	}
}

// A satiated shark still feeds at the mouth while frenzying.
func TestSharkFrenzyFeedsWhenSatiated(t *testing.T) {
	newSetup := func(frenzy bool) *World {
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
		sh.Hunger = 0
		sh.RetargetAt = 1e9
		sh.Target = nil
		sh.TargetPred = nil
		sh.Depth = 0.7
		if frenzy {
			sh.FrenzyUntil = 5
			sh.NextFrenzyAt = 15
		} else {
			sh.FrenzyUntil = 0
			sh.NextFrenzyAt = 0
		}
		w.Predators = nil
		w.Fish = []Fish{
			{X: 10.5, Y: 12, Vx: 0, Vy: 0, Species: 2, Alive: true, Depth: 0.7, MillDir: 1},
		}
		return w
	}
	calm := newSetup(false)
	calm.Step(1.0 / 60)
	if !calm.Fish[0].Alive {
		t.Fatal("calm satiated shark ate, gate broken")
	}
	burst := newSetup(true)
	burst.Step(1.0 / 60)
	if burst.Fish[0].Alive {
		t.Fatal("frenzying satiated shark did not feed")
	}
}

// The 10s cooldown holds: an off-cooldown school with prey nearby stays
// calm while its gate lies in the future.
func TestPredSchoolFrenzyCooldownHolds(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = []Fish{
		{X: 30, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	a := testPredator(Tuna, 5, 29, 12)
	b := testPredator(Tuna, 5, 30, 12)
	a.School, b.School = 5, 5
	a.NextFrenzyAt, b.NextFrenzyAt = 100, 100
	w.Predators = []Predator{a, b}

	// Exercise only the individual roll path: a full Step would also roll
	// the universal frenzy, which ignores cooldowns by design.
	for i := 0; i < 120; i++ {
		w.Time += 1.0 / 60
		w.updatePredSchoolFrenzy(1.0 / 60)
	}
	for i := range w.Predators {
		if w.inPredFrenzy(&w.Predators[i]) {
			t.Fatalf("member %d frenzied during cooldown", i)
		}
	}
}

// A universal frenzy triggers every hunter even on individual cooldown and
// with no prey anywhere: no universal cooldown, ignores individual gates,
// but imposes the individual 10s cooldown.
func TestUniversalFrenzyTriggersAllAndIgnoresCooldown(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}, {Kind: Classic, Size: 0.7}}, 0)
	w.Resize(80, 24)
	w.Fish = nil
	for i := range w.Sharks {
		w.Sharks[i].NextFrenzyAt = 100
		w.Sharks[i].FrenzyUntil = 0
	}
	a := testPredator(Tuna, 5, 29, 12)
	b := testPredator(Tuna, 5, 30, 12)
	a.School, b.School = 5, 5
	a.NextFrenzyAt, b.NextFrenzyAt = 100, 100
	w.Predators = []Predator{a, b}

	w.triggerUniversalFrenzy()

	for i := range w.Sharks {
		if !w.inSharkFrenzy(&w.Sharks[i]) {
			t.Fatalf("shark %d not frenzying after universal", i)
		}
		if w.Sharks[i].NextFrenzyAt != w.Sharks[i].FrenzyUntil+frenzyCooldown {
			t.Fatalf("shark %d cooldown != until+10s after universal", i)
		}
	}
	until := w.Predators[0].FrenzyUntil
	if until <= w.Time {
		t.Fatal("universal did not start a predator burst")
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if !w.inPredFrenzy(p) {
			t.Fatalf("predator %d not frenzying after universal", i)
		}
		if p.FrenzyUntil != until {
			t.Fatalf("predator %d not synced to universal until: %.2f vs %.2f", i, p.FrenzyUntil, until)
		}
		if p.NextFrenzyAt != until+frenzyCooldown {
			t.Fatalf("predator %d cooldown != until+10s after universal", i)
		}
	}
}

// A universal fired mid-burst extends (never shortens): a short burst grows
// to the universal duration, a longer burst keeps its end time.
func TestUniversalFrenzyExtendsOngoing(t *testing.T) {
	w := NewWorld([]SharkSpec{{Kind: Classic, Size: 1}}, 0)
	w.Resize(80, 24)
	w.Fish = nil
	w.Predators = nil
	sh := &w.Sharks[0]

	sh.FrenzyUntil = w.Time + 1
	sh.NextFrenzyAt = sh.FrenzyUntil + frenzyCooldown
	short := sh.FrenzyUntil
	w.triggerUniversalFrenzy()
	if sh.FrenzyUntil < short {
		t.Fatalf("universal shortened a burst: %.2f -> %.2f", short, sh.FrenzyUntil)
	}
	if sh.FrenzyUntil <= w.Time+frenzyDurationMin-1e-9 {
		t.Fatalf("universal did not extend a short burst: until=%.2f now=%.2f", sh.FrenzyUntil, w.Time)
	}
	if sh.NextFrenzyAt != sh.FrenzyUntil+frenzyCooldown {
		t.Fatal("cooldown not re-based on extended burst")
	}

	sh.FrenzyUntil = w.Time + 100
	sh.NextFrenzyAt = sh.FrenzyUntil + frenzyCooldown
	w.triggerUniversalFrenzy()
	if sh.FrenzyUntil != w.Time+100 {
		t.Fatalf("universal shortened a longer burst: %.2f", sh.FrenzyUntil)
	}
	if sh.NextFrenzyAt != sh.FrenzyUntil+frenzyCooldown {
		t.Fatal("cooldown not preserved on longer burst")
	}
}

// A forced large-dt roll triggers a ready school with prey near, proving
// the trigger path fires end to end.
func TestPredSchoolFrenzyTriggersWithPrey(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(80, 24)
	w.Sharks = nil
	w.Fish = []Fish{
		{X: 30, Y: 12, Vx: 0, Vy: 0, Species: 0, Alive: true, Depth: 0.8, MillDir: 1},
	}
	a := testPredator(Tuna, 5, 29, 12)
	b := testPredator(Tuna, 5, 30, 12)
	a.School, b.School = 5, 5
	a.NextFrenzyAt, b.NextFrenzyAt = 0, 0
	w.Predators = []Predator{a, b}

	w.updatePredSchoolFrenzy(1000)

	if w.predSchoolFrenzyUntil(5) <= w.Time {
		t.Fatal("ready school with prey near never triggered")
	}
	if math.Abs(w.predSchoolNextFrenzyAt(5)-(w.predSchoolFrenzyUntil(5)+frenzyCooldown)) > 1e-9 {
		t.Fatal("school cooldown != until+10s after trigger")
	}
}
