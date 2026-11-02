package species

import "testing"

// Three size-only fish types: small (min 20), medium (min 12), large
// (min 7). No maximum school size. Only large is shark-edible. Glyphs are
// kept: ',' small, '-' medium, '=' large.
func TestSizeBuckets(t *testing.T) {
	if len(SPECIES) != 3 {
		t.Fatalf("len(SPECIES) = %d, want 3 size-only types", len(SPECIES))
	}
	want := []struct {
		name    string
		size    SizeClass
		min     int
		glyph   rune
		edible  bool
		milling bool
	}{
		{"small", SizeSmall, 20, ',', false, false},
		{"medium", SizeMedium, 12, '-', false, true},
		{"large", SizeLarge, 7, '=', true, false},
	}
	for i, w := range want {
		sp := SPECIES[i]
		if sp.Name != w.name {
			t.Fatalf("SPECIES[%d].Name = %q, want %q", i, sp.Name, w.name)
		}
		if sp.Size != w.size {
			t.Fatalf("type %q size = %q, want %q", w.name, sp.Size, w.size)
		}
		if sp.SchoolSize[0] != w.min {
			t.Fatalf("type %q min school = %d, want %d", w.name, sp.SchoolSize[0], w.min)
		}
		if sp.SchoolSize[1] != 0 {
			t.Fatalf("type %q max school = %d, want 0 (no maximum)", w.name, sp.SchoolSize[1])
		}
		if sp.Glyph != w.glyph {
			t.Fatalf("type %q glyph = %q, want %q", w.name, sp.Glyph, w.glyph)
		}
		if IsLarge(i) != w.edible {
			t.Fatalf("type %q IsLarge = %v, want %v", w.name, IsLarge(i), w.edible)
		}
		if (sp.Mill > 0) != w.milling {
			t.Fatalf("type %q milling = %v, want %v", w.name, sp.Mill > 0, w.milling)
		}
	}
	for _, idx := range []int{-1, 3, 99} {
		if IsLarge(idx) {
			t.Fatalf("type index %d should not be large", idx)
		}
	}
}
