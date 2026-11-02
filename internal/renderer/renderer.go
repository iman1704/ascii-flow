package renderer

import (
	"math"
	"strings"

	"github.com/iman1704/ascii-flow/internal/mathx"
	"github.com/iman1704/ascii-flow/internal/species"
	"github.com/iman1704/ascii-flow/internal/world"
)

// pxCell mirrors world's reference pixel width of one terminal cell.
// It rescales px/s speeds (shark swim factor) into cell units.
const pxCell = 12.0

// cellAspect mirrors the canvas ch = cw*1.45 row-height ratio. One terminal
// row is ~1.45x one col, so pixel-diagonal conversions scale rows by this.
const cellAspect = 1.45

// fishAlphas is the simulated opacity ramp: bucket 0 is far/ghostly,
// bucket 4 is near/solid. The spread is deliberately dramatic so depth
// reads at a glance even next to alarm glints.
var fishAlphas = [5]float64{0.08, 0.28, 0.52, 0.78, 1.0}

// fishBucketFG is the dramatic neutral fade: far buckets are dim grey,
// near buckets are near-white. Brightness carries depth with only a
// whisper of cool tint, so size+fade dominate.
var fishBucketFG = [5][3]int{
	{45, 50, 60},
	{75, 85, 100},
	{125, 135, 155},
	{180, 190, 210},
	{235, 242, 255},
}

// Fish depth bounds mirror world's fishDepthMin/Max: the full bucket ramp
// spans this range so a bottom-layer fish is always ghostly.
const (
	fishDepthMin = 0.35
	fishDepthMax = 1.0
)

// Depth perspective: far creatures render smaller/dimmer and lift toward
// the horizon; near creatures sit lower and pop forward. Vertical drift
// plus wake motion makes depth movement unmistakable.
const (
	fishParallaxRows = 2.0
	sharkLiftRows    = 2.5
)

// Surface ripples expand only from activity near the top rows; deeper
// movement stays in the wake field.
const (
	rippleSpeed = 6.0
	rippleWidth = 1.2
	maxRipples  = 24
	// rippleBaseRadius is the max ring radius at scale 1. Rings grow
	// to base*scale, so small rings also die young: lifetime is
	// radius/speed.
	rippleBaseRadius = 6.0
	// feedRippleScale/Strength size the school ring around bait-fish
	// feeding schools: wider and stronger than a catch ripple (1.0 /
	// 0.7) so the ring reads around the school instead of blending
	// into wake churn. Per-fish feeders stay ring-free (see
	// depositWake), so this is the school's only ripple voice.
	feedRippleScale    = 1.4
	feedRippleStrength = 0.9
	// Rings need upward motion toward the surface to form.
	upwardMinSpeed = 0.5
	// Emission periods: every creature in the surface band rings on
	// its own staggered slot, so rings trace back to a creature.
	rippleFishInterval   = 1.6
	rippleHunterInterval = 1.1
)

type ripple struct {
	x, y     float64
	birth    float64
	strength float64
	// scale sizes the ring: bigger creatures and faster action
	// expand wider rings.
	scale float64
}

// Renderer holds the cell grid. Ports AsciiRenderer's cols/rows plus its
// wake (energy/heat), shark raster and fish-bucket buffers.
type Renderer struct {
	cols, rows int
	ripples    []ripple

	dotPhase []float64
	dotFreq  []float64
	energy   []float64
	tmp      []float64
	heat     []float64

	sharkFlag   []uint8
	sharkShade  []float64
	sharkKey    []uint32
	sharkDepth  []float64
	sharkKind   []uint8
	sharkShadow []uint8
	spine       []float64
	arc         []float64
	arcTotal    float64
	shape       *SharkShape
	sharkID     int
	finTipLeft  float64
	finTipRight float64

	hitFlag  uint8
	hitShade float64
	hitS     float64
	hitV     float64
	hitDist  float64

	fishStamp   []uint32
	frameID     uint32
	fishBuckets [][]int
	fishGlyph   []rune

	cells []cell
}

type cell struct {
	ch    rune
	fg    [3]int
	hasFg bool
	bg    [3]int
	hasBg bool
}

// New returns an empty renderer; call Resize before Render.
func New() *Renderer {
	return &Renderer{shape: shapes["classic"]}
}

// Resize sets the grid in cells. Ports AsciiRenderer.resize, minus the
// pixel/dpr/font-metric inputs: term.GetSize already yields cells.
// Buffers are rebuilt whenever either dimension changes, not just when the
// area changes: an 80x24 grid and a 96x20 grid hold the same cell count but
// index it differently, so reusing the old layout would scatter the wake,
// heat and shark layers onto the wrong cells.
func (r *Renderer) Resize(cols, rows int) {
	if cols == r.cols && rows == r.rows {
		return
	}
	r.cols, r.rows = cols, rows
	n := cols * rows
	r.dotPhase = make([]float64, n)
	r.dotFreq = make([]float64, n)
	r.energy = make([]float64, n)
	r.tmp = make([]float64, n)
	r.heat = make([]float64, n)
	r.sharkFlag = make([]uint8, n)
	r.sharkShade = make([]float64, n)
	r.sharkKey = make([]uint32, n)
	r.sharkDepth = make([]float64, n)
	r.sharkKind = make([]uint8, n)
	r.sharkShadow = make([]uint8, n)
	r.fishStamp = make([]uint32, n)
	r.fishGlyph = make([]rune, n)
	r.cells = make([]cell, n)
	r.spine = make([]float64, world.SHARK_NODES*2)
	r.arc = make([]float64, world.SHARK_NODES)
	r.fishBuckets = make([][]int, len(fishAlphas))
	r.ripples = r.ripples[:0]
	for i := 0; i < n; i++ {
		col := i % cols
		row := i / cols
		r.dotPhase[i] = mathx.Hash3(col, row, 23) * math.Pi * 2
		r.dotFreq[i] = 0.3 + mathx.Hash3(col, row, 37)*1.1
	}
}

