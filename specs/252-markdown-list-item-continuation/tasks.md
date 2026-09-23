# #252 — Implementation checklist

Ordered so each step leaves the package building and its existing tests green.
References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. Tests first (US1–US5)

- [x] T1.1 Add a decoding helper in `adf_test.go` that turns `MarkdownToADF` output into
      a typed tree (type, text, attrs, content) for structural assertions.
- [x] T1.2 Add the US1 tests: ticket example, ordered variant, unindented continuation
      after a blank line ends the list, several blank lines.
- [x] T1.3 Add the US2 tests: indented and lazy continuation without a blank line,
      lazy line under a nested item, heading / quote / rule / table right under an
      item end the list.
- [x] T1.4 Add the US3 tests: code block after a blank line, without one, not indented
      enough, text after the item's code block.
- [x] T1.5 Add the US4 test: `- a\n  - x\n\n  para\n- b`, and the D6 invariant over
      every new case.
- [x] T1.6 Add round-trip cases to `TestMarkdownToADFCoversTheReportSubset` and the US5
      ADF → Markdown → ADF test.
- [x] T1.7 Run `go test ./internal/trackerapi/` and confirm the new tests fail for the
      reason the ticket describes.

## 2. Parser (FR1–FR6, FR8)

- [x] T2.1 Extract `fencedCodeBlock(lines, i, lang, strip)` and `dedent` from
      `parseMarkdownBlocks` (P4); existing tests stay green.
- [x] T2.2 Add `endsListItem` (P3.1).
- [x] T2.3 Rewrite the `parseMarkdownList` loop with the item state of P1, the blank-line
      look-ahead of P2 and the continuation rules of P3.

## 3. Renderer (FR7)

- [x] T3.1 Rewrite `renderADFList` per P5.

## 4. Verify

- [x] T4.1 `gofmt -l internal/trackerapi`, `go vet ./internal/trackerapi/`,
      `go test ./internal/trackerapi/`.
- [x] T4.2 `go test ./...` under WSL; report any failure with its baseline on `main`.
- [x] T4.3 Update the header comment of `adf.go` if the supported subset it lists
      changed.
- [x] T4.4 Re-read the diff as a reviewer.
