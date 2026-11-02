// Package world ports src/world.ts: the artificial-life simulation of fish
// schools (boids) and hunting sharks.
//
// Fish: Reynolds boids (separation / alignment / cohesion) in Couzin-style
// zones, with a blind spot behind each fish. Alarm spreads fish to fish, so
// a school bolts even when only its edge sees the shark.
//
// Sharks: Reynolds steering (wander + pursuit + separation) on a
// follow-the-leader spine. Hunger builds over time and drops on a catch.
//
// UNITS PORTING NOTE: the TS sim works in canvas pixels with the arena sized
// by world.resize(w, h, cellW, cellH). The terminal port works in cell units
// instead: Resize takes cols and rows with 1 cell ≈ 12px (pxCell). Every
// px-tuned constant (perception, separation, speeds, CURRENT_SCALE) is
// rescaled to cells here; rates (align, hunger, decay) are unit-free and
// copied verbatim.
package world

import (
	"math"
	"sort"

	"github.com/stackedlol/ascii-sharks/internal/mathx"
	"github.com/stackedlol/ascii-sharks/internal/species"
)

// SHARK_NODES is the spine node count per shark. Ports SHARK_NODES.
const SHARK_NODES = 20

// pxCell is the reference pixel width of one terminal cell. All px/s and
// px/s² constants from src/world.ts are divided by it.
const pxCell = 12.0

// cellAspect is the reference row-height ÷ col-width ratio. The canvas
// renderer uses ch = cw*1.45, and terminal glyphs are also taller than
// wide, so pixel-area conversions (shark length, fish density) scale the
// height axis by this.
const cellAspect = 1.45

// SharkKind ports the SharkKind union. The zero value is invalid; use
// DefaultSharks or parse via Valid.
type SharkKind string

// Shark kinds, one per rasterised body plan in the renderer.
const (
	Classic     SharkKind = "classic"
	Hammerhead  SharkKind = "hammerhead"
	Zebra       SharkKind = "zebra"
	Tiger       SharkKind = "tiger"
	GreatHammer SharkKind = "greathammer"
	Mako        SharkKind = "mako"
	Thresher    SharkKind = "thresher"
	Nurse       SharkKind = "nurse"
)

// Valid reports whether k is a known shark kind.
func (k SharkKind) Valid() bool {
	switch k {
	case Classic, Hammerhead, Zebra, Tiger, GreatHammer, Mako, Thresher, Nurse:
		return true
	}
	return false
}

// SharkSpec selects which sharks to spawn; Size is relative to the biggest.
// Ports SharkSpec.
type SharkSpec struct {
	Kind SharkKind
	Size float64
}

// DefaultSharks ports DEFAULT_SHARKS: four classics at different depths.
func DefaultSharks() []SharkSpec {
	return []SharkSpec{
		{Kind: Classic, Size: 1},
		{Kind: Classic, Size: 0.7},
		{Kind: Classic, Size: 0.84},
		{Kind: Classic, Size: 0.77},
	}
}

// allSharkKinds is the natural-mode spawn pool: every valid body plan.
var allSharkKinds = []SharkKind{
	Classic, Hammerhead, Zebra, Tiger, GreatHammer, Mako, Thresher, Nurse,
}



// Fish ports the Fish interface.
type Fish struct {
	X, Y    float64
	Vx, Vy  float64
	Species int
	// Alarm is 0 calm → 1 panicking; spreads to neighbours.
	Alarm     float64
	Alive     bool
	RespawnAt float64
	// Phase is the tail-beat phase.
	Phase float64
	// Turn is smoothed angular velocity (rad/s).
	Turn float64
	// Wander is the smooth correlated heading noise state (-1..1): an
	// Ornstein-Uhlenbeck-like walk that replaces per-step white jitter,
	// so calm fish meander instead of twitching. Burst is the fixed
	// per-fish glide phase (0..TAU) desyncing burst-and-glide surges.
	Wander float64
	Burst  float64
	// Depth is 0.35 deep/far → 1 near/shallow. Integrated via Dz.
	Depth   float64
	Dz      float64
	MillDir int8 // +1 | -1: school's shared rotation sense when milling
	// SurfaceRollAt gates the next roll for a voluntary surface break;
	// while Time < SurfaceUntil the fish breaks the surface and rings.
	SurfaceRollAt float64
	SurfaceUntil  float64
	// FleeDir is the depth-escape direction for the current encounter:
	// +1 rises toward the surface, -1 dives deeper, 0 undecided. A
	// level attack always rises. It holds stable while the threat
	// lasts and resets once fear fades.
	FleeDir int8
	// BoltUntil ends the fresh-scare vertical dart, and BoltSpent
	// latches it: a scare triggers boltDuration of full-strength
	// bolting, then escape continues in-plane, and the dart cannot
	// re-trigger until fear fades out. The zero value is armed, so
	// freshly spawned fish startle normally.
	BoltUntil float64
	BoltSpent bool
}

// Shark ports the Shark interface.
type Shark struct {
	ID   int
	Kind SharkKind
	// Size is the relative scale factor from the spawn spec, so a resize
	// can recompute Length = baseLength * Size absolutely.
	Size   float64
	Length float64
	// Nodes are spine positions, snout first: [x0, y0, x1, y1, ...].
	Nodes   []float64
	Heading float64
	Speed   float64
	// Phase is the tail-beat phase driving the renderer's body wave.
	Phase float64
	// Turn is smoothed angular velocity (rad/s).
	Turn        float64
	Hunger      float64
	Wander      float64
	OrbitDir    int8 // +1 | -1
	OrbitRadius float64
	Target      *Fish
	// TargetPred is the hunted schooling predator, if any. Sharks eat
	// fish first but opportunistically chase a nearby tuna/mackerel.
	TargetPred *Predator
	RetargetAt float64
	// FrenzyUntil marks the end of an individual feeding burst
	// (Time < FrenzyUntil means frenzying); NextFrenzyAt gates the next
	// roll to FrenzyUntil + 10s.
	FrenzyUntil  float64
	NextFrenzyAt float64
	// Depth is 0 far/dim → 1 near/bright. Integrated via Dz.
	Depth float64
	Dz    float64
	// BreachAt gates the next voluntary surface break; while Time is
	// below BreachUntil the shark rises and cruises along the surface.
	BreachAt    float64
	BreachUntil float64
}

// WorldEvent ports the WorldEvent interface.
type WorldEvent struct {
	Kind string // "eat" | "splash"
	X, Y float64
}

// Boid gains. Separation and flee are accelerations, so the px/s² values
// from src/world.ts are rescaled to cells/s²; align/cohesion rates are
// unit-free and copied verbatim.
const (
	fishSeparationK = 420 / pxCell
	fishAlignK      = 1.4
	fishCohesionK   = 1.0
	fishFleeK       = 1100 / pxCell
	blindCos        = -0.6 // ~127° either side of the heading is visible
	// fishSeparationMax caps the per-neighbour repulsion kick: at full
	// overlap it still pushes ~50 cells/s² (far above cruise and
	// grouping pulls, so fish never stack), but the spikes stay small
	// enough for the cruise relax to absorb.
	fishSeparationMax = 1.5
)

// Roam drive: coherent school-level travel. The field is near-uniform
// across the tank (wavelength ~125 cells, so neighbours feel the same
// push and the school moves as one) and holds a direction for tens of
// seconds (periods ~50-60s), touring calm schools through the water
// column instead of shaking them in place. roamAmp sits well below
// flee (~90 cells/s²) and the grouping pulls, so escape and regrouping
// still win.
const (
	roamAmp = 1.8
	// roamVertical carries a real share of the roam drive: calm
	// schools tour the water column on slow vertical loops instead
	// of only crossing the tank sideways. It still fades with alarm
	// so escape and regrouping win.
	roamVertical = 1.0
	// midWaterK is the calm-water spring holding schools loosely in
	// the water column: at 10 rows off mid it pulls ~1.4 cells/s²,
	// enough to keep schools from parking on the floor or ceiling
	// but soft enough to let the roam drive tour them up and down.
	// Far below flee (~90), surface breaks (~45) and feeding (9).
	midWaterK = 0.14
	// boltDuration is the fresh-scare vertical dart: full-strength
	// bolting lasts this long (seconds) per scare; BoltSpent holds
	// it shut until fear fades, so chronic exposure cannot extend it.
	boltDuration = 0.4
	// groupDampK matches grouping rendezvous to the mates' mean
	// velocity, so schools close and lock formation instead of
	// slingshotting past each other at cruise pace.
	groupDampK = 1.0
)