// Render builds one fullscreen frame: cursor-home prefix + rows of runes
// with ANSI SGR colours. Ports AsciiRenderer.render.
func (r *Renderer) Render(w *world.World, dt float64) string {
	if r.cols <= 0 || r.rows <= 0 {
		return ""
	}
	if dt <= 0 {
		dt = 1.0 / 30
	}
	t := w.Time

	r.consumeEvents(w, t)
	r.rasterizeSharks(w.Sharks, t)
	r.rasterizePredators(w.Predators, t)
	r.depositWake(w, t, dt)
	r.diffuse(dt)
	r.pruneRipples(t)

	r.markFish(w)
	r.compose(t)

	return r.emit()
}

func (r *Renderer) cellIndex(x, y float64) int {
	c := int(math.Floor(x))
	rw := int(math.Floor(y))
	if c < 0 || rw < 0 || c >= r.cols || rw >= r.rows {
		return -1
	}
	return rw*r.cols + c
}

func (r *Renderer) stamp(field []float64, x, y float64, radius int, amount float64) {
	c0 := int(math.Floor(x))
	r0 := int(math.Floor(y))
	for rr := r0 - radius; rr <= r0+radius; rr++ {
		for c := c0 - radius; c <= c0+radius; c++ {
			if c < 0 || rr < 0 || c >= r.cols || rr >= r.rows {
				continue
			}
			d := math.Hypot(float64(c-c0), float64(rr-r0)*1.4)
			if d > float64(radius)+0.5 {
				continue
			}
			i := rr*r.cols + c
			field[i] = math.Min(1.6, field[i]+amount*(1-d/(float64(radius)+1)))
		}
	}
}

func (r *Renderer) surfaceBand() float64 {
	band := float64(r.rows) * 0.14
	if band < 3 {
		band = 3
	}
	if band > 6 {
		band = 6
	}
	return band
}

func (r *Renderer) spawnRipple(x, y, birth, strength, scale float64) {
	rp := ripple{x: x, y: y, birth: birth, strength: strength, scale: mathx.Clamp(scale, 0.3, 2.8)}
	if len(r.ripples) >= maxRipples {
		oldest := 0
		for i := range r.ripples {
			if r.ripples[i].birth < r.ripples[oldest].birth {
				oldest = i
			}
		}
		r.ripples[oldest] = rp
		return
	}
	r.ripples = append(r.ripples, rp)
}

// fishRippleScale sizes a ring by body size and action: large fish ring
// bigger than small ones, and fast or alarmed fish ring bigger than
// cruising ones.
func fishRippleScale(f *world.Fish, speed float64) float64 {
	base := 0.65
	cruise := 3.0
	if f.Species >= 0 && f.Species < len(species.SPECIES) {
		switch species.SPECIES[f.Species].Size {
		case species.SizeLarge:
			base = 0.85
		case species.SizeMedium:
			base = 0.65
		default:
			base = 0.45
		}
		cruise = species.SPECIES[f.Species].Cruise / pxCell
	}
	action := mathx.Clamp(speed/cruise, 0, 2)
	return base * (0.8 + 0.45*action + 0.25*f.Alarm)
}

// hunterRippleScale sizes shark and schooling-predator rings by body
// length and speed: a hunting burst rings wider than a patrol.
func hunterRippleScale(length, speed float64) float64 {
	return (0.6 + length/30) * (0.85 + 0.75*mathx.Clamp(speed/7.5, 0, 1.2))
}

// rippleDue reports whether a creature's emission slot turned over this
// frame. Slots are staggered per creature, so emission is periodic and
// attributable, never random.
func rippleDue(t, dt, interval, phase float64) bool {
	if dt <= 0 {
		return false
	}
	return math.Floor(t/interval+phase) != math.Floor((t-dt)/interval+phase)
}

func ripplePhase(idx int) float64 {
	p := float64(idx) * 0.61803398875
	return p - math.Floor(p)
}

// rippleExtent returns a ring's max radius and lifetime from its scale.
func rippleExtent(scale float64) (maxR, life float64) {
	maxR = rippleBaseRadius * scale
	return maxR, maxR / rippleSpeed
}

func (r *Renderer) pruneRipples(t float64) {
	kept := r.ripples[:0]
	for _, rp := range r.ripples {
		_, life := rippleExtent(rp.scale)
		if t-rp.birth < life {
			kept = append(kept, rp)
		}
	}
	r.ripples = kept
}

func (r *Renderer) consumeEvents(w *world.World, t float64) {
	band := r.surfaceBand()
	for _, e := range w.Events {
		if e.Kind == "eat" {
			r.stamp(r.heat, e.X, e.Y, 2, 1)
			r.stamp(r.energy, e.X, e.Y, 3, 0.9)
			// A catch is creature-caused, so it may ring near the
			// surface. Ambient splashes only feed the wake field:
			// rings never appear without a creature behind them.
			if e.Y < band+2 {
				r.spawnRipple(e.X, e.Y, t, 0.7, 1.0)
			}
		} else if e.Kind == "feed" {
			// Bait fish nibbling plankton: stir the water and ring the
			// school if shallow. No heat: nobody was caught.
			r.stamp(r.energy, e.X, e.Y, 4, 0.9)
			if e.Y < band+2 {
				r.spawnRipple(e.X, e.Y, t, feedRippleStrength, feedRippleScale)
			}
		} else {
			r.stamp(r.energy, e.X, e.Y, 6, 1.4)
		}
	}
	// Ports AsciiRenderer.consumeEvents: the renderer owns the queue, so
	// events from every sub-step in a frame accumulate until this render.
	w.Events = w.Events[:0]
}

