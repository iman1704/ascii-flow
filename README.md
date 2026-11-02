# asciiflow

A relaxing ocean screensaver for your terminal.

This is the Go port of [stackedlol/ascii-sharks](https://github.com/stackedlol/ascii-sharks).

## Run it

```
asciiflow start
```

Sit back and watch. Press any key to exit.

## Command-line options

All flags belong to the `start` command:

```
asciiflow start [flags]
```

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `--fps` | int | `30` | Frames per second. Values above `240` are capped at `240`; values of `0` or less fall back to `30`. |
| `--no-predators` | string | `""` (everyone hunts) | Leave hunters out: `all`, `sharks`, or `pelagics` (tuna, mackerel). A bare `--no-predators` means `all`. |
| `-h, --help` | — | — | Show help for the `start` command. |

### `--no-predators` values

| Value | Sharks | Tuna and mackerel |
| --- | --- | --- |
| `all` | out | out |
| `sharks` | out | hunt |
| `pelagics` | hunt | out |

Any other value is an error. Bait fish always stay.

### Examples

Run with the defaults:

```
asciiflow start
```

Set the playback speed:

```
asciiflow start --fps 60
```

A calm ocean with no hunters at all:

```
asciiflow start --no-predators all
```

Sharks only (no tuna or mackerel):

```
asciiflow start --no-predators pelagics
```

Just run it with no options and you'll get the default ocean.


## Features

### Natural mode

Every run is an area-scaled, cap-only ocean:

- Fish, schooling predators, and sharks all get maximum populations that grow with the terminal size.
- The first seed fills those caps once. After that there is no minimum: missing fish, tuna, mackerel, or shark kinds are not deliberately restored. Normal respawns and canvas entries fill space over time while staying under the caps.
- Sharks spawn as any of the eight kinds at a random `0.5×–1.15×` size.
- Four predator schools seed the water: two tuna and two mackerel. Later merging, splitting, despawns, and entries can change that count.
- Eaten or removed fish and schooling predators return with the default replacement timing.
- Resizing adjusts the caps. Shrinking marks the excess as leavers so they swim out of view instead of vanishing; growing only raises the caps.

`--fps` and `--no-predators` are still available.

### Fish types

Three size-only fish types, no named species. New schools pick a size with equal weighting and start at its minimum size:

- Small — `,` glyph, schools from 20 fish. Fast, strongly aligned ribbons.
- Medium — `-` glyph, schools from 12 fish. Mills into rotating bait balls.
- Large — `=` glyph spread over two cells, schools from 7 fish. Loose groups, and the only type sharks eat.

There is no maximum school size: close schools of the same size merge into a single larger school, and schools break apart only when chased by predators.

### Predator types

Eight shark body plans, swimming at random kinds and sizes:

- Classic, hammerhead, zebra, tiger, greathammer (great hammerhead), mako, thresher, nurse.

Plus two schooling predators that hunt prey fish in schools, and flee sharks (which hunt them back):

- Albacore tuna — school of 6–8 with long sickle pectorals and a lunate tail.
- Spanish mackerel — school of 8–11, slender with a forked tail. Distinct from the large prey-fish type above.

## Origin and credits

- Original source project: [stackedlol/ascii-sharks](https://github.com/stackedlol/ascii-sharks) — the fish, shark, and ASCII rendering algorithms this Go terminal version is ported from.

The files under [`docs/`](docs/) describe the upstream TypeScript modules; see [`docs/go-port.md`](docs/go-port.md) for how each one maps to a Go package here.

## Development

Common commands for the dev workflow:

```
# run from source (any key exits)
go run . start

# build the asciiflow binary and run it
go build -o asciiflow .
./asciiflow start

# run the full test suite
go test ./...

# format and vet
go fmt ./...
go vet ./...
```