// Depth dynamics. Depth is a perspective axis (0 far → 1 near) integrated
// via Dz in depth-units/s. depthScale converts a depth gap into
// cell-equivalent distance for 3D interaction ranges.
const (
	depthScale = 9.0

	// fleeDepthEps is the depth gap below which an attack counts as
	// level: the prey then always rises toward the surface.
	fleeDepthEps = 0.05

	fishDepthMin = 0.35
	fishDepthMax = 1.0
	// Ambient bob amplitude around the per-fish baseline, plus the dive
	// excursion when fully alarmed. depthSpread staggers each fish's
	// own baseline (keyed off its fixed Burst phase), so the school
	// reads as a spread water column instead of one flat layer.
	fishDepthAmp    = 0.14
	fishDepthDive   = 0.3
	fishDepthSpread = 0.15
	// Spring (1/s² per unit gap) and damping (1/s) for the fish depth axis.
	fishDepthSpring = 3.0
	fishDepthDamp   = 2.5
	fishDepthMaxV   = 0.8

	sharkDepthMin = 0.0
	sharkDepthMax = 1.0
	// Porpoising while patrolling: amplitude in depth units, freq in rad/s.
	sharkPorpoiseAmp = 0.12
	sharkPorpoiseFrq = 0.7
	sharkDepthSpring = 2.0
	sharkDepthDamp   = 1.5
	sharkDepthMaxV   = 0.7
)

// fishParallaxRows mirrors the renderer's fish parallax lift: shallow fish
// sit lower on screen, deep fish lift toward the horizon. The sim uses the
// same value so near-surface checks match what the renderer draws.
const fishParallaxRows = 2.0

// predatorLiftRows mirrors the renderer's shark/predator parallax lift.
const predatorLiftRows = 2.5

// surfaceBand returns the near-surface band in rows, matching the
// renderer's surfaceBand so sim and drawing agree on where ripples live.
func surfaceBand(rows int) float64 {
	band := float64(rows) * 0.14
	if band < 3 {
		band = 3
	}
	if band > 6 {
		band = 6
	}
	return band
}

// fishRenderY returns the screen row the renderer will draw a fish at.
func fishRenderY(f *Fish) float64 {
	return f.Y - (1-f.Depth)*fishParallaxRows
}

// predatorRenderY returns the screen row the renderer will draw a
// schooling predator's snout at.
func predatorRenderY(p *Predator) float64 {
	return p.Nodes[1] - (1-p.Depth)*predatorLiftRows
}

// nearSurfaceFish reports whether a fish is inside the ripple band.
func (w *World) nearSurfaceFish(f *Fish) bool {
	return fishRenderY(f) < surfaceBand(w.Height)
}

// nearSurfacePredator reports whether a schooling predator's snout is
// inside the ripple band.
func (w *World) nearSurfacePredator(p *Predator) bool {
	return predatorRenderY(p) < surfaceBand(w.Height)
}

// World is the simulation arena. Width/Height are in cells (see package
// note). Ports the World class.
type World struct {
	Width, Height int
	Time          float64
	Fish          []Fish
	Sharks        []Shark
	Predators     []Predator
	Events        []WorldEvent
	// NoSharks disables sharks: nothing spawns them and nothing
	// respawns them. Schooling predators (tuna, mackerel) are unaffected.
	NoSharks bool
	// NoPelagics disables schooling predators (tuna, mackerel):
	// nothing spawns them and nothing respawns them. Sharks are
	// unaffected.
	NoPelagics bool
	// Natural selects the cap-only ocean preset: populations scale with
	// the terminal area, sharks spawn as random kinds/sizes, and four
	// predator schools seed the water. Replacement timing matches the
	// default. The initial seed fills the caps once; afterwards nothing is
	// force-spawned to maintain a population or species mix.
	Natural bool
	// FeedingSpots are live plankton blooms bait fish gather at.
	// Nil when the water holds none; see feeding.go.
	FeedingSpots []FeedingSpot

	sharkSpecs []SharkSpec
	fishCount  int // 0 = scale with arena area
	// Canvas entry timers: entries swim fresh creatures in every
	// ~10-20s; exits happen by movement alone through leaky walls.
	// See canvas.go.
	nextFishEnterAt  float64
	nextPredEnterAt  float64
	nextSharkEnterAt float64
	// nextSharkID issues IDs to sharks entering off-canvas.
	nextSharkID int
	lastSpawn   float64 // time of the last respawn wave; trickles wait for it
	// lastPredSpawn gates small predator respawn waves separately so one
	// straggler tuna does not trickle back alone.
	lastPredSpawn float64
	// predatorNextSchool issues fresh schooling-predator school IDs.
	predatorNextSchool int
	// predAvoid records merge-refusal cooldowns per school pair (sorted
	// ID pair key → time), so a refused merge stays refused just for
	// that encounter instead of re-rolling every step.
	predAvoid map[uint64]float64
	// baseLength is the arena-derived shark scale at the current size, so
	// Resize can recompute each shark's Length absolutely.
	baseLength float64

	// Neighbourhood state. Ports perception/sepX/sepY and the spatial hash.
	perception float64
	sepX       []float64
	sepY       []float64
	foreignSep float64
	bucketCols int
	bucketRows int
	bucketHead []int
	bucketCnt  []uint16
	bucketNext []int
}

// NewWorld ports the World constructor. Nil specs select DefaultSharks;
// fishCount <= 0 scales the population with arena area.
func NewWorld(specs []SharkSpec, fishCount int) *World {
	if specs == nil {
		specs = DefaultSharks()
	}
	return &World{sharkSpecs: specs, fishCount: fishCount}
}

// minSideFor is the aspect-corrected short side of a cols×rows arena.
func minSideFor(cols, rows int) float64 {
	return math.Min(float64(cols), float64(rows)*cellAspect)
}

// baseLengthFor derives the reference shark length for an arena: the
// src/world.ts clamp(minSide * 0.44, 170, 430) in px with minSide =
// min(W*cellW, H*cellH). One col is ~12px and one row is ~1.45x that
// (renderer ch = cw*1.45), so the length that spans the same columns as
// canvas is clamp(min(W, H*1.45) * 0.44, 170/12, 430/12).
func baseLengthFor(cols, rows int) float64 {
	return mathx.Clamp(minSideFor(cols, rows)*0.44, 170/pxCell, 430/pxCell)
}

// targetFishCountFor derives the area-scaled population for an arena: the
// cell area converted back to px² so the src/world.ts density (W*H/4400
// clamped to 85..340) applies. One cell is pxCell wide and
// pxCell*cellAspect tall.
func targetFishCountFor(cols, rows int) int {
	total := int(math.Round(float64(cols) * float64(rows) * pxCell * pxCell * cellAspect / 4400))
	return int(mathx.Clamp(float64(total), 85, 340))
}

// targetPredatorCountFor derives the natural-mode cap for schooling
// predators. The floor keeps the seeded four schools above the 3-member
// school threshold on small panes; it is a maximum, not a guaranteed count.
func targetPredatorCountFor(cols, rows int) int {
	total := int(math.Round(float64(cols) * float64(rows) / 300))
	return int(mathx.Clamp(float64(total), 12, 28))
}

// targetSharkCountFor derives the natural-mode shark cap: one shark on a
// small pane, growing to at most four on a large one.
func targetSharkCountFor(cols, rows int) int {
	total := int(math.Round(float64(cols) * float64(rows) / 1600))
	return int(mathx.Clamp(float64(total), 1, 4))
}

// Resize sets the arena size in cells and seeds on first call. Ports
// resize(width, height, cellW, cellH): the cellW/cellH pixel-to-cell mapping
// collapses because the arena is already in cells (1 cell ≈ 1 grid unit, so
// perception/foreignSep are plain cell counts matching cellW*5.5/cellW*3).
//
// After the first call the LOD tracks the live size: shark lengths and orbit
// radii scale to the new base length and the default fish population grows
// or shrinks toward the area target, so detail follows the pane instead of
// freezing at the startup size.
func (w *World) Resize(cols, rows int) {
	first := w.Width == 0
	oldBase := w.baseLength
	oldMin := minSideFor(w.Width, w.Height)
	w.Width, w.Height = cols, rows
	w.perception = 5.5
	w.sepX = make([]float64, len(species.SPECIES))
	w.sepY = make([]float64, len(species.SPECIES))
	for i, sp := range species.SPECIES {
		w.sepX[i] = sp.Spacing[0]
		w.sepY[i] = sp.Spacing[1]
	}
	w.foreignSep = 3
	if first {
		w.Seed()
		return
	}
	newBase := baseLengthFor(cols, rows)
	newMin := minSideFor(cols, rows)
	w.rescaleSharks(oldBase, newBase, oldMin, newMin)
	w.rescalePredators(oldBase, newBase)
	for i := range w.Predators {
		p := &w.Predators[i]
		p.OrbitRadius *= func() float64 {
			if oldMin > 0 {
				return newMin / oldMin
			}
			return 1
		}()
	}
	w.baseLength = newBase
	if !w.Natural {
		w.trackFishPopulation(targetFishCountFor(cols, rows))
	}
	// Keep every living thing on screen: fish, shark nodes and predator
	// spines alike are pulled into the new bounds, so a resize never
	// clears the life and hunters do not swim back in from off-screen.
	for i := range w.Fish {
		w.Fish[i].X = mathx.Clamp(w.Fish[i].X, 0, float64(cols))
		w.Fish[i].Y = mathx.Clamp(w.Fish[i].Y, 0, float64(rows))
	}
	for i := range w.Sharks {
		sh := &w.Sharks[i]
		for k := 0; k < SHARK_NODES; k++ {
			sh.Nodes[k*2] = mathx.Clamp(sh.Nodes[k*2], 0, float64(cols))
			sh.Nodes[k*2+1] = mathx.Clamp(sh.Nodes[k*2+1], 0, float64(rows))
		}
	}
	for i := range w.Predators {
		p := &w.Predators[i]
		for k := 0; k < SHARK_NODES; k++ {
			p.Nodes[k*2] = mathx.Clamp(p.Nodes[k*2], 0, float64(cols))
			p.Nodes[k*2+1] = mathx.Clamp(p.Nodes[k*2+1], 0, float64(rows))
		}
	}
	// Plankton drifts with the pane: keep blooms on screen like the life.
	for i := range w.FeedingSpots {
		s := &w.FeedingSpots[i]
		s.X = mathx.Clamp(s.X, 0, float64(cols))
		s.Y = mathx.Clamp(s.Y, 0, float64(rows))
	}
	// Natural mode enforces maxima through entry caps only: over-cap
	// creatures stay until movement carries them out.
}

