#!/bin/sh
# Managed by Sectile — Claude Code `Notification` hook.
#
# Fires when the agent asks for a permission or otherwise waits for the user.
# It raises no alert of its own: it reports the state to the local agent, and
# the desktop application turns that into a real system notification. A banner
# raised from here would be attributed to a scripting host, could carry no
# icon, and would work on one platform only.
#
# Two rules govern everything below: never write on stdout, which the agent
# reads back, and always exit 0, which is the difference between a notification
# and an interrupted session.

exec 1>/dev/null
trap 'exit 0' HUP INT TERM

payload=$(cat 2>/dev/null)

# The payload is JSON, but jq cannot be assumed present on the workstation and a
# hook is not the place to install a dependency. Only `cwd` is read, and a value
# that cannot be read is simply absent.
cwd=$(printf '%s' "$payload" | tr -d '\n' | sed -n 's/.*"cwd"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
session=""
if [ -n "$cwd" ]; then
    session=$(basename "$cwd" 2>/dev/null)
fi
[ -n "$session" ] || session="Claude Code"

command -v curl >/dev/null 2>&1 || exit 0

# A session Sectile launched carries its run, and reports against it: the run is
# marked as waiting and the whole UI follows.
loopback="${SECTILE_LOOPBACK_URL:-$SECTILE_AGENT_URL}"
if [ -n "$SECTILE_RUN_ID" ] && [ -n "$loopback" ] && [ -n "$SECTILE_AGENT_TOKEN" ]; then
    curl -sS -m 3 -o /dev/null -X POST \
        "$loopback/control/runs/$SECTILE_RUN_ID/waiting" \
        -H "Authorization: Bearer $SECTILE_AGENT_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"waiting":true}' >/dev/null 2>&1 || true
    exit 0
fi

# Any other Claude Code session still deserves the banner — the whole point is
# telling several sessions apart. It has no run, so it reports itself by name,
# using the connection the agent publishes for its companions.
info="$HOME/.taskflow/agent-connection.json"
[ -f "$info" ] || exit 0
raw=$(tr -d '\n' < "$info" 2>/dev/null)
url=$(printf '%s' "$raw" | sed -n 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
token=$(printf '%s' "$raw" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
[ -n "$url" ] && [ -n "$token" ] || exit 0

# The name reaches a JSON string: strip what would end it.
session=$(printf '%s' "$session" | tr -d '"\\')
curl -sS -m 3 -o /dev/null -X POST \
    "$url/desktop/session-alert" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"session\":\"$session\",\"state\":\"waiting\"}" >/dev/null 2>&1 || true

exit 0
