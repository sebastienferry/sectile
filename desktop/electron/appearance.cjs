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

module.exports = {APPEARANCES, normalizeAppearance, windowColors}
