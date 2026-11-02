# react module

React binding that mounts the ocean simulation onto a `<canvas>`.

## API

- `AsciiOcean({ className?, sharks? }) -> JSX.Element`
  - `className?: string` — appended to the canvas's `ascii-ocean` class. Parent should use a monospace font.
  - `sharks?: SharkSpec[]` — initial shark roster, fixed for the canvas lifetime.
  - Renders `<canvas aria-hidden="true">`, creates an `OceanEngine` on mount, calls `engine.destroy()` on unmount.
