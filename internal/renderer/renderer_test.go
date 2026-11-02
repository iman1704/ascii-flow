package renderer

import (
	"strings"
	"testing"

	"github.com/stackedlol/ascii-sharks/internal/world"
)

// Depth moves on screen: far fish lift toward the horizon while near
// fish hold their row. Size+fade carries depth; position shifts with it.
func TestFishParallaxShift(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 20, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 0.4},
			{X: 30, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 1},
		},
	}
	r.markFish(w)
	nearIdx := r.cellIndex(30, 20)
	farIdx := r.cellIndex(20, 20-(1-0.4)*fishParallaxRows)
	foundNear, foundFar := false, false
	for _, bucket := range r.fishBuckets {
		for _, i := range bucket {
			if i == nearIdx {
				foundNear = true
			}
			if i == farIdx {
				foundFar = true
			}
		}
	}
	if !foundNear {
		t.Fatalf("near fish missing at row 20 (idx %d)", nearIdx)
	}
	if !foundFar {
		t.Fatalf("far fish missing at lifted row (idx %d)", farIdx)
	}
	if farIdx == r.cellIndex(20, 20) {
		t.Fatal("far fish did not shift rows with depth")
	}
}

// Depth fade is dramatic and neutral: far fish render much dimmer than
// near fish, with only a whisper of cool tint so brightness/size dominate.
func TestFishDepthTintDramatic(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 10, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 0.35},
			{X: 40, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 1},
		},
	}
	r.markFish(w)
	r.drawFish()
	farIdx := r.cellIndex(10, 20-(1-0.35)*fishParallaxRows)
	nearIdx := r.cellIndex(40, 20)
	far, near := r.cells[farIdx], r.cells[nearIdx]
	if !far.hasFg || !near.hasFg {
		t.Fatalf("fish cells missing fg: far=%v near=%v", far.hasFg, near.hasFg)
	}
	farSum := far.fg[0] + far.fg[1] + far.fg[2]
	nearSum := near.fg[0] + near.fg[1] + near.fg[2]
	if farSum == 0 || nearSum == 0 {
		t.Fatalf("fish fg black: far=%v near=%v", far.fg, near.fg)
	}
	if float64(nearSum) < 3*float64(farSum) {
		t.Fatalf("depth fade not dramatic: near=%d far=%d", nearSum, farSum)
	}
	// Neutral fade: only a whisper of blue, so size+brightness dominate.
	if d := far.fg[2] - far.fg[0]; d < 0 || d > 30 {
		t.Fatalf("far fish tint not neutral fade: fg=%v", far.fg)
	}
}

// Depth: the same shark spine covers more cells when near than when far.
func TestSharkDepthScalesRaster(t *testing.T) {
	near := testShark(world.Classic, 20, 30, 15)
	near.Depth = 1
	far := testShark(world.Classic, 20, 30, 15)
	far.Depth = 0
	if countBody(rasterFlagsFor(near)) <= countBody(rasterFlagsFor(far)) {
		t.Fatal("near shark raster not larger than far shark raster")
	}
}

func rasterFlagsFor(sh world.Shark) []uint8 {
	r := New()
	r.Resize(60, 30)
	r.rasterizeSharks([]world.Shark{sh}, 0)
	out := make([]uint8, len(r.sharkFlag))
	copy(out, r.sharkFlag)
	return out
}

// A straight horizontal shark spine of the given length centred at (x, y).
func testShark(kind world.SharkKind, length, x, y float64) world.Shark {
	nodes := make([]float64, world.SHARK_NODES*2)
	seg := length / float64(world.SHARK_NODES-1)
	for k := 0; k < world.SHARK_NODES; k++ {
		nodes[k*2] = x - seg*float64(k)
		nodes[k*2+1] = y
	}
	return world.Shark{
		ID: 1, Kind: kind, Length: length, Nodes: nodes,
		Heading: 0, Speed: 30 / pxCell, Depth: 1,
	}
}

func rasterFlags(kind world.SharkKind) []uint8 {
	r := New()
	r.Resize(60, 30)
	sh := testShark(kind, 20, 30, 15)
	r.rasterizeSharks([]world.Shark{sh}, 0)
	out := make([]uint8, len(r.sharkFlag))
	copy(out, r.sharkFlag)
	return out
}

func countBody(flags []uint8) int {
	n := 0
	for _, f := range flags {
		if f == flagBody || f == flagMark || f == flagRidge || f == flagFin || f == flagEye {
			n++
		}
	}
	return n
}

