export interface SidebarShortcutEvent {
  key: string
  metaKey: boolean
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  repeat: boolean
  defaultPrevented: boolean
  /** The platform's modifier is Cmd rather than Ctrl. */
  mac: boolean
  /** The focus is inside a terminal. */
  inTerminal: boolean
  /** A modal dialog is open. */
  modalOpen: boolean
}
export function isMacPlatform(nav: {platform?: string; userAgentData?: {platform?: string}} | undefined): boolean
export function sidebarShortcutAction(e: SidebarShortcutEvent): 'toggle' | 'ignore'
export function sidebarShortcutLabel(mac: boolean): string
export function sidebarShortcutAria(mac: boolean): string
