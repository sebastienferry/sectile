// Drives the CodeMirror conversation composer (#916) the way the textarea tests
// used to: find it, replace its text, read it, move the caret, press keys.
import { act, fireEvent, screen } from "@testing-library/react";
import { EditorView } from "@codemirror/view";

export const findComposer = (name = "Next message") => screen.findByRole("combobox", { name });
export const getComposer = (name = "Next message") => screen.getByRole("combobox", { name });
export const queryComposer = (name = "Next message") => screen.queryByRole("combobox", { name });

function view(element: HTMLElement): EditorView {
  const editor = EditorView.findFromDOM(element);
  if (!editor) throw new Error("not a composer editor");
  return editor;
}

/** Replaces the whole text, caret at the end, as typing it would. */
export function typeIn(element: HTMLElement, text: string, caret = text.length) {
  const editor = view(element);
  act(() => { if (!editor.hasFocus) editor.focus(); editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: text }, selection: { anchor: caret }, userEvent: "input.type" }); });
}
export const valueOf = (element: HTMLElement) => view(element).state.doc.toString();
export function setCaret(element: HTMLElement, position: number) { act(() => { view(element).dispatch({ selection: { anchor: position } }); }); }
export const caretOf = (element: HTMLElement) => view(element).state.selection.main.head;
export const press = (element: HTMLElement, key: string, init: KeyboardEventInit = {}) => fireEvent.keyDown(element, { key, ...init });
