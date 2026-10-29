# Desktop provider login

The authenticated loopback agent exposes `GET /desktop/provider-login?provider=claude|codex`.
It returns `installed`, the resolved executable `path` when installed, and
`authentication` (`connected`, `disconnected`, `unknown`). Official status commands
run with an eight-second deadline and hidden Windows consoles. No CLI output or
credential content is returned. Exit zero indicates credentials are present;
this is not a model entitlement or remote token validity check.

`POST` on the same route opens the official login command in a visible terminal.
Its `opened` response confirms only the terminal launch. The desktop rechecks
status for two minutes and also offers manual refresh. Login uses the same home
and CLI resolution as execution. macOS uses Terminal; Windows uses a command
window; Linux requires x-terminal-emulator. Only Claude and Codex are accepted.
