# renderer module

Draws the World as an ASCII grid on canvas: cell grid, wake field, fish glyphs, rasterised sharks.

## API

```
paintGrid(g: Canvas2D, W: float, H: float, cw: float, ch: float, dpr: float) -> void

class AsciiRenderer:
  init(canvas: HtmlCanvas) -> AsciiRenderer
  cellWidth -> float (read-only)
  cellHeight -> float (read-only)
  setFontFamily(family: string) -> void
  resize(width: float, height: float, dpr: float) -> void
  render(world: World, dt: float) -> void
```
