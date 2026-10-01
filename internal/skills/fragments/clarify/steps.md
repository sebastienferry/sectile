1. Re-read the assigned branch and worktree. If docs/clarifications/<n>.md already exists,
   this run continues an existing clarification into Round N. If not, this is Round 1, unless the
   report is ignored by Git and the ticket already carries published rounds (step 6).
2. In Round 1:
   a. Restate the request in two sentences, including what is out of scope.
   b. List ambiguities, worst first. Only list an ambiguity if two readings lead to different code.
   c. Name critical dependencies: other services, migrations, missing data, third-party limits.
   d. Resolve reversible technical choices using existing code and project conventions.
   e. Formulate essential product questions that alter acceptance criteria, with your recommended option.
   f. Write docs/clarifications/<n>.md, commit with docs(spec): clarify #<n> (round 1), unless the file
      is ignored by Git (step 6).
   g. Publish the round as described below, then ask any open questions interactively
      when the owner is present or in the ticket discussion when unattended.
3. In Round N (follow-up after owner answers):
   a. Read the owner's answers from the interactive prompt or ticket comments via get_task.
   b. Append a dated section: "## Round N - answers from the owner (<date>)" to docs/clarifications/<n>.md.
   c. Explicitly record settled choices and any reversed prior assumptions.
   d. Address newly surfaced ambiguities or dependencies.
   e. Commit updates with docs(spec): clarify #<n> (round N), unless the file is ignored by Git (step 6).
   f. Publish the round as described below. If follow-up product questions remain,
      ask them and stop without transitioning.
4. Exit condition:
   Rounds continue until the owner confirms that the clarification is satisfactory (or zero open
   product questions remain in unattended pickup). Never transition new → clarified while product
   questions remain open.
5. Persist the settled scope, decisions, and assumptions in the report before concluding.
6. Dropped artefacts: `<n>` is the task key without its leading `#` (`487` for `#487`). Before
   committing, run `git check-ignore -q docs/clarifications/<n>.md`. When it succeeds, the project
   drops its specification artefacts on this workstation: write and update the file in the worktree,
   never commit it, never force it with `git add -f`, and include the settled decisions
   in the round section used as the transition note, saying that the report file stays local to the worktree.
   The published rounds are then the only shared record. A fresh worktree does not hold a report
   written elsewhere: when the file is ignored and missing, read the rounds already published on
   the ticket (the Clarification Report comments and each `## Round N` section in them), rebuild the
   file from them in order, and continue with the next round. Never restart at Round 1 while the
   ticket carries a published round.

7. Optional stage publication: read `pushStageCommits` from `get_project_context`
   (or the supplied project configuration); missing or false means off. When true,
   after each stage commit push the actual assigned work branch with a plain push.
   Use `git push -u origin <branch>` on first publication and `git push origin <branch>`
   afterwards. Never force. Report a refused push and continue the stage; retain the
   commit for retry. Ignored artifacts are never committed or force-added, so writing
   them alone triggers no push. This setting does not replace required PR publication.

### Publish every clarification round

- Every round, interactive or unattended, publishes its section in full: Round 1
  uses the whole initial report; Round N uses only its newly appended section.
  Retain the Markdown file as the chronological history, even when it stays local.
  Include the report path, commit/local status, settled decisions, open questions
  and publication failures in that section; a path or summary alone is insufficient.
- Standalone intermediate rounds with open questions use `add_comment`. The final
  round uses its full section as the `transition_stage` note, with no separate
  `add_comment` for the same content. Verify each response before claiming publication.
- Managed runs call no comment or stage tool: put the full round section in the
  supplied result note and let Sectile publish it through its completion contract.
- If a section exceeds the tracker comment limit (GitHub 65,536 characters; Jira
  about 32,767), split at Markdown paragraph boundaries into numbered parts, reserving
  space for the server header and part numbering. Use at most 30,000 characters per
  part for either tracker, and split an oversized paragraph without dropping text.
  Standalone intermediate parts use `add_comment` in order. For a final round, post
  all preceding parts with `add_comment` and use only the last numbered part as the
  transition note, so each part appears once. Managed runs keep the complete section
  in the result note and report any completion-contract size limitation rather than
  bypassing the contract.
