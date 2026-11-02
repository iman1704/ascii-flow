// Canvas behaviors: creatures may leave the canvas by movement alone and
// new ones may enter on timers.
//
// There is no deliberate exit scheduler: soft walls are leaky, so roam,
// current, wander, flight, or pursuit can carry a creature past the edge.
// Anything fully past a grace zone just off-screen despawns, so nothing pops
// out at the border. There is no minimum population: natural exits can in
// principle empty the canvas. Entries swim fresh creatures in from a random
// edge every ~10-20s per group, capped at the maxima (area fish target,
// predator roster maxima, shark roster size) with
// freshly rolled variants: a new school size for fish, new young-to-grown
// sizes for predators, a roster pick for sharks.
package world

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/iman1704/ascii-flow/internal/mathx"
	"github.com/iman1704/ascii-flow/internal/species"
)

// churnRand is a dedicated random source for canvas entries: scheduling and
// rolling entries never draws from the shared sequence the sim itself
// uses, so worlds that never reach an entry wave behave bit-for-bit like ones
// without this feature.
var churnRand = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9E3779B97F4A7C15))

// churnEvery rolls one entry interval. churnRange rolls [lo, hi),
// churnInt rolls [lo, hi], churnCoin flips a biased coin.
func churnEvery() float64 { return churnRange(canvasEveryMin, canvasEveryMax) }

func churnRange(lo, hi float64) float64 { return lo + churnRand.Float64()*(hi-lo) }

func churnInt(lo, hi int) int { return lo + churnRand.IntN(hi-lo+1) }

func churnCoin(p float64) bool { return churnRand.Float64() < p }

// churnSpecies rolls a species index weighted like species.PickSpecies.
func churnSpecies() int {
	total := 0.0
	for _, s := range species.SPECIES {
		total += s.Weight
	}
	r := churnRand.Float64() * total
	for i := range species.SPECIES {
		r -= species.SPECIES[i].Weight
		if r <= 0 {
			return i
		}
	}
	return 0
}

const (
	// canvasGrace is how far past the edge a creature swims before it
	// is removed: far enough that the renderer (which clips off-grid
	// cells) shows a clean slide-out and brief flee overshoots past
	// the edge survive, close enough that real exits complete quickly.
	canvasGrace = 4.0
	// canvasEveryMin/Max bound the per-group entry interval: a lively
	// turnover where the roster visibly refreshes over time.
	canvasEveryMin = 10.0
	canvasEveryMax = 20.0
)

// maxFish is the entry cap for bait fish: the explicit fish count or the
// area-scaled target.
func (w *World) maxFish() int {
	if w.fishCount > 0 {
		return w.fishCount
	}
	return targetFishCountFor(w.Width, w.Height)
}

// maxPredators is the entry cap for schooling predators: the natural
// area-scaled cap, or the roster maxima (tuna 8 + mackerel 11).
func (w *World) maxPredators() int {
	if w.Natural {
		return targetPredatorCountFor(w.Width, w.Height)
	}
	total := 0
	for _, spec := range predatorRoster {
		total += spec.countRand[1]
	}
	return total
}

// maxSharks is the entry cap for sharks: the natural area-scaled cap, the
// configured roster size, or 0 when sharks are disabled.
func (w *World) maxSharks() int {
	if w.NoSharks {
		return 0
	}
	if w.Natural {
		return targetSharkCountFor(w.Width, w.Height)
	}
	return len(w.sharkSpecs)
}

// liveFishCount, livePredatorCount and liveSharkCount count creatures that
// are currently alive. Natural mode uses them instead of slice lengths so
// bodies waiting to respawn do not consume the area cap.
func (w *World) liveFishCount() int {
	n := 0
	for i := range w.Fish {
		if w.Fish[i].Alive {
			n++
		}
	}
	return n
}

func (w *World) livePredatorCount() int {
	n := 0
	for i := range w.Predators {
		if w.Predators[i].Alive {
			n++
		}
	}
	return n
}

