# engine module

Wires the simulation (`World`) to canvas output (`AsciiRenderer`): fixed-timestep
game loop, canvas resize, pointer input, off-screen pausing, and
reduced-motion support.

## API

- `class OceanEngine`
  - `constructor(canvas, sharks?)` — Create and start the loop bound to a canvas element. `sharks` optionally overrides the default shark population.
  - `destroy()` — Stop the loop and detach all listeners/observers. Returns nothing.
