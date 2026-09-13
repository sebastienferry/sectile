## 1. Header behavior
- [x] 1.1 Centralize selected header formatting: identity, local name or fetched title, and selected skill; retain empty-selection and missing-title fallbacks.
- [x] 1.2 Refresh header presentation after metadata completion and local rename without reconnecting or clearing the console; remove stale cached titles after successful responses with no usable title.
- [x] 1.3 Ensure task switching and historical execution selection resolve only the current task's title and selected run's skill.

## 2. Layout and documentation
- [x] 2.1 Add single-line title truncation, full hover/accessibility text, and layout constraints that keep toolbar controls usable at narrow supported window sizes.
- [x] 2.2 Document header contents, local-name precedence, and missing-title behavior in `desktop/README.md`.

## 3. Verification
- [x] 3.1 Add desktop UI tests for title precedence, missing key/ID fallback, no selection, delayed metadata, title refresh/clear, metadata failure, and local rename persistence.
- [x] 3.2 Cover switching during metadata requests, historical skills, literal text rendering, and console attachment/output preservation during metadata-only updates.
- [x] 3.3 Verify long-title truncation and full-text access with toolbar controls visible and keyboard-operable at a narrow supported size; inspect a screenshot.
- [x] 3.4 Run desktop build and desktop UI tests, fix in-scope failures, and record results.
- [x] 3.5 Run strict OpenSpec validation and review the final implementation against every scenario.
