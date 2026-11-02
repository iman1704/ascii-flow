// Feeding frenzy: occasional whole-school (schooling predators),
// individual (sharks), or universal (ocean-wide) hunting bursts.
//
// Schools trigger as a unit: every live member shares one FrenzyUntil /
// NextFrenzyAt pair (kept in sync each step so joiners adopt the school's
// burst and splits inherit it). Sharks trigger alone. A frenzying school
// boosts the trigger odds of nearby schools (contagion). After a frenzy
// ends the school/individual waits FrenzyCooldown before it can roll again.
//
// During a frenzy the hunter ignores its hunger gate, steers harder toward
// prey, swims faster, and holds its target lock longer. Rendering reuses
// the existing eat/splash effects, so this file is simulation-only.
package world

import (
	"math"

	"github.com/stackedlol/ascii-sharks/internal/mathx"
)

const (
	// FrenzyDurationMin/Max bound one burst: "a few seconds".
	frenzyDurationMin = 4.0
	frenzyDurationMax = 6.0
	// FrenzyCooldown gates re-triggering after a burst ends.
	frenzyCooldown = 10.0

	// Per-second trigger odds while huntable prey is near: 1/40 per
	// second on average for each school / shark.
	predFrenzyBaseRate  = 0.025
	sharkFrenzyBaseRate = 0.025
	// Universal (ocean-wide) trigger odds: one global roll per second at
	// 1/80 on average. No cooldown of its own and no prey gate; firing
	// imposes the individual 10s cooldown on every hunter.
	universalFrenzyRate = 0.0125
	// Contagion: a calm school within this distance of a frenzying
	// school rolls at BaseRate*ContagionBoost.
	predFrenzyContagionRadius = 15.0
	predFrenzyContagionBoost  = 6.0

	// Omnidirectional prey-near gates: frenzy only rolls when a meal is
	// close enough to actually chase.
	predFrenzyPreyRadius  = 16.0
	sharkFrenzyPreyRadius = 20.0

	// Hunt boost while frenzying.
	frenzySpeedBoost = 1.3
	frenzySteerBoost = 0.6
	// Longer target lock while frenzying (normal: 0.5s predators / 0.45s
	// sharks).
	frenzyRetargetAfter = 1.0
)

// inPredFrenzy reports whether p is currently bursting.
func (w *World) inPredFrenzy(p *Predator) bool {
	return w.Time < p.FrenzyUntil
}

// inSharkFrenzy reports whether sh is currently bursting.
func (w *World) inSharkFrenzy(sh *Shark) bool {
	return w.Time < sh.FrenzyUntil
}

// syncPredSchoolFrenzy unifies FrenzyUntil/NextFrenzyAt across each live
// school (max wins), so joiners adopt the burst, splits inherit it, and
// merges respect the longest cooldown.
func (w *World) syncPredSchoolFrenzy() {
	type span struct {
		frenzy, next float64
	}
	best := make(map[int]span)
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School == 0 {
			continue
		}
		s := best[p.School]
		if p.FrenzyUntil > s.frenzy {
			s.frenzy = p.FrenzyUntil
		}
		if p.NextFrenzyAt > s.next {
			s.next = p.NextFrenzyAt
		}
		best[p.School] = s
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School == 0 {
			continue
		}
		if s := best[p.School]; s.frenzy > 0 || s.next > 0 {
			p.FrenzyUntil = s.frenzy
			p.NextFrenzyAt = s.next
		}
	}
}

// predSchoolIDs collects live school IDs with at least minMembers members.
func (w *World) predSchoolIDs(minMembers int) []int {
	counts := make(map[int]int)
	for i := range w.Predators {
		p := &w.Predators[i]
		if p.Alive && p.School != 0 {
			counts[p.School]++
		}
	}
	var ids []int
	for id, n := range counts {
		if n >= minMembers {
			ids = append(ids, id)
		}
	}
	return ids
}

// predSchoolFrenzyUntil returns the shared burst end for a school (max over
// members; 0 when calm).
func (w *World) predSchoolFrenzyUntil(id int) float64 {
	best := 0.0
	for i := range w.Predators {
		p := &w.Predators[i]
		if p.Alive && p.School == id && p.FrenzyUntil > best {
			best = p.FrenzyUntil
		}
	}
	return best
}

