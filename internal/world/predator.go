// Schooling predators (albacore tuna, Spanish mackerel): a third agent
// type between prey Fish and Sharks. They school with boids, cooperatively
// hunt prey fish, and flee sharks; sharks opportunistically hunt them back.
package world

import (
	"math"

	"github.com/stackedlol/ascii-sharks/internal/mathx"
)

// PredatorKind selects the schooling-predator species. Kept separate from
// SharkKind so shark validation never accepts a tuna.
type PredatorKind string

const (
	Tuna     PredatorKind = "tuna"
	Mackerel PredatorKind = "mackerel"
)

// Valid reports whether k is a known schooling-predator kind.
func (k PredatorKind) Valid() bool {
	switch k {
	case Tuna, Mackerel:
		return true
	}
	return false
}

// Predator ports the Shark spine layout (snout-first Nodes over
// SHARK_NODES) at a much smaller Length so the renderer can rasterise the
// same follow-the-leader body with distinct tuna/mackerel shapes.
type Predator struct {
	ID   int
	Kind PredatorKind
	// School is the dynamic school ID. 0 means a lone individual with no
	// school; any school that drops below 3 members dissolves back to 0,
	// and 3 individuals meeting count as a school again.
	School int
	Size   float64
	Length float64
	// Nodes are spine positions, snout first: [x0, y0, x1, y1, ...].
	Nodes   []float64
	Heading float64
	Speed   float64
	Phase   float64
	Turn    float64
	Hunger  float64
	Wander  float64
	// OrbitDir/OrbitRadius drive the lazy patrol when no prey is near.
	OrbitDir    int8
	OrbitRadius float64
	Target      *Fish
	RetargetAt  float64
	Depth       float64
	Dz          float64
	Alive       bool
	RespawnAt   float64
	// FrenzyUntil marks the end of a school-wide feeding burst
	// (Time < FrenzyUntil means frenzying); NextFrenzyAt gates the next
	// school roll to FrenzyUntil + 10s. Kept in sync across the school.
	FrenzyUntil  float64
	NextFrenzyAt float64
	// FleeDir is the depth-escape direction while chased by a shark:
	// +1 rises toward the surface, -1 dives deeper, 0 undecided. A
	// level attack always rises. It holds stable while panic lasts.
	FleeDir int8
}

// Predator schools: kind, relative size (× baseLength), school size range.
var predatorRoster = []struct {
	kind      PredatorKind
	size      float64
	school    int
	countRand [2]int
}{
	{kind: Tuna, size: 0.32, school: 0, countRand: [2]int{6, 8}},
	{kind: Mackerel, size: 0.27, school: 1, countRand: [2]int{8, 11}},
}

// naturalPredatorSchools is the number of schools natural mode seeds: two
// tuna and two mackerel. Afterwards the normal merge/split/churn logic owns
// the school count.
const naturalPredatorSchools = 4

// predatorBaseSize returns the roster base size for a kind, or 0 if unknown.
func predatorBaseSize(kind PredatorKind) float64 {
	for _, spec := range predatorRoster {
		if spec.kind == kind {
			return spec.size
		}
	}
	return 0
}

// spawnPredatorSchools seeds one tuna school and one mackerel school with
// mostly-level headings, mirroring Seed's shark/fish placement.
func (w *World) spawnPredatorSchools() {
	W := float64(w.Width)
	H := float64(w.Height)
	if W <= 0 || H <= 0 || w.baseLength <= 0 {
		return
	}
	minSide := minSideFor(w.Width, w.Height)
	id := len(w.Predators) + 1
	for _, spec := range predatorRoster {
		n := mathx.RandInt(spec.countRand[0], spec.countRand[1])
		cx := mathx.Rand(W*0.15, W*0.85)
		cy := mathx.Rand(H*0.2, H*0.8)
		dir := mathx.Rand(-0.4, 0.4)
		if mathx.Rand(0, 1) < 0.5 {
			dir += math.Pi
		}
		var orbitDir int8 = 1
		if spec.school%2 == 1 {
			orbitDir = -1
		}
		for k := 0; k < n; k++ {
			size := spec.size * mathx.Rand(0.5, 1.15)
			length := w.baseLength * size
			x := mathx.Clamp(cx+mathx.Rand(-2, 2), 2, W-2)
			y := mathx.Clamp(cy+mathx.Rand(-1.5, 1.5), 2, H-2)
			heading := dir + mathx.Rand(-0.25, 0.25)
			nodes := make([]float64, SHARK_NODES*2)
			seg := length / float64(SHARK_NODES-1)
			for s := 0; s < SHARK_NODES; s++ {
				nodes[s*2] = x - math.Cos(heading)*seg*float64(s)
				nodes[s*2+1] = y - math.Sin(heading)*seg*float64(s)
			}
			w.Predators = append(w.Predators, Predator{
				ID:           id,
				Kind:         spec.kind,
				School:       spec.school + 1,
				Size:         size,
				Length:       length,
				Nodes:        nodes,
				Heading:      heading,
				Speed:        63 / pxCell,
				Phase:        mathx.Rand(0, mathx.TAU),
				Hunger:       mathx.Rand(0.25, 0.75),
				OrbitDir:     orbitDir,
				OrbitRadius:  minSide * mathx.Rand(0.25, 0.38),
				Depth:        mathx.Rand(0.6, 0.85),
				Alive:        true,
				NextFrenzyAt: mathx.Rand(5, 15),
			})
			id++
		}
	}
}

