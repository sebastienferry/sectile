# Implementation checklist

- [x] 1. Extract shared relay types; move agent entrypoint, configuration, MCP and execution source/tests to `cmd/agent`.
- [x] 2. Move server local workspace/terminal/LLM execution behind the agent contract; handle disconnected agents and validate completion evidence without local Git.
- [x] 3. Implement the confirmed server-side HTTP tracker transport and explicit credentials; cover error and pagination behavior.
- [x] 4. Split Makefile builds/releases; update Electron resources, launcher and generated MCP commands.
- [x] 5. Update README, architecture and three-party contract documentation; record the boundary decision in an ADR.
- [x] 6. Run strict specification validation, both binary builds, Go tests/vet, web tests/typecheck/lint/build and desktop tests/build.
- [ ] 7. Review the complete diff against current remote main, address feedback, push and verify the existing PR head and readiness.