// Different shark species must rasterise differently: the old placeholder
// drew every kind as the same '#' spine, so a hammerhead's cephalofoil and
// the per-species fin/body plans had no visual effect.
func TestSharkSpeciesRasterDiffer(t *testing.T) {
	classic := rasterFlags(world.Classic)
	hammer := rasterFlags(world.Hammerhead)
	if countBody(classic) == 0 {
		t.Fatal("classic shark rasterised no body cells")
	}
	same := len(classic) == len(hammer)
	if same {
		for i := range classic {
			if classic[i] != hammer[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatal("hammerhead raster identical to classic; species shapes have no effect")
	}
	if shapes["hammerhead"].head == nil {
		t.Fatal("hammerhead shape lost its cephalofoil")
	}
	if shapes["classic"].head != nil {
		t.Fatal("classic shape should have no cephalofoil")
	}
}

// Depth must dominate alarm in fish opacity: a far panicking fish stays
// ghostly next to a near calm one, so depth reads at a glance.
func TestFishDepthDominatesAlarm(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 10, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 0.35, Alarm: 1, Turn: 5},
			{X: 40, Y: 20, Vx: 1, Species: 0, Alive: true, Depth: 1},
		},
	}
	r.markFish(w)
	bucketOf := func(want int) int {
		for b, bucket := range r.fishBuckets {
			for _, i := range bucket {
				if i == want {
					return b
				}
			}
		}
		return -1
	}
	farIdx := r.cellIndex(10, 20-(1-0.35)*fishParallaxRows)
	nearIdx := r.cellIndex(40, 20)
	far, near := bucketOf(farIdx), bucketOf(nearIdx)
	if far < 0 || near < 0 {
		t.Fatalf("fish missing from buckets: far=%d near=%d", far, near)
	}
	if far >= near {
		t.Fatalf("far panicking fish not dimmer than near calm one: far=%d near=%d", far, near)
	}
	if near < 3 {
		t.Fatalf("near calm fish not solid: bucket %d", near)
	}
	if far > 1 {
		t.Fatalf("far fish not ghostly: bucket %d", far)
	}
}

// Depth must scale shark size+fade dramatically: the same spine renders
// much brighter and larger up close than far away, with only a whisper
// of cool tint so fade dominates.
func TestSharkDepthDimsBody(t *testing.T) {
	maxBg := func(depth float64) (int, [3]int) {
		r := New()
		r.Resize(60, 30)
		sh := testShark(world.Classic, 20, 30, 15)
		sh.Depth = depth
		r.rasterizeSharks([]world.Shark{sh}, 0)
		r.drawSharks(0)
		best := 0
		var bestBg [3]int
		for _, c := range r.cells {
			if !c.hasBg {
				continue
			}
			if s := c.bg[0] + c.bg[1] + c.bg[2]; s > best {
				best = s
				bestBg = c.bg
			}
		}
		return best, bestBg
	}
	near, nearBg := maxBg(1)
	far, farBg := maxBg(0)
	if near == 0 || far == 0 {
		t.Fatalf("shark drew no body cells: near=%d far=%d", near, far)
	}
	if float64(near) < 3.5*float64(far) {
		t.Fatalf("near shark not dramatically brighter than far one: near=%d far=%d", near, far)
	}
	if countBody(rasterFlagsFor(testSharkAtDepth(1))) <= countBody(rasterFlagsFor(testSharkAtDepth(0))) {
		t.Fatal("near shark raster not larger than far shark raster")
	}
	// Whisper of cool: far is bluer than near, but not strongly blue.
	farBlue := float64(farBg[2]) / float64(farBg[0]+1)
	nearBlue := float64(nearBg[2]) / float64(nearBg[0]+1)
	if farBlue < 1.15 || farBlue >= 1.5 {
		t.Fatalf("far shark tint not subtle cool: bg=%v", farBg)
	}
	if farBlue <= nearBlue {
		t.Fatalf("far shark not bluer than near: far=%v near=%v", farBg, nearBg)
	}
}

func testSharkAtDepth(depth float64) world.Shark {
	sh := testShark(world.Classic, 20, 30, 15)
	sh.Depth = depth
	return sh
}