// spawnNaturalPredatorSchools seeds natural mode's four area-capped schools:
// two tuna and two mackerel, splitting the predator cap across them.
func (w *World) spawnNaturalPredatorSchools() {
	W := float64(w.Width)
	H := float64(w.Height)
	if W <= 0 || H <= 0 || w.baseLength <= 0 {
		return
	}
	cap := targetPredatorCountFor(w.Width, w.Height)
	if cap <= 0 {
		return
	}
	minSide := minSideFor(w.Width, w.Height)
	kinds := [naturalPredatorSchools]PredatorKind{Tuna, Mackerel, Tuna, Mackerel}
	id := len(w.Predators) + 1
	for s, kind := range kinds {
		n := cap / naturalPredatorSchools
		if s < cap%naturalPredatorSchools {
			n++
		}
		if n <= 0 {
			continue
		}
		base := predatorBaseSize(kind)
		// Spread the four centres around the tank so same-kind schools
		// start far enough apart to read as separate schools.
		a := float64(s)/float64(naturalPredatorSchools)*mathx.TAU + mathx.Rand(-0.25, 0.25)
		cx := W*0.5 + math.Cos(a)*W*mathx.Rand(0.18, 0.32)
		cy := H*0.5 + math.Sin(a)*H*mathx.Rand(0.18, 0.32)
		cx = mathx.Clamp(cx, 3, W-3)
		cy = mathx.Clamp(cy, 3, H-3)
		dir := mathx.Rand(-0.4, 0.4)
		if mathx.Rand(0, 1) < 0.5 {
			dir += math.Pi
		}
		var orbitDir int8 = 1
		if s%2 == 1 {
			orbitDir = -1
		}
		for k := 0; k < n; k++ {
			size := base * mathx.Rand(0.5, 1.15)
			length := w.baseLength * size
			x := mathx.Clamp(cx+mathx.Rand(-2, 2), 2, W-2)
			y := mathx.Clamp(cy+mathx.Rand(-1.5, 1.5), 2, H-2)
			heading := dir + mathx.Rand(-0.25, 0.25)
			nodes := make([]float64, SHARK_NODES*2)
			seg := length / float64(SHARK_NODES-1)
			for node := 0; node < SHARK_NODES; node++ {
				nodes[node*2] = x - math.Cos(heading)*seg*float64(node)
				nodes[node*2+1] = y - math.Sin(heading)*seg*float64(node)
			}
			w.Predators = append(w.Predators, Predator{
				ID:           id,
				Kind:         kind,
				School:       s + 1,
				Size:         size,
				Length:       length,
				Nodes:        nodes,
				Heading:      heading,
				Speed:        63 / pxCell,
				Phase:        mathx.Rand(0, mathx.TAU),
				Hunger:       mathx.Rand(0.25, 0.75),
				OrbitDir:     orbitDir,
				OrbitRadius:  minSide * mathx.Rand(0.25, 0.38),
				Depth:        mathx.Rand(0.6, 0.85),
				Alive:        true,
				NextFrenzyAt: mathx.Rand(5, 15),
			})
			id++
		}
	}
}

// rescalePredators recomputes each predator's Length absolutely from its
// spec Size, matching rescaleSharks so repeated resizes converge.
func (w *World) rescalePredators(oldBase, newBase float64) {
	if oldBase <= 0 || newBase == oldBase {
		return
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		if p.Size <= 0 {
			continue
		}
		newLength := newBase * p.Size
		if p.Length > 0 && newLength != p.Length {
			k := newLength / p.Length
			sx, sy := p.Nodes[0], p.Nodes[1]
			for n := 1; n < SHARK_NODES; n++ {
				p.Nodes[n*2] = sx + (p.Nodes[n*2]-sx)*k
				p.Nodes[n*2+1] = sy + (p.Nodes[n*2+1]-sy)*k
			}
		}
		p.Length = newLength
	}
}

// School interaction ranges, in cells. Join/form sit inside perception so
// newly (re)joined members immediately feel cohesion; fission needs a
// clearly larger gap so loose schools do not flicker IDs.
const (
	predSchoolRadius = 6.0
	predJoinRadius   = 5.0
	predFormRadius   = 5.0
	predFissionGap   = 9.0
	predMergeRadius  = 7.0
	// predLonerHuntRadius is how far a lone individual may roam from
	// company and still hunt: beyond this it abandons the pursuit and
	// regroups instead of chasing solo.
	predLonerHuntRadius = 10.0
	// predMergeChance is the per-encounter merge probability: schools
	// usually merge on meeting, but sometimes refuse just that encounter.
	predMergeChance    = 0.8
	predRefuseCooldown = 6.0
)

// ensurePredatorSchools lazily initialises school bookkeeping for worlds
// that bypass Seed (notably tests that hand-build Predators).
func (w *World) ensurePredatorSchools() {
	if w.predAvoid == nil {
		w.predAvoid = make(map[uint64]float64)
	}
	if w.predatorNextSchool <= 0 {
		maxID := 2
		for i := range w.Predators {
			if w.Predators[i].School > maxID {
				maxID = w.Predators[i].School
			}
		}
		w.predatorNextSchool = maxID + 1
	}
}

