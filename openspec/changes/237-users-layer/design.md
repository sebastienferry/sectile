# Design

## Context
Identity is resolved in exactly two places today, and both return a user id and
nothing more. `webSessionUser` (`internal/handlers/pairing.go`) reads the
session cookie when an OIDC provider is configured, honours the
`SECTILE_DEV_IDENTITY` header otherwise, and falls back to `ImplicitUser`.
`resolveAgentCredential` (`internal/handlers/agent_api.go`) maps a workstation
API key to its user for the agent WebSocket, `/api/v1/agent/*` and `/mcp`.
`RequireSession` wraps the mux in `cmd/server/main.go` and is inert without a
provider. The MCP server (`taskmcp.NewServer(db, sessions)`) never learns who
called it: `start_run` records a run with no owner.

Executions live in `task_activities`, whose columns are listed explicitly in
the insert and in four reads (`.agents/MEMORY.md`, sections 5 and 6). Runs are
routed through `agentDispatcher`, keyed by `(userID, projectID)`; the caller's
own identity is what finds an agent, in `run-skill`, `tty-skill`,
`cancel-run` and `HandleAgentDispatch`, except that the last one prefers a
`userId` supplied in the body.

## Decisions

### One resolver returns a principal, not a string
`webSessionUser` and `resolveAgentCredential` both produce a `principal{UserID,
Role, Mode}` through a shared `h.principalFor(userID)` lookup. Every
authorization check reads the principal; no handler compares user ids by hand
or reads `users.role` itself. The check helpers are three: `requireSession`,
`requireAdmin`, and `requireOwnerOrAdmin(ownerID)`. Each writes `401` when
there is no principal and `403` with a stable message when the role or the
owner does not match, so the interface can tell "sign in" from "not allowed".

**Rejected:** a permission table (`can(user, action, resource)`). Two roles and
one ownership rule do not justify it, and the clarification deferred any third
role.

### `users.role`, with the claim as the authority when there is one
`users` gains `role TEXT NOT NULL DEFAULT 'member'` through the idempotent
`ALTER TABLE ... ADD COLUMN` pattern of `initIdentitySchema`; existing rows
become `member`. The `users` queries list their columns explicitly and are
updated together.

Who sets the role depends on the sign-in mode:

- **OIDC with `SECTILE_OIDC_ROLE_CLAIM` set.** At every sign-in the claim is
  read from the UserInfo response the flow already fetches; when it is absent
  there, from the ID token payload returned by the token endpoint, decoded
  without signature verification for the same reason ADR 0008 accepts UserInfo
  over TLS. The claim may be a string or an array of strings. It grants
  `admin` when one value equals `SECTILE_OIDC_ADMIN_GROUP`, `member`
  otherwise, and the stored role is overwritten. An admin's manual change is
  therefore temporary and the users view says so.
- **OIDC without a role claim, and local mode.** The stored role is the
  authority. The first user created while no admin exists becomes `admin`;
  admins change roles from the users view. This also covers an existing OIDC
  deployment upgrading with users already present: they are all `member` after
  the migration, and the next person to sign in becomes admin, exactly as a
  fresh installation would.

The last admin cannot demote themselves: the users view refuses it with a
message rather than leaving a board nobody can administer.

**Rejected:** `SECTILE_ADMIN_SUBJECTS`, an environment list of admin
identities. The owner chose the claim; a second bootstrap path would be one
more thing to keep in mind and would silently disagree with the claim.

**Rejected:** reading roles from Auth0's Management API or Okta's Groups API.
The claim is the standard channel and needs no extra credential. Auth0 puts
roles in a namespaced claim through a post-login Action
(`api.idToken.setCustomClaim`); Okta adds a `groups` claim on the
authorization server. Both are tenant configuration, verified during
implementation against a tenant, and documented in `README.md` with the two
variables.

### Sign-in modes: `oidc`, `local`, `implicit`
`auth` exposes the mode. `oidc` when `SECTILE_OIDC_ISSUER` is set, unchanged.
Otherwise `local`, with one exception: while the database holds no local
account, the interface keeps `ImplicitUser` (role `admin`) so a personal
deployment, or a fresh one, opens the board as before. The first account
created through the e-mail screen ends that: `webSessionUser` then returns the
cookie's user or nothing.

Local sign-in is `POST /auth/local` with `{"email"}`. It normalises the address
(trim, lower case), upserts a user with subject `local|<email>`, applies the
first-admin rule, opens a web session with the existing `CreateWebSession` and
cookie, and answers `{"userId","role","mode"}`. `GET /auth/login` without a
provider redirects to `/signin?redirect=<safe path>`, an interface route, so
the one URL the interface already uses works in both modes. `/auth/logout` is
unchanged. `publicPath` gains `/auth/local`; `/signin` is static.

`SECTILE_DEV_IDENTITY` and the `X-Sectile-User` header are removed. Users
created by it (`dev|<name>` subjects) stay in the table as ordinary users.
`implicit_user_test.go` moves to the local mode.

**Rejected:** a mailed magic link. Excluded by the owner for this ticket; it is
the first hardening if the local mode outlives the transition.

**Rejected:** keeping `ImplicitUser` alongside local accounts. Two ways to be
somebody on the same server defeats the guard: once an account exists, an
anonymous request is a stranger.

