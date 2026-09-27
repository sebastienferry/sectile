export function configShortcutAction(event) {
 if (typeof event.key !== 'string' || event.key !== ',') return 'ignore'
 const modifier = event.mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
 if (!modifier || event.shiftKey || event.altKey || event.repeat || event.defaultPrevented || event.modalOpen) return 'ignore'
 return event.configurationOpen ? 'consume' : 'open'
}

export const configShortcutLabel = mac => mac ? '⌘,' : 'Ctrl+,'
export const configShortcutAria = mac => mac ? 'Meta+Comma' : 'Control+Comma'