// predPairKey orders two school IDs into one cooldown-map key.
func predPairKey(a, b int) uint64 {
	if a > b {
		a, b = b, a
	}
	return uint64(a)<<32 | uint64(b)
}

// predSchoolCenter returns the 2D centroid and live member count of a
// school; ok is false when the school has no live members.
func (w *World) predSchoolCenter(id int) (cx, cy float64, ok bool) {
	var n int
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive || p.School != id {
			continue
		}
		cx += p.Nodes[0]
		cy += p.Nodes[1]
		n++
	}
	if n == 0 {
		return 0, 0, false
	}
	return cx / float64(n), cy / float64(n), true
}

// predRegroupTarget tells a loner where to head: the centroid of the
// closest same-kind school, else the snout of the nearest same-kind
// predator. ok is false when no same-kind company exists at all.
func (w *World) predRegroupTarget(p *Predator) (tx, ty float64, ok bool) {
	best := math.Inf(1)
	seen := make(map[int]bool)
	for i := range w.Predators {
		q := &w.Predators[i]
		if !q.Alive || q.Kind != p.Kind || q.School == 0 || seen[q.School] {
			continue
		}
		seen[q.School] = true
		if ccx, ccy, cok := w.predSchoolCenter(q.School); cok {
			d := math.Hypot(ccx-p.Nodes[0], ccy-p.Nodes[1])
			if d < best {
				best = d
				tx, ty = ccx, ccy
				ok = true
			}
		}
	}
	if ok {
		return tx, ty, true
	}
	for i := range w.Predators {
		q := &w.Predators[i]
		if !q.Alive || q.Kind != p.Kind || q == p {
			continue
		}
		d := math.Hypot(q.Nodes[0]-p.Nodes[0], q.Nodes[1]-p.Nodes[1])
		if d < best {
			best = d
			tx, ty = q.Nodes[0], q.Nodes[1]
			ok = true
		}
	}
	return tx, ty, ok
}

// maintainPredatorSchools recomputes school membership once per step:
// sub-threshold schools dissolve to loners, loners join the nearest school
// or group up into a new one (3 count as a school), clearly separated
// fragments split off, and close same-kind schools usually merge.
func (w *World) maintainPredatorSchools() {
	w.ensurePredatorSchools()
	alive := make([]int, 0, len(w.Predators))
	for i := range w.Predators {
		if w.Predators[i].Alive {
			alive = append(alive, i)
		}
	}
	if len(alive) == 0 {
		return
	}

	// Dissolve sub-threshold schools: a lone ID is an individual, not a
	// school, so its last member swims as a loner until it regroups.
	counts := make(map[int]int)
	for _, i := range alive {
		if s := w.Predators[i].School; s != 0 {
			counts[s]++
		}
	}
	for _, i := range alive {
		if s := w.Predators[i].School; s != 0 && counts[s] < 3 {
			w.Predators[i].School = 0
		}
	}

	// Loners join the nearest same-kind school in reach, else group with
	// the nearest loners into a brand-new school.
	for _, i := range alive {
		p := &w.Predators[i]
		if p.School != 0 {
			continue
		}
		best, bestD := -1, predJoinRadius
		for _, j := range alive {
			if i == j {
				continue
			}
			q := &w.Predators[j]
			if q.School == 0 || q.Kind != p.Kind {
				continue
			}
			dz := (q.Depth - p.Depth) * depthScale
			d := math.Hypot(q.Nodes[0]-p.Nodes[0], math.Hypot(q.Nodes[1]-p.Nodes[1], dz))
			if d < bestD {
				bestD = d
				best = j
			}
		}
		if best >= 0 {
			p.School = w.Predators[best].School
			continue
		}
		best, bestD = -1, predFormRadius
		second, secondD := -1, predFormRadius
		for _, j := range alive {
			if i == j {
				continue
			}
			q := &w.Predators[j]
			if q.School != 0 || q.Kind != p.Kind {
				continue
			}
			dz := (q.Depth - p.Depth) * depthScale
			d := math.Hypot(q.Nodes[0]-p.Nodes[0], math.Hypot(q.Nodes[1]-p.Nodes[1], dz))
			if d < bestD {
				second, secondD = best, bestD
				bestD = d
				best = j
			} else if d < secondD {
				secondD = d
				second = j
			}
		}
		if best >= 0 && second >= 0 {
			id := w.predatorNextSchool
			w.predatorNextSchool++
			p.School = id
			w.Predators[best].School = id
			w.Predators[second].School = id
		}
	}

	// Fission: one ID spread over clearly separated clumps becomes one
	// school per clump (clumps below 3 revert to loners). Components link
	// within perception; only clumps past the fission gap split off.
	bySchool := make(map[int][]int)
	for _, i := range alive {
		if s := w.Predators[i].School; s != 0 {
			bySchool[s] = append(bySchool[s], i)
		}
	}
	for _, members := range bySchool {
		if len(members) < 3 {
			continue
		}
		comps := predComponents(w, members, predSchoolRadius)
		if len(comps) < 2 {
			continue
		}
		// Largest clump keeps the ID.
		big := 0
		for c := 1; c < len(comps); c++ {
			if len(comps[c]) > len(comps[big]) {
				big = c
			}
		}
		for c, comp := range comps {
			if c == big {
				continue
			}
			if predMinDist(w, comp, comps[big]) <= predFissionGap {
				continue
			}
			if len(comp) >= 3 {
				nid := w.predatorNextSchool
				w.predatorNextSchool++
				for _, i := range comp {
					w.Predators[i].School = nid
				}
			} else {
				w.Predators[comp[0]].School = 0
			}
		}
	}

	// Fusion: close same-kind schools usually merge; a refusal holds
	// only for that encounter via cooldown.
	bySchool = make(map[int][]int)
	for _, i := range alive {
		if s := w.Predators[i].School; s != 0 {
			bySchool[s] = append(bySchool[s], i)
		}
	}
	type center struct {
		x, y float64
		n    int
		kind PredatorKind
	}
	centers := make(map[int]center, len(bySchool))
	for id, members := range bySchool {
		var x, y float64
		for _, i := range members {
			x += w.Predators[i].Nodes[0]
			y += w.Predators[i].Nodes[1]
		}
		centers[id] = center{x / float64(len(members)), y / float64(len(members)), len(members), w.Predators[members[0]].Kind}
	}
	for idA, A := range centers {
		for idB, B := range centers {
			if idA >= idB || A.kind != B.kind {
				continue
			}
			if math.Hypot(A.x-B.x, A.y-B.y) >= predMergeRadius {
				continue
			}
			key := predPairKey(idA, idB)
			if w.Time < w.predAvoid[key] {
				continue
			}
			if mathx.Rand(0, 1) < predMergeChance {
				keep, drop := idA, idB
				if B.n > A.n {
					keep, drop = drop, keep
				}
				for _, i := range bySchool[drop] {
					w.Predators[i].School = keep
				}
				delete(w.predAvoid, key)
			} else {
				w.predAvoid[key] = w.Time + predRefuseCooldown
			}
		}
	}
	if len(w.predAvoid) > 32 {
		for k, t := range w.predAvoid {
			if t <= w.Time {
				delete(w.predAvoid, k)
			}
		}
	}
}