// rescaleSharks recomputes each shark's Length from its spec Size against
// the new base length and stretches its spine and orbit to match. Lengths
// are set absolutely (never chained ratios), so repeated resizes converge
// to the same values as a fresh seed at the new size.
func (w *World) rescaleSharks(oldBase, newBase, oldMin, newMin float64) {
	if oldBase <= 0 || newBase == oldBase {
		return
	}
	orbitK := 1.0
	if oldMin > 0 {
		orbitK = newMin / oldMin
	}
	for i := range w.Sharks {
		sh := &w.Sharks[i]
		size := sh.Size
		if size <= 0 && oldBase > 0 {
			size = sh.Length / oldBase
		}
		if size <= 0 {
			continue
		}
		sh.Size = size
		newLength := newBase * size
		if sh.Length > 0 && newLength != sh.Length {
			k := newLength / sh.Length
			sx, sy := sh.Nodes[0], sh.Nodes[1]
			for n := 1; n < SHARK_NODES; n++ {
				sh.Nodes[n*2] = sx + (sh.Nodes[n*2]-sx)*k
				sh.Nodes[n*2+1] = sy + (sh.Nodes[n*2+1]-sy)*k
			}
		}
		sh.Length = newLength
		sh.OrbitRadius *= orbitK
	}
}

// trackFishPopulation moves the default area-scaled population toward the
// target for the current size. Explicit-count worlds stay pinned.
func (w *World) trackFishPopulation(target int) {
	if w.fishCount > 0 {
		return
	}
	if delta := target - len(w.Fish); delta > 0 {
		w.spawnSchools(delta)
	} else if delta < 0 {
		w.cullFish(-delta)
	}
}

// spawnSchools appends up to n fish in size-type schools placed inside
// the current arena. Each school spawns at the type minimum (small 20,
// medium 12, large 7) with no maximum; only the final chunk is truncated
// to fit n. Close schools of the same size merge emergently through
// same-size cohesion in stepFish.
func (w *World) spawnSchools(n int) {
	W := float64(w.Width)
	H := float64(w.Height)
	if W <= 0 || H <= 0 || n <= 0 {
		return
	}
	for added := 0; added < n; {
		spec := species.PickSpecies()
		sp := species.SPECIES[spec]
		size := sp.SchoolSize[0]
		if size <= 0 {
			size = 1
		}
		if size > n-added {
			size = n - added
		}
		var millDir int8 = 1
		if mathx.Rand(0, 1) < 0.5 {
			millDir = -1
		}
		cx := mathx.Rand(W*0.06, W*0.94)
		cy := mathx.Rand(H*0.08, H*0.92)
		// Mostly level headings with real pitch variety: schools set
		// off cruising sideways, but with enough up/down tilt that
		// they work the water column from the start.
		dir := mathx.Rand(-0.7, 0.7)
		if mathx.Rand(0, 1) < 0.5 {
			dir += math.Pi
		}
		for k := 0; k < size; k++ {
			w.Fish = append(w.Fish, Fish{
				X:             cx + mathx.Rand(-1, 1)*w.sepX[spec]*math.Sqrt(float64(size)),
				Y:             cy + mathx.Rand(-1, 1)*w.sepY[spec]*math.Sqrt(float64(size))*0.6,
				Vx:            math.Cos(dir) * sp.Cruise / pxCell,
				Vy:            math.Sin(dir) * sp.Cruise / pxCell,
				Species:       spec,
				Alive:         true,
				Phase:         mathx.Rand(0, mathx.TAU),
				Depth:         mathx.Rand(0.55, 1),
				Dz:            0,
				MillDir:       millDir,
				SurfaceRollAt: mathx.Rand(2, 6),
				Wander:        mathx.Rand(-1, 1) * 0.2,
				Burst:         mathx.Rand(0, mathx.TAU),
			})
		}
		added += size
	}
	w.bucketNext = make([]int, len(w.Fish))
}

// cullFish removes n fish, preferring live ones picked at random so shrunken
// arenas thin out instead of keeping a startup-size crowd.
func (w *World) cullFish(n int) {
	for ; n > 0 && len(w.Fish) > 0; n-- {
		pick := -1
		for tries := 0; tries < 8; tries++ {
			j := mathx.RandInt(0, len(w.Fish)-1)
			if w.Fish[j].Alive {
				pick = j
				break
			}
		}
		if pick < 0 {
			pick = len(w.Fish) - 1
		}
		w.Fish[pick] = w.Fish[len(w.Fish)-1]
		w.Fish = w.Fish[:len(w.Fish)-1]
	}
	w.bucketNext = make([]int, len(w.Fish))
}

// Seed spawns the shark roster and fish schools by species. Ports seed:
// orbit placement with a snout-first spine for sharks, species-weighted
// schools with mostly-level headings for fish. Shark lengths use the
// cell-rescaled baseLength; cruise speeds are rescaled px→cells at spawn.
func (w *World) Seed() {
	W := float64(w.Width)
	H := float64(w.Height)
	if W <= 0 || H <= 0 {
		return
	}
	minSide := minSideFor(w.Width, w.Height)
	baseLength := baseLengthFor(w.Width, w.Height)
	w.baseLength = baseLength

	if w.Natural {
		n := targetSharkCountFor(w.Width, w.Height)
		w.sharkSpecs = make([]SharkSpec, 0, n)
		for i := 0; i < n; i++ {
			w.sharkSpecs = append(w.sharkSpecs, SharkSpec{
				Kind: allSharkKinds[mathx.RandInt(0, len(allSharkKinds)-1)],
				Size: mathx.Rand(0.5, 1.15),
			})
		}
	}

	count := len(w.sharkSpecs)
	if w.NoSharks {
		count = 0
	}
	w.Sharks = make([]Shark, 0, count)
	for i, spec := range w.sharkSpecs {
		if w.NoSharks {
			break
		}
		length := baseLength * spec.Size
		orbitRadius := minSide * mathx.Rand(0.28, 0.4)
		a := (float64(i)/float64(count))*mathx.TAU + mathx.Rand(-0.4, 0.4)
		var orbitDir int8 = 1
		if i%2 == 1 {
			orbitDir = -1
		}
		x := W/2 + math.Cos(a)*orbitRadius*(W/minSide)*0.9
		y := H/2 + math.Sin(a)*orbitRadius
		x = mathx.Clamp(x, 2, W-2)
		y = mathx.Clamp(y, 2, H-2)
		heading := a + float64(orbitDir)*math.Pi/2
		nodes := make([]float64, SHARK_NODES*2)
		seg := length / float64(SHARK_NODES-1)
		for n := 0; n < SHARK_NODES; n++ {
			nodes[n*2] = x - math.Cos(heading)*seg*float64(n)
			nodes[n*2+1] = y - math.Sin(heading)*seg*float64(n)
		}
		w.Sharks = append(w.Sharks, Shark{
			ID:           i + 1,
			Kind:         spec.Kind,
			Size:         spec.Size,
			Length:       length,
			Nodes:        nodes,
			Heading:      heading,
			Speed:        45 / pxCell,
			Phase:        mathx.Rand(0, mathx.TAU),
			Hunger:       mathx.Rand(0.25, 0.75),
			OrbitDir:     orbitDir,
			OrbitRadius:  orbitRadius,
			Depth:        mathx.Clamp(spec.Size, 0, 1),
			Dz:           0,
			NextFrenzyAt: mathx.Rand(5, 15),
			BreachAt:     mathx.Rand(6, 14),
		})
	}
	// Smaller (farther) sharks draw first so the big one swims over them.
	sort.Slice(w.Sharks, func(i, j int) bool { return w.Sharks[i].Length < w.Sharks[j].Length })

	total := w.fishCount
	if total <= 0 {
		total = targetFishCountFor(w.Width, w.Height)
	}
	w.Fish = make([]Fish, 0, total)
	w.spawnSchools(total)
	w.Predators = nil
	w.predatorNextSchool = 3
	if !w.NoPelagics {
		if w.Natural {
			w.spawnNaturalPredatorSchools()
			w.predatorNextSchool = naturalPredatorSchools + 1
		} else {
			w.spawnPredatorSchools()
		}
	}
	w.predAvoid = make(map[uint64]float64)
	w.nextSharkID = count + 1
	w.seedCanvasTimers()
}

