// The three values of the Appearance setting (#507), in the order the
// segmented control shows them. The main process owns the stored value.
export const APPEARANCE_CHOICES=[
 {value:'system',label:'System'},
 {value:'dark',label:'Dark'},
 {value:'light',label:'Light'}
]

// The console shows CLI output that picks its own ANSI colours, so each mode
// carries a full palette rather than a background alone: a light background
// with xterm's default ANSI white and yellow would leave text unreadable. The
// dark theme keeps the background and foreground the console always had.
export const TERMINAL_THEMES={
 dark:{
  background:'#11151c',foreground:'#d8e0ec',cursor:'#d8e0ec',cursorAccent:'#11151c',selectionBackground:'#39465a',
  black:'#1a212d',red:'#ff7b7b',green:'#62d3be',yellow:'#f2c46d',blue:'#6ab0ff',magenta:'#c792ea',cyan:'#79c0ff',white:'#d8e0ec',
  brightBlack:'#5b6b7e',brightRed:'#ff9292',brightGreen:'#80e6ce',brightYellow:'#ffd98a',brightBlue:'#a6c8ff',brightMagenta:'#dcb5f5',brightCyan:'#9ecbff',brightWhite:'#ffffff'
 },
 light:{
  background:'#f5f6f8',foreground:'#111315',cursor:'#111315',cursorAccent:'#f5f6f8',selectionBackground:'#c8d6ea',
  black:'#111315',red:'#b42318',green:'#146c43',yellow:'#8a5a00',blue:'#0049a8',magenta:'#8250df',cyan:'#0e6f7a',white:'#6b7480',
  brightBlack:'#48515d',brightRed:'#d92d20',brightGreen:'#1a7f37',brightYellow:'#9a6700',brightBlue:'#2e6fd8',brightMagenta:'#a855f7',brightCyan:'#0f766e',brightWhite:'#8c949e'
 }
}

export function terminalTheme(dark){return dark?TERMINAL_THEMES.dark:TERMINAL_THEMES.light}
