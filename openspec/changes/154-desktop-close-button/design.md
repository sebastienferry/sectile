# Design

The two glyphs live in the `iconPaths` map in `desktop/src/main.js`, which injects each entry into a shared 24 by 24 `<svg>` with `fill="none"`, `stroke="currentColor"`, `stroke-width="1.6"`, and round caps and joins. The change is confined to two entries of that map:

- `stop` becomes two stroked diagonals forming a cross, replacing the filled `<rect>`. The filled rectangle carried its own `fill="currentColor" stroke="none"` overrides, which disappear with it, so the control inherits the shared stroke treatment like every other icon.
- `shutdown` becomes a disconnect mark, a vertical bar over an open arc, replacing the outlined `<rect>`.

Nothing else moves. The markup in the header and toolbar, the `aria-label` and `title` attributes on both buttons, the enable and disable logic that keys the toolbar control off the execution status, the stop handler and its closure offer, and the `#stop`, `#shutdown`, and `.icon-button` rules in `desktop/src/style.css` are untouched. Colour comes from `currentColor`, so both controls keep their existing tint.

Two alternatives were rejected. Reusing the web surface's `StateGlyph` was rejected because the desktop renderer is plain DOM with its own icon map and importing the React component would pull in a build dependency for a cosmetic change. Changing the labels alongside the glyphs was rejected because the actions are unchanged and assistive technology should keep announcing what the buttons do.

Verification is visual plus the existing gates: `openspec validate 154-desktop-close-button --strict` and the project test target. The desktop tests assert on log text, command previews, closure, runtime, data directory, skill mode, notifications, and pairing, none of which read the icon map, so no test changes are required and none are added for path data.

The assigned branch is `feat/154`. The clarification left one question open, recorded below.