func (w *World) liveSharkCount() int {
	return len(w.Sharks)
}

// seedCanvasTimers staggers the first entry waves 10-20s out so the seeded
// ocean reads full before new creatures start swimming in.
func (w *World) seedCanvasTimers() {
	w.nextFishEnterAt = w.Time + churnEvery()
	w.nextPredEnterAt = w.Time + churnEvery()
	w.nextSharkEnterAt = w.Time + churnEvery()
}

// canvasDue reports whether an entry timer fires now, rolling it forward
// another 10-20s when it does. A zero timer means a world that bypassed
// Seed (notably hand-built test worlds): schedule instead of firing so the
// first step never spawns.
func (w *World) canvasDue(timer *float64) bool {
	if *timer == 0 {
		*timer = w.Time + churnEvery()
		return false
	}
	if w.Time >= *timer {
		*timer = w.Time + churnEvery()
		return true
	}
	return false
}

// spineOutside reports whether every spine node is past the grace zone, so
// long bodies slide fully out before they are removed.
func spineOutside(nodes []float64, W, H, g float64) bool {
	if len(nodes) == 0 {
		return false
	}
	for k := 0; k < len(nodes); k += 2 {
		if x, y := nodes[k], nodes[k+1]; x >= -g && x <= W+g && y >= -g && y <= H+g {
			return false
		}
	}
	return true
}

// sweepCanvasExits removes anything that swam past the grace zone by
// movement alone and clears hunt targets on it so nobody chases a ghost
// off-screen.
func (w *World) sweepCanvasExits() {
	W, H := float64(w.Width), float64(w.Height)
	g := canvasGrace
	outside := func(x, y float64) bool { return x < -g || x > W+g || y < -g || y > H+g }

	var goneFish []*Fish
	for i := len(w.Fish) - 1; i >= 0; i-- {
		if f := &w.Fish[i]; f.Alive && outside(f.X, f.Y) {
			goneFish = append(goneFish, f)
			w.Fish[i] = w.Fish[len(w.Fish)-1]
			w.Fish = w.Fish[:len(w.Fish)-1]
		}
	}
	var gonePred []*Predator
	for i := len(w.Predators) - 1; i >= 0; i-- {
		if p := &w.Predators[i]; p.Alive && spineOutside(p.Nodes, W, H, g) {
			gonePred = append(gonePred, p)
			w.Predators[i] = w.Predators[len(w.Predators)-1]
			w.Predators = w.Predators[:len(w.Predators)-1]
		}
	}
	for i := len(w.Sharks) - 1; i >= 0; i-- {
		if sh := &w.Sharks[i]; spineOutside(sh.Nodes, W, H, g) {
			w.Sharks[i] = w.Sharks[len(w.Sharks)-1]
			w.Sharks = w.Sharks[:len(w.Sharks)-1]
		}
	}
	if len(goneFish) > 0 {
		gone := func(f *Fish) bool {
			for _, g := range goneFish {
				if g == f {
					return true
				}
			}
			return false
		}
		for i := range w.Sharks {
			if t := w.Sharks[i].Target; t != nil && gone(t) {
				w.Sharks[i].Target = nil
			}
		}
		for i := range w.Predators {
			if t := w.Predators[i].Target; t != nil && gone(t) {
				w.Predators[i].Target = nil
			}
		}
	}
	if len(gonePred) > 0 {
		gone := func(p *Predator) bool {
			for _, g := range gonePred {
				if g == p {
					return true
				}
			}
			return false
		}
		for i := range w.Sharks {
			if t := w.Sharks[i].TargetPred; t != nil && gone(t) {
				w.Sharks[i].TargetPred = nil
			}
		}
	}
}