// predComponents groups member indices into proximity clumps, linking
// pairs within radius.
func predComponents(w *World, members []int, radius float64) [][]int {
	var comps [][]int
	for _, i := range members {
		mine := -1
		for c, comp := range comps {
			for _, j := range comp {
				a, b := &w.Predators[i], &w.Predators[j]
				dz := (a.Depth - b.Depth) * depthScale
				d := math.Hypot(a.Nodes[0]-b.Nodes[0], math.Hypot(a.Nodes[1]-b.Nodes[1], dz))
				if d <= radius {
					mine = c
					break
				}
			}
			if mine >= 0 {
				break
			}
		}
		if mine < 0 {
			comps = append(comps, []int{i})
		} else {
			comps[mine] = append(comps[mine], i)
		}
	}
	return comps
}

// predMinDist returns the smallest snout distance between two clumps.
func predMinDist(w *World, a, b []int) float64 {
	best := math.Inf(1)
	for _, i := range a {
		for _, j := range b {
			pa, pb := &w.Predators[i], &w.Predators[j]
			dz := (pa.Depth - pb.Depth) * depthScale
			d := math.Hypot(pa.Nodes[0]-pb.Nodes[0], math.Hypot(pa.Nodes[1]-pb.Nodes[1], dz))
			if d < best {
				best = d
			}
		}
	}
	return best
}