// Splash scares nearby fish outward. Ports splash with the radius and
// impulse rescaled px→cells.
func (w *World) Splash(x, y float64) {
	w.Events = append(w.Events, WorldEvent{Kind: "splash", X: x, Y: y})
	const r = 240 / pxCell
	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		dx := f.X - x
		dy := f.Y - y
		d := math.Hypot(dx, dy)
		if d > r || d < 1e-3 {
			continue
		}
		// Flash expansion: everybody nearby bolts straight out.
		// Depth falloff: the splash is near-surface, so deep fish feel
		// less of the push but still startle.
		k := (1 - d/r) * (260 / pxCell)
		k *= 1 - 0.5*math.Min(1, math.Abs(f.Depth-0.8)/0.5)
		f.Vx += dx / d * k
		f.Vy += dy / d * k
		f.Alarm = 1
	}
}

// Step advances fish, schooling predators and sharks by dt seconds. Ports
// step, including the respawn of eaten fish as new schools. Events
// accumulate until the renderer consumes them, so multi-step frames keep
// every eat/splash.
func (w *World) Step(dt float64) {
	w.Time += dt
	w.rebuildHash()
	w.stepFish(dt)
	w.updateFeedingSpots(dt)
	// Universal first so its timers are visible to the individual rolls
	// below (which skip hunters already bursting) and to this step's
	// movement; extensions use max-wins so order cannot shorten a burst.
	w.updateUniversalFrenzy(dt)
	w.stepPredators(dt)
	w.stepSharks(dt)
	w.respawn()
	if !w.NoPelagics {
		w.respawnPredators()
	}
	w.sweepCanvasExits()
	w.spawnCanvasEntries()
}

func (w *World) rebuildHash() {
	size := w.perception
	if size <= 0 {
		size = 5.5
	}
	const pad = 2
	w.bucketCols = int(math.Ceil(float64(w.Width)/size)) + pad*2
	w.bucketRows = int(math.Ceil(float64(w.Height)/size)) + pad*2
	n := w.bucketCols * w.bucketRows
	if len(w.bucketHead) != n {
		w.bucketHead = make([]int, n)
		w.bucketCnt = make([]uint16, n)
	}
	for i := range w.bucketHead {
		w.bucketHead[i] = -1
	}
	for i := range w.bucketCnt {
		w.bucketCnt[i] = 0
	}
	if len(w.bucketNext) != len(w.Fish) {
		w.bucketNext = make([]int, len(w.Fish))
	}
	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		b := w.bucketOf(f.X, f.Y)
		w.bucketNext[i] = w.bucketHead[b]
		w.bucketHead[b] = i
		w.bucketCnt[b]++
	}
}

// sameSpeciesMean returns the mean 2D offset and mean velocity from
// fish i to live same-species mates within maxDist (3D distance, so
// deep fish count less). A non-positive maxDist means unlimited range.
// n is the mate count; n == 0 means no mate in range.
func (w *World) sameSpeciesMean(i int, maxDist float64) (dx, dy, dvx, dvy float64, n int) {
	f := &w.Fish[i]
	var sx, sy, svx, svy float64
	for j := range w.Fish {
		if j == i {
			continue
		}
		o := &w.Fish[j]
		if !o.Alive || o.Species != f.Species {
			continue
		}
		ox := o.X - f.X
		oy := o.Y - f.Y
		if maxDist > 0 {
			dz := (o.Depth - f.Depth) * depthScale
			if math.Hypot(ox, math.Hypot(oy, dz)) > maxDist {
				continue
			}
		}
		sx += ox
		sy += oy
		svx += o.Vx
		svy += o.Vy
		n++
	}
	if n == 0 {
		return 0, 0, 0, 0, 0
	}
	m := 1 / float64(n)
	return sx * m, sy * m, svx*m - f.Vx, svy*m - f.Vy, n
}

func (w *World) bucketOf(x, y float64) int {
	size := w.perception
	if size <= 0 {
		size = 5.5
	}
	c := int(mathx.Clamp(math.Floor(x/size)+2, 0, float64(w.bucketCols-1)))
	r := int(mathx.Clamp(math.Floor(y/size)+2, 0, float64(w.bucketRows-1)))
	return r*w.bucketCols + c
}