// predSchoolNextFrenzyAt returns the shared cooldown gate for a school.
func (w *World) predSchoolNextFrenzyAt(id int) float64 {
	best := 0.0
	for i := range w.Predators {
		p := &w.Predators[i]
		if p.Alive && p.School == id && p.NextFrenzyAt > best {
			best = p.NextFrenzyAt
		}
	}
	return best
}

// nearestFishDist returns the 3D (depth-scaled) distance from (x,y,depth) to
// the closest live fish, or +Inf when the ocean is empty.
func (w *World) nearestFishDist(x, y, depth float64) float64 {
	best := math.Inf(1)
	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		dz := (f.Depth - depth) * depthScale
		if d := math.Hypot(f.X-x, math.Hypot(f.Y-y, dz)); d < best {
			best = d
		}
	}
	return best
}

// nearestPredDist returns the distance to the closest live schooling
// predator (shark meals), or +Inf when none swim.
func (w *World) nearestPredDist(x, y, depth float64) float64 {
	best := math.Inf(1)
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive {
			continue
		}
		dz := (p.Depth - depth) * depthScale
		if d := math.Hypot(p.Nodes[0]-x, math.Hypot(p.Nodes[1]-y, dz)); d < best {
			best = d
		}
	}
	return best
}

// triggerPredSchoolFrenzy starts a burst for every live member of a school:
// shared end time, shared 10s cooldown, and one shared lead fish so the
// whole school darts at the same bait ball.
func (w *World) triggerPredSchoolFrenzy(id int) {
	until := w.Time + mathx.Rand(frenzyDurationMin, frenzyDurationMax)
	next := until + frenzyCooldown
	var lead *Fish
	if cx, cy, ok := w.predSchoolCenter(id); ok {
		best := math.Inf(1)
		var depth float64
		n := 0
		for i := range w.Predators {
			p := &w.Predators[i]
			if p.Alive && p.School == id {
				depth += p.Depth
				n++
			}
		}
		if n > 0 {
			depth /= float64(n)
		}
		for i := range w.Fish {
			f := &w.Fish[i]
			if !f.Alive {
				continue
			}
			dz := (f.Depth - depth) * depthScale
			if d := math.Hypot(f.X-cx, math.Hypot(f.Y-cy, dz)); d < best {
				best = d
				lead = f
			}
		}
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School != id {
			continue
		}
		p.FrenzyUntil = until
		p.NextFrenzyAt = next
		if lead != nil {
			p.Target = lead
			p.RetargetAt = w.Time + frenzyRetargetAfter
		}
	}
}

// triggerSharkFrenzy starts an individual burst for one shark.
func (w *World) triggerSharkFrenzy(sh *Shark) {
	sh.FrenzyUntil = w.Time + mathx.Rand(frenzyDurationMin, frenzyDurationMax)
	sh.NextFrenzyAt = sh.FrenzyUntil + frenzyCooldown
	// Hunt immediately instead of patrolling until the next retarget tick.
	if sh.RetargetAt > w.Time {
		sh.RetargetAt = w.Time
	}
}

// nearestFishTo returns the closest live fish to (x, y, depth), or nil when
// the ocean is empty.
func (w *World) nearestFishTo(x, y, depth float64) *Fish {
	best := math.Inf(1)
	var lead *Fish
	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		dz := (f.Depth - depth) * depthScale
		if d := math.Hypot(f.X-x, math.Hypot(f.Y-y, dz)); d < best {
			best = d
			lead = f
		}
	}
	return lead
}