func (r *Renderer) depositWake(w *world.World, t, dt float64) {
	band := r.surfaceBand()
	if dt <= 0 {
		dt = 1.0 / 30
	}
	spawns := 0
	for idx := range w.Fish {
		f := &w.Fish[idx]
		if !f.Alive {
			continue
		}
		ry := f.Y - (1-f.Depth)*fishParallaxRows
		if i := r.cellIndex(f.X, ry); i >= 0 {
			k := 0.25 + 0.75*f.Depth
			streak := math.Min(0.04, math.Abs(f.Dz)*0.06)
			amt := (0.012+f.Alarm*0.03)*k + streak
			sw := 1 - ry/band
			if sw > 0 {
				if sw > 1 {
					sw = 1
				}
				amt *= 1 + sw*(1.5+f.Alarm*2)
				// Any fish in the band rings on its own slot, calm
				// or fleeing, while moving up toward the surface.
				// A fish on a voluntary surface break rings for its
				// whole break, even when cruising level at the top.
				// Fish calmly milling on a feeding spot stay
				// ring-free here: the school rings as one ring
				// around the ball via feed events instead of one
				// ripple per fish.
				speed := math.Hypot(f.Vx, f.Vy)
				surfacing := f.Vy < -upwardMinSpeed ||
					(w.Time < f.SurfaceUntil && f.Depth > 0.85)
				if spawns < 4 && speed > 0.5 && surfacing && !w.FishMillingOnSpot(idx) &&
					rippleDue(t, dt, rippleFishInterval, ripplePhase(idx)) {
					r.spawnRipple(f.X, ry, t, 0.45+0.5*f.Alarm, fishRippleScale(f, speed))
					spawns++
				}
			}
			r.energy[i] = math.Min(1.6, r.energy[i]+amt)
		}
	}
	for si := range w.Sharks {
		sh := &w.Sharks[si]
		depthC := mathx.Clamp(sh.Depth, 0, 1)
		lift := (1 - depthC) * sharkLiftRows
		k := 0.25 + 0.75*depthC
		streak := math.Min(0.05, math.Abs(sh.Dz)*0.07)
		for j := world.SHARK_NODES - 6; j < world.SHARK_NODES; j++ {
			y := sh.Nodes[j*2+1] - lift
			i := r.cellIndex(sh.Nodes[j*2], y)
			if i >= 0 {
				amt := (0.05+sh.Speed*pxCell*0.0012)*k + streak
				if sw := 1 - y/band; sw > 0 {
					if sw > 1 {
						sw = 1
					}
					amt *= 1 + sw*1.5
				}
				r.energy[i] = math.Min(1.6, r.energy[i]+amt)
			}
		}
		// A shark at the surface rings from its snout on its slot,
		// but only while moving up toward the surface.
		if len(sh.Nodes) >= 2 && spawns < 4 && sh.Speed > 0.5 &&
			math.Sin(sh.Heading)*sh.Speed < -upwardMinSpeed &&
			sh.Nodes[1]-lift < band &&
			rippleDue(t, dt, rippleHunterInterval, ripplePhase(sh.ID)) {
			r.spawnRipple(sh.Nodes[0], sh.Nodes[1]-lift, t, 0.8, hunterRippleScale(sh.Length, sh.Speed))
			spawns++
		}
	}
	for pi := range w.Predators {
		p := &w.Predators[pi]
		if !p.Alive {
			continue
		}
		depthC := mathx.Clamp(p.Depth, 0, 1)
		lift := (1 - depthC) * sharkLiftRows
		k := 0.25 + 0.75*depthC
		for j := world.SHARK_NODES - 4; j < world.SHARK_NODES; j++ {
			y := p.Nodes[j*2+1] - lift
			i := r.cellIndex(p.Nodes[j*2], y)
			if i >= 0 {
				amt := 0.03 * k
				if sw := 1 - y/band; sw > 0 {
					if sw > 1 {
						sw = 1
					}
					amt *= 1 + sw*1.5
				}
				r.energy[i] = math.Min(1.6, r.energy[i]+amt)
			}
		}
		// Schooling predators ring like sharks when at the surface,
		// but only while moving up toward it.
		if len(p.Nodes) >= 2 && spawns < 4 && p.Speed > 0.5 &&
			math.Sin(p.Heading)*p.Speed < -upwardMinSpeed &&
			p.Nodes[1]-lift < band &&
			rippleDue(t, dt, rippleHunterInterval, ripplePhase(p.ID+1000)) {
			r.spawnRipple(p.Nodes[0], p.Nodes[1]-lift, t, 0.7, hunterRippleScale(p.Length, p.Speed))
			spawns++
		}
	}
}

// diffuse blurs + decays the wake so it spreads like ripples and fades.
func (r *Renderer) diffuse(dt float64) {
	cols, rows := r.cols, r.rows
	decay := math.Exp(-dt * 1.5)
	for row := 0; row < rows; row++ {
		for c := 0; c < cols; c++ {
			i := row*cols + c
			l := r.energy[i]
			if c > 0 {
				l = r.energy[i-1]
			}
			rt := r.energy[i]
			if c < cols-1 {
				rt = r.energy[i+1]
			}
			u := r.energy[i]
			if row > 0 {
				u = r.energy[i-cols]
			}
			d := r.energy[i]
			if row < rows-1 {
				d = r.energy[i+cols]
			}
			r.tmp[i] = (r.energy[i]*4 + l + rt + u + d) * 0.125 * decay
		}
	}
	r.energy, r.tmp = r.tmp, r.energy
	heatDecay := math.Exp(-dt * 2.2)
	for i := range r.heat {
		r.heat[i] *= heatDecay
	}
}

