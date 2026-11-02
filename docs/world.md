# world

Artificial-life simulation: fish schools (boids) and hunting sharks.

## API

- `const SHARK_NODES: int` — spine nodes per shark (20).
- `type SharkKind = string` — one of classic, hammerhead, zebra, tiger, greathammer, mako, thresher, nurse.
- `type SharkSpec { kind: SharkKind, size: number }` — which sharks to spawn; size relative to biggest.
- `type Fish { x, y, vx, vy: number, species: int, alarm: number (0-1), alive: bool, respawnAt: number, phase: number, turn: number, depth: number, millDir: +1 | -1 }`
- `type Shark { id: int, kind: SharkKind, length: number, nodes: float[] (snout-first xy pairs), heading, speed, phase, turn, hunger: number, wander: number, orbitDir: +1 | -1, orbitRadius: number, target: Fish | null, retargetAt: number, depth: number }`
- `type WorldEvent { kind: "eat" | "splash", x, y: number }`
- `class World(sharkSpecs?: SharkSpec[], fishCount?: int)`
  - `width, height, time: number; fish: Fish[]; sharks: Shark[]; events: WorldEvent[]; pointer { x, y, active }`
  - `resize(width, height: number, cellW, cellH: number) -> void` — set arena size; seeds on first call.
  - `seed() -> void` — spawn sharks and fish schools.
  - `splash(x, y: number) -> void` — scare nearby fish outward.
  - `step(dt: number) -> void` — advance fish, sharks, respawns by dt seconds.