// stepPredators advances schooling predators: boid schooling within their
// own school, cooperative pursuit of prey fish (shared lead + encircle),
// strong flee from sharks, hunger cycle, follow-the-leader spine, feeding.
func (w *World) stepPredators(dt float64) {
	W := float64(w.Width)
	H := float64(w.Height)
	minSide := math.Min(W, H)
	cxW, cyW := W/2, H/2
	var cur mathx.Vec

	// Recompute school membership before anyone moves: split fragments
	// earn new IDs, loners join or group into trios, close schools may merge.
	w.maintainPredatorSchools()
	// Sync shared burst timers, then roll new school-wide frenzies.
	w.syncPredSchoolFrenzy()
	w.updatePredSchoolFrenzy(dt)

	for pi := range w.Predators {
		p := &w.Predators[pi]
		if !p.Alive {
			continue
		}
		L := p.Length
		nodes := p.Nodes
		hx, hy := nodes[0], nodes[1]

		p.Hunger = min(1, p.Hunger+dt*0.025)

		dx := math.Cos(p.Heading)
		dy := math.Sin(p.Heading)

		// Schoolmates: same dynamic school ID within perception. Only
		// members (School != 0) school; loners regroup further below.
		// O(N^2) is fine for ~20 predators.
		var avx, avy, cx, cy, sx, sy float64
		var n int
		// Personal space scales with body length so larger individuals
		// keep more room and stay visually distinct. Capped below the
		// school perception radius so neighbours still cohere.
		sepR := mathx.Clamp(p.Length, 3.5, 5.0)
		if p.School != 0 {
			for oi := range w.Predators {
				if oi == pi {
					continue
				}
				o := &w.Predators[oi]
				if !o.Alive || o.School != p.School {
					continue
				}
				ox := o.Nodes[0] - hx
				oy := o.Nodes[1] - hy
				dz := (o.Depth - p.Depth) * depthScale
				d := math.Hypot(ox, math.Hypot(oy, dz))
				if d >= predSchoolRadius || d < 1e-3 {
					continue
				}
				ovx := math.Cos(o.Heading) * o.Speed
				ovy := math.Sin(o.Heading) * o.Speed
				avx += ovx
				avy += ovy
				cx += ox
				cy += oy
				n++
				if d < sepR {
					sx -= ox / d * (1 - d/sepR)
					sy -= oy / d * (1 - d/sepR)
				}
			}
		}

		// Wander: slowly drifting offset ahead of the snout.
		p.Wander = mathx.Clamp(p.Wander+(mathx.Rand(0, 1)-0.5)*dt*3.0, -1.0, 1.0)
		dx += math.Cos(p.Heading+p.Wander) * 0.35
		dy += math.Sin(p.Heading+p.Wander) * 0.35

		// Hunt: retarget twice a second; schools share leads because every
		// member scores the same bait ball, so they converge and encircle.
		// Frenzying schools hold the lock longer (1s) and ignore hunger.
		frenzy := w.inPredFrenzy(p)
		if w.Time >= p.RetargetAt {
			p.Target = w.findFishPrey(p)
			if frenzy {
				p.RetargetAt = w.Time + frenzyRetargetAfter
			} else {
				p.RetargetAt = w.Time + 0.5
			}
		}
		// Loners hunt only near company: an individual too far from its
		// school abandons the pursuit and regroups instead of chasing
		// solo. Frenzy bursts override, so a bursting loner still hunts.
		if p.School == 0 && !frenzy && p.Target != nil {
			if tx, ty, ok := w.predRegroupTarget(p); ok {
				if math.Hypot(tx-hx, ty-hy) > predLonerHuntRadius {
					p.Target = nil
				}
			}
		}
		t := p.Target
		hunting, closing := false, 0.0
		if t != nil && t.Alive && (p.Hunger > 0.35 || frenzy) {
			tx := t.X + t.Vx*0.3 - hx
			ty := t.Y + t.Vy*0.3 - hy
			tz := (t.Depth - p.Depth) * depthScale
			d := math.Hypot(tx, math.Hypot(ty, tz))
			if d == 0 {
				d = 1
			}
			planar := math.Hypot(tx, ty)
			if planar < 1e-3 {
				planar = 1e-3
			}
			wt := 1.2 + 2.0*p.Hunger
			if frenzy {
				wt = 1.2 + 2.0*1.0 + frenzySteerBoost
			}
			dx += tx / planar * wt
			dy += ty / planar * wt
			hunting = true
			closing = mathx.Clamp(1-d/(L*1.4), 0, 1)
			// Encirclement: tangential pull around close prey so the
			// school circles the bait ball before darting through.
			if closing > 0.25 {
				dx += -ty / planar * 0.8 * closing * float64(p.OrbitDir)
				dy += tx / planar * 0.8 * closing * float64(p.OrbitDir)
			}
		} else {
			// Patrol: lazy circuit while cruising between hunts.
			rx := (hx - cxW) / (W / minSide)
			ry := hy - cyW
			r := math.Hypot(rx, ry)
			if r == 0 {
				r = 1
			}
			radius := p.OrbitRadius * (1 + 0.15*math.Sin(w.Time*0.06+float64(p.ID)))
			radial := mathx.Clamp((radius-r)/radius, -1, 1)
			dx += ((-ry/r)*float64(p.OrbitDir) + (rx/r)*radial*0.9) * 0.45
			dy += ((rx/r)*float64(p.OrbitDir) + (ry/r)*radial*0.9) * 0.45
		}

		// Flee sharks: measured to the front-half spine nodes, with the
		// fountain-effect sidestep prey fish use. fleeSharkDepth tracks
		// the most threatening shark so the predator flees away in
		// depth: dive when the shark is shallower, rise when deeper.
		panic := 0.0
		fleeSharkDepth := p.Depth
		bestQ := -1.0
		for si := range w.Sharks {
			sh := &w.Sharks[si]
			fearR := sh.Length*0.55 + 50/pxCell
			dzs := (p.Depth - sh.Depth) * depthScale
			best := math.Inf(1)
			var bx, by float64
			for k := 0; k <= 12; k += 3 {
				nx := sh.Nodes[k*2]
				ny := sh.Nodes[k*2+1]
				d2 := (hx-nx)*(hx-nx) + (hy-ny)*(hy-ny) + dzs*dzs
				if d2 < best {
					best = d2
					bx, by = nx, ny
				}
			}
			d := math.Sqrt(best)
			if d >= fearR || d < 1e-3 {
				continue
			}
			wt := 1 - d/fearR
			wt *= wt
			planar := math.Hypot(hx-bx, hy-by)
			if planar < 1e-3 {
				planar = 1e-3
			}
			awayX := (hx - bx) / planar
			awayY := (hy - by) / planar
			shx := math.Cos(sh.Heading)
			shy := math.Sin(sh.Heading)
			side := 1.0
			if shx*(hy-sh.Nodes[1])-shy*(hx-sh.Nodes[0]) < 0 {
				side = -1
			}
			dx += (awayX*0.8 - shy*side*0.8) * 3.2 * wt
			dy += (awayY*0.8 + shx*side*0.8) * 3.2 * wt
			if q := math.Sqrt(wt); q > panic {
				panic = q
			}
			if q := math.Sqrt(wt); q > bestQ {
				bestQ = q
				fleeSharkDepth = sh.Depth
			}
		}

		// Depth-escape direction: away from the dominant shark's depth.
		// Shallower shark -> dive deeper; deeper shark -> rise toward
		// the surface; level attack -> always rise toward the surface.
		fleeDir := int8(0)
		if panic > 0 {
			gap := fleeSharkDepth - p.Depth
			if gap > fleeDepthEps {
				fleeDir = -1
				p.FleeDir = 0
			} else if gap < -fleeDepthEps {
				fleeDir = 1
				p.FleeDir = 0
			} else {
				p.FleeDir = 1
				fleeDir = p.FleeDir
			}
		} else {
			p.FleeDir = 0
		}

		// Directed vertical bolt: rising predators dash upward so a
		// shark harrying a school at the top rings; diving predators
		// push downward away from a shallower shark.
		if panic > 0 && fleeDir > 0 {
			dy -= 3.5 * panic
		} else if panic > 0 && fleeDir < 0 {
			dy += 3.5 * panic
		}

		// Schooling forces, gated by state so splits only happen on the
		// hunt or under attack: calm members hold a tight school, hunters
		// loosen as they close, and panicked members scatter outward.
		if n > 0 {
			cohesionW := 0.7
			if hunting {
				cohesionW *= 1 - 0.6*closing
			}
			if panic > 0 {
				cohesionW *= 1 - 0.5*panic
			}
			cohesionW = max(cohesionW, 0.15)
			pvx := math.Cos(p.Heading) * p.Speed
			pvy := math.Sin(p.Heading) * p.Speed
			dx += (avx/float64(n)-pvx)*0.55 + cx/float64(n)*cohesionW + sx*1.8
			dy += (avy/float64(n)-pvy)*0.55 + cy/float64(n)*cohesionW + sy*1.8
			if panic > 0.35 {
				if ccx, ccy, ok := w.predSchoolCenter(p.School); ok {
					ox, oy := hx-ccx, hy-ccy
					if d := math.Hypot(ox, oy); d > 1e-3 {
						k := 1.8 * (panic - 0.35) / 0.65
						dx += ox / d * k
						dy += oy / d * k
					}
				}
			}
		} else if p.School != 0 {
			// Straggler: still a member but out of touch; pull back to
			// the school centre so calm drift never strands it alone.
			// Flee-first: the pull fades while panic is high and wins
			// back once the chase ends, so stragglers flee as a group
			// instead of turning into the shark to regroup.
			if regroup := 1 - min(1, panic*1.2); regroup > 0 {
				if ccx, ccy, ok := w.predSchoolCenter(p.School); ok {
					ox, oy := ccx-hx, ccy-hy
					if d := math.Hypot(ox, oy); d > 1e-3 {
						dx += ox / d * 1.4 * regroup
						dy += oy / d * 1.4 * regroup
					}
				}
			}
		} else if tx, ty, ok := w.predRegroupTarget(p); ok {
			// Loner: make hard for the closest school so solo swims
			// stay brief — no individuals except while actively
			// hunted. The pull only fades under direct attack
			// (panic near 1) and is full strength otherwise, and
			// loners never hunt far from company (see above), so
			// the regroup always wins once the chase ends.
			if regroup := 1 - min(1, panic*0.7); regroup > 0 {
				ox, oy := tx-hx, ty-hy
				if d := math.Hypot(ox, oy); d > 1e-3 {
					dx += ox / d * 3.5 * regroup
					dy += oy / d * 3.5 * regroup
				}
			}
		}
		// Distinct same-kind schools keep a little spacing so a refused
		// merge reads as two schools passing, not one overlapping mass.
		for oi := range w.Predators {
			if oi == pi {
				continue
			}
			o := &w.Predators[oi]
			if !o.Alive || o.Kind != p.Kind || o.School == 0 || p.School == 0 || o.School == p.School {
				continue
			}
			ox := o.Nodes[0] - hx
			oy := o.Nodes[1] - hy
			dz := (o.Depth - p.Depth) * depthScale
			d := math.Hypot(ox, math.Hypot(oy, dz))
			if d >= 4.0 || d < 1e-3 {
				continue
			}
			dx -= ox / d * (1 - d/4.0) * 0.6
			dy -= oy / d * (1 - d/4.0) * 0.6
		}

		// Leaky soft walls: strong enough to turn calm patrols back,
		// but pursuit or flight can overpower them, so exits happen
		// by movement alone.
		{
			const m = 36 / pxCell
			if hx < m {
				dx += (m - hx) * 1.2
			} else if hx > W-m {
				dx -= (hx - (W - m)) * 1.2
			}
			if hy < m {
				dy += (m - hy) * 1.2
			} else if hy > H-m {
				dy -= (hy - (H - m)) * 1.2
			}
		}

		// Ambient current.
		mathx.Current(hx*pxCell, hy*pxCell, w.Time, &cur)
		dx += cur.X * 0.4 / pxCell
		dy += cur.Y * 0.4 / pxCell

		desired := math.Atan2(dy, dx)
		// Small bodies turn tighter than sharks. Loners wheel around
		// fast so regrouping never waits on a slow U-turn.
		agility := p.Speed / (L * 0.18)
		if hunting {
			agility = min(agility, 2.5+panic)
		} else if p.School == 0 {
			agility = min(agility, 3.0)
		} else {
			agility = min(agility, 1.4+panic*2)
		}
		maxTurn := agility * dt
		turned := mathx.Clamp(mathx.AngleDiff(p.Heading, desired), -maxTurn, maxTurn)
		p.Heading += turned
		p.Turn += (turned/dt - p.Turn) * min(1, dt*3)

		// 1.5x pace to match the faster bait fish, keeping hunts closing.
		cruise := 63/pxCell + L*0.075
		targetSpeed := cruise
		if hunting {
			// Subtle burst: ~1.15-1.5x cruise so hunts read without
			// outrunning the fleeing fish by lengths.
			targetSpeed = cruise * (1.15 + closing*0.35)
			if frenzy {
				targetSpeed *= frenzySpeedBoost
			}
		}
		if panic > 0 {
			targetSpeed *= 1 + panic*0.4
		}
		if n == 0 {
			// Out of touch with the school: hurry. Loners hurry
			// hardest (1.5x) so individuals rejoin fast; stragglers
			// still hurry (1.25x) but hold formation.
			if p.School == 0 {
				targetSpeed *= 1.5
			} else {
				targetSpeed *= 1.25
			}
		}
		p.Speed += (targetSpeed - p.Speed) * min(1, dt*1.4)
		p.Phase += dt * (3 + p.Speed/1.6)

		// Depth: porpoise on patrol, dive toward prey when hunting.
		// A shark-chased predator flees away in depth instead: rise
		// when the shark is deeper, dive when it is shallower.
		var depthTarget float64
		if panic > 0 && fleeDir > 0 {
			depthTarget = sharkDepthMax
		} else if panic > 0 && fleeDir < 0 {
			depthTarget = mathx.Clamp(0.72-0.4*panic, sharkDepthMin, sharkDepthMax)
		} else if hunting && t != nil && t.Alive {
			depthTarget = mathx.Clamp(t.Depth, sharkDepthMin, sharkDepthMax)
		} else {
			depthTarget = 0.72 + sharkPorpoiseAmp*math.Sin(w.Time*sharkPorpoiseFrq+p.Phase*0.5)
		}
		p.Dz += ((depthTarget-p.Depth)*sharkDepthSpring - p.Dz*sharkDepthDamp) * dt
		p.Dz = mathx.Clamp(p.Dz, -sharkDepthMaxV, sharkDepthMaxV)
		p.Depth += p.Dz * dt
		if p.Depth < sharkDepthMin {
			p.Depth = sharkDepthMin
			if p.Dz < 0 {
				p.Dz = 0
			}
		} else if p.Depth > sharkDepthMax {
			p.Depth = sharkDepthMax
			if p.Dz > 0 {
				p.Dz = 0
			}
		}

		nodes[0] += math.Cos(p.Heading) * p.Speed * dt
		nodes[1] += math.Sin(p.Heading) * p.Speed * dt

		// Follow-the-leader spine with a looser bend limit than sharks.
		seg := L / float64(SHARK_NODES-1)
		const maxBend = 0.22
		prevAngle := p.Heading + math.Pi
		for k := 1; k < SHARK_NODES; k++ {
			px := nodes[(k-1)*2]
			py := nodes[(k-1)*2+1]
			angle := math.Atan2(nodes[k*2+1]-py, nodes[k*2]-px)
			angle = prevAngle + mathx.Clamp(mathx.AngleDiff(prevAngle, angle), -maxBend, maxBend)
			nodes[k*2] = px + math.Cos(angle)*seg
			nodes[k*2+1] = py + math.Sin(angle)*seg
			prevAngle = angle
		}

		// Feed on prey fish at the mouth, with depth alignment required.
		// Frenzying hunters feed even when satiated.
		if p.Hunger > 0.15 || frenzy {
			mouthR := L*0.08 + 6/pxCell
			mx, my := nodes[0], nodes[1]
			for i := range w.Fish {
				f := &w.Fish[i]
				if !f.Alive {
					continue
				}
				ox := f.X - mx
				oy := f.Y - my
				oz := (f.Depth - p.Depth) * depthScale
				if ox*ox+oy*oy+oz*oz < mouthR*mouthR {
					f.Alive = false
					f.RespawnAt = w.respawnDeadline(3, 8)
					p.Hunger = math.Max(0, p.Hunger-0.22)
					w.Events = append(w.Events, WorldEvent{Kind: "eat", X: f.X, Y: f.Y})
					break
				}
			}
		}
	}
}

