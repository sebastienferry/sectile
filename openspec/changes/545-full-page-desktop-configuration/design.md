# Design

## Context

The desktop renderer currently uses one shared native dialog for transient
flows. `openSettings()` and `openProject()` build separate configuration
interfaces inside it. They share many visual and behavioral primitives but
force a user to leave one configuration context before entering the other.
The new Configuration page must unite those contexts while retaining the
normal desktop selection state and the existing configuration behavior.

## Decisions

### Render one dedicated Configuration page

The renderer will introduce a page-level Configuration lifecycle, separate
from `showDialog()`. The page replaces the normal desktop main content; it
does not call `showModal()` or reuse dialog-only controls. Existing field and
panel construction will be reused where practical so their behavior remains
unchanged.

Both the Settings entry point and a project's Project settings entry point
open this same page. They only differ in the initially selected sidebar item.

**Rejected:** two independent full-page surfaces. That preserves the same
separation users are asking to remove and duplicates navigation and lifecycle
handling.

### Group one sidebar in workstation-then-project order

The Configuration sidebar presents a General group containing the existing
workstation categories, followed by a group named for the selected project
containing its existing configuration categories. General is always first.
Only the project explicitly selected by the Project settings entry point is
included; the page does not aggregate categories from other projects.

Opening Settings selects its default General category. Opening Project
settings selects the requested project category while retaining General above
it in the same sidebar.

**Rejected:** putting project categories before General or merging both sets
into an unlabeled flat list. Either choice loses the requested hierarchy and
makes ownership of a setting unclear.

### Back restores the existing main desktop state

The unified page will have one real button with an accessible name such as
Back. Activating it returns to the normal desktop page and restores its
already-held project, selected task/execution, terminal, and sidebar state.
It does not reload the application, clear selection, or invoke an unrelated
dialog close handler. Keyboard activation follows native button behavior.

**Rejected:** relying on Escape or a close icon alone. Those affordances are
ambiguous for a page and do not provide the requested visible return action.

### Page departure invalidates asynchronous rendering

The page lifecycle will own a generation, connection check, or equivalent
validity guard. An asynchronous workstation setting, project setting, log, or
refresh result may update Configuration only while that page remains current.
Leaving the page before a request settles must not recreate configuration
content, alter the restored main page, or apply stale status.

Existing modal cleanup remains responsible for modal-only flows.

### Share responsive layout without modal geometry

The unified sidebar retains the existing responsive breakpoint: on narrow
widths, group labels, categories, and content remain usable in the existing
responsive layout. Dialog width, maximum-height, sticky dialog footer, and
dialog-specific heading rules will not define the page layout. Project save
actions remain available only in the same categories that currently persist
local configuration; read-only panels remain read-only/action panels.

### Retain the modal implementation for unrelated flows

`showDialog()` and its cleanup stay in place for confirmations, add-project,
and other non-configuration dialogs. The change is intentionally narrow:
changing Configuration must not change how an unrelated dialog opens, closes,
or clears its content.

## Affected areas

- `desktop/src/main.js`: Configuration page visibility and lifecycle, unified
  sidebar groups, entry-point selection, Back behavior, and async validity
  guards.
- `desktop/src/style.css`: full-page composition, grouped sidebar, responsive
  layout, and removal of configuration dependence on dialog geometry.
- `desktop/tests/*.ui.cjs`: unified navigation, retained configuration
  behavior, and modal regression coverage.
