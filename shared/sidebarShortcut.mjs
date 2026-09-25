// The sidebar shortcut (#474), decided in one place so that the web app and the
// desktop app cannot disagree on it. Cmd+B on macOS, Ctrl+B elsewhere: on macOS
// Ctrl+B is never taken, and elsewhere Ctrl+B stays with a focused terminal,
// where it means something (tmux prefix, readline, Claude Code's background).

export function isMacPlatform(nav) {
 const platform = nav?.userAgentData?.platform ?? nav?.platform ?? ''
 return /mac/i.test(platform)
}

// 'toggle' asks the caller to cancel the event and toggle the sidebar. 'ignore'
// asks it to leave the event untouched, so that the key reaches whatever has
// the focus.
export function sidebarShortcutAction(e) {
 if (typeof e.key !== 'string' || e.key.toLowerCase() !== 'b') return 'ignore'
 const modifier = e.mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey
 if (!modifier || e.shiftKey || e.altKey || e.repeat) return 'ignore'
 if (e.defaultPrevented || e.modalOpen) return 'ignore'
 if (e.inTerminal && !e.mac) return 'ignore'
 return 'toggle'
}

// What a tooltip shows, and what aria-keyshortcuts announces.
export const sidebarShortcutLabel = mac => mac ? '⌘B' : 'Ctrl+B'
export const sidebarShortcutAria = mac => mac ? 'Meta+B' : 'Control+B'
