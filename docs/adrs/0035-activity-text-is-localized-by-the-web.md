# ADR 0035: Activity text is localized by the web

Status: Accepted (#532)

## Context

An activity carries three pieces of text written by the server: its `action`,
its `summary` and its `steps`. The server builds them from a fixed set of
French templates (`Exécution de %s sur l'agent local`,
`Tâche ciblée : %s - %s`, `Synchronisation %s en file d'attente`,
`✅ Ticket %s %s mis à jour avec le label « %s »`…) in
`internal/handlers`, `internal/db` and `internal/runner`, and stores the
resulting sentence. Nothing in the row says which template produced it.

The web interface now speaks French or English per viewer (batch #526 to
#534). Once the Sectile-owned strings of the Activities view were translated,
an English viewer still read French in every activity: the labels around the
text changed language, the text itself did not.

Three facts constrained the answer:

- **Activities are persisted.** Thousands of rows written before this change
  exist in every deployment and will never be rewritten. Whatever the web does
  must work on them.
- **The server writes French on purpose.** `AGENTS.md` makes what Sectile
  shows at runtime (activity steps and summaries included) a surface that
  keeps the language it speaks today; translating it in passing is a product
  decision of its own.
- **Not all activity text is Sectile's.** A step may quote an external error,
  a custom command line or a tracker's answer; the output is the agent's own
  prose. None of it can be translated safely.

## Decision

**The web recognizes the server's known activity templates and renders them
in the viewer's language; any other text is shown as stored.**

- `web/src/lib/activityText.ts` exports
  `localizeActivityText(text, locale)`. It is applied to an activity's
  `action`, `summary`, each step and the skill name, never to its output.
- The French value of each template in the web catalog
  (`operations.activityTemplates`) is the server's wording with `{0}`, `{1}`…
  in place of its `%s`/`%d`/`%v` parameters. The recognizer compiles each
  French value into an anchored, whole-string pattern with one lazy capture
  per parameter; the English value renders the same captures. The most
  specific template (the one with the most literal text) is tried first.
- French returns the text unchanged. English renders the matched template
  with its parameters verbatim. Unmatched text is returned unchanged in both
  languages.
- **No API, storage or server change.** The server keeps writing French, and
  the rows already stored are rendered by the same rule as new ones.

## Consequences

- **The server's template strings become a contract.** A reworded template
  on the server stops matching and shows in French until its catalog entry is
  updated. `web/tests/activityText.test.mjs` holds one real sample per
  template, which is where the mismatch is caught when both sides change in
  the same pull request.
- **A new server template shows in French until the web learns it.** Nothing
  breaks and nothing is lost: the fallback is the stored text, which is what
  every viewer read before this change.
- **Parameters are never translated.** Task keys, titles, tracker and project
  names, statuses, labels, device names and quoted errors are carried over as
  captured. An English sentence may therefore quote a French status or a
  French error from the tracker; that is the tracker's text, not Sectile's.
- **Templates that embed further Sectile prose in a parameter are left out**
  (for example `Mise à jour sur %s : %s`, whose second parameter is a joined
  list of French change descriptions), since rendering them would produce a
  sentence in two languages.
- **The cost is a scan of about a hundred anchored patterns per rendered
  string**, negligible next to rendering the row.

## Alternatives rejected

- **Message identifiers and parameters stored by the server.** The server
  would write `{ "id": "agentLaunch.action", "params": ["clarify"] }` beside,
  or instead of, the sentence, and every client would render it. This is the
  cleaner design, but it changes the API and the storage, needs a migration,
  and still needs a fallback for every row persisted before it, which is this
  recognizer anyway. It also makes the desktop client and any MCP consumer
  depend on a message catalog they do not have today.
- **Translating arbitrary text.** Machine translation, or a word-level
  dictionary, would touch agent output, external errors and command lines,
  which must be quoted exactly, and would make the English interface say
  things the server never wrote.

## When to revisit

- **A third UI language**, or a client other than the web (the desktop app,
  an MCP consumer) that must render activities in the viewer's language: the
  catalog would then be duplicated per client, and server-side identifiers
  with parameters become worth their migration.
- **Server templates that change often**, to the point where keeping the web
  catalog in step costs more than the migration above.
