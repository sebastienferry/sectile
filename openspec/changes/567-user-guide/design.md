# Design

## Decisions

- Place the guide at `docs/USER_GUIDE.md`, with a contents list and sections in journey order. Link it from `README.md` and `docs/README.md`.
- Keep the prose in English, with exact current French UI labels where useful. Describe behavior rather than transcribing every control.
- Cross-link `desktop/README.md`, the root README, and relevant architecture decisions for installation and advanced details.
- Check claims against the current web, Desktop, agent, and workflow sources. Use example values only.

## Alternatives considered

- Expand `desktop/README.md`: rejected because the ticket spans browser, tracker, agent, CLI, and Desktop use.
- Put full setup steps in `README.md`: rejected because it would make the project overview too long and duplicate the guide.

## Validation

Validate OpenSpec strictly, verify local Markdown links and section coverage, and run the repository's documentation-relevant checks. No runtime code is changed.
