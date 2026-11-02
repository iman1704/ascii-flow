// Package species ports src/species.ts: fish size-type definitions and
// weighted random selection for new schools.
//
// Simplified model: no more named species (goby, sardine, ...). Three
// size-only types — small, medium, large — each keeping its glyph and a
// distinct boid archetype. Schools form by size with minimum sizes
// (small 20, medium 12, large 7) and no maximum.
package species

import (
	"math/rand/v2"
)

// SizeClass is the visual size variant of a fish.
type SizeClass string

const (
	SizeSmall  SizeClass = "small"
	SizeMedium SizeClass = "medium"
	SizeLarge  SizeClass = "large"
)

// Species ports the Species interface. Spacing is in grid cells so every
// fish gets its own glyph. Behaviour weights follow Couzin et al. 2002:
// strong Align gives polarised schools and ribbons, low Align gives swarms,
// and a tangential Mill pull gives a rotating bait ball.
type Species struct {
	Name         string
	Cruise       float64
	MaxSpeed     float64
	MinSpeed     float64
	Weight       float64
	SchoolSize   [2]int
	Spacing      [2]float64
	Align        float64
	Cohesion     float64
	Jitter       float64
	Mill         float64
	VerticalDrag float64
	Size         SizeClass
	Glyph        rune
}

// IsLarge reports whether the type index is a large fish that sharks eat.
// Only the large size class (index 2) is edible.
func IsLarge(idx int) bool {
	if idx < 0 || idx >= len(SPECIES) {
		return false
	}
	return SPECIES[idx].Size == SizeLarge
}

// SPECIES is the 3 size-only fish types. Behaviour archetypes are reused
// from the old named species: small swims like the anchovy (fast,
// strongly aligned ribbons), medium mills like the sardine (rotating bait
// ball), large drifts like the mackerel (small, loose groups).
// SchoolSize[0] is the minimum school size; SchoolSize[1] is 0 meaning no
// maximum — schools only break when chased by predators.
var SPECIES = []Species{
	// Paces run 1.5x the original port so the tank reads brisk;
	// sharks and schooling predators scale by the same factor, so
	// hunts still close down.
	{Name: "small", Cruise: 69, MaxSpeed: 117, MinSpeed: 33, Weight: 1,
		SchoolSize: [2]int{20, 0}, Spacing: [2]float64{1.5, 1},
		Align: 3.2, Cohesion: 0.45, Jitter: 30, Mill: 0, VerticalDrag: 0.7,
		Size: SizeSmall, Glyph: ','},
	{Name: "medium", Cruise: 54, MaxSpeed: 96, MinSpeed: 27, Weight: 1,
		SchoolSize: [2]int{12, 0}, Spacing: [2]float64{1.8, 1.1},
		Align: 1.6, Cohesion: 1.6, Jitter: 40, Mill: 70, VerticalDrag: 0.4,
		Size: SizeMedium, Glyph: '-'},
	{Name: "large", Cruise: 48, MaxSpeed: 90, MinSpeed: 24, Weight: 1,
		SchoolSize: [2]int{7, 0}, Spacing: [2]float64{2.6, 1.3},
		Align: 2, Cohesion: 0.9, Jitter: 25, Mill: 0, VerticalDrag: 1,
		Size: SizeLarge, Glyph: '='},
}

var totalWeight = func() float64 {
	sum := 0.0
	for _, s := range SPECIES {
		sum += s.Weight
	}
	return sum
}()

// PickSpecies returns a random index into SPECIES, weighted by Weight.
func PickSpecies() int {
	r := rand.Float64() * totalWeight
	for i := range SPECIES {
		r -= SPECIES[i].Weight
		if r <= 0 {
			return i
		}
	}
	return 0
}
