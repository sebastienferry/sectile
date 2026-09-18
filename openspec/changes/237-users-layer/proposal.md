# A user layer: roles, owned executions and a temporary local sign-in

Ticket: [#237](https://github.com/sebastienferry/sectile/issues/237).
Clarification: `docs/clarifications/237.md`.

## Why
Sectile can already tell people apart. ADR 0008 signs them in through OpenID
Connect, ADR 0011 binds each workstation to a user through an API key, and the
session guard refuses anonymous interface calls once a provider is configured.
What it cannot do is treat two signed-in people differently. There is no role:
every session may rewrite the global settings and the tracker credentials of
everyone. There is no owner on an execution: `POST /api/agent/dispatch` trusts
a `userId` taken from the request body, and a member who presses Stop on a
colleague's run routes the cancel to their own agent, which answers that it has
no such run, and the server then closes the colleague's run as an orphan while
the process keeps running on the other machine.

Multi-user also depends on an identity provider being reachable. A team that
wants to try the shared server before its Okta or Auth0 tenant is wired up has
only the `SECTILE_DEV_IDENTITY` header switch, which was never meant to be used.

## What Changes
- **Two roles**, `admin` and `member`, stored on the user. An admin manages
  users and their roles, projects, global settings, tracker credentials,
  anyone's workstations and anyone's execution. A member does everything else.
- **Executions have an owner.** Every user still sees every running execution
  on the tickets they display. Only the owner or an admin can stop one, and a
  member can only trigger executions on their own agent. Cancel is routed to
  the owner's agent, not the caller's.
- **Roles come from the identity provider** when one is configured: a group or
  role claim named by configuration, with one value that grants `admin`.
  Everyone else is `member`. One provider is active at a time; the existing
  flow already accepts Okta and Auth0 as configuration.
- **A temporary local sign-in** replaces `SECTILE_DEV_IDENTITY` while no
  provider is configured: an account is created from an e-mail address alone,
  the first account is admin, and the implicit single user disappears the
  moment that first account exists. It identifies people without
  authenticating them, says so in the interface, and is disabled as soon as a
  provider is configured.
- **Attribution.** Executions and comments record the user who created them.
- **The interface signs people in** instead of failing silently: a 401 from the
  interface API leads to a sign-in screen that returns to the requested page.
- **The profile shows** the sign-in mode, the identity, the role and the
  projects where the user has a registered agent. Admins get a users view.

## Non-goals
Passwords or mailed links in the local mode: just an e-mail is required to log
in. A `project_members` table or any visibility filter on projects or tasks;
the board stays shared, and the user-to-project binding remains the agent
registration per `(user, project)`. Per-task permissions. Several simultaneous
providers. Any change to how the tracker itself authenticates, or migrating
tracker `team_members` into Sectile users.

## Impact
`internal/auth` (role claim), `internal/handlers/auth.go`, `pairing.go`,
`agent_api.go`, `remote_run.go`, `handlers.go` (local sign-in, guard, admin
checks, ownership, dispatch), `internal/taskmcp` (caller identity on tools),
`internal/db/identity.go`, `sessions.go`, `remoterun.go`, `comments.go`,
`db.go` (`users.role`, `task_activities.user_id`, `task_comments.user_id`),
`internal/models`, `web/src` (sign-in screen, 401 handling, profile, users
view, execution owner), `README.md`, `.env.sample`,
`docs/contracts/server-agent-v1.md`, `docs/CAPABILITIES.md`, ADR 0013,
`.agents/MEMORY.md`.
