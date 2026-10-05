// The three values of the Appearance setting (#507), in the order the
// segmented control shows them. The main process owns the stored value.
export const APPEARANCE_CHOICES=[
 {value:'system',label:'System'},
 {value:'dark',label:'Dark'},
 {value:'light',label:'Light'}
]

// How a Claude project prompt opens (see electron/appearance.cjs).
export const CONSOLE_VIEW_CHOICES=[
 {value:'terminal',label:'Terminal'},
 {value:'conversation',label:'Conversation'}
]

// The console shows CLI output that picks its own ANSI colours, so each mode
// carries a full palette rather than a background alone: a light background
// with xterm's default ANSI white and yellow would leave text unreadable. Every
// light ANSI colour reaches 4.5:1 (WCAG AA) against the light background. The
// dark theme keeps the background and foreground the console always had.
export const TERMINAL_THEMES={
 dark:{
  background:'#11151c',foreground:'#d8e0ec',cursor:'#d8e0ec',cursorAccent:'#11151c',selectionBackground:'#39465a',
  black:'#1a212d',red:'#ff7b7b',green:'#62d3be',yellow:'#f2c46d',blue:'#6ab0ff',magenta:'#c792ea',cyan:'#79c0ff',white:'#d8e0ec',
  brightBlack:'#5b6b7e',brightRed:'#ff9292',brightGreen:'#80e6ce',brightYellow:'#ffd98a',brightBlue:'#a6c8ff',brightMagenta:'#dcb5f5',brightCyan:'#9ecbff',brightWhite:'#ffffff'
 },
 light:{
  background:'#f5f6f8',foreground:'#111315',cursor:'#111315',cursorAccent:'#f5f6f8',selectionBackground:'#c8d6ea',
  black:'#111315',red:'#b42318',green:'#146c43',yellow:'#8a5a00',blue:'#0049a8',magenta:'#8250df',cyan:'#0e6f7a',white:'#5c6470',
  brightBlack:'#48515d',brightRed:'#c82a1d',brightGreen:'#1a7f37',brightYellow:'#946300',brightBlue:'#2b68cc',brightMagenta:'#9149d6',brightCyan:'#0f766e',brightWhite:'#6a717b'
 }
}

export function terminalTheme(dark){return dark?TERMINAL_THEMES.dark:TERMINAL_THEMES.light}

// A CLI picks its own colours: Claude Code's dark theme prints bold questions in truecolor white, which no palette
// remaps. In light mode xterm darkens any cell colour below the WCAG AA ratio against its background; dark keeps
// colours as printed.
export const LIGHT_MINIMUM_CONTRAST=4.5
export function terminalOptions(dark){return {minimumContrastRatio:dark?1:LIGHT_MINIMUM_CONTRAST,theme:terminalTheme(dark)}}
