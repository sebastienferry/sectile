#!/bin/sh
# Managed by Sectile — Claude Code hook.
#
# One script, registered on every event that moves a session between working
# and waiting for the user. The payload names the event, and the table below
# turns it into a state. It is one script on purpose: the first release had a
# script per event, and nothing cleared the wait once a permission was granted,
# so a session showed as waiting for the whole turn that followed.
#
#   Notification  a permission prompt, an idle prompt, an elicitation  -> waiting
#   Stop          the turn ended, the agent awaits the next prompt      -> waiting
#   UserPromptSubmit, PreToolUse, PostToolUse  the agent is working     -> working
#
# It raises no alert of its own: it reports the state to the local agent, and
# the desktop application turns that into a real system notification. A banner
# raised from here would be attributed to a scripting host, could carry no
# icon, and would work on one platform only.
#
# Two rules govern everything below: never write on stdout, which the agent
# reads back, and always exit 0, which is the difference between a report and
# an interrupted session.

exec 1>/dev/null
trap 'exit 0' HUP INT TERM

payload=$(cat 2>/dev/null | tr -d '\n')

# The payload is JSON, but jq cannot be assumed present on the workstation and a
# hook is not the place to install a dependency. Three string fields are read,
# and a value that cannot be read is simply absent.
field() {
    printf '%s' "$payload" | sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"
}
event=$(field hook_event_name)

# `state` is what a session with a run reports; `alert` is what a session with
# no run announces about itself, and only two events are worth a banner.
state="" alert=""
case "$event" in
    Notification)
        # Claude Code notifies for more than prompts: a sign-in, a quota notice,
        # a finished sub-agent. Only a prompt means the session waits. A payload
        # with no type is an older Claude Code, and is read as a prompt.
        case "$(field notification_type)" in
            ""|permission_prompt|idle_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input)
                state=waiting alert=waiting ;;
            *) exit 0 ;;
        esac ;;
    Stop) state=waiting alert=completed ;;
    UserPromptSubmit|PreToolUse|PostToolUse) state=working ;;
    *) exit 0 ;;
esac

command -v curl >/dev/null 2>&1 || exit 0

# A session Sectile launched carries its run, and reports against it: the run is
# marked as waiting, or as working again, and the whole UI follows. The agent
# decides what the report means for the run — an autonomous run, for one, never
# waits — and relays it only when the state actually changes.
loopback="${SECTILE_LOOPBACK_URL:-$SECTILE_AGENT_URL}"
if [ -n "$SECTILE_RUN_ID" ] && [ -n "$loopback" ] && [ -n "$SECTILE_AGENT_TOKEN" ]; then
    waiting=false
    [ "$state" = waiting ] && waiting=true
    curl -sS -m 2 -o /dev/null -X POST \
        "$loopback/control/runs/$SECTILE_RUN_ID/waiting" \
        -H "Authorization: Bearer $SECTILE_AGENT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"waiting\":$waiting}" >/dev/null 2>&1 || true
    exit 0
fi

# Any other Claude Code session still deserves the banner — the whole point is
# telling several sessions apart. It has no run, so it reports itself by name,
# using the connection the agent publishes for its companions. Only a start of
# wait and an end of turn are announced: a banner is a transition, and resuming
# is not one anybody needs told about.
[ -n "$alert" ] || exit 0
info="$HOME/.taskflow/agent-connection.json"
[ -f "$info" ] || exit 0
raw=$(tr -d '\n' < "$info" 2>/dev/null)
url=$(printf '%s' "$raw" | sed -n 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
token=$(printf '%s' "$raw" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
[ -n "$url" ] && [ -n "$token" ] || exit 0

session=""
cwd=$(field cwd)
if [ -n "$cwd" ]; then
    session=$(basename "$cwd" 2>/dev/null)
fi
[ -n "$session" ] || session="Claude Code"
# The name reaches a JSON string: strip what would end it.
session=$(printf '%s' "$session" | tr -d '"\\')
curl -sS -m 2 -o /dev/null -X POST \
    "$url/desktop/session-alert" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "{\"session\":\"$session\",\"state\":\"$alert\"}" >/dev/null 2>&1 || true

exit 0
