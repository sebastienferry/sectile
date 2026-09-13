## Decisions
Use the existing #203033 background and transparent base border. Make the compact selected selector more specific than the compact transparent-background selector. Do not suppress keyboard focus outlines or change JavaScript selection logic.

## Alternatives
Removing all borders would alter row dimensions. Applying a global outline reset would hide keyboard focus. Reworking the sidebar DOM is unnecessary.

## Validation
Build the desktop, run the existing Electron UI suite, and inspect computed selected/unselected styles, stable dimensions, and keyboard focus. Run project frontend lint/build/tests and Go checks before publication. No new permanent test is needed for this small reversible CSS change; use runtime visual checks with the existing UI fixture.

## Clarification
No comments or parent were supplied. The desktop sidebar matches the reported selection line; the web list already uses background selection. No open product decisions or external dependencies.