// sampleShark classifies one point against the current shark (spine/arc
// already built). Body coordinates: s along the spine (0 snout → 1 tail),
// v across it. Writes hit* fields and returns whether the point is on the
// shark. Ports AsciiRenderer.sampleShark; inputs are in cell units.
func (r *Renderer) sampleShark(px, py, L float64) bool {
	N := world.SHARK_NODES
	best := math.Inf(1)
	bestS := 0.0
	bestSign := 1.0
	bestNx, bestNy := 0.0, 0.0
	for k := 0; k < N-1; k++ {
		ax := r.spine[k*2]
		ay := r.spine[k*2+1]
		sx := r.spine[(k+1)*2] - ax
		sy := r.spine[(k+1)*2+1] - ay
		len2 := sx*sx + sy*sy
		if len2 == 0 {
			len2 = 1
		}
		u := mathx.Clamp(((px-ax)*sx+(py-ay)*sy)/len2, 0, 1)
		qx := ax + sx*u
		qy := ay + sy*u
		d2 := (px-qx)*(px-qx) + (py-qy)*(py-qy)
		if d2 < best {
			best = d2
			length := math.Sqrt(len2)
			bestS = (r.arc[k] + u*length) / r.arcTotal
			if sx*(py-ay)-sy*(px-ax) >= 0 {
				bestSign = 1
			} else {
				bestSign = -1
			}
			bestNx = (-sy / length) * bestSign
			bestNy = (sx / length) * bestSign
		}
	}

	a := math.Sqrt(best)
	s := bestS
	u := s * L
	shape := r.shape
	w := shape.halfWidth(s) * L
	r.hitDist = a
	r.hitS = s
	r.hitV = a * bestSign

	if head := shape.head; head != nil && u < L*(head.sweep+head.depth+head.notch) {
		span := L * head.span
		rr := a / span
		front := L*head.sweep*rr*rr + L*head.notch*math.Max(0, 1-rr/0.14)
		depth := L * head.depth * (1 - 0.2*rr*rr*rr*rr)
		if rr <= 1 && u >= front && u <= front+depth {
			if rr > 0.84 {
				r.hitFlag = flagEye
				r.hitShade = 0
			} else {
				r.hitFlag = flagBody
				shade := 0.62 - 0.2*rr
				if u < front+L*0.012 {
					shade += 0.1
				}
				r.hitShade = shade
			}
			return true
		}
	}

	if a <= w {
		across := 0.0
		if w != 0 {
			across = a / w
		}
		facing := (bestNx*-0.6 + bestNy*-0.8) * across
		shade := 0.58 - across*across*0.42 + facing*0.26
		flag := uint8(flagBody)
		if s > shape.dorsal[0] && s < shape.dorsal[1] && across < 0.3 {
			flag = flagRidge
			shade = 1
		} else if s > shape.secondDorsal[0] && s < shape.secondDorsal[1] && across < 0.4 {
			shade = math.Max(shade, 0.82)
		} else if shape.eye != nil && s > shape.eye[0] && s < shape.eye[1] && across > 0.6 {
			flag = flagEye
			shade = 0
		} else if s > shape.gills[0] && s < shape.gills[1] && across > 0.58 {
			if int(math.Floor((s-shape.gills[0])/(shape.gills[1]-shape.gills[0])*5))%2 == 0 {
				shade *= 0.45
			}
		} else if s < 0.05 {
			shade = math.Max(shade, 0.7)
		} else if r.shape == shapes["zebra"] && s > 0.08 && s < 0.55 &&
			(across < 0.1 || math.Abs(across-0.6) < 0.08) {
			shade = math.Max(shade, 0.8)
		} else if r.marked(s, a*bestSign, L, across) {
			flag = flagMark
		}
		r.hitFlag = flag
		r.hitShade = shade
		return true
	}

	tip := r.finTipRight
	if bestSign < 0 {
		tip = r.finTipLeft
	}
	p0, p1, p2 := shape.pectoral[0], shape.pectoral[1], shape.pectoral[2]
	if inTriangle(u, a, L*p0, 0, L*p1, 0, L*p2, tip) &&
		!(shape.falcate && inTriangle(u, a, L*p1, 0, L*(p1+(p2-p1)*0.33), tip*0.55, L*p2, tip)) {
		if r.marked(s, a*bestSign, L, 0) {
			r.hitFlag = flagMark
		} else {
			r.hitFlag = flagFin
		}
		r.hitShade = 0.38 + 0.22*(1-mathx.Clamp(a/tip, 0, 1))
		return true
	}
	v0, v1, v2, vr := shape.pelvic[0], shape.pelvic[1], shape.pelvic[2], shape.pelvic[3]
	if inTriangle(u, a, L*v0, 0, L*v1, 0, L*v2, L*vr) ||
		(shape.caudal > 0 && inTriangle(u, a, L*0.86, 0, L*0.95, 0, L, L*shape.caudal)) {
		r.hitFlag = flagFin
		r.hitShade = 0.36
		return true
	}
	return false
}

// marked reports whether a body point (s along, v across) falls on a dark
// marking: tiger bars or zebra spots. Ports AsciiRenderer.marked.
func (r *Renderer) marked(s, v, L, across float64) bool {
	if r.shape == shapes["tiger"] {
		if s < 0.2 || s > 0.9 || across > 0.85 {
			return false
		}
		wobble := 0.35 * math.Sin(v/L*25+s*5)
		bar := math.Sin(s*math.Pi*2*9 + wobble)
		return bar > 0.65-0.5*mathx.Smoothstep(0.2, 0.42, s)+0.4*mathx.Smoothstep(0.8, 0.9, s)
	}
	if r.shape == shapes["zebra"] {
		if s < 0.07 {
			return false
		}
		gu := (s * L) / (L * 0.06)
		gv := v / (L * 0.045)
		cu := math.Floor(gu)
		cv := math.Floor(gv)
		jx := mathx.Hash3(int(cu), int(cv), r.sharkID)*0.5 + 0.25
		jy := mathx.Hash3(int(cu), int(cv), r.sharkID+11)*0.5 + 0.25
		du := gu - cu - jx
		dv := gv - cv - jy
		return du*du+dv*dv < 0.08
	}
	if r.shape == shapes["mackerel"] {
		// Spanish mackerel flank bars: dense vertical striping mid-body.
		if s < 0.2 || s > 0.75 || across > 0.85 {
			return false
		}
		wobble := 0.3 * math.Sin(v/L*30+s*7)
		bar := math.Sin(s*math.Pi*2*14 + wobble)
		return bar > 0.55-0.4*mathx.Smoothstep(0.2, 0.4, s)+0.35*mathx.Smoothstep(0.65, 0.75, s)
	}
	if r.shape == shapes["tuna"] {
		// Albacore finlets: a dotted row aft of the second dorsal.
		if s < 0.64 || s > 0.88 || across > 0.5 {
			return false
		}
		gu := (s * L) / (L * 0.035)
		gv := v / (L * 0.03)
		cu := math.Floor(gu)
		cv := math.Floor(gv)
		jx := mathx.Hash3(int(cu), int(cv), r.sharkID)*0.5 + 0.25
		jy := mathx.Hash3(int(cu), int(cv), r.sharkID+23)*0.5 + 0.25
		du := gu - cu - jx
		dv := gv - cv - jy
		return du*du+dv*dv < 0.1
	}
	return false
}

