# Adjustment review — #64

PR: https://github.com/sebastienferry/taskflow/pull/92
Branch: `feat/64`
Integrated default baseline: `adc4a71` (`origin/main`).

## Review scope and feedback

Reviewed the complete branch change against the accepted clarification and
OpenSpec behavior contract: catalog/transport identities, migration and policy
preservation, caller and template updates, tests and current documentation.
Verified all 54 managed instruction diffs are exactly the eight tool-name
substitutions with no additional content changes. PR reviews, discussion and
inline comments were successfully retrieved; all were empty. No remote CI checks
were configured on the PR at review time.

## Findings and dispositions

1. **Fixed: upgraded stdio bridge could expose an old upstream catalog.** The
   original transparent discovery would register legacy tools when the server
   had not been upgraded. The bridge now validates the complete canonical catalog
   before starting its stdio transport and returns an actionable upgrade error
   for unexpected names, duplicates or missing tools. There is no alias or name
   translation. Regression tests exercise old names, forbidden product prefixes
   and an incomplete canonical catalog against real HTTP MCP discovery.
2. **Fixed: broken tool table and stale service prose.** Removed the blank line
   splitting the architecture's eight-tool table and updated remaining MCP
   service references and mixed-version failure guidance.
3. **Resolved: baseline documentation conflict.** Retained both the MCP naming
   contract and the incoming desktop skill-result lookup contract. No production
   merge conflicts occurred.
4. **Retained by design: conservative policy reconciliation.** Unknown patterns,
   conflicting defaults and separate legacy policies require manual changes;
   they do not authorize broader permissions. No changes to stored custom
   instructions, deployment paths or workflow ownership were needed.

## Replayable validation

- [x] `go test ./cmd/server -run 'TestMCP|TestDesktopFinishesCanonicalRun'`
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] `make test` — 22 frontend tests passed, TypeScript succeeded, oxlint exited
  successfully with warnings.
- [x] `make binary-build` — frontend and embedded server/agent binary built;
  Vite reported the existing large-chunk warning.
- [x] `openspec validate 64-sectile-mcp-tool-names --strict`
- [x] `git diff --check`

Tests and build ran after default-branch integration and the catalog fix. The
stdio integration harness runs the real bridge in child processes; no prebuilt
external executable is needed for its protocol assertions. The final generated
binary was also built. Live services and client registrations were not restarted
or changed.

## Local work preservation

Unrelated local configuration and workflow edits remain in the named stash
`preserve local #64 configuration during adjustment`. The reviewed branch excludes
those edits and preserves the earlier specification and implementation commits.
Human merge and handoff remain separate actions.
