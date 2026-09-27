# Tasks

## 1. Establish the unified Configuration lifecycle

- [x] 1.1 Add a dedicated renderer lifecycle for one Configuration page that
      can replace and restore the normal desktop main content without resetting
      selected project or execution state.
- [x] 1.2 Move workstation Settings to that lifecycle, with General selected
      by default and a visible, keyboard-accessible Back action.
- [x] 1.3 Move Project settings to the same lifecycle, selecting the requested
      project's category while retaining General in the same sidebar.
- [x] 1.4 Guard asynchronous page updates so results that settle after Back do
      not alter the restored main desktop view.

## 2. Unite the sidebar and preserve configuration behavior

- [x] 2.1 Render General workstation categories first and the selected
      project's categories second, under clear group labels in one sidebar.
- [x] 2.2 Reuse or adapt the existing category layout and styles for full-page
      use, including the narrow-width responsive layout.
- [x] 2.3 Preserve existing selected-category defaults, controls, validation,
      persistence, save actions, read-only panels, and loading/error states.
- [x] 2.4 Keep the shared dialog lifecycle and close affordance for all
      unrelated modal flows.

## 3. Verify behavior

- [x] 3.1 Add or update UI tests proving that Settings opens the unified page
      with General first, exposes Back, and returns to the normal desktop view
      without clearing current state.
- [x] 3.2 Add or update UI tests proving that Project settings opens the same
      page with General followed by that project's sections and the requested
      project category selected.
- [x] 3.3 Cover workstation and project saves in their corresponding sections
      of the unified page.
- [x] 3.4 Cover a late asynchronous Configuration result after Back and prove
      it does not overwrite the main page.
- [x] 3.5 Cover at least one unrelated modal flow to prove it remains a modal
      dialog with its existing close behavior.
- [x] 3.6 Run `cd desktop && npm test`, then build and run
      `npm run test:ui`.

Final validation: 142 desktop unit tests passed; the full UI run passed 58 of
59 tests, then the outdated modal tooltip assertion was updated and its focused
rerun passed. Agent/config Go tests, Go vet, desktop build and strict OpenSpec
validation passed. See accepted-ui-refinements.md for owner-approved scope changes.
