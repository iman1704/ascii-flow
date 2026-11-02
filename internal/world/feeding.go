// Bait-fish feeding frenzy: drifting plankton spots that schooling fish
// gather at, mill around, and nibble away.
//
// Spots spawn on average once every 10s (Poisson roll per step), several
// can coexist up to a cap, each lasts a few seconds and is eaten away
// faster the bigger the crowd on it. Spots are invisible: the only
// visuals are the bait ball itself plus one ring of ripples encircling
// the school (emitted as "feed" events the renderer turns into wake
// without the red catch glow). Any hunter nearby scatters the feeders
// through the normal fear path; survivors return while the spot still
// stands.
package world

import (
	"math"

	"github.com/iman1704/ascii-flow/internal/mathx"
)

const (
	// feedingSpawnRate is the Poisson spawn rate: on average one fresh
	// plankton spot every 10s.
	feedingSpawnRate = 0.1
	// feedingLifetimeMin/Max bound one spot's timed life: long enough
	// for nearby schools to arrive (a few seconds of travel) and feed.
	feedingLifetimeMin = 8.0
	feedingLifetimeMax = 12.0
	// feedingMaxSpots caps coexisting spots so the water never fills.
	feedingMaxSpots = 4
	// feedingFoodMax is the plankton in a fresh spot; each calm fish on
	// it nibbles feedingNibbleRate per second. Sized so a typical
	// school (~12 fish, ~6/s) takes roughly the full timed life
	// (8-12s) to finish a spot; small schools leave leftovers that
	// expire, huge merged balls still finish early.
	feedingFoodMax    = 60.0
	feedingNibbleRate = 0.5
	// feedingAttractRadius is the 3D range the gathering pull reaches;
	// feedingFeedRadius is the nibbling range, sized to cover the whole
	// milling ball (not just its core) so a gathered crowd actually
	// feeds; feedingMillRadius is the tight circling range.
	feedingAttractRadius = 18.0
	feedingFeedRadius    = 4.0
	feedingMillRadius    = 4.5
	// feedingPull steers gatherers at the spot; feedingMill spins the
	// ball tangentially. The pull scales with the 1.5x pace so rushing
	// fish still reach the ball, then fades inside mill range while
	// millers slow down and drop the roam drive, so the ball brakes
	// into a lingering orbit instead of charging through. Both stay
	// far below flee accelerations, so a predator always wins lunch.
	feedingPull = 13.5
	feedingMill = 7.0
	// feedFearGate interrupts feeding: any direct threat above this fear
	// (shark or schooling predator) skips the gathering pull entirely.
	feedFearGate = 0.15
	// feedingCalmAlarm: panicking fish do not nibble, so a scattered
	// school stops depleting the spot until it calms down.
	feedingCalmAlarm = 0.7
	// feedingRippleInterval throttles edge-ripple events per spot, and
	// feedingRippleMembers is the minimum ball size that rings: a lone
	// passerby leaves no trace, a gathered school rings its edge.
	feedingRippleInterval = 0.4
	feedingRippleMembers  = 3
	// feedingRippleDouble rings twice per interval (opposite edges) once
	// the ball reaches this size, so big frenzies read bigger.
	feedingRippleDouble = 8
)

// FeedingSpot is one plankton bloom: a position, the depth it drifts at,
// and the remaining Food. BornAt/ExpiresAt bound its timed life;
// nextRipple throttles its ripple events.
type FeedingSpot struct {
	X, Y, Depth       float64
	BornAt, ExpiresAt float64
	Food              float64
	nextRipple        float64
}

// newFeedingSpotAt builds a spot at an explicit position (tests and any
// future directed spawns); random blooms go through spawnFeedingSpot.
func (w *World) newFeedingSpotAt(x, y, depth float64) FeedingSpot {
	return FeedingSpot{
		X:         x,
		Y:         y,
		Depth:     depth,
		BornAt:    w.Time,
		ExpiresAt: w.Time + mathx.Rand(feedingLifetimeMin, feedingLifetimeMax),
		Food:      feedingFoodMax,
	}
}

// spawnFeedingSpot blooms one plankton spot, biased to near-surface water
// so feeding crowds ring the surface. Spots stay inside the arena.
func (w *World) spawnFeedingSpot() {
	if len(w.FeedingSpots) >= feedingMaxSpots {
		return
	}
	W := float64(w.Width)
	H := float64(w.Height)
	if W <= 0 || H <= 0 {
		return
	}
	band := surfaceBand(int(H))
	lo, hi := 1.5, band+1.0
	if hi > H-1.5 {
		hi = H - 1.5
	}
	if hi <= lo {
		hi = lo + 1
	}
	w.FeedingSpots = append(w.FeedingSpots, FeedingSpot{
		X:         mathx.Rand(W*0.08, W*0.92),
		Y:         mathx.Rand(lo, hi),
		Depth:     mathx.Rand(0.85, 1.0),
		BornAt:    w.Time,
		ExpiresAt: w.Time + mathx.Rand(feedingLifetimeMin, feedingLifetimeMax),
		Food:      feedingFoodMax,
	})
}

// nearestFeedingSpot returns the index and 3D (depth-scaled) distance of
// the closest spot, or -1/+Inf when the water holds none.
func (w *World) nearestFeedingSpot(x, y, depth float64) (int, float64) {
	best := math.Inf(1)
	idx := -1
	for i := range w.FeedingSpots {
		s := &w.FeedingSpots[i]
		dz := (depth - s.Depth) * depthScale
		if d := math.Hypot(x-s.X, math.Hypot(y-s.Y, dz)); d < best {
			best = d
			idx = i
		}
	}
	return idx, best
}

