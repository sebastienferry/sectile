# ADR 0025: The board is shown inside the desktop, signed in by the workstation key

Status: Proposed

Amends [ADR 0003](0003-local-desktop-consoles.md) ("Navigation is blocked") for one
window. Relies on [ADR 0011](0011-one-api-key-for-agent-and-mcp.md) and
[ADR 0015](0015-sign-in-is-mandatory-and-settings-are-personal.md).
Finding: [`docs/spikes/396-desktop-board.md`](../spikes/396-desktop-board.md).

## Context

Sectile Desktop hosts the consoles; the board lives in the server's web interface.
The desktop's board actions open that interface in the default browser, so a
workstation user moves between two applications, and the browser needs a sign-in of
its own even though the workstation is already paired to that user.

ADR 0003 made the desktop window load only its own renderer and block navigation, so
that the renderer holding the preload bridge never runs remote content. ADR 0011 gave
every workstation one API key, and the server already resolves that key to its user on
the web routes (`webSessionUser`), exactly as it resolves a session cookie.

## Decision

**The desktop shows the server's board in a window of its own.** The board actions
(the **Connected** link, the project heading's web action, open-task) open or bring
forward a single board window loading the connected server's web interface, at the
task when one is asked for.

**The window is signed in by the workstation key, added by the main process.** Every
request of that window whose origin is the server's, issued by that window and by a
frame of the server's origin, carries `Authorization: Bearer <key>`. The key never
enters the page. The window refuses the server's `/auth/` routes: the key is its only
identity, so a key that expires or is revoked leaves it signed out rather than signed
in as somebody else.

**The window is isolated from the rest of the desktop.** It has no preload, runs
sandboxed with context isolation, in a session partition per server origin held in
memory. Pages of the server stay in it; any other web address opens in the default
browser; other schemes are refused. Permissions are refused except writing to the
clipboard. Another key or another server replaces the window and clears its storage;
closing the main window closes it.

**The browser stays the fallback.** When the desktop holds no key, or holds one saved
for another server than the one the agent is connected to, the board actions open the
browser as they did before.

ADR 0003 is amended accordingly: navigation remains blocked in the desktop's own
window; the board window is the one place remote content is loaded, and it is the one
window without the bridge.

## Consequences

- The board is one click away from the consoles, already signed in, with the whole web
  interface available: card moves, task detail, skill launch, live updates.
- No server or web change is required. The web's **Sign out** cannot end a key, and
  in the board window it reports that it could not sign out; `/api/me` telling the web
  that the caller is a key would let it hide the action (follow-up).
- The key is now sent by a second component of the desktop, the board window, but
  only to the origin it was issued for; the tests pin that scope.
- A workstation whose key was issued by another server keeps the browser path, and so
  does a desktop connected to an agent it did not start and holding no key.

## Rejected

- **A `WebContentsView` inside the main window**: the same isolation needs more layout
  code and mixes the main window's rules with the board's.
- **The web sign-in inside the window**: a second sign-in on a paired workstation,
  identity-provider origins to allow, and a board that could be someone else's.
- **Handing the key to the page**: page script, and anything it loads, could read it.
- **A board drawn by the desktop, or a board without a server**: ruled out when #396
  was clarified; they duplicate the web board or reverse the topology.
