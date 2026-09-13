# Implementation checklist

- [ ] 1. Extract shared relay types; move agent entrypoint, configuration, MCP and execution source/tests to `cmd/agent`.
- [ ] 2. Move server local workspace/terminal/LLM execution behind the agent contract; handle disconnected agents and validate completion evidence without local Git.
- [ ] 3. Replace server tracker CLI transports and repository discovery with native HTTP adapters and explicit configuration; cover error and pagination behavior.
- [ ] 4. Split Makefile builds/releases; update Electron resources, launcher and generated MCP commands.
- [ ] 5. Update README, architecture and three-party contract documentation; record the boundary decision in an ADR.
- [ ] 6. Run strict specification validation, both binary builds, Go tests/vet, web tests/typecheck/lint/build and desktop tests/build.
- [ ] 7. Review the complete diff against current remote main, address feedback, push and verify the existing PR head and readiness.