// Turning/panicking fish must bucket brighter than calm far fish, so hard
// turns glint instead of every fish drawing the same dash.
func TestFishBrightnessBucketing(t *testing.T) {
	r := New()
	r.Resize(60, 30)
	w := &world.World{
		Width: 60, Height: 30,
		Fish: []world.Fish{
			{X: 10, Y: 10, Vx: 1, Species: 0, Alive: true, Depth: 0.55},
			{X: 40, Y: 10, Vx: 1, Species: 0, Alive: true, Depth: 1, Alarm: 1, Turn: 5},
		},
	}
	r.markFish(w)
	calm, panic := -1, -1
	// Depth lifts: far calm fish render above their Y row.
	calmIdx := r.cellIndex(10, 10-(1-0.55)*fishParallaxRows)
	panicIdx := r.cellIndex(40, 10)
	for b, bucket := range r.fishBuckets {
		for _, i := range bucket {
			if i == calmIdx {
				calm = b
			}
			if i == panicIdx {
				panic = b
			}
		}
	}
	if calm < 0 || panic < 0 {
		t.Fatalf("fish missing from buckets: calm=%d panic=%d", calm, panic)
	}
	if panic <= calm {
		t.Fatalf("panicking fish not brighter: calm bucket %d, panic bucket %d", calm, panic)
	}
}

// Task 9: consecutive frames of a live ocean must differ, so the program
// visibly moves: fish swim, sharks hunt, and the wake diffuses.
func TestFramesChangeOverTime(t *testing.T) {
	w := world.NewWorld([]world.SharkSpec{{Kind: world.Classic, Size: 1}}, 40)
	w.Resize(80, 24)
	r := New()
	r.Resize(80, 24)
	first := r.Render(w, 1.0/30)
	for i := 0; i < 30; i++ {
		w.Step(1.0 / 60)
	}
	if second := r.Render(w, 1.0/30); first == second {
		t.Fatal("ocean frames identical after 30 steps; nothing moves")
	}
}

// Task 7: resizing to a different shape with the same cell count must still
// rebuild the grid buffers and frame at the new dimensions. An 80x24 grid
// and a 96x20 grid both hold 1920 cells but index them differently, so
// keeping the old buffers would scatter wake, heat and shark layers.
func TestResizeReshapesSameAreaBuffers(t *testing.T) {
	r := New()
	r.Resize(80, 24)
	r.energy[0] = 1.6
	r.Resize(96, 20)
	if len(r.energy) != 96*20 || len(r.cells) != 96*20 {
		t.Fatalf("buffers not reshaped: energy=%d cells=%d", len(r.energy), len(r.cells))
	}
	for i, e := range r.energy {
		if e != 0 {
			t.Fatalf("stale wake energy survived reshape at cell %d", i)
		}
	}
	w := world.NewWorld([]world.SharkSpec{{Kind: world.Classic, Size: 1}}, 40)
	w.Resize(96, 20)
	for i := 0; i < 5; i++ {
		w.Step(1.0 / 60)
	}
	frame := r.Render(w, 1.0/30)
	if got := strings.Count(frame, "\x1b[K"); got != 20 {
		t.Fatalf("reshaped frame has %d row clears, want 20", got)
	}
}

// A seeded ocean must frame as a coloured multi-glyph picture: fish
// size-class glyphs (',' small, '-' medium, '=' large), shark
// skin/fin/edge glyphs, and wake dots after a splash.
func TestRenderFrameLayers(t *testing.T) {
	w := world.NewWorld([]world.SharkSpec{{Kind: world.Classic, Size: 1}}, 40)
	w.Resize(80, 24)
	for i := 0; i < 5; i++ {
		w.Step(1.0 / 60)
	}
	w.Splash(40, 12)
	r := New()
	r.Resize(80, 24)
	frame := r.Render(w, 1.0/30)

	if !strings.HasPrefix(frame, "\x1b[H") {
		t.Fatal("frame missing cursor-home prefix")
	}
	if !strings.Contains(frame, "\x1b[") {
		t.Fatal("frame has no ANSI colour; species shading lost")
	}
	if !strings.ContainsAny(frame, ",-=") {
		t.Fatal("frame has no fish size-class glyphs")
	}
	if !strings.ContainsAny(frame, skinRamp+finGlyphs+ridgeGlyphs+edgeHeavy+edgeLight) {
		t.Fatal("frame has no shark body/fin/edge glyphs")
	}
	// SKIN_RAMP itself contains a single '#' for dark skin cells, so only
	// a run of them means the old placeholder spine is back.
	if strings.Contains(frame, "#####") {
		t.Fatal("frame still uses placeholder '#' shark blocks")
	}
	if !strings.Contains(frame, "·") && !strings.Contains(frame, "*") {
		t.Fatal("frame has no wake/heat dots after a splash")
	}
	if got := strings.Count(frame, "\x1b[K"); got != 24 {
		t.Fatalf("expected 24 row clears, got %d", got)
	}
}