// findFishPrey scores prey fish in the forward cone, preferring close
// stragglers in sparse buckets like findPrey but at predator scale.
func (w *World) findFishPrey(p *Predator) *Fish {
	rng := p.Length*1.2 + 120/pxCell
	hx, hy := p.Nodes[0], p.Nodes[1]
	fx, fy := math.Cos(p.Heading), math.Sin(p.Heading)
	var best *Fish
	bestScore := math.Inf(1)
	currentScore := math.Inf(1)

	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		ox := f.X - hx
		oy := f.Y - hy
		oz := (f.Depth - p.Depth) * depthScale
		d := math.Hypot(ox, math.Hypot(oy, oz))
		if d > rng || d < 1e-3 {
			continue
		}
		planar := math.Hypot(ox, oy)
		if planar < 1e-3 {
			planar = 1e-3
		}
		cos := (ox*fx + oy*fy) / planar
		if cos < 0.2 {
			continue
		}
		crowd := w.bucketCnt[w.bucketOf(f.X, f.Y)]
		score := d * (1 + (1-cos)*0.8) * (1 + float64(crowd)*0.07)
		if score < bestScore {
			bestScore = score
			best = f
		}
		if f == p.Target {
			currentScore = score
		}
	}
	stick := 1.3
	if w.Time < p.FrenzyUntil {
		stick = 1.6
	}
	if p.Target != nil && currentScore < bestScore*stick {
		return p.Target
	}
	return best
}

