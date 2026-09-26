# Unite workstation and project configuration on one desktop page

## Why

Workstation Settings and a project's configuration currently open as separate
modal dialogs, even though both are parts of one configuration experience.
Their category navigation, long forms, asynchronous data, and project actions
need the space and continuity of a full-page application surface. Users should
be able to see general settings first and then the selected project's settings
in one coherent sidebar.

## What Changes

- The desktop provides one full-page Configuration surface instead of separate
  Settings and Project configuration dialogs.
- Its sidebar lists the workstation-wide categories under General first, then
  the selected project's configuration categories under that project's name.
- Opening Settings enters the unified page with a General category selected;
  opening Project settings enters it with the corresponding project category
  selected.
- One visible, keyboard-accessible Back action returns from Configuration to
  the normal desktop main page.
- Existing configuration categories, controls, saving, validation, responsive
  behavior, and asynchronous loading remain available in their respective
  sections.
- All other dialog-driven flows remain modal and retain their existing close
  behavior.

## Impact

The desktop renderer and its styles are affected, principally
`desktop/src/main.js` and `desktop/src/style.css`. Existing workstation and
project settings UI coverage will be updated, with focused unified-navigation,
state-preservation, and modal-regression scenarios added. This introduces the
new `desktop-configuration-pages` capability.

## Out of scope

- Changing any configuration field, default, validation rule, server request,
  or persistence format.
- Changing the project-disconnection flow or server/deployment panels.
- Combining configuration from multiple projects in one page.
- Moving dialogs other than Configuration to full-page surfaces.
- Adding browser-style history, deep links, migrations, or dependencies.