### Executions and comments record their owner
`task_activities` gains `user_id TEXT NOT NULL DEFAULT ''` and `task_comments`
gains the same, both through idempotent `ALTER TABLE`. The five explicit
column lists on `task_activities` (`insertTaskActivity`, the two reads in
`db.go`, `GetActivities`, `GetActivityByID`) and the `remoterun.go` insert are
edited in one commit, with a test that inserts and reads back the owner.
`models.TaskActivity` gains `userId` and `userName` (resolved at read time from
`users`, empty for legacy rows).

Who writes it:

- `start_run` over MCP: the caller's principal. The `/mcp` handler resolves the
  bearer before the SDK sees the request and stores the principal in the
  request context; the SDK passes that context to every tool handler, and
  `taskmcp` reads it through a small `CallerFrom(ctx)` helper. A tool with no
  caller in its context records no owner rather than guessing.
- Agent-owned runs (`RunActionAgent`): the agent's user, known from its
  credential at `Register`. The dispatcher already carries `UserID` on the
  message.
- `run-skill`, `tty-skill`, transitions, tracker operations and comments from
  the web: the session principal. `AddComment` stops reading
  `settings.UserName` and uses the principal's display name or e-mail.
- Existing rows keep an empty owner and are treated as admin-only.

### Ownership rule on stop and dispatch
`handleCancelRemoteRun` loads the run, requires owner-or-admin, and routes the
cancel to the **owner's** agent (`DispatchAndWait(run.UserID, ...)`). This fixes
the orphan-closing bug for admins too: the run is only closed as orphaned when
the agent that should have it says it does not. A legacy run without owner is
admin-only and routed to the caller's agent, as today.

`HandleAgentDispatch` takes the target user from the body only when the caller
is an admin; a member's request is always routed to their own agent, whatever
the body says. `run-skill` and `tty-skill` already use the session user and
need only the `403` when a member names another user's agent through the
`agentUserId` they do not have yet, which is to say nothing changes there
beyond recording the owner.

The web card shows the owner on each running execution and keeps the Stop
control visible for everyone; a member pressing it on someone else's run
receives the server's `403` text. Hiding the control would suggest the
execution is not there, which contradicts the shared board.

**Rejected:** enforcing ownership only in the interface. The desktop app and
MCP clients reach the same routes with a key.

### Admin-only mutations
The list is a single table in `internal/handlers/authz.go`, mirrored by a test
that names each guarded route with the expected status for a member and for an
admin, in the manner of `TestOnlyIntendedPathsBypassTheSessionGuard`:

| Route | Admin-only |
|---|---|
| `POST`/`PUT /api/settings` | yes |
| `POST /api/setup/tracker`, `/api/setup/tracker/check` | yes |
| `POST /api/projects`, `PUT`/`PATCH`/`DELETE /api/projects/{id}` | yes (see Open) |
| `GET /api/users`, `PUT /api/users/{id}` | yes |
| `GET`/`DELETE /api/devices?userId=<other>` | yes; own keys stay member |
| `POST /api/agent/dispatch` with a foreign `userId` | yes |
| `POST /api/tasks/{id}/cancel-run` on a foreign run | yes |

Everything else stays open to any signed-in user, including task edits,
transitions, comments and launching skills on one's own agent.

### Interface: one place learns about 401
`web/src/main.tsx` installs a same-origin `fetch` wrapper: a `401` on a `/api/`
URL calls `redirectToSignIn(location.pathname + location.search)` once, which
goes to `/signin?redirect=...`. The thirty-odd raw `fetch` calls in
`AppContext.tsx` and the hooks are left alone.

`SignInScreen.tsx` is rendered by `App` at `/signin` and whenever `/api/me`
answers `signedIn: false` with a mode other than `implicit`. In `local` mode it
is an e-mail form posting to `/auth/local` with an explicit notice that this
mode identifies without authenticating and is meant for a trusted network
until a provider is connected. In `oidc` mode it is the existing sign-in link.
After success it navigates to the `redirect` path, kept safe by the server's
`safeRedirect` rule reproduced client side (a path, no `//`).

`/api/me` gains `mode` and `role`. `ProfileModal` shows both, lists the projects
where the user has a registered agent (from `/api/agent/status` filtered on the
caller), and, for admins, a **Users** section: e-mail, display name, role,
last sign-in, a role selector, and the note that the provider claim overrides
it when one is configured. The `userName` / `userEmail` / `userAvatar` fields
are hidden once the principal is not the implicit user; the columns are
retired in a later ticket.

**Rejected:** rewriting every fetch through a client wrapper. Right in the long
run, out of proportion here.

### Contract and documentation
`docs/contracts/server-agent-v1.md` gains, under "Workstation API keys and
identity", the `mode` and `role` fields of `/api/v1/agent/identity` and
`/api/me`, the owner on run records, and the `403` on foreign stop and
dispatch. ADR 0013 records the role model, the claim as authority, the local
mode and its limits, and marks ADR 0008's "roles would be a separate decision"
as decided. `.agents/MEMORY.md` gains the sixth `task_activities` column-list
site (`remoterun.go`) and the principal rule.

## Open
- **Project mutations.** The clarification names "projects" among what an
  admin manages, without saying whether editing an existing project's settings
  (skill mode, PR stage, tracker) is admin-only or only creation and deletion.
  This design makes all three admin-only and flags it; the alternative is one
  row less in the table above.
- **Auth0 claim shape.** Whether the tenant emits the roles array in UserInfo
  as well as in the ID token is verified during implementation; the fallback to
  the ID token payload exists for the case where it does not.
