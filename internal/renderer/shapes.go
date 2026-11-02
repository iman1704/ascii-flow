// Package renderer ports src/renderer.ts: drawing the World as an ASCII grid.
//
// TERMINAL MAPPING: the canvas version rasterises into device pixels
// (cellW/cellH from measured font metrics, dpr scaling, 2D fillText with
// per-glyph RGB + alpha, supersampled shark edges). Here a terminal cell IS
// the grid unit, so Resize takes cols/rows directly and Render returns a
// string of runes + ANSI SGR escapes written to stdout. There is no
// font/DPR handling. The wake field, colour ramps and supersampled shark
// rasteriser are ported from src/renderer.ts into cell units.
package renderer

import (
	"math"

	"github.com/iman1704/ascii-flow/internal/mathx"
)

// Cell classification for shark raster cells. Ports FLAG_*.
const (
	flagBody  = 1
	flagEdge  = 2
	flagFin   = 3
	flagRidge = 4
	flagEye   = 5
	flagMark  = 6
)

var (
	dotRGB = [5][3]float64{
		{48, 52, 235},
		{52, 205, 60},
		{228, 50, 44},
		{190, 212, 255},
		{110, 230, 255},
	}
	wakeIndex = len(dotRGB) - 1
)

const alphaLevels = 6

var accentRGB = [7][3]int{
	{0xe0, 0x89, 0x2c}, {0xe0, 0x3a, 0x32}, {0xb4, 0x4a, 0xe0},
	{0x33, 0xc8, 0xcc}, {0x3c, 0xb5, 0x4a}, {0x31, 0x50, 0xe8},
	{0xd9, 0xc1, 0x4a},
}

// Sparse glyphs on bright skin, dense glyphs in shadow.
const skinRamp = ":;t%SX8#@"

const finGlyphs = "%;tS:"
const ridgeGlyphs = "@8%"
const edgeHeavy = ":;"
const edgeLight = ".,'`"
const markGlyphs = "#%@"

// Sub-cell sample offsets for anti-aliased silhouettes.
var subsamples = [8]float64{0.25, 0.3, 0.75, 0.3, 0.25, 0.72, 0.75, 0.72}

// SharkShape ports the SharkShape interface: body outline and fin layout.
// All values are fractions of length.
type SharkShape struct {
	halfWidth    func(s float64) float64
	pectoral     [4]float64
	falcate      bool
	pelvic       [4]float64
	caudal       float64
	dorsal       [2]float64
	secondDorsal [2]float64
	head         *sharkHead
	eye          *[2]float64
	gills        [2]float64
	tint         [3]float64
}

type sharkHead struct {
	span  float64
	sweep float64
	depth float64
	notch float64
}

func classicHalfWidth(s float64) float64 {
	if s < 0.13 {
		return 0.09 * math.Pow(math.Sin((s/0.13)*math.Pi/2), 0.95)
	}
	if s < 0.3 {
		return 0.09 + 0.016*mathx.Smoothstep(0.13, 0.3, s)
	}
	if s < 0.84 {
		return 0.019 + 0.087*math.Pow(1-(s-0.3)/0.54, 1.25)
	}
	k := (s - 0.84) / 0.16
	return 0.019*(1-k) + 0.012*math.Sin(k*math.Pi)
}

func inTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (px-bx)*(ay-by) - (ax-bx)*(py-by)
	d2 := (px-cx)*(by-cy) - (bx-cx)*(py-cy)
	d3 := (px-ax)*(cy-ay) - (cx-ax)*(py-ay)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

