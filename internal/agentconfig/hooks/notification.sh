#!/bin/sh
# Managed by Sectile — Claude Code `Notification` hook.
#
# Fires when the agent asks for a permission or otherwise waits for the user.
# It raises a desktop alert naming the session, and, when the session was
# launched by Sectile, reports the wait to the local agent so the run shows as
# waiting in the UI.
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
# The name reaches osascript inside a quoted string: strip what would end it.
session=$(printf '%s' "$session" | tr -d '"\\')

if command -v osascript >/dev/null 2>&1; then
    osascript -e "display notification \"$session is waiting for you\" with title \"Agent waiting\" sound name \"Funk\"" >/dev/null 2>&1 || true
fi

# A session Sectile did not launch carries none of these, and reports nothing.
loopback="${SECTILE_LOOPBACK_URL:-$SECTILE_AGENT_URL}"
if [ -n "$SECTILE_RUN_ID" ] && [ -n "$loopback" ] && [ -n "$SECTILE_AGENT_TOKEN" ] && command -v curl >/dev/null 2>&1; then
    curl -sS -m 3 -o /dev/null -X POST \
        "$loopback/control/runs/$SECTILE_RUN_ID/waiting" \
        -H "Authorization: Bearer $SECTILE_AGENT_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"waiting":true}' >/dev/null 2>&1 || true
fi

exit 0
