// The three values of the Appearance setting (#507), in the order the
// segmented control shows them: System (monitor), Light (sun), Dark (moon).
export const APPEARANCE_CHOICES=[
 {value:'system',label:'System',icon:'<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></svg>'},
 {value:'light',label:'Light',icon:'<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/></svg>'},
 {value:'dark',label:'Dark',icon:'<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>'}
]

// How a Claude project prompt opens (see electron/appearance.cjs).
export const CONSOLE_VIEW_CHOICES=[
 {value:'terminal',label:'Terminal',icon:'<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7.5 10 2.5 2.5-2.5 2.5"/><path d="M13 15h4"/></svg>'},
 {value:'conversation',label:'Conversation',icon:'<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/><path d="M8 9h8M8 13h5"/></svg>'}
]

// The permission mode a new Claude conversation starts in, with the labels of
// the composer's own Permission mode select (see electron/appearance.cjs).
export const CONVERSATION_MODE_CHOICES=[
 {value:'default',label:'Ask before edits'},
 {value:'acceptEdits',label:'Accept edits'},
 {value:'auto',label:'Auto mode'},
 {value:'plan',label:'Plan mode'}
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
