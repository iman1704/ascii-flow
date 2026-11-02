// Package mathx ports src/math.ts: numeric helpers and the divergence-free
// ocean-current field that advects the simulation.
package mathx

import (
	"math"
	"math/rand/v2"
)

// TAU is 2π. Ports TAU.
const TAU = math.Pi * 2

// Clamp constrains v to [lo, hi]. Ports clamp.
func Clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Rand returns a uniform random in [lo, hi). Ports rand.
func Rand(lo, hi float64) float64 {
	return lo + rand.Float64()*(hi-lo)
}

// RandInt returns a uniform random integer in [lo, hi]. Ports randInt.
func RandInt(lo, hi int) int {
	return lo + rand.IntN(hi-lo+1)
}

// Smoothstep interpolates 0→1 smoothly. Ports smoothstep.
func Smoothstep(e0, e1, x float64) float64 {
	t := Clamp((x-e0)/(e1-e0), 0, 1)
	return t * t * (3 - 2*t)
}

// AngleDiff is the shortest signed angle from a to b, in (-π, π]. Ports angleDiff.
func AngleDiff(a, b float64) float64 {
	d := math.Mod(b-a, TAU)
	if d > math.Pi {
		d -= TAU
	}
	if d < -math.Pi {
		d += TAU
	}
	return d
}

// Hash3 is a cheap integer hash → [0, 1). Deterministic, so textures stay
// stable. Ports hash3; uint32 arithmetic reproduces Math.imul's 32-bit wrap.
func Hash3(x, y, z int) float64 {
	h := uint32(x)*374761393 ^ uint32(y)*668265263 ^ uint32(z)*1442695041
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h) / 4294967296
}

// Vec is a 2D velocity, the Go equivalent of math.ts's {x, y} out-param.
type Vec struct{ X, Y float64 }

// Current is the divergence-free ocean current at (x, y, t): the curl of a
// slowly drifting stream function, so it moves schools without bunching them
// into a corner. Ports current; out is written and returned, as in TS.
func Current(x, y, t float64, out *Vec) *Vec {
	const (
		k1    = 1.0 / 260
		k2    = 1.0 / 310
		k3    = 1.0 / 190
		scale = 3400
	)
	a := x*k1 + t*0.07
	b := y*k2 - t*0.05
	c := (x+y)*k3 + t*0.11
	// u = ∂ψ/∂y, v = -∂ψ/∂x with ψ = sin(a)cos(b) + 0.5 sin(c)
	out.X = (-k2*math.Sin(a)*math.Sin(b) + 0.5*k3*math.Cos(c)) * scale
	out.Y = (-k1*math.Cos(a)*math.Cos(b) - 0.5*k3*math.Cos(c)) * scale
	return out
}