// rasterizeSharks builds a smooth, undulating render spine from the sim
// spine, then supersamples every cell in each shark's bounding box against
// the shape. Ports AsciiRenderer.rasterizeSharks in cell units.
func (r *Renderer) rasterizeSharks(sharks []world.Shark, t float64) {
	for i := range r.sharkFlag {
		r.sharkFlag[i] = 0
	}
	for i := range r.sharkShadow {
		r.sharkShadow[i] = 0
	}
	for _, sh := range sharks {
		// Depth perspective: effective length shrinks with distance for a
		// dramatic near/far size gap, and the silhouette lifts toward
		// the horizon when far.
		L := sh.Length * (0.45 + 0.75*mathx.Clamp(sh.Depth, 0, 1))
		if L < 2 {
			L = 2
		}
		lift := (1 - mathx.Clamp(sh.Depth, 0, 1)) * sharkLiftRows
		N := world.SHARK_NODES
		n := sh.Nodes
		swim := mathx.Clamp(sh.Speed*pxCell/60, 0.4, 1.4)
		shape, ok := shapes[string(sh.Kind)]
		if !ok {
			shape = shapes["classic"]
		}
		r.shape = shape
		r.sharkID = sh.ID

		for k := 0; k < N; k++ {
			s := float64(k) / float64(N-1)
			a := k - 1
			if a < 0 {
				a = 0
			}
			b := k + 1
			if b > N-1 {
				b = N - 1
			}
			tx := n[a*2] - n[b*2]
			ty := n[a*2+1] - n[b*2+1]
			tl := math.Hypot(tx, ty)
			if tl == 0 {
				tl = 1
			}
			tx /= tl
			ty /= tl
			amp := L * 0.05 * (0.1 + 0.9*math.Pow(s, 1.8)) * swim
			off := amp * math.Sin(sh.Phase-s*5.2)
			r.spine[k*2] = n[k*2] - ty*off
			r.spine[k*2+1] = n[k*2+1] + tx*off - lift
		}
		r.arc[0] = 0
		for k := 1; k < N; k++ {
			r.arc[k] = r.arc[k-1] + math.Hypot(
				r.spine[k*2]-r.spine[(k-1)*2],
				r.spine[k*2+1]-r.spine[(k-1)*2+1])
		}
		r.arcTotal = r.arc[N-1]
		if r.arcTotal == 0 {
			r.arcTotal = 1
		}

		bank := mathx.Clamp(sh.Turn*0.45, -0.35, 0.35)
		flex := 1 + 0.06*math.Sin(sh.Phase*0.5)
		reachFin := shape.pectoral[3]
		r.finTipRight = L * reachFin * flex * (1 - bank)
		r.finTipLeft = L * reachFin * flex * (1 + bank)

		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for k := 0; k < N; k++ {
			if r.spine[k*2] < minX {
				minX = r.spine[k*2]
			}
			if r.spine[k*2] > maxX {
				maxX = r.spine[k*2]
			}
			if r.spine[k*2+1] < minY {
				minY = r.spine[k*2+1]
			}
			if r.spine[k*2+1] > maxY {
				maxY = r.spine[k*2+1]
			}
		}
		reach := L * 0.36
		c0 := max(0, int(math.Floor(minX-reach)))
		c1 := min(r.cols-1, int(math.Floor(maxX+reach)))
		r0 := max(0, int(math.Floor(minY-reach)))
		r1 := min(r.rows-1, int(math.Floor(maxY+reach)))
		if c0 > c1 || r0 > r1 {
			continue
		}

		// Ports skipBeyond = reach + hypot(cw, ch): one cell diagonal is
		// hypot(1, cellAspect) in col units, since a row is ~1.45x a col.
		skipBeyond := reach + math.Hypot(1, cellAspect)
		shadowDc := int(math.Round(L*0.035)) + 1
		// Ports round(L*0.02/ch)+1: the row offset shrinks by the aspect.
		shadowDr := int(math.Round(L*0.02/cellAspect)) + 1

		for row := r0; row <= r1; row++ {
			for c := c0; c <= c1; c++ {
				x := float64(c)
				y := float64(row)
				centerHit := r.sampleShark(x+0.5, y+0.5, L)
				if !centerHit && r.hitDist > skipBeyond {
					continue
				}
				flag := r.hitFlag
				shade := r.hitShade
				s := r.hitS
				v := r.hitV

				hits := 0
				if centerHit {
					hits = 1
				}
				for k := 0; k < len(subsamples); k += 2 {
					if r.sampleShark(x+subsamples[k], y+subsamples[k+1], L) {
						hits++
					}
				}
				if hits == 0 {
					continue
				}
				coverage := float64(hits) / 5

				i := row*r.cols + c
				key := uint32(mathx.Hash3(int(math.Round(s*52)), int(math.Round(v/0.9)), sh.ID) * 4294967295)
				r.sharkKey[i] = key
				r.sharkDepth[i] = sh.Depth
				r.sharkKind[i] = uint8(kindIndex(string(sh.Kind)))

				if centerHit && coverage >= 0.6 {
					r.sharkFlag[i] = flag
					flicker := mathx.Hash3(c, row, int(math.Floor(t*6))) - 0.5
					r.sharkShade[i] = mathx.Clamp(shade+flicker*0.06, 0, 1)
					if sc := c + shadowDc; sc < r.cols {
						if sr := row + shadowDr; sr < r.rows {
							r.sharkShadow[sr*r.cols+sc] = 1
						}
					}
				} else {
					r.sharkFlag[i] = flagEdge
					r.sharkShade[i] = coverage
				}
			}
		}
	}
}

