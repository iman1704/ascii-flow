# math module (src/math.ts)

Small numeric helpers and the ocean-current field used by the simulation.

## API

- `TAU: float` — 2π constant.
- `clamp(v, lo, hi) -> float` — Constrain `v` to `[lo, hi]`.
- `rand(lo, hi) -> float` — Uniform random in `[lo, hi)`.
- `randInt(lo, hi) -> int` — Uniform random integer in `[lo, hi]`.
- `smoothstep(e0, e1, x) -> float` — Smooth 0→1 interpolation.
- `angleDiff(a, b) -> float` — Shortest signed angle from `a` to `b`, in (−π, π].
- `hash3(x, y, z) -> float` — Deterministic integer hash to `[0, 1)`.
- `current(x, y, t, out) -> out` — Divergence-free ocean current at position `(x, y)` and time `t`; writes velocity into `out {x, y}` and returns it.
