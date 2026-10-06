# Specification #745 - Claude settings and preset rules

- Ticket: https://github.com/sebastienferry/sectile/issues/745
- Branch: `feat/745`
- Clarification: `docs/clarifications/745.md` (rounds 1 and 2, confirmed by
  the owner on 2026-10-06)
- Extends: `specs/700-desktop-sandbox-configuration/`,
  `specs/730-global-sandbox-settings/`
- Framework: Spec Kit

## Summary

The Desktop "Sandbox" settings category, at workstation and project level, is
renamed "Claude settings", since it holds Claude Code's permission rules as
well as its sandbox. The workstation category gains presets: ready-made sets
of entries the owner applies in one click instead of typing them or approving
prompts one by one. There is one preset per toolchain, a Common preset, and a
Dangerous actions preset of deny rules. Applying a preset copies its entries
into the workstation lists, where they are ordinary entries.

## Scope

In scope: the rename of every user-visible "Sandbox settings" string, the
preset catalogue, applying and removing a preset in the workstation category,
the "Apply recommended" action, and the changelog.

Out of scope:

- Storage keys (`claudeSandbox`, `claudeSandboxProjects`), the
  `/desktop/workstation/sandbox` and project endpoints, the generated
  `--settings` file and its path, and the settings layout. No migration.
- Presets at project level. A project keeps adding its own entries by hand.
- Headless runs keep `bypassPermissions`. Allow rules change nothing for
  them, while deny rules still apply.
- Other engines, custom command templates, the web client, and the owner's own
  Claude Code settings files.
- Seeding presets on install or on upgrade.

## Vocabulary

- **Claude settings**: the category formerly named "Sandbox": the sandbox
  state, allowed network domains, extra writable paths, allow rules and deny
  rules (ADR 0048, ADR 0050).
- **Workstation lists**: the four lists of the workstation Claude settings.
- **Preset**: a named, fixed set of entries for one or more of the four lists,
  shipped with Desktop.
- **Applicable entries**: the entries of a preset this platform applies: all
  of them on macOS and Linux, the allow and deny rules only on Windows, where
  Claude Code's sandbox does not run.
- **Applied preset**: a preset whose applicable entries are all present in the
  workstation lists as currently edited.

## User stories

### US1 (P1) - The category reads "Claude settings"

1. Given the workstation settings, then the navigation lists "Claude
   settings" where it listed "Sandbox", between "AI engines" and "Deployment".
2. Given a project's settings, then its last category reads "Claude settings".
3. Given the workstation category, then its save button reads "Save Claude
   settings", a successful save reads "Claude settings saved", and the
   stopped-agent notice reads "Claude settings are unavailable while the local
   agent is stopped. Start the agent to edit them."
4. Given any text of either category, of a conversation activity, of an agent
   error or of a headless run activity that named "Sandbox settings" or
   "Sandbox values", then it names "Claude settings" instead. A text written
   in French stays in French: "réglages Claude du projet".
5. Given the state control, then it is still labelled "Claude Code sandbox":
   it toggles exactly that.
6. Given values saved before the change, then they read back unchanged, and a
   launch receives the same `--settings` file.

### US2 (P1) - Apply a preset in one click

As an owner, I open the workstation Claude settings and apply the presets of
the toolchains I use, instead of building the lists entry by entry.

1. Given the workstation category, then a "Presets" section lists every
   preset of the catalogue (see "Catalogue"), each with its name, a one-line
   description, and its entries grouped by list, which can be expanded.
2. Given a preset that is not applied, when I choose "Apply" on it, then each
   applicable entry missing from its list is appended to that list, in the
   catalogue's order. An entry already present is not added twice, and the
   preset then reads "Applied".
3. Given an applied preset, then each of its entries appears in the lists as
   an ordinary entry, removable with its "Remove" button like any other.
4. Given an applied preset, when I remove one of its entries by hand, then
   the preset no longer reads "Applied" and offers "Apply" again.
5. Given applied presets, when I choose "Save Claude settings", then the
   lists are saved as any edit is. Until then, nothing reaches the agent, and
   leaving the category without saving drops the change, as for any edit.
6. Given saved workstation lists, then every covered project applies them
   through the existing merge (ADR 0050), with no change to that merge.
7. Given an allow entry of an applied preset that is also a deny entry of an
   applied preset, then the existing "In both lists" warning names it, and
   the deny rule wins.

### US3 (P1) - Remove a preset

1. Given an applied preset, when I choose "Remove" on it, then each of its
   entries leaves its list, except an entry that another applied preset also
   holds.
