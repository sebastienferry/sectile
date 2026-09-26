# Design

## Context

The desktop renderer currently uses one shared native dialog for transient
flows. `openSettings()` and `openProject()` build the two configuration
interfaces inside that dialog, sharing its heading, close control, footer, and
cleanup lifecycle. They need to become application surfaces while preserving
the existing setting controls and the normal desktop selection state.

## Decisions

### Render configuration through a dedicated page lifecycle

The renderer will introduce a page-level configuration container or equivalent
page-rendering lifecycle, separate from `showDialog()`. Opening Settings or a
project configuration replaces the normal desktop main content with the
configuration surface; it does not call `showModal()` or reuse dialog-only
controls. The existing category and field construction will be reused where
practical so their behavior remains unchanged.

**Rejected:** enlarging the existing dialog. A larger dialog would preserve the
same modal interaction model and would not satisfy the requested full-page
navigation.

### Back restores the existing main desktop state

Each configuration page will have a real button with an accessible name such
as Back. Activating it returns to the normal desktop page and restores its
already-held project, selected task/execution, terminal, and sidebar state.
It does not reload the application, clear selection, or invoke an unrelated
dialog close handler. Keyboard activation follows the native button behavior.

**Rejected:** relying on Escape or a close icon alone. Those affordances are
ambiguous for a page and do not provide the requested visible return action.

### Page departure invalidates asynchronous rendering

The page lifecycle will own a generation, connection check, or equivalent
validity guard. An asynchronous settings, project, log, or refresh result may
update the page only while that page remains current. Leaving the page before a
request settles must not recreate configuration content, alter the restored
main page, or apply a stale connection status.

Existing modal cleanup remains responsible for modal-only flows.

### Share responsive settings layout without modal geometry

The full-page surfaces retain the existing category navigation and responsive
breakpoint: on narrow widths categories wrap above the content. Dialog width,
maximum-height, sticky dialog footer, and dialog-specific heading rules will
not define the page layout. Project save actions remain available only in the
same categories that currently persist local configuration; read-only panels
remain read-only/action panels.

### Retain the modal implementation for unrelated flows

`showDialog()` and its cleanup stay in place for confirmations, add-project,
and other non-settings dialogs. The change is intentionally narrow: changing
the settings navigation mechanism must not change how any unrelated dialog
opens, closes, or clears its content.

## Affected areas

- `desktop/src/main.js`: page visibility and lifecycle, Settings and project
  configuration entry points, Back behavior, and async validity guards.
- `desktop/src/style.css`: full-page configuration composition, responsive
  layout, and removal of settings-specific dependence on dialog geometry.
- `desktop/tests/*.ui.cjs`: full-page navigation, retained configuration
  behavior, and modal regression coverage.
