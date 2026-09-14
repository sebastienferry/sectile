# Project open tasks

## Why
The desktop project sidebar requires navigating through New task and entering a search before an existing ticket can be discovered. Users need to browse a project's open tasks without knowing a title or key.

## What Changes
- Add an accessible list icon on each project row, revealed on hover or keyboard focus.
- Open the existing launch dialog with the project's open tasks loaded immediately.
- Preserve search, skill selection and explicit launch, with loading, empty and recoverable error states.

## Impact
Desktop renderer, UI regression tests and desktop documentation. No API, schema or dispatch changes.

## Non-goals
Web sidebar changes, new task creation changes and automatic execution on opening the list.