// shapes ports SHAPES: body outline and fin layout per shark kind.
// All values are fractions of length.
var shapes = map[string]*SharkShape{
	"classic": {
		halfWidth: classicHalfWidth,
		pectoral:  [4]float64{0.15, 0.43, 0.55, 0.26},
		falcate:   true,
		pelvic:    [4]float64{0.57, 0.65, 0.69, 0.085},
		caudal:    0.085,
		dorsal:    [2]float64{0.33, 0.47}, secondDorsal: [2]float64{0.71, 0.76},
		eye: &[2]float64{0.1, 0.14}, gills: [2]float64{0.19, 0.29},
		tint: [3]float64{0.92, 0.97, 1.05},
	},
	"hammerhead": {
		halfWidth: func(s float64) float64 {
			if s < 0.06 {
				return 0.045
			}
			if s < 0.3 {
				return 0.045 + 0.061*mathx.Smoothstep(0.06, 0.3, s)
			}
			return classicHalfWidth(s) * 0.92
		},
		head:     &sharkHead{span: 0.17, sweep: 0.035, depth: 0.09, notch: 0},
		pectoral: [4]float64{0.17, 0.4, 0.5, 0.22},
		falcate:  true,
		pelvic:   [4]float64{0.56, 0.63, 0.67, 0.075},
		caudal:   0.11,
		dorsal:   [2]float64{0.28, 0.44}, secondDorsal: [2]float64{0.7, 0.74},
		gills: [2]float64{0.16, 0.26},
		tint:  [3]float64{0.96, 0.98, 0.9},
	},
	"tiger": {
		halfWidth: func(s float64) float64 {
			if s < 0.13 {
				return 0.1 * math.Pow(math.Sin((s/0.13)*math.Pi/2), 0.45)
			}
			if s < 0.3 {
				return 0.1 + 0.014*mathx.Smoothstep(0.13, 0.3, s)
			}
			if s < 0.84 {
				return 0.019 + 0.095*math.Pow(1-(s-0.3)/0.54, 1.2)
			}
			k := (s - 0.84) / 0.16
			return 0.019*(1-k) + 0.012*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.15, 0.42, 0.53, 0.25},
		falcate:  true,
		pelvic:   [4]float64{0.57, 0.65, 0.69, 0.085},
		caudal:   0.095,
		dorsal:   [2]float64{0.33, 0.46}, secondDorsal: [2]float64{0.71, 0.75},
		eye: &[2]float64{0.08, 0.12}, gills: [2]float64{0.19, 0.28},
		tint: [3]float64{1.02, 0.95, 0.84},
	},
	"zebra": {
		halfWidth: func(s float64) float64 {
			if s < 0.1 {
				return 0.075 * math.Pow(math.Sin((s/0.1)*math.Pi/2), 0.7)
			}
			if s < 0.28 {
				return 0.075 + 0.025*mathx.Smoothstep(0.1, 0.28, s)
			}
			if s < 0.55 {
				return 0.022 + 0.078*math.Pow(1-(s-0.28)/0.27, 1.1)
			}
			k := (s - 0.55) / 0.45
			return 0.022*(1-k*k) + 0.012*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.12, 0.3, 0.36, 0.19},
		pelvic:   [4]float64{0.42, 0.48, 0.51, 0.065},
		dorsal:   [2]float64{0.36, 0.44}, secondDorsal: [2]float64{0.49, 0.53},
		eye: &[2]float64{0.06, 0.09}, gills: [2]float64{0.13, 0.2},
		tint: [3]float64{1.08, 0.97, 0.74},
	},
	"greathammer": {
		halfWidth: func(s float64) float64 {
			if s < 0.05 {
				return 0.04
			}
			if s < 0.3 {
				return 0.04 + 0.07*mathx.Smoothstep(0.05, 0.3, s)
			}
			return classicHalfWidth(s) * 0.95
		},
		head:     &sharkHead{span: 0.21, sweep: 0.006, depth: 0.085, notch: 0.014},
		pectoral: [4]float64{0.16, 0.38, 0.49, 0.24},
		falcate:  true,
		pelvic:   [4]float64{0.55, 0.62, 0.67, 0.085},
		caudal:   0.13,
		dorsal:   [2]float64{0.24, 0.45}, secondDorsal: [2]float64{0.68, 0.74},
		gills: [2]float64{0.15, 0.25},
		tint:  [3]float64{0.86, 0.93, 1.02},
	},
	"mako": {
		halfWidth: func(s float64) float64 {
			if s < 0.17 {
				return 0.088 * math.Pow(math.Sin((s/0.17)*math.Pi/2), 1.5)
			}
			if s < 0.32 {
				return 0.088 + 0.012*mathx.Smoothstep(0.17, 0.32, s)
			}
			if s < 0.86 {
				return 0.014 + 0.086*math.Pow(1-(s-0.32)/0.54, 1.45)
			}
			k := (s - 0.86) / 0.14
			return 0.014*(1-k) + 0.01*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.18, 0.4, 0.5, 0.21},
		falcate:  true,
		pelvic:   [4]float64{0.58, 0.64, 0.68, 0.06},
		caudal:   0.13,
		dorsal:   [2]float64{0.34, 0.45}, secondDorsal: [2]float64{0.76, 0.79},
		eye: &[2]float64{0.11, 0.15}, gills: [2]float64{0.2, 0.3},
		tint: [3]float64{0.78, 0.9, 1.2},
	},
	"thresher": {
		halfWidth: func(s float64) float64 {
			if s < 0.08 {
				return 0.07 * math.Pow(math.Sin((s/0.08)*math.Pi/2), 0.75)
			}
			if s < 0.2 {
				return 0.07 + 0.02*mathx.Smoothstep(0.08, 0.2, s)
			}
			if s < 0.48 {
				return 0.012 + 0.078*math.Pow(1-(s-0.2)/0.28, 1.3)
			}
			k := (s - 0.48) / 0.52
			return 0.012*(1-k) + 0.006*math.Sin(k*math.Pi*0.9) + 0.002
		},
		pectoral: [4]float64{0.12, 0.27, 0.36, 0.2},
		falcate:  true,
		pelvic:   [4]float64{0.37, 0.41, 0.44, 0.045},
		dorsal:   [2]float64{0.24, 0.32}, secondDorsal: [2]float64{0.45, 0.47},
		eye: &[2]float64{0.05, 0.1}, gills: [2]float64{0.11, 0.18},
		tint: [3]float64{1.0, 0.9, 1.08},
	},
	"nurse": {
		halfWidth: func(s float64) float64 {
			if s < 0.08 {
				return 0.085 * math.Pow(math.Sin((s/0.08)*math.Pi/2), 0.4)
			}
			if s < 0.3 {
				return 0.085 + 0.02*mathx.Smoothstep(0.08, 0.3, s)
			}
			if s < 0.72 {
				return 0.022 + 0.083*math.Pow(1-(s-0.3)/0.42, 1.1)
			}
			k := (s - 0.72) / 0.28
			return 0.022*(1-k*k) + 0.014*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.16, 0.34, 0.4, 0.17},
		pelvic:   [4]float64{0.45, 0.52, 0.55, 0.075},
		dorsal:   [2]float64{0.5, 0.58}, secondDorsal: [2]float64{0.63, 0.69},
		eye: &[2]float64{0.05, 0.075}, gills: [2]float64{0.14, 0.22},
		tint: [3]float64{1.1, 0.96, 0.76},
	},
	// Albacore tuna: deep torpedo body, very long sickle pectorals, big
	// lunate tail, small dorsal plus a finlet row aft. No shark recycles
	// this plan: the pectoral reach (0.30) exceeds every shark's.
	"tuna": {
		halfWidth: func(s float64) float64 {
			if s < 0.12 {
				return 0.075 * math.Pow(math.Sin((s/0.12)*math.Pi/2), 0.8)
			}
			if s < 0.35 {
				return 0.075 + 0.012*mathx.Smoothstep(0.12, 0.35, s)
			}
			if s < 0.8 {
				return 0.012 + 0.075*math.Pow(1-(s-0.35)/0.45, 1.35)
			}
			k := (s - 0.8) / 0.2
			return 0.012*(1-k) + 0.008*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.18, 0.3, 0.44, 0.3},
		falcate:  true,
		pelvic:   [4]float64{0.55, 0.6, 0.63, 0.06},
		caudal:   0.16,
		dorsal:   [2]float64{0.35, 0.43}, secondDorsal: [2]float64{0.6, 0.64},
		eye: &[2]float64{0.07, 0.11}, gills: [2]float64{0.15, 0.24},
		tint: [3]float64{0.55, 0.78, 1.25},
	},
	// Spanish mackerel: slender pointed-snouth body, short rounded
	// pectorals, forked tail, narrow dorsal pair, barred flanks.
	"mackerel": {
		halfWidth: func(s float64) float64 {
			if s < 0.1 {
				return 0.055 * math.Pow(math.Sin((s/0.1)*math.Pi/2), 1.0)
			}
			if s < 0.3 {
				return 0.055 + 0.008*mathx.Smoothstep(0.1, 0.3, s)
			}
			if s < 0.8 {
				return 0.01 + 0.053*math.Pow(1-(s-0.3)/0.5, 1.4)
			}
			k := (s - 0.8) / 0.2
			return 0.01*(1-k) + 0.007*math.Sin(k*math.Pi)
		},
		pectoral: [4]float64{0.16, 0.24, 0.3, 0.13},
		pelvic:   [4]float64{0.5, 0.55, 0.58, 0.05},
		caudal:   0.13,
		dorsal:   [2]float64{0.28, 0.35}, secondDorsal: [2]float64{0.54, 0.57},
		eye: &[2]float64{0.06, 0.09}, gills: [2]float64{0.13, 0.2},
		tint: [3]float64{0.62, 1.02, 0.82},
	},
}

// kindOrder ports KINDS for stable kind indices, extended with the two
// schooling-predator kinds so they share the shark raster buffers.
var kindOrder = []string{
	"classic", "hammerhead", "tiger", "zebra",
	"greathammer", "mako", "thresher", "nurse",
	"tuna", "mackerel",
}

func kindIndex(kind string) int {
	for i, k := range kindOrder {
		if k == kind {
			return i
		}
	}
	return 0
}
