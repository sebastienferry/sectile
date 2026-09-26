# Tasks

## 1. Establish the full-page configuration lifecycle

- [ ] 1.1 Add a dedicated renderer lifecycle for a configuration page that can
      replace and restore the normal desktop main content without resetting
      selected project or execution state.
- [ ] 1.2 Move workstation Settings to that lifecycle and add a visible,
      keyboard-accessible Back action.
- [ ] 1.3 Move project configuration to that lifecycle and add the same Back
      behavior while preserving the project's identity and local actions.
- [ ] 1.4 Guard asynchronous page updates so results that settle after Back do
      not alter the restored main desktop view.

## 2. Preserve the configuration experience

- [ ] 2.1 Reuse or adapt the settings category layout and styles for full-page
      use, including the narrow-width category layout.
- [ ] 2.2 Preserve existing selected-category defaults, controls, validation,
      persistence, save actions, read-only panels, and loading/error states.
- [ ] 2.3 Keep the shared dialog lifecycle and close affordance for all
      unrelated modal flows.

## 3. Verify behavior

- [ ] 3.1 Add or update UI tests proving that Settings is a full page, exposes
      Back, and returns to the normal desktop view without clearing current
      state.
- [ ] 3.2 Add or update UI tests proving the same navigation behavior for a
      project's configuration and preserving its local save behavior.
- [ ] 3.3 Cover a late asynchronous configuration result after Back and prove
      it does not overwrite the main page.
- [ ] 3.4 Cover at least one unrelated modal flow to prove it remains a modal
      dialog with its existing close behavior.
- [ ] 3.5 Run `cd desktop && npm test`, then build and run
      `npm run test:ui`.