// rasterizePredators draws schooling predators into the same shark layers
// so tuna/mackerel read as small hunters, not prey glyphs. It never clears
// the buffers: Render rasterises sharks first (which clears), then calls
// this, so sharks stay on top except where a nearer predator overwrites a
// farther shark cell via the depth check below.
func (r *Renderer) rasterizePredators(preds []world.Predator, t float64) {
	for _, p := range preds {
		if !p.Alive {
			continue
		}
		L := p.Length * (0.45 + 0.75*mathx.Clamp(p.Depth, 0, 1))
		if L < 2 {
			L = 2
		}
		lift := (1 - mathx.Clamp(p.Depth, 0, 1)) * sharkLiftRows
		N := world.SHARK_NODES
		n := p.Nodes
		if len(n) < N*2 {
			continue
		}
		swim := mathx.Clamp(p.Speed*pxCell/60, 0.4, 1.4)
		shape, ok := shapes[string(p.Kind)]
		if !ok {
			shape = shapes["tuna"]
		}
		r.shape = shape
		r.sharkID = p.ID + 1000

		for k := 0; k < N; k++ {
			s := float64(k) / float64(N-1)
			a := k - 1
			if a < 0 {
				a = 0
			}
			b := k + 1
			if b > N-1 {
				b = N - 1
			}
			tx := n[a*2] - n[b*2]
			ty := n[a*2+1] - n[b*2+1]
			tl := math.Hypot(tx, ty)
			if tl == 0 {
				tl = 1
			}
			tx /= tl
			ty /= tl
			amp := L * 0.06 * (0.1 + 0.9*math.Pow(s, 1.8)) * swim
			off := amp * math.Sin(p.Phase-s*6.0)
			r.spine[k*2] = n[k*2] - ty*off
			r.spine[k*2+1] = n[k*2+1] + tx*off - lift
		}
		r.arc[0] = 0
		for k := 1; k < N; k++ {
			r.arc[k] = r.arc[k-1] + math.Hypot(
				r.spine[k*2]-r.spine[(k-1)*2],
				r.spine[k*2+1]-r.spine[(k-1)*2+1])
		}
		r.arcTotal = r.arc[N-1]
		if r.arcTotal == 0 {
			r.arcTotal = 1
		}

		bank := mathx.Clamp(p.Turn*0.4, -0.35, 0.35)
		flex := 1 + 0.06*math.Sin(p.Phase*0.5)
		reachFin := shape.pectoral[3]
		r.finTipRight = L * reachFin * flex * (1 - bank)
		r.finTipLeft = L * reachFin * flex * (1 + bank)

		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for k := 0; k < N; k++ {
			if r.spine[k*2] < minX {
				minX = r.spine[k*2]
			}
			if r.spine[k*2] > maxX {
				maxX = r.spine[k*2]
			}
			if r.spine[k*2+1] < minY {
				minY = r.spine[k*2+1]
			}
			if r.spine[k*2+1] > maxY {
				maxY = r.spine[k*2+1]
			}
		}
		reach := L * 0.42
		c0 := max(0, int(math.Floor(minX-reach)))
		c1 := min(r.cols-1, int(math.Floor(maxX+reach)))
		r0 := max(0, int(math.Floor(minY-reach)))
		r1 := min(r.rows-1, int(math.Floor(maxY+reach)))
		if c0 > c1 || r0 > r1 {
			continue
		}

		skipBeyond := reach + math.Hypot(1, cellAspect)
		shadowDc := int(math.Round(L*0.035)) + 1
		shadowDr := int(math.Round(L*0.02/cellAspect)) + 1

		for row := r0; row <= r1; row++ {
			for c := c0; c <= c1; c++ {
				x := float64(c)
				y := float64(row)
				centerHit := r.sampleShark(x+0.5, y+0.5, L)
				if !centerHit && r.hitDist > skipBeyond {
					continue
				}
				flag := r.hitFlag
				shade := r.hitShade
				s := r.hitS
				v := r.hitV

				hits := 0
				if centerHit {
					hits = 1
				}
				for k := 0; k < len(subsamples); k += 2 {
					if r.sampleShark(x+subsamples[k], y+subsamples[k+1], L) {
						hits++
					}
				}
				if hits == 0 {
					continue
				}
				coverage := float64(hits) / 5

				i := row*r.cols + c
				// Depth z-order: a farther predator never paints over a
				// nearer shark cell.
				if r.sharkFlag[i] != 0 && r.sharkDepth[i] > p.Depth+0.05 {
					continue
				}
				key := uint32(mathx.Hash3(int(math.Round(s*52)), int(math.Round(v/0.9)), p.ID+1000) * 4294967295)
				r.sharkKey[i] = key
				r.sharkDepth[i] = p.Depth
				r.sharkKind[i] = uint8(kindIndex(string(p.Kind)))

				if centerHit && coverage >= 0.6 {
					r.sharkFlag[i] = flag
					flicker := mathx.Hash3(c, row, int(math.Floor(t*6))) - 0.5
					r.sharkShade[i] = mathx.Clamp(shade+flicker*0.06, 0, 1)
					if sc := c + shadowDc; sc < r.cols {
						if sr := row + shadowDr; sr < r.rows {
							r.sharkShadow[sr*r.cols+sc] = 1
						}
					}
				} else {
					r.sharkFlag[i] = flagEdge
					r.sharkShade[i] = coverage
				}
			}
		}
	}
}