2. Given an entry the owner typed by hand before applying a preset that
   holds the same entry, when the preset is removed, then that entry leaves
   the list too: the lists do not record where an entry came from.
3. Given a preset that is partially present, then it offers "Apply" (adding
   the missing entries) and not "Remove".

### US4 (P1) - Apply the recommended presets on a fresh workstation

1. Given workstation lists that are all empty, then the Presets section shows
   an "Apply recommended" action that applies Common and Dangerous actions,
   and says what it applies.
2. Given any non-empty workstation list, then "Apply recommended" is not
   shown.
3. Given an upgrade, or a first start of Desktop, then no preset is applied
   until the owner chooses one.

### US5 (P2) - Windows

1. Given Windows, when I apply a preset, then only its allow and deny rules
   are added. The preset says that its domains and writable paths do not
   apply on this platform, and "Applied" is judged on its rules alone.
2. Given Windows, then a preset with no rules (none in the catalogue today)
   offers no "Apply".

### US6 (P2) - The deny preset says what it is

1. Given the Dangerous actions preset, then its description says that a deny
   rule matches the command as Claude Code writes it. The same program run
   another way, such as through `sh -c` or a script, is not matched: it is a
   guardrail, not a security boundary.

## Catalogue

Rules use Claude Code's documented form `Bash(<command> *)`. A trailing ` *`
also matches the bare command, and `:*` is its legacy equivalent. Commands
Claude Code already runs without asking (`ls`, `cat`, `grep`, `find`, `head`,
`wc`, read-only `git` forms) get no allow rule. Paths with `~` are kept as
typed; Claude Code expands them. Each toolchain lists both the macOS and the
Linux cache location where they differ.

### Common

- Allow: `Bash(git add *)`, `Bash(git commit *)`, `Bash(git switch *)`,
  `Bash(git fetch *)`, `Bash(git pull *)`, `Bash(gh pr view *)`,
  `Bash(gh pr checks *)`, `Bash(gh issue view *)`.
- Domains: `github.com`, `api.github.com`, `codeload.github.com`,
  `*.githubusercontent.com`, `gitlab.com`.

### Go

- Allow: `Bash(go build *)`, `Bash(go test *)`, `Bash(go vet *)`,
  `Bash(go mod *)`, `Bash(gofmt *)`, `Bash(golangci-lint *)`.
- Domains: `proxy.golang.org`, `sum.golang.org`.
- Writable paths: `~/Library/Caches/go-build`, `~/.cache/go-build`,
  `~/go/pkg/mod`.

### Node / TypeScript

- Allow: `Bash(npm ci)`, `Bash(npm install *)`, `Bash(npm run *)`,
  `Bash(npm test *)`, `Bash(npx tsc *)`, `Bash(node --test *)`,
  `Bash(pnpm install *)`, `Bash(pnpm run *)`, `Bash(pnpm test *)`,
  `Bash(yarn install *)`, `Bash(yarn run *)`, `Bash(yarn test *)`.
- Domains: `registry.npmjs.org`, `registry.yarnpkg.com`.
- Writable paths: `~/.npm`, `~/Library/Caches/Yarn`, `~/.cache/yarn`,
  `~/Library/pnpm`, `~/.local/share/pnpm`.

### Python

- Allow: `Bash(pytest *)`, `Bash(python -m pytest *)`,
  `Bash(python3 -m pytest *)`, `Bash(pip install *)`, `Bash(uv sync *)`,
  `Bash(uv pip install *)`, `Bash(ruff *)`, `Bash(mypy *)`.
- Domains: `pypi.org`, `files.pythonhosted.org`.
- Writable paths: `~/Library/Caches/pip`, `~/.cache/pip`, `~/.cache/uv`.

### Java / Kotlin

- Allow: `Bash(mvn compile *)`, `Bash(mvn test *)`, `Bash(mvn package *)`,
  `Bash(mvn verify *)`, `Bash(./mvnw compile *)`, `Bash(./mvnw test *)`,
  `Bash(./mvnw package *)`, `Bash(./mvnw verify *)`, `Bash(gradle build *)`,
  `Bash(gradle test *)`, `Bash(gradle check *)`, `Bash(./gradlew build *)`,
  `Bash(./gradlew test *)`, `Bash(./gradlew check *)`.
- Domains: `repo.maven.apache.org`, `repo1.maven.org`, `plugins.gradle.org`,
  `services.gradle.org`, `downloads.gradle.org`.
- Writable paths: `~/.m2`, `~/.gradle`.

### Rust

- Allow: `Bash(cargo build *)`, `Bash(cargo test *)`, `Bash(cargo check *)`,
  `Bash(cargo clippy *)`, `Bash(cargo fmt *)`.
