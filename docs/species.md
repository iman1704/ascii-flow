# species module

Fish species definitions and weighted random selection for new schools.

## API

- `Species` — record with fields: name, cruise/maxSpeed/minSpeed, weight, schoolSize[2], spacing[2], align, cohesion, jitter, mill, verticalDrag.
- `SPECIES: Species[]` — the 5 built-in species (sardine, anchovy, silverside, mackerel, goby).
- `pickSpecies() -> number` — returns a random index into SPECIES, weighted by each species' weight.
