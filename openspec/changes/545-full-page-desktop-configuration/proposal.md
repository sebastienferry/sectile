# Open workstation and project configuration as full desktop pages

## Why

The desktop application currently presents both workstation Settings and a
project's configuration in the shared modal dialog. Their category navigation,
long forms, asynchronous data, and project actions now exceed the natural
scope of a transient dialog. Users need room to work through configuration
without losing their place in the desktop application.

## What Changes

- Workstation Settings opens as a full-page desktop surface rather than a modal
  dialog.
- Project configuration opens as a full-page desktop surface rather than a
  modal dialog.
- Each surface provides a visible, keyboard-accessible Back action that
  returns to the normal desktop main page.
- Existing configuration categories, controls, saving, validation, responsive
  behavior, and asynchronous loading remain available on their respective
  pages.
- All other dialog-driven flows remain modal and retain their existing close
  behavior.

## Impact

The desktop renderer and its styles are affected, principally
`desktop/src/main.js` and `desktop/src/style.css`. The existing workstation and
project settings UI coverage will be updated, with focused navigation and
modal-regression scenarios added. This introduces the new
`desktop-configuration-pages` capability.

## Out of scope

- Changing any configuration field, default, validation rule, server request,
  or persistence format.
- Changing the project-disconnection flow or server/deployment panels.
- Moving dialogs other than workstation Settings and project configuration to
  full-page surfaces.
- Adding browser-style history, deep links, migrations, or dependencies.
