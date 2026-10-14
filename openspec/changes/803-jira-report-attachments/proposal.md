# Jira report attachments

## Why
Full stage reports currently pass through Markdown-to-ADF conversion on Jira. Formatting and size failures can prevent publication. The owner accepted the baseline in docs/clarifications/803.md, Round 3.

## What Changes
- Publish full Jira reports as unchanged Markdown attachments with short plain-text summary comments.
- Explicitly identify intermediate clarification reports; stage transitions and managed stage results are reports by definition.
- Retain every full report in Sectile before remote publication and expose it in the task view.
- Preserve immutable publication history and reuse uploaded attachments on retries.
- Return visible upload warnings while allowing otherwise successful stage transitions; still attempt the summary comment.
- Extend tools, managed result contracts and generated issue skills with report metadata and meaningful summaries, preserving older stage callers.

## Scope
Jira intermediate clarification rounds and clarified, specified, implemented, reviewed and finished reports, from standalone and managed execution. GitHub/GitLab publication, ordinary discussion, historic report migration and general Markdown-to-ADF repair are excluded.

## Impact
Tracker transport, report persistence and retrieval, MCP and managed result contracts, stage operation retries, task view, generated skills and user-facing release notes. No production implementation occurs in this change's specification stage. Open product questions: None.