- Domains: `crates.io`, `index.crates.io`, `static.crates.io`.
- Writable paths: `~/.cargo`.

### .NET

- Allow: `Bash(dotnet build *)`, `Bash(dotnet test *)`,
  `Bash(dotnet restore *)`, `Bash(dotnet format *)`.
- Domains: `api.nuget.org`.
- Writable paths: `~/.nuget/packages`.

### Infrastructure (read-only)

- Allow: `Bash(terraform fmt *)`, `Bash(terraform validate *)`,
  `Bash(terraform init *)`, `Bash(terraform plan *)`,
  `Bash(terragrunt validate *)`, `Bash(terragrunt plan *)`,
  `Bash(kubectl get *)`, `Bash(kubectl describe *)`, `Bash(kubectl logs *)`,
  `Bash(helm template *)`, `Bash(helm lint *)`.
- Domains: `registry.terraform.io`, `releases.hashicorp.com`.
- Writable paths: `~/.terraform.d`.

### Dangerous actions (deny)

- Infrastructure: `Bash(terraform apply *)`, `Bash(terraform destroy *)`,
  `Bash(terraform import *)`, `Bash(terraform state rm *)`,
  `Bash(terraform state mv *)`, `Bash(terraform state push *)`,
  `Bash(terragrunt apply *)`, `Bash(terragrunt destroy *)`,
  `Bash(terragrunt run-all apply *)`, `Bash(terragrunt run-all destroy *)`,
  `Bash(kubectl apply *)`, `Bash(kubectl delete *)`, `Bash(kubectl patch *)`,
  `Bash(kubectl replace *)`, `Bash(helm install *)`, `Bash(helm upgrade *)`,
  `Bash(helm uninstall *)`, `Bash(helm rollback *)`.
- Cloud: `Bash(gcloud * delete *)`, `Bash(aws * delete-*)`,
  `Bash(aws s3 rm *)`, `Bash(aws ec2 terminate-instances *)`,
  `Bash(az * delete *)`.
- Git: `Bash(git push --force*)`, `Bash(git push -f *)`,
  `Bash(git push * --force*)`, `Bash(git push * -f)`,
  `Bash(git push * -f *)`.
- Publishing: `Bash(npm publish *)`, `Bash(pnpm publish *)`,
  `Bash(yarn publish *)`, `Bash(cargo publish *)`, `Bash(twine upload *)`,
  `Bash(mvn deploy *)`, `Bash(./mvnw deploy *)`, `Bash(docker push *)`,
  `Bash(gh release create *)`.
- System: `Bash(sudo *)`.
- Secrets: `Read(./**/.env)`, `Read(./**/.env.local)`,
  `Read(./**/.env.*.local)`, `Read(./**/.env.production)`,
  `Read(./**/*.tfstate)`, `Read(~/.ssh/**)`, `Read(~/.aws/**)`,
  `Read(~/.config/gcloud/**)`.

`rm -rf` and `git reset --hard` are deliberately absent (clarification,
decision 5).

## Functional requirements

- FR-1: Every user-visible "Sandbox settings" / "Sandbox values" string of
  Desktop, of the agent's conversation activities and errors, and of the
  headless refusal line names "Claude settings", in the surface's language.
  The "Claude Code sandbox" control keeps its name.
- FR-2: No stored key, endpoint, payload field, generated file, category id or
  settings layout changes.
- FR-3: The catalogue is fixed, versioned with Desktop, and exactly the one
  above. Each entry passes the panel's own entry rules (trimmed, one line,
  non-empty), and no preset holds an entry twice.
- FR-4: Applying a preset appends its missing applicable entries in catalogue
  order. Removing it takes out its entries that no other applied preset
  holds. Both act on the lists being edited, and Save persists them.
- FR-5: A preset reads "Applied" if and only if all its applicable entries are
  present in the lists being edited.
- FR-6: "Apply recommended" is shown only while the four workstation lists
  are empty, and applies Common then Dangerous actions.
- FR-7: Nothing applies a preset without the owner choosing it.
- FR-8: Presets appear in the workstation category only.
- FR-9: `CHANGELOG.md` describes the category as "Claude settings" in its
  unreleased #700 and #730 lines, and gains one line for the presets.

## Success criteria

- An owner sets up a Go, Node or Python workstation in at most three clicks
  and one save.
- A Claude Code launch that receives the Dangerous actions preset refuses
  `terraform apply`, and a sandboxed `go test` with the Common and Go presets
  applied downloads its modules and writes its build cache without a prompt.
