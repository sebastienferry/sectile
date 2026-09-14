# Implementation checklist

- [x] 1. Expose optional actual start time and capture it on successful PTY launch; test lifecycle and API compatibility.
- [x] 2. Implement deterministic task representative and row ordering; test status priority, timestamp fallback, ties, and non-mutation.
- [x] 3. Verify desktop refresh, history, archive filtering, and selection behavior with UI coverage.
- [x] 4. Update desktop and agent contract documentation.
- [x] 5. Run strict specification validation, build, static analysis, and tests; review the full diff against current origin/main.
- [x] 6. Update the existing draft PR with implementation and validation evidence, verify its head, and mark it ready.
