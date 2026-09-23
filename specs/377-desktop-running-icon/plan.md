# Implementation plan

The desktop is an Electron renderer built with Vite. It consumes `shared/runStates.ts` and already sets the resolved state on `.run-state[data-run-state]` in `desktop/src/main.js`.

Add a desktop-only CSS rotation rule on the running SVG and a one-second linear infinite keyframe in `desktop/src/style.css`. Disable it under `prefers-reduced-motion: reduce`. Apply to the SVG rather than its container to keep the header label still. No data contract or dependency changes.

Keep `runStateSvg` static: it also supplies notification artwork. Introducing animation into shared serialized SVG would unnecessarily change those consumers. Existing state resolution and DOM reuse already provide the required transitions.

Add `desktop/tests/run-state-animation.ui.cjs` using the existing Electron/stub-agent pattern. Assert real animation progress and stable animation identity across polling, stationary labels and other states, waiting/resume transitions, and reduced-motion toggling. Update `CHANGELOG.md` under Fixed. This small presentation correction does not require an ADR or README changes.

Validation: desktop build, unit tests, targeted regression test (fails on baseline), then the desktop UI suite. No lint script is defined; use Node syntax checking for the new test and `git diff --check`.
