#!/bin/sh
# Managed by Sectile — Claude Code `Stop` hook.
#
# Fires when the agent finishes its turn.
# It raises a desktop alert naming the session, and, when the session was
# launched by Sectile, clears the waiting mark the `Notification` hook set, so a
# resumed run stops showing as waiting.
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
    osascript -e "display notification \"$session finished its turn\" with title \"Agent done\" sound name \"Glass\"" >/dev/null 2>&1 || true
fi

# A session Sectile did not launch carries none of these, and reports nothing.
loopback="${SECTILE_LOOPBACK_URL:-$SECTILE_AGENT_URL}"
if [ -n "$SECTILE_RUN_ID" ] && [ -n "$loopback" ] && [ -n "$SECTILE_AGENT_TOKEN" ] && command -v curl >/dev/null 2>&1; then
    curl -sS -m 3 -o /dev/null -X POST \
        "$loopback/control/runs/$SECTILE_RUN_ID/waiting" \
        -H "Authorization: Bearer $SECTILE_AGENT_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"waiting":false}' >/dev/null 2>&1 || true
fi

exit 0
