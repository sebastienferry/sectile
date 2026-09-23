# #234 — Implementation checklist

References: [`spec.md`](./spec.md), [`plan.md`](./plan.md).

## 1. CI job (FR1–FR6, D1–D4)

- [x] T1.1 Add `lint:gofmt` to `.gitlab-ci.yml` under "Tests": `stage: test`, `needs: []`,
      `cache: []`, `GOMODCACHE: /go/pkg/mod`, no `allow_failure`, and a comment saying why
      the cache must stay out of the checkout.
- [x] T1.2 Script: prepend `$(go env GOROOT)/bin` to `PATH`, print `command -v gofmt` and
      `go version`, run `make fmt-check`.
- [x] T1.3 Leave the `needs` of `publish:image`, `publish:binaries` and `promote:dev` as they are.
- [x] T1.4 Validate with `glab ci lint` against the mirror project.

## 2. Docs (FR7)

- [x] T2.1 README "Formatting and checks": name `lint:gofmt`, keep `make fmt-check` as the
      reproduction, and say that requiring it on `main` is a branch-protection setting that
      needs the GitLab → GitHub status integration.

## 3. Test plan

- [x] T3.1 WSL replay of the job script on the clean tree: passes, and logs the `gofmt` of
      the go.mod toolchain.
- [x] T3.2 Same replay with an unformatted `.go` file added: fails and names the file.
- [x] T3.3 Same replay with an unparseable `.go` file: fails.
- [x] T3.4 Same replay with a module cache (unformatted sources) inside the tree under a
      dot-directory: fails. This shows why D2 is needed.
- [ ] T3.5 Push, and read the `feat/234` mirror pipeline: `lint:gofmt` is green.
