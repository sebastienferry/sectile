// Cmd+Enter on macOS, Ctrl+Enter elsewhere, activates a dialog's default
// button without leaving the keyboard, as GitHub, Slack or Gmail do.
// Shift+Enter stays a new line, which it is in every text field here.
export function defaultActionShortcut(event) {
  if (event.key !== 'Enter' || event.isComposing || event.repeat || event.shiftKey || event.altKey) return false
  return event.mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
}

export const defaultActionLabel = mac => mac ? '⌘↵' : 'Ctrl+Enter'

const enabledSubmit = form => [...form.querySelectorAll('button[type=submit],button:not([type])')].find(button => !button.disabled && !button.hidden && form.contains(button))

// The control the shortcut activates in root: the submit button of the form
// holding the focus, else of the first form with one, else a button marked
// data-default-action. A confirmation outside any form is never activated
// unless it is marked: the shortcut submits what was typed, it does not
// confirm what was not asked.
export function defaultActionTarget(root, active) {
  const own = active?.closest?.('form')
  const forms = [...(own && root.contains(own) ? [own] : []), ...root.querySelectorAll('form')]
  for (const form of forms) {
    const button = enabledSubmit(form)
    if (button) return { form, button }
  }
  const marked = [...root.querySelectorAll('[data-default-action]')].find(button => !button.disabled && !button.hidden)
  return marked ? { button: marked } : null
}