// findPredatorPrey lets sharks opportunistically chase a nearby schooling
// predator in the forward cone.
func (w *World) findPredatorPrey(sh *Shark) *Predator {
	rng := sh.Length*0.9 + 160/pxCell
	hx, hy := sh.Nodes[0], sh.Nodes[1]
	fx, fy := math.Cos(sh.Heading), math.Sin(sh.Heading)
	var best *Predator
	bestScore := math.Inf(1)
	currentScore := math.Inf(1)

	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive {
			continue
		}
		ox := p.Nodes[0] - hx
		oy := p.Nodes[1] - hy
		oz := (p.Depth - sh.Depth) * depthScale
		d := math.Hypot(ox, math.Hypot(oy, oz))
		if d > rng || d < 1e-3 {
			continue
		}
		planar := math.Hypot(ox, oy)
		if planar < 1e-3 {
			planar = 1e-3
		}
		cos := (ox*fx + oy*fy) / planar
		if cos < 0.2 {
			continue
		}
		score := d * (1 + (1-cos)*0.8) * 0.9
		if score < bestScore {
			bestScore = score
			best = p
		}
		if p == sh.TargetPred {
			currentScore = score
		}
	}
	stick := 1.3
	if w.Time < sh.FrenzyUntil {
		stick = 1.6
	}
	if sh.TargetPred != nil && currentScore < bestScore*stick {
		return sh.TargetPred
	}
	return best
}

