package renderer

import (
	"testing"

	"github.com/iman1704/ascii-flow/internal/world"
)

// Schooling predators must rasterise as bodies, and each species must draw
// differently from sharks and from each other: the old placeholder risk is
// a tuna that is pixel-identical to a small classic shark.
func TestPredatorSpeciesRasterDiffer(t *testing.T) {
	tuna := predatorRasterFlags("tuna", 8)
	mackerel := predatorRasterFlags("mackerel", 8)
	shark := rasterFlags(world.Classic)
	_ = shark
	sharkSmall := func() []uint8 {
		r := New()
		r.Resize(60, 30)
		sh := testShark(world.Classic, 8, 30, 15)
		r.rasterizeSharks([]world.Shark{sh}, 0)
		out := make([]uint8, len(r.sharkFlag))
		copy(out, r.sharkFlag)
		return out
	}()
	if countBody(tuna) == 0 {
		t.Fatal("tuna rasterised no body cells")
	}
	if countBody(mackerel) == 0 {
		t.Fatal("mackerel rasterised no body cells")
	}
	if sameFlags(tuna, mackerel) {
		t.Fatal("tuna raster identical to mackerel; predator shapes have no effect")
	}
	if sameFlags(tuna, sharkSmall) {
		t.Fatal("tuna raster identical to classic shark; predator shape recycles the shark design")
	}
	if shapes["tuna"].pectoral[3] <= shapes["classic"].pectoral[3] {
		t.Fatal("tuna lost its long albacore pectorals")
	}
}

func predatorRasterFlags(kind string, length float64) []uint8 {
	r := New()
	r.Resize(60, 30)
	p := testPredator(kind, length)
	r.rasterizePredators([]world.Predator{p}, 0)
	out := make([]uint8, len(r.sharkFlag))
	copy(out, r.sharkFlag)
	return out
}

// testPredator builds a straight horizontal predator spine of the given
// length centred at (30, 15), heading east.
func testPredator(kind string, length float64) world.Predator {
	nodes := make([]float64, world.SHARK_NODES*2)
	seg := length / float64(world.SHARK_NODES-1)
	for k := 0; k < world.SHARK_NODES; k++ {
		nodes[k*2] = 30 - seg*float64(k)
		nodes[k*2+1] = 15
	}
	return world.Predator{
		ID: 1, Kind: world.PredatorKind(kind), Length: length, Nodes: nodes,
		Heading: 0, Speed: 30 / pxCell, Depth: 1, Alive: true,
	}
}

func sameFlags(a, b []uint8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