// stepFish ports the boid update: separation / alignment / cohesion with a
// rear blind spot, alarm contagion between neighbours, fountain-effect flee
// from shark flanks, soft walls, ambient current and speed relaxation.
func (w *World) stepFish(dt float64) {
	W := float64(w.Width)
	H := float64(w.Height)
	R := w.perception
	if R <= 0 {
		R = 5.5
	}
	R2 := R * R
	cols, rows := w.bucketCols, w.bucketRows
	// Alarm drains in about a second without fresh threat: quick
	// enough that schools settle back into cruising between hunts,
	// slow enough that a real chase holds panic while it lasts.
	calmDecay := math.Exp(-dt * 1.1)
	contagion := math.Min(1, dt*6)
	var cur mathx.Vec

	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		sp := species.SPECIES[f.Species]
		// Alarmed fish want more room: the school flashes apart around a
		// shark and closes back up once the threat fades.
		spread := 1 + 1.8*f.Alarm
		sepX := w.sepX[f.Species] * spread
		sepY := w.sepY[f.Species] * spread

		speed := math.Hypot(f.Vx, f.Vy)
		if speed == 0 {
			speed = 1
		}
		hx := f.Vx / speed
		hy := f.Vy / speed

		var ax, ay, sx, sy, avx, avy, cx, cy float64
		var n int
		neighbourAlarm := 0.0

		bc := int(mathx.Clamp(math.Floor(f.X/R)+2, 0, float64(cols-1)))
		br := int(mathx.Clamp(math.Floor(f.Y/R)+2, 0, float64(rows-1)))
		for r := max(br-1, 0); r <= min(br+1, rows-1); r++ {
			for c := max(bc-1, 0); c <= min(bc+1, cols-1); c++ {
				for j := w.bucketHead[r*cols+c]; j != -1; j = w.bucketNext[j] {
					if j == i {
						continue
					}
					o := &w.Fish[j]
					dx := o.X - f.X
					dy := o.Y - f.Y
					dz := (o.Depth - f.Depth) * depthScale
					d2 := dx*dx + dy*dy + dz*dz
					if d2 >= R2 || d2 <= 1e-6 {
						continue
					}
					d := math.Sqrt(d2)
					if d < 1e-6 {
						continue
					}
					// Elliptical personal space: wide along x, tight along
					// y. Repulsion is omnidirectional (lateral line):
					// neighbours behind push too. Vision-only repulsion
					// would drag a dense school backward, since every
					// fish would feel the mate ahead and ignore the one
					// behind, braking the whole school to a crawl.
					// The kick is capped so overlap still pushes hard
					// but the cruise relax absorbs the kicks.
					qx := dx / sepX
					qy := dy / sepY
					q2 := qx*qx + qy*qy
					if q2 < 1 {
						q := math.Sqrt(q2)
						if q == 0 {
							q = 1e-3
						}
						k := (1 - q) / q
						if k > fishSeparationMax {
							k = fishSeparationMax
						}
						sx -= qx * k
						sy -= qy * k
					}
					// Blind zone directly behind the fish: vision
					// (alignment, cohesion) and the alarm relay do not
					// see back there.
					if (dx*hx+dy*hy)/d <= blindCos {
						continue
					}
					if o.Species == f.Species {
						avx += o.Vx
						avy += o.Vy
						cx += dx
						cy += dy
						n++
					} else if d < w.foreignSep {
						// Different species keep their own schools.
						k := ((1 - d/w.foreignSep) / d) * 0.6
						sx -= dx * k
						sy -= dy * k
					}
					if o.Alarm > neighbourAlarm {
						neighbourAlarm = o.Alarm
					}
				}
			}
		}

		ax += sx * fishSeparationK
		ay += sy * fishSeparationK
		if n > 0 {
			alignK := fishAlignK * sp.Align
			cohesionK := fishCohesionK * sp.Cohesion * (1 - 0.8*f.Alarm)
			ax += (avx/float64(n)-f.Vx)*alignK + (cx/float64(n))*cohesionK
			ay += (avy/float64(n)-f.Vy)*alignK + (cy/float64(n))*cohesionK
			if sp.Mill > 0 {
				// Circle the local centre: the school spins into a bait ball.
				cl := math.Hypot(cx, cy)
				if cl == 0 {
					cl = 1
				}
				mill := sp.Mill / pxCell * (1 - f.Alarm)
				ax += -cy / cl * float64(f.MillDir) * mill
				ay += cx / cl * float64(f.MillDir) * mill
			}
		}

		// Predators. Distance is measured to the nearest point along the
		// front half of the shark, so fish also give the flanks a wide berth.
		// Depth counts via depthScale, so deep fish feel safer.
		// fleePredDepth tracks the depth of the most threatening hunter so
		// the prey can flee away in depth: dive when the hunter is
		// shallower, rise when it is deeper, 50/50 when level.
		//
		// Two threat levels: fear is the instant reflex (escape
		// steering, depth bolt) with a square-root falloff, so even a
		// mid-range hunter triggers a dodge. dread is the sustained
		// threat feeding the alarm state and the feeding gate, with a
		// quadratic falloff: only a close hunter keeps the school
		// panicking, so mid-range passes startle without boiling the
		// whole tank.
		fear := 0.0
		dread := 0.0
		fleePredDepth := f.Depth
		bestQ := -1.0
		for si := range w.Sharks {
			sh := &w.Sharks[si]
			fearR := sh.Length*0.6 + 60/pxCell
			dzs := (f.Depth - sh.Depth) * depthScale
			best := math.Inf(1)
			var bx, by float64
			for k := 0; k <= 12; k += 3 {
				nx := sh.Nodes[k*2]
				ny := sh.Nodes[k*2+1]
				d2 := (f.X-nx)*(f.X-nx) + (f.Y-ny)*(f.Y-ny) + dzs*dzs
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
			// Flee in the 2D plane; depth escape is handled by the dive drive.
			planar := math.Hypot(f.X-bx, f.Y-by)
			if planar < 1e-3 {
				planar = 1e-3
			}
			awayX := (f.X - bx) / planar
			awayY := (f.Y - by) / planar
			// Fountain effect: break to whichever side of the shark's path.
			shx := math.Cos(sh.Heading)
			shy := math.Sin(sh.Heading)
			side := 1.0
			if shx*(f.Y-sh.Nodes[1])-shy*(f.X-sh.Nodes[0]) < 0 {
				side = -1
			}
			ax += (awayX*0.7 - shy*side*0.9) * fishFleeK * wt
			ay += (awayY*0.7 + shx*side*0.9) * fishFleeK * wt
			q := math.Sqrt(wt)
			if q > fear {
				fear = q
			}
			if wt > dread {
				dread = wt
			}
			if q > bestQ {
				bestQ = q
				fleePredDepth = sh.Depth
			}
		}
		// Schooling predators scare prey too, but less than a shark: a
		// smaller fear radius and damped impulse so tuna harass schools
		// without emptying the water the way sharks do.
		for pi := range w.Predators {
			p := &w.Predators[pi]
			if !p.Alive {
				continue
			}
			fearR := p.Length*0.6 + 30/pxCell
			dzs := (f.Depth - p.Depth) * depthScale
			best := math.Inf(1)
			var bx, by float64
			for k := 0; k <= 12; k += 3 {
				nx := p.Nodes[k*2]
				ny := p.Nodes[k*2+1]
				d2 := (f.X-nx)*(f.X-nx) + (f.Y-ny)*(f.Y-ny) + dzs*dzs
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
			planar := math.Hypot(f.X-bx, f.Y-by)
			if planar < 1e-3 {
				planar = 1e-3
			}
			awayX := (f.X - bx) / planar
			awayY := (f.Y - by) / planar
			phx := math.Cos(p.Heading)
			phy := math.Sin(p.Heading)
			side := 1.0
			if phx*(f.Y-p.Nodes[1])-phy*(f.X-p.Nodes[0]) < 0 {
				side = -1
			}
			ax += (awayX*0.7 - phy*side*0.9) * fishFleeK * 0.55 * wt
			ay += (awayY*0.7 + phx*side*0.9) * fishFleeK * 0.55 * wt
			qp := math.Sqrt(wt) * 0.7
			if qp > fear {
				fear = qp
			}
			if wt*0.7 > dread {
				dread = wt * 0.7
			}
			if qp > bestQ {
				bestQ = qp
				fleePredDepth = p.Depth
			}
		}

		// Depth-escape direction: away from the dominant hunter's depth.
		// Shallower hunter -> dive deeper; deeper hunter -> rise toward
		// the surface; level attack -> always rise toward the surface.
		fleeDir := int8(0)
		if fear > 0 {
			gap := fleePredDepth - f.Depth
			if gap > fleeDepthEps {
				fleeDir = -1
				f.FleeDir = 0
			} else if gap < -fleeDepthEps {
				fleeDir = 1
				f.FleeDir = 0
			} else {
				f.FleeDir = 1
				fleeDir = f.FleeDir
			}
		} else {
			f.FleeDir = 0
		}

		// Bait feeding: calm fish near a plankton spot gather and mill
		// around it into a tight bait ball. A close hunter (sustained
		// dread) or outright panic (high alarm, e.g. contagion with no
		// hunter in view) interrupts: the pull is skipped, flee and
		// dive drives win outright, and feeders return once they calm
		// down. A mere mid-range startle (reflex fear only) does not
		// spoil lunch.
		feedingDepth := 0.0
		feeding := false
		milling := false
		feedingCloseness := 0.0
		if dread <= feedFearGate && f.Alarm < feedingCalmAlarm {
			if si, sd := w.nearestFeedingSpot(f.X, f.Y, f.Depth); si >= 0 && sd < feedingAttractRadius {
				s := &w.FeedingSpots[si]
				dx := s.X - f.X
				dy := s.Y - f.Y
				d := math.Hypot(dx, dy)
				if d < 1e-3 {
					d = 1e-3
				}
				closeness := 1 - sd/feedingAttractRadius
				pull := feedingPull * (0.35 + 0.65*closeness)
				if d < feedingMillRadius {
					// Settled bait ball: fade the inward pull toward the
					// centre so arrivals brake into an orbit instead of
					// charging through, while the tangential drive holds
					// them circling the spot. The centre fade also kills
					// the singularity kick at d≈0, so fish on top of the
					// spot coast and wander rather than jitter.
					radialK := d / feedingMillRadius
					centreFade := math.Min(1, d/0.5)
					tang := feedingMill * (1.2 - 0.5*d/feedingMillRadius) * centreFade
					ax += dx / d * pull * radialK
					ay += dy / d * pull * radialK
					ax += -dy / d * float64(f.MillDir) * tang
					ay += dx / d * float64(f.MillDir) * tang
					milling = true
				} else {
					// Still travelling: full pull toward the spot.
					ax += dx / d * pull
					ay += dy / d * pull
				}
				feeding = true
				feedingCloseness = closeness
				feedingDepth = s.Depth
			}
		}

		nearSurface := w.nearSurfaceFish(f)

		breaking := nearSurface && w.Time < f.SurfaceUntil

		// Directed vertical bolt: a fresh scare triggers a short
		// full-strength dart (rising prey dashes upward so hunts near
		// the top churn ripples; diving prey pushes downward), then
		// escape continues in-plane instead of pinning max vertical
		// speed for the whole encounter. The dart latches until fear
		// fades, so chronic exposure cannot hold it open. Direct fear
		// wins over a voluntary surface break either way.
		if fear < 0.05 {
			f.BoltSpent = false
		} else if !f.BoltSpent && fear > feedFearGate {
			f.BoltSpent = true
			f.BoltUntil = w.Time + boltDuration
		}
		bolting := w.Time < f.BoltUntil
		if bolting && fleeDir > 0 {
			ay -= fishFleeK * 0.8 * fear
		} else if bolting && fleeDir < 0 {
			ay += fishFleeK * 0.8 * fear
		} else if breaking && fear == 0 {
			// Break the surface: climb to the top rows and hold there.
			surfTarget := surfaceBand(w.Height) - 1.0
			if surfTarget < 1.5 {
				surfTarget = 1.5
			}
			if f.Y > surfTarget {
				ay -= fishFleeK * 0.5 * math.Min(1, (f.Y-surfTarget)/4)
			}
			ay += (surfTarget - f.Y) * 0.5
		}

		// Grouping: individuals and small groups (below the species
		// minimum school size) always steer toward the mean position of
		// their own size class, so stragglers migrate to the closest
		// school no matter how far it is. Full schools of the same size
		// within mergeRange pull together into a single larger school,
		// while other sizes stay apart via the foreign-separation above.
		// The pull is constant (not fear-gated): flee accelerations dwarf
		// it while a predator is close, so escape still wins and the
		// school reassembles after. Close in, the pull fades with the
		// mean offset so the school does not jitter around its centre.
		// Both pulls also match the mates' mean velocity (damped
		// rendezvous): at cruise pace an undamped spring slingshots
		// schools past each other instead of merging them.
		minSize := sp.SchoolSize[0]
		if n+1 < minSize {
			if mx, my, mvx, mvy, mates := w.sameSpeciesMean(i, 0); mates > 0 {
				if d := math.Hypot(mx, my); d > 1e-3 {
					k := 7.0 * math.Min(1, d/2)
					ax += mx/d*k + mvx*groupDampK
					ay += my/d*k + mvy*groupDampK
				}
			}
		} else {
			const mergeRange = 20.0
			if mx, my, mvx, mvy, mates := w.sameSpeciesMean(i, mergeRange); mates > 0 {
				if d := math.Hypot(mx, my); d > 1e-3 {
					k := 3.0 * math.Min(1, d/2)
					ax += mx/d*k + mvx*groupDampK
					ay += my/d*k + mvy*groupDampK
				}
			}
		}

		// Alarm: sustained close threat (dread) sets it, neighbours
		// pass it along (a quarter weaker each hop, so an edge sighting
		// ripples instead of saturating the school), and it fades on
		// its own. Reflex-only startles dodge without panicking.
		alarm := max(f.Alarm*calmDecay, dread)
		if relayed := neighbourAlarm * 0.75; relayed > alarm {
			alarm += (relayed - alarm) * contagion
		}
		f.Alarm = alarm

		// Depth: ambient bob around a per-fish baseline plus a directed
		// alarm escape. Spring-damper toward the target keeps motion
		// smooth and bounded. Each fish holds its own staggered band
		// (fixed Burst offset), so the school spreads through the
		// water instead of stacking on one layer. Rising fish hold
		// shallow so their upward dash rings at the surface. The
		// shared drift term (same for the whole size class) carries
		// the school up and down together; the phased bob stays per
		// fish so neighbours still shimmer individually.
		baseline := 0.775 + fishDepthSpread*math.Sin(f.Burst*2) +
			fishDepthAmp*math.Sin(w.Time*0.5+f.Phase) +
			0.08*math.Sin(w.Time*0.13+float64(f.Species)*2.1)
		target := baseline - fishDepthDive*f.Alarm
		if breaking && fear == 0 {
			target = fishDepthMax
		} else if feeding && fear == 0 && feedingDepth > target {
			// Hold shallow at the plankton while nibbling.
			target = feedingDepth
		} else if fear > 0 && fleeDir > 0 {
			// Hunter deeper: rise toward the surface.
			target = baseline + (fishDepthMax-baseline)*0.9*f.Alarm
		}
		// Hunter shallower or contagion-only alarm keeps the default
		// dive target above: flee deeper away from the hunter.
		f.Dz += ((target-f.Depth)*fishDepthSpring - f.Dz*fishDepthDamp) * dt
		f.Dz += (mathx.Rand(0, 1) - 0.5) * 0.6 * dt
		f.Dz = mathx.Clamp(f.Dz, -fishDepthMaxV, fishDepthMaxV)
		f.Depth += f.Dz * dt
		if f.Depth < fishDepthMin {
			f.Depth = fishDepthMin
			if f.Dz < 0 {
				f.Dz = 0
			}
		} else if f.Depth > fishDepthMax {
			f.Depth = fishDepthMax
			if f.Dz > 0 {
				f.Dz = 0
			}
		}

		// Leaky soft walls: strong enough to turn calm cruising,
		// roam, and wander back, but flight or a hard chase can
		// overpower them, so exits happen by movement alone instead
		// of a deliberate exit drive.
		{
			const m = 36 / pxCell
			if f.X < m {
				ax += (m - f.X) * 7
			} else if f.X > W-m {
				ax -= (f.X - (W - m)) * 7
			}
			if f.Y < m {
				ay += (m - f.Y) * 7
			} else if f.Y > H-m {
				ay -= (f.Y - (H - m)) * 7
			}
		}

		// Ambient current and a little individual restlessness. The
		// current's vertical share stays below the horizontal one so
		// sustained vertical advection cannot park a calm school on
		// the floor or ceiling, while vertical eddies still work the
		// roam loops below.
		mathx.Current(f.X*pxCell, f.Y*pxCell, w.Time, &cur)
		ax += cur.X * 0.5 / pxCell
		ay += cur.Y * 0.2 / pxCell
		// Roam drive: a tank-scale field holding each direction for
		// tens of seconds, offset per size class so size classes tour
		// independently. Neighbours feel nearly the same push, so the
		// school travels as one instead of each fish's jitter
		// cancelling out in the average. It loops through the water
		// column as well as across the tank, and fades with alarm so
		// fleeing still wins outright.
		roamK := 1 - 0.8*f.Alarm
		// Settled feeders drop the travel drive so the roam field cannot
		// drag the bait ball off the spot; travellers keep most of it so
		// distant schools still tour into range.
		if milling {
			roamK *= 0.15
		} else if feeding {
			roamK *= 1 - 0.5*feedingCloseness
		}
		if roamK > 0 {
			so := float64(f.Species)
			ax += math.Sin(f.Y*0.05+w.Time*0.11+so*2.1) * roamAmp * roamK
			ay += math.Sin(f.X*0.05-w.Time*0.13+so*1.3) * roamAmp * roamVertical * roamK
		}
		// Individual restlessness as smooth correlated wander, not
		// per-step white jitter: an Ornstein-Uhlenbeck-like walk over a
		// heading offset, so calm fish meander instead of twitching.
		// The sweep (±1.3 rad at full deflection) stays lateral, and
		// panic straightens the swim: fleeing fish commit to the dart
		// instead of wiggling through it.
		f.Wander = mathx.Clamp(f.Wander+(mathx.Rand(0, 1)-0.5)*dt*5, -1, 1)
		if wanderMag := sp.Jitter / pxCell * 0.6 * (1 - 0.6*f.Alarm); wanderMag > 0 {
			wa := math.Atan2(f.Vy, f.Vx) + f.Wander*1.3
			ax += math.Cos(wa) * wanderMag
			ay += math.Sin(wa) * wanderMag
		}
		// Swim mostly level, except when bolting from danger: the
		// drag is soft enough to let roam-driven vertical tours
		// persist instead of killing every dive on the spot.
		ay -= f.Vy * sp.VerticalDrag * (1 - 0.7*f.Alarm)
		// Mid-water spring: calm cruisers drift back toward mid-water,
		// so currents and the roam drive can't park a school on the
		// floor (or ceiling) to slide along the edge. It stays far
		// below the feeding pull, so bait balls still gather at surface
		// spots, and it skips fleeing, surface breaks and feeding.
		if fear == 0 && !breaking && !feeding {
			if k := midWaterK * (1 - f.Alarm); k > 0 {
				ay += (H*0.5 - f.Y) * k
			}
		}

		// Relax toward a preferred speed that rises with panic. Subtle
		// chase bursts: ~1.4x at full alarm so flight reads clearly
		// without turning the whole school into rockets. Calm fish add
		// burst-and-glide: a slow per-fish surge (±13% at ~0.45Hz,
		// desynced by Burst) that reads as tail-beat bursts between
		// glides instead of a motor hum. It averages to zero, so the
		// school mean still sits at cruise; panic drops it for a steady
		// sprint.
		preferred := sp.Cruise / pxCell * (1 + 0.4*f.Alarm)
		preferred *= 1 + 0.13*(1-f.Alarm)*math.Sin(w.Time*2.8+f.Burst)
		// Milling feeders slow to a nibbling pace so the tangential drive
		// can hold a tight orbit; at full cruise the turning circle is too
		// wide and the ball sails straight through the spot.
		if milling {
			preferred *= 0.45
		}
		ax += (preferred - speed) * hx * 1.6
		ay += (preferred - speed) * hy * 1.6

		// Heading + speed integration with natural limits, mirroring
		// the schooling-predator steering: a speed-dependent turning
		// circle (tighter when panicked, wider at speed) carves turns
		// instead of snapping, and a longitudinal acceleration cap
		// turns kicks into surges instead of teleports.
		before := math.Atan2(f.Vy, f.Vx)
		desVx := f.Vx + ax*dt
		desVy := f.Vy + ay*dt
		desSpeed := math.Hypot(desVx, desVy)
		desHeading := before
		if desSpeed > 1e-3 {
			desHeading = math.Atan2(desVy, desVx)
		}
		cruise := sp.Cruise / pxCell
		agility := (2.6 + 5.0*f.Alarm) / (1 + 0.35*speed/cruise)
		turned := mathx.Clamp(mathx.AngleDiff(before, desHeading), -agility*dt, agility*dt)
		heading := before + turned
		load := fear
		if f.Alarm > load {
			load = f.Alarm
		}
		accelCap := 16.0 + 55.0*load
		newSpeed := desSpeed
		if dv := desSpeed - speed; dv > accelCap*dt {
			newSpeed = speed + accelCap*dt
		} else if dv < -accelCap*dt {
			newSpeed = speed - accelCap*dt
		}
		maxSpeed := sp.MaxSpeed / pxCell * (1 + 0.6*f.Alarm)
		minSpeed := sp.MinSpeed / pxCell
		newSpeed = mathx.Clamp(newSpeed, 0, maxSpeed)
		if newSpeed < minSpeed {
			// Ease back up to the cruise floor instead of
			// teleporting: slow fish ramp, they don't pop.
			if ramped := speed + accelCap*dt; ramped < minSpeed {
				if newSpeed < ramped {
					newSpeed = ramped
				}
			} else if newSpeed < minSpeed {
				newSpeed = minSpeed
			}
		}
		f.Vx = math.Cos(heading) * newSpeed
		f.Vy = math.Sin(heading) * newSpeed
		f.Turn += (mathx.AngleDiff(before, math.Atan2(f.Vy, f.Vx))/dt - f.Turn) * min(1, dt*8)
		f.Phase += dt * (7 + math.Hypot(f.Vx, f.Vy)*1.44)
		f.X = mathx.Clamp(f.X+f.Vx*dt, -6, W+6)
		f.Y = mathx.Clamp(f.Y+f.Vy*dt, -6, H+6)
	}
}

// stepSharks ports the shark update: wander + pursuit/separation steering
// with a speed-dependent turning circle, a follow-the-leader spine, hunger
// that builds over time, and feeding that drops hunger and emits eat events.
func (w *World) stepSharks(dt float64) {
	W := float64(w.Width)
	H := float64(w.Height)
	minSide := math.Min(W, H)
	cxW, cyW := W/2, H/2

	w.updateSharkFrenzy(dt)

	for si := range w.Sharks {
		sh := &w.Sharks[si]
		L := sh.Length
		nodes := sh.Nodes
		hx, hy := nodes[0], nodes[1]

		sh.Hunger = min(1, sh.Hunger+dt*0.015)

		// Momentum.
		dx := math.Cos(sh.Heading)
		dy := math.Sin(sh.Heading)

		// Wander: a slowly drifting offset on a circle ahead of the snout.
		sh.Wander = mathx.Clamp(sh.Wander+(mathx.Rand(0, 1)-0.5)*dt*3.2, -1.1, 1.1)
		dx += math.Cos(sh.Heading+sh.Wander) * 0.45
		dy += math.Sin(sh.Heading+sh.Wander) * 0.45

		// Hunt: fish first, schooling predators opportunistically.
		// Frenzying sharks hold the lock longer and ignore hunger.
		frenzy := w.inSharkFrenzy(sh)
		if w.Time >= sh.RetargetAt {
			sh.Target = w.findPrey(sh)
			sh.TargetPred = w.findPredatorPrey(sh)
			if frenzy {
				sh.RetargetAt = w.Time + frenzyRetargetAfter
			} else {
				sh.RetargetAt = w.Time + 0.45
			}
		}
		t := sh.Target
		if t != nil && !species.IsLarge(t.Species) {
			t = nil
			sh.Target = nil
		}
		huntFish := t != nil && t.Alive && (sh.Hunger > 0.3 || frenzy)
		huntPred := sh.TargetPred != nil && sh.TargetPred.Alive && (sh.Hunger > 0.45 || frenzy)
		breaching := false
		if !huntFish && !huntPred {
			if w.Time >= sh.BreachAt {
				sh.BreachAt = w.Time + mathx.Rand(8, 18)
				sh.BreachUntil = w.Time + mathx.Rand(1.4, 2.6)
			}
			breaching = w.Time < sh.BreachUntil
		}
		hunting, closing := false, 0.0
		if huntFish {
			// Pursuit: aim at where the fish is about to be.
			// Steering stays planar; depth closes via Dz below.
			tx := t.X + t.Vx*0.35 - hx
			ty := t.Y + t.Vy*0.35 - hy
			tz := (t.Depth - sh.Depth) * depthScale
			d := math.Hypot(tx, math.Hypot(ty, tz))
			if d == 0 {
				d = 1
			}
			planar := math.Hypot(tx, ty)
			if planar < 1e-3 {
				planar = 1e-3
			}
			wt := 1.4 + 2.6*sh.Hunger
			if frenzy {
				wt = 1.4 + 2.6*1.0 + frenzySteerBoost
			}
			dx += tx / planar * wt
			dy += ty / planar * wt
			hunting = true
			closing = mathx.Clamp(1-d/(L*0.9), 0, 1)
		} else if huntPred {
			pp := sh.TargetPred
			// A tuna is a bigger meal worth turning for: lead its snout.
			tx := pp.Nodes[0] + math.Cos(pp.Heading)*pp.Speed*0.4 - hx
			ty := pp.Nodes[1] + math.Sin(pp.Heading)*pp.Speed*0.4 - hy
			tz := (pp.Depth - sh.Depth) * depthScale
			d := math.Hypot(tx, math.Hypot(ty, tz))
			if d == 0 {
				d = 1
			}
			planar := math.Hypot(tx, ty)
			if planar < 1e-3 {
				planar = 1e-3
			}
			wt := 1.2 + 2.2*sh.Hunger
			if frenzy {
				wt = 1.2 + 2.2*1.0 + frenzySteerBoost
			}
			dx += tx / planar * wt
			dy += ty / planar * wt
			hunting = true
			closing = mathx.Clamp(1-d/(L*0.9), 0, 1)
		} else if breaching {
			// Surface break: climb to the top rows, then level out and
			// cruise along the surface for a few seconds.
			surfTarget := surfaceBand(w.Height) - 1.0
			if surfTarget < 1.5 {
				surfTarget = 1.5
			}
			if hy > surfTarget {
				dy -= 2.5 * math.Min(1, (hy-surfTarget)/6)
			}
			dy += (surfTarget - hy) * 0.2
		} else {
			// Patrol: a lazy circuit around the headline.
			rx := (hx - cxW) / (W / minSide)
			ry := hy - cyW
			r := math.Hypot(rx, ry)
			if r == 0 {
				r = 1
			}
			radius := sh.OrbitRadius * (1 + 0.15*math.Sin(w.Time*0.05+float64(sh.ID)))
			radial := mathx.Clamp((radius-r)/radius, -1, 1)
			dx += ((-ry/r)*float64(sh.OrbitDir) + (rx/r)*radial*0.9) * 0.55
			dy += ((rx/r)*float64(sh.OrbitDir) + (ry/r)*radial*0.9) * 0.55
		}

		// Give other sharks room. Depth counts, so sharks at different
		// depths can pass closer in the 2D plane.
		for oi := range w.Sharks {
			if oi == si {
				continue
			}
			other := &w.Sharks[oi]
			dz := (sh.Depth - other.Depth) * depthScale
			for k := 0; k < SHARK_NODES; k += 4 {
				ox := hx - other.Nodes[k*2]
				oy := hy - other.Nodes[k*2+1]
				d := math.Hypot(ox, math.Hypot(oy, dz))
				rng := (L + other.Length) * 0.45
				if d < rng && d > 1e-3 {
					wt := (1 - d/rng) * 3
					planar := math.Hypot(ox, oy)
					if planar < 1e-3 {
						continue
					}
					dx += ox / planar * wt
					dy += oy / planar * wt
				}
			}
		}

		// Keep the headline mostly clear: a soft elliptical no-swim zone.
		ex := (hx - cxW) / (W * 0.3)
		ey := (hy - cyW) / (H * 0.16)
		e2 := ex*ex + ey*ey
		if e2 < 1 {
			e := math.Sqrt(e2)
			if e == 0 {
				e = 1e-3
			}
			wt := (1 - e)
			if !hunting {
				wt *= 2.5
			}
			dx += ex / e * wt
			dy += ey / e * wt
		}

		// Leaky turn-back: strong enough to turn calm patrols back,
		// but a hard pursuit or momentum can overcome it, so sharks
		// swim out by movement alone.
		{
			over := math.Max(math.Max(-hx, hx-W), math.Max(-hy, hy-H))
			if over > -L*0.05 {
				wt := mathx.Clamp((over+L*0.05)/(L*0.35), 0, 1) * 3.5
				toX, toY := cxW-hx, cyW-hy
				d := math.Hypot(toX, toY)
				if d == 0 {
					d = 1
				}
				dx += toX / d * wt
				dy += toY / d * wt
			}
		}

		desired := math.Atan2(dy, dx)
		// Sharks can't pivot on the spot: the turning circle grows with speed.
		agility := sh.Speed / (L * 0.22)
		if hunting {
			agility = min(agility, 1.7+closing)
		} else {
			agility = min(agility, 0.8)
		}
		maxTurn := agility * dt
		turned := mathx.Clamp(mathx.AngleDiff(sh.Heading, desired), -maxTurn, maxTurn)
		sh.Heading += turned
		sh.Turn += (turned/dt - sh.Turn) * min(1, dt*3)

		// 1.5x pace to match the faster fish, keeping hunts closing.
		cruise := 45/pxCell + L*0.06
		targetSpeed := cruise
		if hunting {
			// Hunting burst: ~1.6-2.4x cruise, clearly faster than
			// the ~1.6x fleeing fish so hunts close down.
			targetSpeed = cruise * (1.6 + closing*0.8)
			if frenzy {
				targetSpeed *= frenzySpeedBoost
			}
		}
		sh.Speed += (targetSpeed - sh.Speed) * min(1, dt*1.1)
		sh.Phase += dt * (2 + sh.Speed/2)

		// Depth: porpoise on patrol, dive toward prey when hunting.
		// Baseline is the size-derived seed depth so big sharks stay near.
		var depthTarget float64
		if breaching {
			depthTarget = sharkDepthMax
		} else if hunting && t != nil && t.Alive {
			depthTarget = mathx.Clamp(t.Depth, sharkDepthMin, sharkDepthMax)
		} else if hunting && sh.TargetPred != nil && sh.TargetPred.Alive {
			depthTarget = mathx.Clamp(sh.TargetPred.Depth, sharkDepthMin, sharkDepthMax)
		} else {
			base := mathx.Clamp(sh.Size, sharkDepthMin, sharkDepthMax)
			if base <= 0 {
				base = 0.7
			}
			depthTarget = base + sharkPorpoiseAmp*math.Sin(w.Time*sharkPorpoiseFrq+sh.Phase*0.5)
		}
		sh.Dz += ((depthTarget-sh.Depth)*sharkDepthSpring - sh.Dz*sharkDepthDamp) * dt
		sh.Dz = mathx.Clamp(sh.Dz, -sharkDepthMaxV, sharkDepthMaxV)
		sh.Depth += sh.Dz * dt
		if sh.Depth < sharkDepthMin {
			sh.Depth = sharkDepthMin
			if sh.Dz < 0 {
				sh.Dz = 0
			}
		} else if sh.Depth > sharkDepthMax {
			sh.Depth = sharkDepthMax
			if sh.Dz > 0 {
				sh.Dz = 0
			}
		}

		nodes[0] += math.Cos(sh.Heading) * sh.Speed * dt
		nodes[1] += math.Sin(sh.Heading) * sh.Speed * dt

		// Follow-the-leader spine, with a stiffness limit per joint so the
		// body bends into a C at most and never folds back on itself.
		seg := L / float64(SHARK_NODES-1)
		const maxBend = 0.16
		prevAngle := sh.Heading + math.Pi
		for k := 1; k < SHARK_NODES; k++ {
			px := nodes[(k-1)*2]
			py := nodes[(k-1)*2+1]
			angle := math.Atan2(nodes[k*2+1]-py, nodes[k*2]-px)
			angle = prevAngle + mathx.Clamp(mathx.AngleDiff(prevAngle, angle), -maxBend, maxBend)
			nodes[k*2] = px + math.Cos(angle)*seg
			nodes[k*2+1] = py + math.Sin(angle)*seg
			prevAngle = angle
		}

		// Feed. The catch needs vertical alignment too: a fish at the
		// mouth x/y but a full depth layer away escapes. Sharks also take
		// schooling predators: a bigger meal with a bigger hunger drop.
		// Frenzying sharks feed even when satiated.
		if sh.Hunger > 0.15 || frenzy {
			mouthR := L*0.05 + 8/pxCell
			mx, my := nodes[0], nodes[1]
			for i := range w.Fish {
				f := &w.Fish[i]
				if !f.Alive {
					continue
				}
				// Small/medium fish are immune: the mouth passes over them.
				if !species.IsLarge(f.Species) {
					continue
				}
				dx := f.X - mx
				dy := f.Y - my
				dz := (f.Depth - sh.Depth) * depthScale
				if dx*dx+dy*dy+dz*dz < mouthR*mouthR {
					f.Alive = false
					f.RespawnAt = w.respawnDeadline(3, 8)
					sh.Hunger = math.Max(0, sh.Hunger-0.16)
					w.Events = append(w.Events, WorldEvent{Kind: "eat", X: f.X, Y: f.Y})
				}
			}
			for i := range w.Predators {
				p := &w.Predators[i]
				if !p.Alive {
					continue
				}
				ox := p.Nodes[0] - mx
				oy := p.Nodes[1] - my
				oz := (p.Depth - sh.Depth) * depthScale
				if ox*ox+oy*oy+oz*oz < mouthR*mouthR {
					p.Alive = false
					p.RespawnAt = w.respawnDeadline(5, 10)
					p.Target = nil
					sh.Hunger = math.Max(0, sh.Hunger-0.3)
					w.Events = append(w.Events, WorldEvent{Kind: "eat", X: p.Nodes[0], Y: p.Nodes[1]})
					break
				}
			}
		}
	}
}

// findPrey ports the TS picker: prey in the forward cone, preferring close
// fish in sparse buckets (stragglers) over fish buried in a dense school.
func (w *World) findPrey(sh *Shark) *Fish {
	rng := sh.Length*0.9 + 160/pxCell
	hx, hy := sh.Nodes[0], sh.Nodes[1]
	fx, fy := math.Cos(sh.Heading), math.Sin(sh.Heading)
	var best *Fish
	bestScore := math.Inf(1)
	currentScore := math.Inf(1)

	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive {
			continue
		}
		// Sharks only eat large fish; small/medium species are ignored
		// even when they swim through the forward cone.
		if !species.IsLarge(f.Species) {
			continue
		}
		dx := f.X - hx
		dy := f.Y - hy
		dz := (f.Depth - sh.Depth) * depthScale
		d := math.Hypot(dx, math.Hypot(dy, dz))
		if d > rng || d < 1e-3 {
			continue
		}
		planar := math.Hypot(dx, dy)
		if planar < 1e-3 {
			planar = 1e-3
		}
		cos := (dx*fx + dy*fy) / planar
		if cos < 0.2 {
			continue
		}
		crowd := w.bucketCnt[w.bucketOf(f.X, f.Y)]
		score := d * (1 + (1-cos)*0.8) * (1 + float64(crowd)*0.07)
		if score < bestScore {
			bestScore = score
			best = f
		}
		if f == sh.Target {
			currentScore = score
		}
	}
	// Stick with the current target unless something is clearly better.
	// Frenzying sharks hold the lock longer.
	stick := 1.3
	if w.Time < sh.FrenzyUntil {
		stick = 1.6
	}
	if sh.Target != nil && currentScore < bestScore*stick {
		return sh.Target
	}
	return best
}