// canvasEdge picks a random edge-entry point just off-screen with an inward
// heading, mirroring respawn: newcomers swim in instead of popping in.
func (w *World) canvasEdge() (x, y, inward float64) {
	W, H := float64(w.Width), float64(w.Height)
	const edge = 20 / pxCell
	side := churnInt(0, 3)
	along := churnRange(0.15, 0.85)
	x = along * W
	if side == 0 {
		x = -edge
	} else if side == 1 {
		x = W + edge
	}
	y = along * H
	if side == 2 {
		y = -edge
	} else if side == 3 {
		y = H + edge
	}
	inward = math.Atan2(H/2-y, W/2-x) + churnRange(-0.5, 0.5)
	return x, y, inward
}

// spawnCanvasEntries fires entry timers, swimming fresh creatures in while
// below the maxima.
func (w *World) spawnCanvasEntries() {
	if w.Width <= 0 || w.Height <= 0 {
		return
	}
	if w.canvasDue(&w.nextFishEnterAt) {
		w.spawnFishEntry()
	}
	if !w.NoPelagics {
		if w.canvasDue(&w.nextPredEnterAt) {
			w.spawnPredEntry()
		}
	}
	if !w.NoSharks {
		if w.canvasDue(&w.nextSharkEnterAt) {
			w.spawnSharkEntry()
		}
	}
}

// spawnFishEntry swims a fresh school in from a random edge: a new species
// roll at its minimum size (truncated to the remaining room), so each entry
// is a new variant instead of a copy of what left.
func (w *World) spawnFishEntry() {
	room := w.maxFish() - len(w.Fish)
	if w.Natural {
		room = w.maxFish() - w.liveFishCount()
	}
	if room <= 0 {
		return
	}
	x, y, inward := w.canvasEdge()
	spec := churnSpecies()
	sp := species.SPECIES[spec]
	size := sp.SchoolSize[0]
	if size <= 0 {
		size = 1
	}
	if size > room {
		size = room
	}
	var millDir int8 = 1
	if churnCoin(0.5) {
		millDir = -1
	}
	for k := 0; k < size; k++ {
		w.Fish = append(w.Fish, Fish{
			X:             x + churnRange(-1, 1)*w.perception,
			Y:             y + churnRange(-1, 1)*w.perception,
			Vx:            math.Cos(inward) * sp.Cruise / pxCell * 1.4,
			Vy:            math.Sin(inward) * sp.Cruise / pxCell * 1.4,
			Species:       spec,
			Alive:         true,
			Phase:         churnRange(0, mathx.TAU),
			Depth:         churnRange(0.55, 1),
			MillDir:       millDir,
			SurfaceRollAt: w.Time + churnRange(2, 6),
			Wander:        churnRange(-1, 1) * 0.2,
			Burst:         churnRange(0, mathx.TAU),
		})
	}
	w.bucketNext = make([]int, len(w.Fish))
}

