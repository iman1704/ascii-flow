package world

import (
	"math"
	"testing"
)

// Calm fish cruise briskly: a lone small fish with no threats, neighbours
// or plankton settles near its species cruise pace (~5.75 cells/s), well
// above a dawdle. Speeds below that mean the pace constants regressed.
func TestFishCruiseFaster(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(120, 40)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = []Fish{{
		X: 60, Y: 20, Vx: 3, Vy: 0, Species: 0,
		Alive: true, Depth: 0.8, MillDir: 1,
	}}
	w.FeedingSpots = nil
	// Pin canvas churn: this measures one isolated fish, and an exit
	// wave would remove the very fish under test.
	pinCanvasChurn(w)
	var sum float64
	var n int
	for i := 0; i < 1200; i++ {
		w.Step(1.0 / 60)
		w.FeedingSpots = nil
		if i > 120 { // settled past the spin-up
			f := &w.Fish[0]
			sum += math.Hypot(f.Vx, f.Vy)
			n++
		}
	}
	got := sum / float64(n)
	if got < 5.0 || got > 7.0 {
		t.Fatalf("lone small fish cruised at %.2f cells/s, want brisk pace in [5.0, 7.0]", got)
	}
}

// A large school must translate, not just boil: 25 small fish in open
// water cruise with a centroid speed near the species pace instead of
// crawling while the centroid sits still.
func TestSchoolTranslatesAtCruise(t *testing.T) {
	w := NewWorld([]SharkSpec{}, 0)
	w.Resize(240, 100)
	w.Sharks = nil
	w.Predators = nil
	w.Fish = nil
	for k := 0; k < 25; k++ {
		w.Fish = append(w.Fish, Fish{
			X: 60 + float64(k%8), Y: 50 + float64(k/8),
			Vx: 5, Vy: 0, Species: 0, Alive: true,
			Depth: 0.8, MillDir: 1,
		})
	}
	w.FeedingSpots = nil
	// Pin canvas churn: exits would drain the school under test and
	// entries would fold newcomers into the centroid.
	pinCanvasChurn(w)
	centroid := func() (float64, float64) {
		var sx, sy float64
		for j := range w.Fish {
			sx += w.Fish[j].X
			sy += w.Fish[j].Y
		}
		return sx / float64(len(w.Fish)), sy / float64(len(w.Fish))
	}
	var sum float64
	var n int
	px, py := centroid()
	for i := 0; i < 1200; i++ {
		w.Step(1.0 / 60)
		w.FeedingSpots = nil
		cx, cy := centroid()
		if i > 120 { // settled past the spin-up
			sum += math.Hypot(cx-px, cy-py) * 60
			n++
		}
		px, py = cx, cy
	}
	if got := sum / float64(n); got < 4.5 {
		t.Fatalf("school centroid crawled at %.2f cells/s, want translation near cruise (>= 4.5)", got)
	}
}
