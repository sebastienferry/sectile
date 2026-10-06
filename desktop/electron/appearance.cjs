// The desktop's appearance is a workstation preference kept in settings.json,
// independent of the theme the web profile stores on the server (#507). The
// main process applies it through nativeTheme.themeSource, which drives
// prefers-color-scheme in the renderer and the native widgets alike.
const APPEARANCES = ['system', 'dark', 'light']

// A missing value, or one written by something else, follows the OS: that is
// the default, and the file is left as it is until the user picks a value.
function normalizeAppearance(value) {
 return APPEARANCES.includes(value) ? value : 'system'
}

// The colours the main process paints before the renderer does: the window
// background, and the title bar overlay Windows and Linux draw the window
// controls on. They match the stylesheet's --bg and --text of each mode, so a
// light start shows no dark frame.
function windowColors(dark) {
 return dark
  ? {background: '#11151c', symbol: '#d8e0ec'}
  : {background: '#f5f6f8', symbol: '#111315'}
}

// How a Claude project prompt opens: in a terminal (PTY), as it always has, or
// in the structured conversation view. Anything else keeps the terminal.
const CONSOLE_VIEWS = ['terminal', 'conversation']

function normalizeConsoleView(value) {
 return CONSOLE_VIEWS.includes(value) ? value : 'terminal'
}

// The permission mode the first turn of a new Claude conversation runs in:
// the composer's own modes, never bypassPermissions. Anything else is
// acceptEdits, the mode conversations always started in.
const CONVERSATION_MODES = ['default', 'acceptEdits', 'auto', 'plan']

function normalizeConversationMode(value) {
 return CONVERSATION_MODES.includes(value) ? value : 'acceptEdits'
}

module.exports = {APPEARANCES, normalizeAppearance, windowColors, CONSOLE_VIEWS, normalizeConsoleView, CONVERSATION_MODES, normalizeConversationMode}
