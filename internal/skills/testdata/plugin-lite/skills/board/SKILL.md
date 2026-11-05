---
name: board
description: "Publish or update your own Sectile Board, a live page of your Jira tickets in the Sectile stages, read through your Atlassian connector."
---
# Sectile Board

## Goal
Publish this user's own Sectile Board: the page shipped next to this skill (`board.html`), a board of their Jira tickets in the six Sectile columns, read live through their own Atlassian connector. Running the skill again updates the same board to the plugin's current version instead of creating another one.

## Read first
- `board.html`, in this skill's directory. It is finished: only its `__BOARD_CONFIG__` placeholder is filled in.
- The argument, when there is one: the JQL the board opens on.
- The connectors this session exposes, under the names the host gives them.

## Steps
1. **Follow the host's publishing rules first.** In the Claude app the page is published with the Artifact tool: do what that tool requires before a publish (loading its design and capabilities guidance, for example), but change nothing in the page beyond its configuration.
2. **Pick the connector.** The board reads Jira with the `searchJiraIssuesUsingJql` tool of the user's Atlassian connector. Take that connector's display name exactly as the host lists it for pages (in the Claude app, the capabilities guidance maps each connector to its name), never an internal id. No such connector: stop and say the user has to add the Atlassian connector in claude.ai first.
3. **Pick the Jira site.** List the Atlassian sites the connector reaches. With one Jira site, use its hostname (`example.atlassian.net`). With several, ask which one, once.
4. **Pick the opening query.** The argument when there is one; otherwise `assignee = currentUser() AND statusCategory != Done ORDER BY Rank ASC`. The user can change it in the page at any time.
5. **Fill in the page.** Copy `board.html` to the session's scratch directory and replace `__BOARD_CONFIG__` with one JSON object, `{"connector": "<name>", "site": "<hostname>", "jql": "<query>"}`, written with `<` escaped as `<`. Touch nothing else.
6. **Publish, or update.** List the user's artifacts. When one of theirs is titled `Sectile Board`, read it, then publish the filled-in page to its URL so the link stays the same; otherwise publish a new one, with the icon `board`. Declare its capabilities as `{"mcp": {"servers": [{"server": "<name>", "tools": ["searchJiraIssuesUsingJql"]}]}, "room": {}, "user": {"scopes": ["profile"]}}`.
7. **Hand it over.** Give the link, and say: the first opening asks to allow the connector; the query field changes what the board shows; to start work from a card, keep a Claude Code session running `/sectile-lite:dispatch` open beside it.

## Do not
- Do not change the page beyond `__BOARD_CONFIG__`, and do not write it into the repository.
- Do not share the board: it is the user's, private until they share it from its own menu.
- Do not publish a board without a connector it can read Jira with.
- When the host cannot publish pages, publish nothing: give the path of the filled-in file instead.

## Report
- The board's link, and whether it is new or an update of the existing one.
- The connector, site and query it was published with.
- End every reply addressed to a person with the `### 📋 Status Update` block (Done, Remaining (Agent), Pending (User)), written in the language of the conversation.