// respawnPredators revives due predators as an edge-entry school of their
// own kind, keeping each school populated without trickling singles. Natural
// mode keeps the same timing and additionally respects the area cap.
func (w *World) respawnPredators() {
	w.ensurePredatorSchools()
	var due []*Predator
	for i := range w.Predators {
		p := &w.Predators[i]
		if !p.Alive && p.RespawnAt <= w.Time {
			due = append(due, p)
		}
	}
	if len(due) == 0 {
		return
	}
	if w.Natural {
		room := w.maxPredators() - w.livePredatorCount()
		if room <= 0 {
			return
		}
		if len(due) > room {
			due = due[:room]
		}
	}
	if len(due) < 4 && w.Time-w.lastPredSpawn < 9 {
		return
	}
	w.lastPredSpawn = w.Time

	W := float64(w.Width)
	H := float64(w.Height)
	const edge = 20 / pxCell
	side := mathx.RandInt(0, 3)
	along := mathx.Rand(0.15, 0.85)
	x := along * W
	if side == 0 {
		x = -edge
	} else if side == 1 {
		x = W + edge
	}
	y := along * H
	if side == 2 {
		y = -edge
	} else if side == 3 {
		y = H + edge
	}
	inward := math.Atan2(H/2-y, W/2-x) + mathx.Rand(-0.5, 0.5)
	// Revive one kind per wave so the school reforms together. The wave
	// swims under a fresh school ID: it enters clustered, and maintenance
	// merges it with kin schools when they meet.
	kind := due[0].Kind
	base := predatorBaseSize(kind)
	wave := w.predatorNextSchool
	w.predatorNextSchool++
	n := 0
	for _, p := range due {
		if p.Kind != kind {
			continue
		}
		// Each revival rolls a fresh young-to-grown size so respawned
		// schools keep the 0.5x-1.15x variation.
		if base > 0 && w.baseLength > 0 {
			p.Size = base * mathx.Rand(0.5, 1.15)
			p.Length = w.baseLength * p.Size
		}
		p.Alive = true
		p.School = wave
		p.Hunger = mathx.Rand(0.25, 0.6)
		p.Target = nil
		p.RetargetAt = 0
		p.Turn = 0
		p.Wander = 0
		p.FrenzyUntil = 0
		p.NextFrenzyAt = 0
		p.X0(inward, x, y)
		p.Depth = mathx.Rand(0.6, 0.85)
		p.Dz = 0
		n++
		if n >= 8 {
			break
		}
	}
}

// X0 places a revived predator at an edge-entry point with a snout-first
// spine aimed inward.
func (p *Predator) X0(inward, x, y float64) {
	p.Heading = inward + mathx.Rand(-0.4, 0.4)
	p.Speed = 63 / pxCell * 1.3
	seg := p.Length / float64(SHARK_NODES-1)
	sx := x + mathx.Rand(-1, 1)*3
	sy := y + mathx.Rand(-1, 1)*3
	for s := 0; s < SHARK_NODES; s++ {
		p.Nodes[s*2] = sx - math.Cos(p.Heading)*seg*float64(s)
		p.Nodes[s*2+1] = sy - math.Sin(p.Heading)*seg*float64(s)
	}
}
