# Implementation checklist

- [x] 1. Read the clarification and capability scenarios; inspect current creation, cloning, quick-add initialization, and isolated database test fixtures.
- [x] 2. Update creation and cloning defaults to lowercase `new` and the adjacent creation comment; retain helper, status, clone-option, and provider behavior.
- [x] 3. Update the quick-add form default/reset to lowercase `new` while preserving editing and submission behavior.
- [x] 4. Add focused database regression coverage for returned and reloaded creation labels, legacy workflow replacement, custom-label preservation, clone include-labels options, unchanged source tasks, and default/explicit statuses.
- [x] 5. Run focused database tests, `make test`, and `make build`; record results and distinguish pre-existing failures.
- [x] 6. Verify first open and reopen show `#new`, unchanged-default submission sends `new`, and the created task displays `#new` after reload in a label-visible view. Verify custom-label casing remains intact.
- [x] 7. Review the diff for creation-only scope and run `openspec validate 48-default-label-when-adding-a-task --strict`; record acceptance evidence before reporting implementation complete.