// markFish snaps each fish to a cell and buckets it by brightness: far fish
// are dim, near fish brighter, and turning or panicking fish glint. Nearer
// fish claim a cell first when two land in the same one. Depth lifts the
// render row, so far fish rise toward the horizon. Ports
// AsciiRenderer.markFish.
func (r *Renderer) markFish(w *world.World) {
	r.frameID++
	for i := range r.fishBuckets {
		r.fishBuckets[i] = r.fishBuckets[i][:0]
	}
	for pass := 0; pass < 2; pass++ {
		for _, f := range w.Fish {
			if !f.Alive {
				continue
			}
			want := 0
			if f.Depth > 0.78 {
				want = 1
			}
			if want != pass {
				continue
			}
			ry := f.Y - (1-f.Depth)*fishParallaxRows
			i := r.cellIndex(f.X, ry)
			if i < 0 || r.fishStamp[i] == r.frameID || r.sharkFlag[i] != 0 {
				continue
			}
			r.fishStamp[i] = r.frameID
			glyph := '-'
			if f.Species >= 0 && f.Species < len(species.SPECIES) {
				if g := species.SPECIES[f.Species].Glyph; g != 0 {
					glyph = g
				}
			}
			r.fishGlyph[i] = glyph
			// Opacity encodes depth first: the bucket comes from the depth
			// ramp, and alarm/turn glint adds at most one step so a far
			// panicking fish never outshines a near calm one.
			depthT := mathx.Clamp((f.Depth-fishDepthMin)/(fishDepthMax-fishDepthMin), 0, 1)
			b := int(depthT * float64(len(fishAlphas)-1))
			glint := math.Max(mathx.Clamp(math.Abs(f.Turn)/3.5, 0, 1), f.Alarm)
			if glint > 0.5 && b < len(fishAlphas)-1 {
				b++
			}
			b = int(mathx.Clamp(float64(b), 0, float64(len(fishAlphas)-1)))
			r.fishBuckets[b] = append(r.fishBuckets[b], i)
			// Large fish read as bigger: they spread over a second body
			// cell trailing behind the head, both painted '='. The tail
			// is best-effort: out-of-bounds, shark-covered or already
			// claimed cells fall back to the head cell alone.
			if glyph == '=' {
				dir := 1
				if f.Vx < 0 {
					dir = -1
				}
				headC := int(math.Floor(f.X))
				headR := int(math.Floor(ry))
				if j := headR*r.cols + (headC - dir); headC-dir >= 0 && headC-dir < r.cols &&
					j >= 0 && j < len(r.fishStamp) &&
					r.fishStamp[j] != r.frameID && r.sharkFlag[j] == 0 {
					r.fishStamp[j] = r.frameID
					r.fishGlyph[j] = glyph
					r.fishBuckets[b] = append(r.fishBuckets[b], j)
				}
			}
		}
	}
}

// compose paints dots/wake, heat, fish and sharks into the cell buffer.
// Layer order ports AsciiRenderer.render: dots, heat, fish, sharks.
func (r *Renderer) compose(t float64) {
	for i := range r.cells {
		r.cells[i] = cell{ch: ' '}
	}
	r.drawDots(t)
	r.drawRipples(t)
	r.drawHeat()
	r.drawFish()
	r.drawSharks(t)
}

func (r *Renderer) drawRipples(t float64) {
	if len(r.ripples) == 0 {
		return
	}
	for _, rp := range r.ripples {
		age := t - rp.birth
		maxR, life := rippleExtent(rp.scale)
		if age < 0 || age >= life {
			continue
		}
		fade := 1 - age/life
		radius := math.Min(age*rippleSpeed, maxR)
		width := rippleWidth * (0.6 + 0.4*rp.scale)
		maxR += width + 0.5
		c0 := int(math.Floor(rp.x - maxR))
		c1 := int(math.Ceil(rp.x + maxR))
		r0 := int(math.Floor(rp.y - maxR/1.4))
		r1 := int(math.Ceil(rp.y + maxR/1.4))
		if c0 < 0 {
			c0 = 0
		}
		if r0 < 0 {
			r0 = 0
		}
		if c1 >= r.cols {
			c1 = r.cols - 1
		}
		if r1 >= r.rows {
			r1 = r.rows - 1
		}
		for row := r0; row <= r1; row++ {
			for c := c0; c <= c1; c++ {
				d := math.Hypot(float64(c)-rp.x+0.5, (float64(row)-rp.y+0.5)*1.4)
				if math.Abs(d-radius) > width {
					continue
				}
				ring := 1 - math.Abs(d-radius)/width
				a := fade * ring * rp.strength
				if a < 0.12 {
					continue
				}
				if a > 1 {
					a = 1
				}
				i := row*r.cols + c
				r.cells[i].ch = '·'
				r.cells[i].fg = [3]int{int(100 * a), int(190 * a), int(225 * a)}
				r.cells[i].hasFg = true
			}
		}
	}
}

func (r *Renderer) drawDots(t float64) {
	_ = t
	// Ports AsciiRenderer.drawDots with dotKind == 0 everywhere (see the
	// canvas resize: "No ambient dots: the grid stays black. Only the cyan
	// wake lights cells"). The twinkle/caustic/shadow path only applies to
	// ambient dots, so like the source this draws just the WAKE bucket.
	for i, n := 0, r.cols*r.rows; i < n; i++ {
		e := r.energy[i]
		if r.sharkFlag[i] != 0 || r.fishStamp[i] == r.frameID || e < 0.45 {
			continue
		}
		// Wake flare: cyan, brighter with energy. Ports the WAKE bucket.
		level := int(mathx.Clamp(math.Floor((e-0.45)*5), 0, alphaLevels-1))
		a := float64(level+1) / alphaLevels
		fg := [3]int{
			int(dotRGB[wakeIndex][0] * a),
			int(dotRGB[wakeIndex][1] * a),
			int(dotRGB[wakeIndex][2] * a),
		}
		r.cells[i].ch = '·'
		r.cells[i].fg = fg
		r.cells[i].hasFg = true
	}
}

func (r *Renderer) drawHeat() {
	for i, h := range r.heat {
		if h < 0.08 {
			continue
		}
		var ch rune = '·'
		var fg [3]int
		if h > 0.5 {
			if h > 0.6 {
				ch = '*'
			}
			a := math.Min(1, h)
			fg = [3]int{int(255 * a), int(120 * a), int(60 * a)}
		} else {
			a := math.Min(1, h*1.6)
			fg = [3]int{int(220 * a), int(40 * a), int(40 * a)}
		}
		r.cells[i].ch = ch
		r.cells[i].fg = fg
		r.cells[i].hasFg = true
	}
}

func (r *Renderer) drawFish() {
	for b, bucket := range r.fishBuckets {
		if len(bucket) == 0 {
			continue
		}
		fg := fishBucketFG[b]
		for _, i := range bucket {
			if r.sharkFlag[i] != 0 {
				continue
			}
			ch := r.fishGlyph[i]
			if ch == 0 {
				ch = '-'
			}
			r.cells[i].ch = ch
			r.cells[i].fg = fg
			r.cells[i].hasFg = true
		}
	}
}