// spotNearSurface reports whether a spot sits in the ripple band, using
// the same bound as the renderer's feed-event ripple (e.Y < band+2) so
// every spawned spot can ring.
func (w *World) spotNearSurface(s *FeedingSpot) bool {
	ry := s.Y - (1-s.Depth)*fishParallaxRows
	return ry < surfaceBand(w.Height)+2
}

// feedingBall measures the visible ball on a spot: the centroid, outer
// radius and member count of live fish milling within mill range
// (planar distance, matching what the eye sees). It reports ok=false
// below the minimum ringing school so strays leave no trace.
func (w *World) feedingBall(s *FeedingSpot) (cx, cy, r float64, n int, ok bool) {
	var sx, sy float64
	var members []int
	for j := range w.Fish {
		f := &w.Fish[j]
		if !f.Alive {
			continue
		}
		if dx, dy := f.X-s.X, f.Y-s.Y; dx*dx+dy*dy < feedingMillRadius*feedingMillRadius {
			members = append(members, j)
			sx += f.X
			sy += f.Y
		}
	}
	if len(members) < feedingRippleMembers {
		return 0, 0, 0, 0, false
	}
	cx, cy = sx/float64(len(members)), sy/float64(len(members))
	best := 0.0
	for _, j := range members {
		if d := math.Hypot(w.Fish[j].X-cx, w.Fish[j].Y-cy); d > best {
			best = d
		}
	}
	return cx, cy, mathx.Clamp(best, 2.0, 7.0), len(members), true
}

// ringBallEdge emits one feed event on the ball's rim at the given
// bearing, with a little radial jitter so successive rings never stack.
func (w *World) ringBallEdge(cx, cy, r, bearing float64) {
	rr := mathx.Clamp(r+mathx.Rand(-0.75, 0.75), 1.5, 8.0)
	ex := mathx.Clamp(cx+math.Cos(bearing)*rr, 0, float64(w.Width))
	ey := mathx.Clamp(cy+math.Sin(bearing)*rr, 0, float64(w.Height))
	w.Events = append(w.Events, WorldEvent{Kind: "feed", X: ex, Y: ey})
}

// ringSchoolEdge rings the whole school at once: feed events are laid
// around the ball's circumference in one go so the ripples read as a
// ring encircling the school, not per-fish pulses. Bearings are evenly
// spaced with a little jitter plus a random rotation so successive rings
// never stack.
func (w *World) ringSchoolEdge(cx, cy, r float64, n int) {
	count := 3
	if n >= feedingRippleDouble {
		count = 5
	}
	base := mathx.Rand(0, mathx.TAU)
	for k := 0; k < count; k++ {
		bearing := base + float64(k)*mathx.TAU/float64(count) + mathx.Rand(-0.2, 0.2)
		w.ringBallEdge(cx, cy, r, bearing)
	}
}

// FishMillingOnSpot reports whether the i-th fish is calmly milling on a
// live plankton spot (planar distance within mill range). The renderer
// uses it to suppress per-fish ripples for feeders: a feeding school
// rings as one school-level ring instead of one ripple per fish.
func (w *World) FishMillingOnSpot(i int) bool {
	if i < 0 || i >= len(w.Fish) {
		return false
	}
	f := &w.Fish[i]
	if !f.Alive || f.Alarm >= feedingCalmAlarm {
		return false
	}
	for k := range w.FeedingSpots {
		s := &w.FeedingSpots[k]
		dx, dy := f.X-s.X, f.Y-s.Y
		if dx*dx+dy*dy < feedingMillRadius*feedingMillRadius {
			return true
		}
	}
	return false
}

// updateFeedingSpots rolls one Poisson spawn per step, depletes spots under
// calm crowds, rings feed events for shallow crowds, and drops spots that
// timed out or were eaten away. Calm fish resume on their own: stepFish
// re-attracts them while the spot still stands.
func (w *World) updateFeedingSpots(dt float64) {
	if w.Width > 0 && w.Height > 0 && len(w.FeedingSpots) < feedingMaxSpots {
		if mathx.Rand(0, 1) < feedingSpawnRate*dt {
			w.spawnFeedingSpot()
		}
	}
	kept := w.FeedingSpots[:0]
	for i := range w.FeedingSpots {
		s := &w.FeedingSpots[i]
		feeders := 0
		for j := range w.Fish {
			f := &w.Fish[j]
			if !f.Alive || f.Alarm >= feedingCalmAlarm {
				continue
			}
			dx := f.X - s.X
			dy := f.Y - s.Y
			dz := (f.Depth - s.Depth) * depthScale
			if dx*dx+dy*dy+dz*dz < feedingFeedRadius*feedingFeedRadius {
				feeders++
			}
		}
		if feeders > 0 {
			s.Food -= float64(feeders) * feedingNibbleRate * dt
		}
		// Ring the school as one ring around the ball, not the spot
		// centre and not per fish: a gathered school reads as one
		// churning ring instead of per-fish pulses or a single hidden
		// pulse.
		if w.Time >= s.nextRipple && w.spotNearSurface(s) {
			if cx, cy, r, n, ok := w.feedingBall(s); ok {
				w.ringSchoolEdge(cx, cy, r, n)
				s.nextRipple = w.Time + feedingRippleInterval
			}
		}
		if w.Time < s.ExpiresAt && s.Food > 0 {
			kept = append(kept, *s)
		}
	}
	w.FeedingSpots = kept
}