// spawnPredEntry swims a fresh schooling-predator school in from a random
// edge. Normal mode lets the scarcer kind enter (ties split randomly);
// natural mode rolls a kind without trying to restore a missing species.
// Both use a freshly rolled school size and young-to-grown individual sizes.
func (w *World) spawnPredEntry() {
	room := w.maxPredators() - len(w.Predators)
	if w.Natural {
		room = w.maxPredators() - w.livePredatorCount()
	}
	if room < 3 || w.baseLength <= 0 {
		return
	}
	w.ensurePredatorSchools()
	kind := Tuna
	if w.Natural {
		// No scarcity-driven replacement: a fresh school's species is a
		// coin flip, so a missing kind is never deliberately restored.
		if churnCoin(0.5) {
			kind = Mackerel
		}
	} else {
		live := map[PredatorKind]int{}
		for i := range w.Predators {
			if w.Predators[i].Alive {
				live[w.Predators[i].Kind]++
			}
		}
		if live[Mackerel] < live[Tuna] {
			kind = Mackerel
		} else if live[Mackerel] == live[Tuna] && churnCoin(0.5) {
			kind = Mackerel
		}
	}
	base := predatorBaseSize(kind)
	var countRand [2]int
	for _, spec := range predatorRoster {
		if spec.kind == kind {
			countRand = spec.countRand
		}
	}
	if countRand[1] <= 0 {
		return
	}
	n := churnInt(countRand[0], countRand[1])
	if n > room {
		n = room
	}
	x, y, inward := w.canvasEdge()
	wave := w.predatorNextSchool
	w.predatorNextSchool++
	for k := 0; k < n; k++ {
		size := base * churnRange(0.5, 1.15)
		length := w.baseLength * size
		heading := inward + churnRange(-0.4, 0.4)
		nodes := make([]float64, SHARK_NODES*2)
		seg := length / float64(SHARK_NODES-1)
		sx := x + churnRange(-1, 1)*3
		sy := y + churnRange(-1, 1)*3
		for s := 0; s < SHARK_NODES; s++ {
			nodes[s*2] = sx - math.Cos(heading)*seg*float64(s)
			nodes[s*2+1] = sy - math.Sin(heading)*seg*float64(s)
		}
		w.Predators = append(w.Predators, Predator{
			ID:           len(w.Predators) + 1,
			Kind:         kind,
			School:       wave,
			Size:         size,
			Length:       length,
			Nodes:        nodes,
			Heading:      heading,
			Speed:        63 / pxCell * 1.3,
			Phase:        churnRange(0, mathx.TAU),
			Hunger:       churnRange(0.25, 0.6),
			Depth:        churnRange(0.6, 0.85),
			Alive:        true,
			NextFrenzyAt: w.Time + churnRange(5, 15),
		})
	}
}

// spawnSharkEntry swims a fresh shark in from a random edge. Natural mode
// rolls any shark kind and a fresh size; other modes keep the configured
// roster.
func (w *World) spawnSharkEntry() {
	if w.NoSharks || w.baseLength <= 0 {
		return
	}
	if w.Natural {
		if w.liveSharkCount() >= w.maxSharks() {
			return
		}
		kind := allSharkKinds[churnInt(0, len(allSharkKinds)-1)]
		w.appendSharkEntry(kind, churnRange(0.5, 1.15))
		return
	}
	if len(w.sharkSpecs) == 0 {
		return
	}
	if len(w.Sharks) >= w.maxSharks() {
		return
	}
	spec := w.sharkSpecs[churnInt(0, len(w.sharkSpecs)-1)]
	w.appendSharkEntry(spec.Kind, spec.Size)
}

func (w *World) appendSharkEntry(kind SharkKind, size float64) {
	length := w.baseLength * size
	x, y, inward := w.canvasEdge()
	minSide := minSideFor(w.Width, w.Height)
	var orbitDir int8 = 1
	if churnCoin(0.5) {
		orbitDir = -1
	}
	nodes := make([]float64, SHARK_NODES*2)
	seg := length / float64(SHARK_NODES-1)
	for k := 0; k < SHARK_NODES; k++ {
		nodes[k*2] = x - math.Cos(inward)*seg*float64(k)
		nodes[k*2+1] = y - math.Sin(inward)*seg*float64(k)
	}
	id := w.nextSharkID
	w.nextSharkID++
	w.Sharks = append(w.Sharks, Shark{
		ID:           id,
		Kind:         kind,
		Size:         size,
		Length:       length,
		Nodes:        nodes,
		Heading:      inward,
		Speed:        45/pxCell + length*0.06,
		Phase:        churnRange(0, mathx.TAU),
		Hunger:       churnRange(0.25, 0.75),
		OrbitDir:     orbitDir,
		OrbitRadius:  minSide * churnRange(0.28, 0.4),
		Depth:        mathx.Clamp(size, 0, 1),
		NextFrenzyAt: w.Time + churnRange(5, 15),
		BreachAt:     w.Time + churnRange(6, 14),
	})
}
