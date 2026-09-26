# Tasks

## 1. Establish the unified Configuration lifecycle

- [ ] 1.1 Add a dedicated renderer lifecycle for one Configuration page that
      can replace and restore the normal desktop main content without resetting
      selected project or execution state.
- [ ] 1.2 Move workstation Settings to that lifecycle, with General selected
      by default and a visible, keyboard-accessible Back action.
- [ ] 1.3 Move Project settings to the same lifecycle, selecting the requested
      project's category while retaining General in the same sidebar.
- [ ] 1.4 Guard asynchronous page updates so results that settle after Back do
      not alter the restored main desktop view.

## 2. Unite the sidebar and preserve configuration behavior

- [ ] 2.1 Render General workstation categories first and the selected
      project's categories second, under clear group labels in one sidebar.
- [ ] 2.2 Reuse or adapt the existing category layout and styles for full-page
      use, including the narrow-width responsive layout.
- [ ] 2.3 Preserve existing selected-category defaults, controls, validation,
      persistence, save actions, read-only panels, and loading/error states.
- [ ] 2.4 Keep the shared dialog lifecycle and close affordance for all
      unrelated modal flows.

## 3. Verify behavior

- [ ] 3.1 Add or update UI tests proving that Settings opens the unified page
      with General first, exposes Back, and returns to the normal desktop view
      without clearing current state.
- [ ] 3.2 Add or update UI tests proving that Project settings opens the same
      page with General followed by that project's sections and the requested
      project category selected.
- [ ] 3.3 Cover workstation and project saves in their corresponding sections
      of the unified page.
- [ ] 3.4 Cover a late asynchronous Configuration result after Back and prove
      it does not overwrite the main page.
- [ ] 3.5 Cover at least one unrelated modal flow to prove it remains a modal
      dialog with its existing close behavior.
- [ ] 3.6 Run `cd desktop && npm test`, then build and run
      `npm run test:ui`.