// triggerUniversalFrenzy starts one ocean-wide burst: a single shared end
// time for every live hunter, ignoring prey gates and individual cooldowns.
// Bursts already in progress are extended (max wins), never shortened, and
// every hunter lands on the individual 10s cooldown afterwards.
func (w *World) triggerUniversalFrenzy() {
	until := w.Time + mathx.Rand(frenzyDurationMin, frenzyDurationMax)
	for si := range w.Sharks {
		sh := &w.Sharks[si]
		if until > sh.FrenzyUntil {
			sh.FrenzyUntil = until
		}
		sh.NextFrenzyAt = sh.FrenzyUntil + frenzyCooldown
		if sh.RetargetAt > w.Time {
			sh.RetargetAt = w.Time
		}
	}
	seen := make(map[int]bool)
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School == 0 || seen[p.School] {
			continue
		}
		seen[p.School] = true
		var lead *Fish
		if cx, cy, ok := w.predSchoolCenter(p.School); ok {
			depth := 0.72
			n := 0
			var sum float64
			for j := range w.Predators {
				q := &w.Predators[j]
				if q.Alive && q.School == p.School {
					sum += q.Depth
					n++
				}
			}
			if n > 0 {
				depth = sum / float64(n)
			}
			lead = w.nearestFishTo(cx, cy, depth)
		}
		for j := range w.Predators {
			q := &w.Predators[j]
			if !q.Alive || q.School != p.School {
				continue
			}
			if until > q.FrenzyUntil {
				q.FrenzyUntil = until
			}
			q.NextFrenzyAt = q.FrenzyUntil + frenzyCooldown
			if lead != nil {
				q.Target = lead
				q.RetargetAt = w.Time + frenzyRetargetAfter
			}
		}
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School != 0 {
			continue
		}
		if until > p.FrenzyUntil {
			p.FrenzyUntil = until
		}
		p.NextFrenzyAt = p.FrenzyUntil + frenzyCooldown
		if lead := w.nearestFishTo(p.Nodes[0], p.Nodes[1], p.Depth); lead != nil {
			p.Target = lead
			p.RetargetAt = w.Time + frenzyRetargetAfter
		}
	}
}

// updateUniversalFrenzy rolls one global burst per step. No cooldown and no
// prey gate: when it fires, every hunter joins (or extends) the burst.
func (w *World) updateUniversalFrenzy(dt float64) {
	hasSharks := len(w.Sharks) > 0
	hasPreds := false
	for i := range w.Predators {
		if w.Predators[i].Alive {
			hasPreds = true
			break
		}
	}
	if !hasSharks && !hasPreds {
		return
	}
	if mathx.Rand(0, 1) < universalFrenzyRate*dt {
		w.triggerUniversalFrenzy()
	}
}

// updatePredSchoolFrenzy rolls school-wide bursts once per step. Only
// schools (2+ members) off cooldown with prey near the school centre may
// roll; schools near a bursting school roll at the contagion-boosted rate.
func (w *World) updatePredSchoolFrenzy(dt float64) {
	ids := w.predSchoolIDs(2)
	if len(ids) == 0 {
		return
	}
	// Snapshot bursting centres first so one step cannot chain-trigger the
	// whole ocean; neighbours catch it over the following seconds.
	type pt struct{ x, y float64 }
	var bursting []pt
	for _, id := range ids {
		if w.Time < w.predSchoolFrenzyUntil(id) {
			if cx, cy, ok := w.predSchoolCenter(id); ok {
				bursting = append(bursting, pt{cx, cy})
			}
		}
	}
	for _, id := range ids {
		if w.Time < w.predSchoolFrenzyUntil(id) {
			continue
		}
		if w.Time < w.predSchoolNextFrenzyAt(id) {
			continue
		}
		cx, cy, ok := w.predSchoolCenter(id)
		if !ok {
			continue
		}
		depth := 0.72
		n := 0
		for i := range w.Predators {
			p := &w.Predators[i]
			if p.Alive && p.School == id {
				depth += p.Depth
				n++
			}
		}
		if n > 0 {
			depth /= float64(n)
		}
		if w.nearestFishDist(cx, cy, depth) > predFrenzyPreyRadius {
			continue
		}
		rate := predFrenzyBaseRate
		for _, b := range bursting {
			if math.Hypot(cx-b.x, cy-b.y) < predFrenzyContagionRadius {
				rate *= predFrenzyContagionBoost
				break
			}
		}
		if mathx.Rand(0, 1) < rate*dt {
			w.triggerPredSchoolFrenzy(id)
		}
	}
}

// updateSharkFrenzy rolls individual bursts once per step for off-cooldown
// sharks with a meal (fish or schooling predator) in reach.
func (w *World) updateSharkFrenzy(dt float64) {
	for si := range w.Sharks {
		sh := &w.Sharks[si]
		if w.Time < sh.FrenzyUntil {
			continue
		}
		if w.Time < sh.NextFrenzyAt {
			continue
		}
		hx, hy := sh.Nodes[0], sh.Nodes[1]
		fishNear := w.nearestFishDist(hx, hy, sh.Depth) <= sharkFrenzyPreyRadius
		predNear := w.nearestPredDist(hx, hy, sh.Depth) <= sharkFrenzyPreyRadius
		if !fishNear && !predNear {
			continue
		}
		if mathx.Rand(0, 1) < sharkFrenzyBaseRate*dt {
			w.triggerSharkFrenzy(sh)
		}
	}
}