func (r *Renderer) drawSharks(t float64) {
	for i, flag := range r.sharkFlag {
		if flag == 0 {
			continue
		}
		key := r.sharkKey[i]
		shade := r.sharkShade[i]
		// Simulated opacity: far sharks fade toward ghostly, near ones
		// render solid. The range is deliberately dramatic so size+fade
		// reads at a glance, with only a whisper of cool cast when far.
		depthC := mathx.Clamp(r.sharkDepth[i], 0, 1)
		dim := 0.15 + 0.85*depthC
		shape := shapes["classic"]
		if int(r.sharkKind[i]) < len(kindOrder) {
			if s, ok := shapes[kindOrder[r.sharkKind[i]]]; ok {
				shape = s
			}
		}
		tr, tg, tb := shape.tint[0], shape.tint[1], shape.tint[2]
		coolMix := (1 - depthC) * 0.25
		tr = tr*(1-coolMix) + 0.65*coolMix
		tg = tg*(1-coolMix) + 0.8*coolMix
		tb = tb*(1-coolMix) + 1.15*coolMix

		tick := int(math.Floor(t*2.4 + float64(key%997)/97))
		roll := mathx.Hash3(int(key), tick, 7)

		if flag == flagEdge {
			set := edgeHeavy
			if shade < 0.5 {
				set = edgeLight
			}
			g := (70 + shade*80) * dim
			ir, ig, ib := int(g*tr), int(g*tg), int(g*tb+6)
			var fg [3]int
			if roll < 0.07 {
				fg = accentRGB[key%uint32(len(accentRGB))]
			} else {
				fg = [3]int{ir, ig, ib}
			}
			r.cells[i].ch = rune(set[(key>>8)%uint32(len(set))])
			r.cells[i].fg = fg
			r.cells[i].hasFg = true
			r.cells[i].hasBg = false
			continue
		}

		var level float64
		var glyph rune
		switch flag {
		case flagRidge:
			level = 226
			glyph = rune(ridgeGlyphs[(key>>8)%uint32(len(ridgeGlyphs))])
		case flagEye:
			level = 12
			glyph = 'o'
		case flagMark:
			level = 24 + shade*22
			glyph = rune(markGlyphs[(key>>8)%uint32(len(markGlyphs))])
		case flagFin:
			level = 40 + shade*120
			glyph = rune(finGlyphs[(key>>8)%uint32(len(finGlyphs))])
		default:
			level = 46 + shade*160
			jitter := int((key>>4)%3) - 1
			idx := int(mathx.Clamp(math.Floor((1-shade)*8.99)+float64(jitter), 0, float64(len(skinRamp)-1)))
			glyph = rune(skinRamp[idx])
		}
		level *= dim
		bg := [3]int{
			min(255, int(level*tr)),
			min(255, int(level*tg)),
			min(255, int(level*tb+4)),
		}
		var fg [3]int
		hasAccent := false
		if flag == flagEye {
			fg = [3]int{0x9a, 0xa0, 0xad}
			hasAccent = true
		} else if flag == flagMark {
			fg = [3]int{int(70 * tr), int(70 * tg), int(70 * tb)}
			hasAccent = true
		} else if roll < 0.09 {
			fg = accentRGB[key%uint32(len(accentRGB))]
			hasAccent = true
		} else if roll < 0.13 {
			fg = [3]int{255, 255, 255}
			hasAccent = true
		} else if shade > 0.55 {
			fg = [3]int{0x0e, 0x0f, 0x13}
		} else {
			fg = [3]int{0x1b, 0x1d, 0x24}
		}
		_ = hasAccent
		r.cells[i].ch = glyph
		r.cells[i].fg = fg
		r.cells[i].hasFg = true
		r.cells[i].bg = bg
		r.cells[i].hasBg = true
	}
}

func writeFg(b *strings.Builder, fg [3]int) {
	b.WriteString("\x1b[38;2;")
	writeRGB(b, fg)
	b.WriteString("m")
}

func writeBg(b *strings.Builder, bg [3]int) {
	b.WriteString("\x1b[48;2;")
	writeRGB(b, bg)
	b.WriteString("m")
}

func writeRGB(b *strings.Builder, c [3]int) {
	b.WriteString(itoa(clamp255(c[0])))
	b.WriteByte(';')
	b.WriteString(itoa(clamp255(c[1])))
	b.WriteByte(';')
	b.WriteString(itoa(clamp255(c[2])))
}

func clamp255(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [3]byte
	n := 0
	for v > 0 {
		buf[n] = byte('0' + v%10)
		v /= 10
		n++
	}
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf[:n])
}

// emit serialises the cell buffer with minimal SGR changes.
func (r *Renderer) emit() string {
	var b strings.Builder
	b.WriteString("\x1b[H")
	curFg := [3]int{-1, -1, -1}
	curBg := [3]int{-1, -1, -1}
	hasColor := false
	for row := 0; row < r.rows; row++ {
		if row > 0 {
			b.WriteString("\r\n")
		}
		for c := 0; c < r.cols; c++ {
			i := row*r.cols + c
			cell := &r.cells[i]
			if cell.ch == ' ' && !cell.hasBg {
				if hasColor {
					b.WriteString("\x1b[0m")
					hasColor = false
					curFg = [3]int{-1, -1, -1}
					curBg = [3]int{-1, -1, -1}
				}
				b.WriteRune(' ')
				continue
			}
			ch := cell.ch
			if ch == 0 {
				ch = ' '
			}
			if cell.hasBg && (curBg != cell.bg) {
				writeBg(&b, cell.bg)
				curBg = cell.bg
				hasColor = true
			} else if !cell.hasBg && curBg[0] != -1 {
				b.WriteString("\x1b[49m")
				curBg = [3]int{-1, -1, -1}
				hasColor = true
			}
			if cell.hasFg && (curFg != cell.fg) {
				writeFg(&b, cell.fg)
				curFg = cell.fg
				hasColor = true
			} else if !cell.hasFg && curFg[0] != -1 {
				b.WriteString("\x1b[39m")
				curFg = [3]int{-1, -1, -1}
				hasColor = true
			}
			b.WriteRune(ch)
		}
		if hasColor {
			b.WriteString("\x1b[0m")
			hasColor = false
			curFg = [3]int{-1, -1, -1}
			curBg = [3]int{-1, -1, -1}
		}
		b.WriteString("\x1b[K")
	}
	return b.String()
}
