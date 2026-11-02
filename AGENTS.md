# Tech Stack

- Cobra — CLI framework (`start` command and flags).
- golang.org/x/term — fullscreen raw terminal output (hide cursor, raw mode).
- time.Ticker — frame timing (~30fps playback loop).

# Project Structure

- `main.go` — binary entry point, delegates to `cmd.Execute()`.
- `cmd/` — Cobra command tree.
  - `cmd/root.go` — root command (`asciiflow`); registers subcommands.
  - `cmd/start.go` — `start` command; parses `--sharks`, `--fish-count`, `--fps` flags and runs the screensaver.
- `internal/` — core modules.
  - `internal/engine/` — fixed-timestep run loop wiring `World` to `Renderer`; owns terminal setup (raw mode, alternate screen), SIGWINCH resize, keypress exit.
  - `internal/world/` — artificial-life simulation: boid fish schools and steering sharks, arena state, `Step`/`Seed`/`Splash`.
  - `internal/species/` — fish species definitions and weighted random school selection.
  - `internal/mathx/` — numeric helpers (`Clamp`, `Rand`, `Smoothstep`, `AngleDiff`, `Hash3`) and divergence-free ocean current field.
  - `internal/renderer/` — ASCII grid renderer: rasterises `World` state into rune + ANSI frames for stdout.


# Notes
- This is the Go port of [stackedlol/ascii-sharks](https://github.com/stackedlol/ascii-sharks). Keep that attribution in reader-facing docs.
- Ignore any content in `src/` unless explicitly I told to check them 
- Check for pending tasks in TODO.md. Do not do any of them unless explicitly told to. Mark task as finished after finishing it.