// respawnDeadline returns when a dead creature returns, using the default
// caller timing in all modes.
func (w *World) respawnDeadline(minDelay, maxDelay float64) float64 {
	return w.Time + mathx.Rand(minDelay, maxDelay)
}

// respawn revives due fish as a new school swimming in from a random edge,
// keeping the population steady. Ports respawn: edge offsets (20px) and
// cruise speeds (px/s) are rescaled to cells; the wave gating (a trickle
// under 6 waits 9s) is unit-free and copied verbatim. Natural mode keeps
// the same timing and additionally caps revivals at the area maximum.
func (w *World) respawn() {
	var due []*Fish
	for i := range w.Fish {
		f := &w.Fish[i]
		if !f.Alive && f.RespawnAt <= w.Time {
			due = append(due, f)
		}
	}
	if len(due) == 0 {
		return
	}
	if w.Natural {
		room := w.maxFish() - w.liveFishCount()
		if room <= 0 {
			return
		}
		if len(due) > room {
			due = due[:room]
		}
	}
	if len(due) < 6 && w.Time-w.lastSpawn < 9 {
		return
	}
	w.lastSpawn = w.Time

	W := float64(w.Width)
	H := float64(w.Height)
	// src/world.ts offsets the spawn 20px off the edge; cells are ~12px.
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
	spec := species.PickSpecies()
	sp := species.SPECIES[spec]
	var millDir int8 = 1
	if mathx.Rand(0, 1) < 0.5 {
		millDir = -1
	}
	// No maximum school size: the whole due batch revives as one school,
	// so schools grow without a cap and only break when chased.
	n := len(due)
	for _, f := range due[:n] {
		f.Alive = true
		f.Species = spec
		f.Alarm = 0
		f.Turn = 0
		f.MillDir = millDir
		f.X = x + mathx.Rand(-1, 1)*w.perception
		f.Y = y + mathx.Rand(-1, 1)*w.perception
		f.Vx = math.Cos(inward) * sp.Cruise / pxCell * 1.4
		f.Vy = math.Sin(inward) * sp.Cruise / pxCell * 1.4
		f.Depth = mathx.Rand(0.55, 1)
		f.Dz = 0
		f.SurfaceRollAt = w.Time + mathx.Rand(2, 6)
		f.SurfaceUntil = 0
		f.Wander = mathx.Rand(-1, 1) * 0.2
		f.Burst = mathx.Rand(0, mathx.TAU)
	}
}
