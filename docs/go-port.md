# Go port mapping

This package is the Go port of [stackedlol/ascii-sharks](https://github.com/stackedlol/ascii-sharks),
as integrated by [yxflc11/dsh-asciishark](https://github.com/yxflc11/dsh-asciishark)
for the DeepSeek Harness client. The `docs/*.md` module pages
below describe that upstream TypeScript source; this file records how each
module maps onto a Go package.

Target stack (AGENTS.md): Cobra (`start` command + flags),
`golang.org/x/term` fullscreen raw output, `time.Ticker` ~30fps loop.

| TS module (docs/) | Go package | Status | Notes |
|---|---|---|---|
| `math` ([docs/math.md](docs/math.md)) | `internal/mathx` | ported | `Hash3` uses `uint32` math to reproduce `Math.imul` wrap; `current` keeps the `out`-param shape as `*Vec` |
| `species` ([docs/species.md](docs/species.md)) | `internal/species` | ported | values copied verbatim; `math/rand/v2` backs `PickSpecies` |
| `world` ([docs/world.md](docs/world.md)) | `internal/world` | scaffolded | types + `NewWorld`/`Resize`/`Seed`/`Splash`/`Step` signatures; behaviour TODO against `src/world.ts`. Arena units change px → cells, so px-tuned constants must be rescaled |
| `renderer` ([docs/renderer.md](docs/renderer.md)) | `internal/renderer` | scaffolded | cell grid maps 1:1 (no `cw`/`ch`/dpr/font metrics); wake field, colour ramps, supersampled shark rasteriser TODO against `src/renderer.ts` |
| `engine` ([docs/engine.md](docs/engine.md)) | `internal/engine` | scaffolded | fixed-step `STEP`/`MAX_STEPS` accumulator kept; `rAF` → `Ticker`, `ResizeObserver` → `SIGWINCH` + `GetSize`, pointer → timer `Splash`, exit on any key |
| `react` ([docs/react.md](docs/react.md)) | — | dropped | no React in a CLI; nothing to port |
| `index` barrel + `demo.ts` | `main.go`, `cmd/` | ported | `?sharks=` parsing → `--sharks` flag (same `1 - i*0.1` sizing); `--fish-count`, `--fps` added |

Run it: `go run . start` (any key exits), `go run . start --sharks thresher,hammerhead,mako`.
