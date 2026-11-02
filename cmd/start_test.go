package cmd

import (
	"strings"
	"testing"
)

// The start command is the natural ocean by default: no --natural flag,
// no --sharks or --fish-count flags, and --no-predators selects which
// hunters stay out of the water.
func TestNoPredatorsValues(t *testing.T) {
	for _, value := range []string{"all", "sharks", "pelagics"} {
		c := newStartCmd()
		c.SilenceUsage = true
		c.SilenceErrors = true
		c.SetArgs([]string{"--no-predators", value})
		if err := c.Flags().Parse([]string{"--no-predators", value}); err != nil {
			t.Fatalf("--no-predators %q rejected: %v", value, err)
		}
	}
}

func TestNoPredatorsRejectsUnknown(t *testing.T) {
	c := newStartCmd()
	c.SilenceUsage = true
	c.SilenceErrors = true
	c.SetArgs([]string{"--no-predators", "kraken"})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "--no-predators") {
		t.Fatalf("args [--no-predators kraken] returned %v, want a --no-predators error", err)
	}
}

func TestRemovedFlagsAreGone(t *testing.T) {
	for _, name := range []string{"sharks", "fish-count", "natural"} {
		c := newStartCmd()
		if c.Flags().Lookup(name) != nil {
			t.Fatalf("--%s flag still registered", name)
		}
	}
}

func TestNoPredatorsMapping(t *testing.T) {
	cases := []struct {
		value             string
		noSharks, noPelag bool
	}{
		{"", false, false},
		{"all", true, true},
		{"sharks", true, false},
		{"pelagics", false, true},
	}
	for _, c := range cases {
		noSharks, noPelagics, err := parseNoPredators(c.value)
		if err != nil {
			t.Fatalf("parseNoPredators(%q) errored: %v", c.value, err)
		}
		if noSharks != c.noSharks || noPelagics != c.noPelag {
			t.Fatalf("parseNoPredators(%q) = (%v, %v), want (%v, %v)",
				c.value, noSharks, noPelagics, c.noSharks, c.noPelag)
		}
	}
	if _, _, err := parseNoPredators("kraken"); err == nil {
		t.Fatal("parseNoPredators accepted an unknown group")
	}
}
