// Cmd+N on macOS, Ctrl+N elsewhere, opens the new task dialog. A focused
// terminal keeps Ctrl+N, which shells read as "next history line"; Cmd+N
// never reaches a terminal, so on macOS it opens the dialog from anywhere.
export function newTaskShortcutAction(event) {
  if (typeof event.key !== 'string' || event.key.toLowerCase() !== 'n') return 'ignore'
  const modifier = event.mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
  if (!modifier || event.shiftKey || event.altKey || event.repeat || event.defaultPrevented || event.modalOpen) return 'ignore'
  if (!event.mac && event.inTerminal) return 'ignore'
  return 'open'
}

export const newTaskShortcutLabel = mac => mac ? '⌘N' : 'Ctrl+N'
