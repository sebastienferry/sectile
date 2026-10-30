import assert from "node:assert/strict";
import test from "node:test";
import { workspaceShortcut } from "../src/lib/workspace-shortcuts.ts";

const event = (key: string, overrides: Record<string, unknown> = {}) => ({
  key, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false,
  repeat: false, defaultPrevented: false, isComposing: false, ...overrides,
});
const context = (overrides: Record<string, unknown> = {}) => ({
  mac: true, modalOpen: false, terminalFocused: false, editableFocused: false,
  settingsOpen: false, hasProject: true, ...overrides,
});

test("desktop shortcuts open settings, toggle projects and start a project prompt", () => {
  assert.equal(workspaceShortcut(event(",", { metaKey: true }), context()), "settings");
  assert.equal(workspaceShortcut(event("Comma", { metaKey: true }), context()), "settings");
  assert.equal(workspaceShortcut(event("b", { metaKey: true }), context()), "sidebar");
  assert.equal(workspaceShortcut(event("n", { metaKey: true }), context()), "prompt");
  assert.equal(workspaceShortcut(event("n", { metaKey: true }), context({ hasProject: false })), "folder");
  assert.equal(workspaceShortcut(event("b", { metaKey: true }), context({ settingsOpen: true })), "none");
});

test("shortcuts leave terminal input, editable fields and dialogs alone", () => {
  assert.equal(workspaceShortcut(event("b", { ctrlKey: true }), context({ mac: false, terminalFocused: true })), "none");
  assert.equal(workspaceShortcut(event("n", { ctrlKey: true }), context({ mac: false, editableFocused: true })), "none");
  assert.equal(workspaceShortcut(event(",", { metaKey: true }), context({ modalOpen: true })), "none");
  assert.equal(workspaceShortcut(event("n", { metaKey: true, repeat: true }), context()), "none");
  assert.equal(workspaceShortcut(event("b", { metaKey: true, isComposing: true }), context()), "none");
  assert.equal(workspaceShortcut(event("b", { ctrlKey: true }), context()), "none");
});
