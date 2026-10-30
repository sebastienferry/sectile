export type WorkspaceShortcut = "none" | "settings" | "sidebar" | "prompt" | "folder";

interface ShortcutEvent {
  key: string;
  metaKey: boolean;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  repeat: boolean;
  defaultPrevented: boolean;
  isComposing: boolean;
}

interface ShortcutContext {
  mac: boolean;
  modalOpen: boolean;
  terminalFocused: boolean;
  editableFocused: boolean;
  settingsOpen: boolean;
  hasProject: boolean;
}

export function workspaceShortcut(event: ShortcutEvent, context: ShortcutContext): WorkspaceShortcut {
  const modifier = context.mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey;
  if (!modifier || event.altKey || event.shiftKey || event.repeat || event.defaultPrevented || event.isComposing || context.modalOpen) return "none";
  const key = event.key.toLowerCase();
  if (key === ",") return "settings";
  if (key !== "b" && key !== "n") return "none";
  if (!context.mac && (context.terminalFocused || context.editableFocused)) return "none";
  if (key === "b") return context.settingsOpen ? "none" : "sidebar";
  return context.hasProject ? "prompt" : "folder";
}

export function shortcutModifier(mac: boolean): string {
  return mac ? "⌘" : "Ctrl+";
}
