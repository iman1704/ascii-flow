package world

import "testing"

// Hunter toggles must each gate their own group while keeping bait fish:
// "all" leaves out sharks and pelagics, "sharks" only sharks,
// "pelagics" only tuna and mackerel.
func TestHunterTogglesStayEmpty(t *testing.T) {
	cases := []struct {
		name              string
		noSharks, noPelag bool
		wantSharks        bool
		wantPredators     bool
	}{
		{"all", true, true, false, false},
		{"sharks", true, false, false, true},
		{"pelagics", false, true, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWorld(nil, 30)
			w.NoSharks = c.noSharks
			w.NoPelagics = c.noPelag
			w.Resize(80, 24)
			if got := len(w.Sharks) != 0; got == c.wantSharks {
				t.Fatalf("sharks present = %v, want present = %v", !got, c.wantSharks)
			}
			if got := len(w.Predators) != 0; got == c.wantPredators {
				t.Fatalf("predators present = %v, want present = %v", !got, c.wantPredators)
			}
			if len(w.Fish) == 0 {
				t.Fatal("expected bait fish to still spawn")
			}
			for i := 0; i < 120; i++ {
				pinCanvasChurn(w)
				w.Step(1.0 / 60)
			}
			if got := len(w.Sharks) != 0; got == c.wantSharks {
				t.Fatalf("sharks present after steps = %v, want present = %v", !got, c.wantSharks)
			}
			if got := len(w.Predators) != 0; got == c.wantPredators {
				t.Fatalf("predators present after steps = %v, want present = %v", !got, c.wantPredators)
			}
		})
	}
}
