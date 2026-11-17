// The conversation composer's editor (#916): CodeMirror 6, so a command, a
// skill or a mention can show as a real inline chip, deleted at once by
// Backspace. The draft stays a plain string owned by the parent: chips are
// presentation only, and what is sent is exactly the text.
// See docs/adrs/0019-codemirror-conversation-composer.md.

import { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useRef } from "react";
import { Compartment, EditorSelection, EditorState, Prec, RangeSetBuilder, StateEffect, StateField, type Extension } from "@codemirror/state";
import { Decoration, EditorView, WidgetType, keymap, placeholder as placeholderExtension, type DecorationSet } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";

/** A range of the draft shown as an inline chip. */
export interface EditorChip { from: number; to: number; kind: string; label: string; /** SVG path data of a 24×24 icon shown before the label. */ icon?: string }

export interface ComposerEditorHandle {
  focus(): void;
  /** The caret position, or undefined while text is selected. */
  caret(): number | undefined;
  /** Whether the caret is on the first ("up") or last ("down") rendered row; undefined without layout. */
  caretOnEdgeRow(direction: "up" | "down"): boolean | undefined;
  /** Replaces `from..to` with `text` and puts the caret after it, as typing would. */
  replace(from: number, to: number, text: string): void;
  /** Whether an IME composition is running. */
  composing(): boolean;
  /** The current text, which may be ahead of the parent's value while React has not rendered yet. */
  text(): string;
  readonly dom: HTMLElement | null;
}

export interface ComposerEditorProps {
  value: string;
  onChange: (value: string) => void;
  ariaLabel: string;
  placeholder?: string;
  maxLength?: number;
  readOnly?: boolean;
  /** The chips of a document. Called on every change; keep it cheap. */
  chips?: (doc: string) => EditorChip[];
  /** Runs before CodeMirror's own keys; return true when the key was handled. */
  onKey?: (event: KeyboardEvent) => boolean;
  /** Called with the caret (undefined while text is selected) whenever it or the text moves. */
  onCaret?: (caret: number | undefined) => void;
  onFocus?: () => void;
  onBlur?: () => void;
  /** Extra attributes for the editable element: ARIA combobox state, description. */
  attributes?: Record<string, string | undefined>;
  className?: string;
  autoFocus?: boolean;
}

class ChipWidget extends WidgetType {
  constructor(readonly chip: EditorChip) { super(); }
  eq(other: ChipWidget) { return other.chip.label === this.chip.label && other.chip.kind === this.chip.kind && other.chip.icon === this.chip.icon; }
  toDOM() {
    const element = document.createElement("span");
    element.className = `composer-chip-token composer-chip-${this.chip.kind}`;
    element.dataset.kind = this.chip.kind;
    if (this.chip.icon) {
      const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      svg.setAttribute("viewBox", "0 0 24 24");
      svg.setAttribute("aria-hidden", "true");
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", this.chip.icon);
      svg.append(path);
      element.append(svg);
    }
    element.append(this.chip.label);
    return element;
  }
  ignoreEvent() { return false; }
}

const setChipSource = StateEffect.define<(doc: string) => EditorChip[]>();

function chipField(initial: (doc: string) => EditorChip[]) {
  const build = (state: EditorState, source: (doc: string) => EditorChip[]) => {
    const builder = new RangeSetBuilder<Decoration>();
    const chips = source(state.doc.toString())
      .filter(chip => chip.from < chip.to && chip.to <= state.doc.length)
      .sort((a, b) => a.from - b.from);
    let last = -1;
    for (const chip of chips) {
      if (chip.from < last) continue;
      builder.add(chip.from, chip.to, Decoration.replace({ widget: new ChipWidget(chip) }));
      last = chip.to;
    }
    return builder.finish();
  };
  const sourceField = StateField.define<(doc: string) => EditorChip[]>({
    create: () => initial,
    update: (value, transaction) => transaction.effects.reduce((current, effect) => effect.is(setChipSource) ? effect.value : current, value),
  });
  const decorations = StateField.define<DecorationSet>({
    create: state => build(state, state.field(sourceField)),
    update: (value, transaction) => transaction.docChanged || transaction.effects.some(effect => effect.is(setChipSource)) ? build(transaction.state, transaction.state.field(sourceField)) : value,
    provide: field => [EditorView.decorations.from(field), EditorView.atomicRanges.of(view => view.state.field(field))],
  });
  return [sourceField, decorations];
}

const noChips = () => [];
const editableCompartment = new Compartment();
const placeholderCompartment = new Compartment();
const editable = (readOnly?: boolean) => [EditorState.readOnly.of(!!readOnly), EditorView.editable.of(!readOnly)];

export const ComposerEditor = forwardRef<ComposerEditorHandle, ComposerEditorProps>(function ComposerEditor(props, ref) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  // The latest props, read by CodeMirror callbacks created once.
  const latest = useRef(props);
  latest.current = props;

  useLayoutEffect(() => {
    const parent = host.current!;
    const reportCaret = (state: EditorState) => {
      const main = state.selection.main;
      latest.current.onCaret?.(main.empty ? main.head : undefined);
    };
    const extensions: Extension[] = [
      Prec.highest(keymap.of([{ any: (_view, event) => latest.current.onKey?.(event) ?? false }])),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.lineWrapping,
      placeholderCompartment.of(placeholderExtension(props.placeholder ?? "")),
      editableCompartment.of(editable(props.readOnly)),
      EditorState.changeFilter.of(transaction => !props.maxLength || transaction.newDoc.length <= props.maxLength || transaction.newDoc.length <= transaction.startState.doc.length),
      chipField(props.chips ?? noChips),
      EditorView.updateListener.of(update => {
        if (update.docChanged) {
          const text = update.state.doc.toString();
          if (text !== latest.current.value) latest.current.onChange(text);
        }
        if (update.docChanged || update.selectionSet) reportCaret(update.state);
        if (update.focusChanged) (update.view.hasFocus ? latest.current.onFocus : latest.current.onBlur)?.();
      }),
      EditorView.contentAttributes.of({ "aria-label": props.ariaLabel, "aria-multiline": "true" }),
    ];
    const editor = new EditorView({ parent, state: EditorState.create({ doc: props.value, extensions, selection: EditorSelection.cursor(props.value.length) }) });
    view.current = editor;
    if (props.autoFocus) editor.focus();
    return () => { editor.destroy(); view.current = null; };
    // The editor is created once; later prop changes are pushed below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A value set by the parent (send, history, Edit) replaces the document, caret at the end.
  useLayoutEffect(() => {
    const editor = view.current;
    if (!editor) return;
    const current = editor.state.doc.toString();
    if (current === props.value) return;
    editor.dispatch({ changes: { from: 0, to: current.length, insert: props.value }, selection: EditorSelection.cursor(props.value.length) });
  }, [props.value]);

  useEffect(() => { view.current?.dispatch({ effects: setChipSource.of(props.chips ?? noChips) }); }, [props.chips]);
  useEffect(() => { view.current?.dispatch({ effects: editableCompartment.reconfigure(editable(props.readOnly)) }); }, [props.readOnly]);
  useEffect(() => { view.current?.dispatch({ effects: placeholderCompartment.reconfigure(placeholderExtension(props.placeholder ?? "")) }); }, [props.placeholder]);

  // ARIA state changes often (active option): set it on the element directly.
  const attributes = props.attributes ?? {};
  // Undefined becomes null so an attribute that goes away is removed rather than dropped from the key.
  const attributesKey = JSON.stringify(attributes, (_key, value) => value === undefined ? null : value);
  useLayoutEffect(() => {
    const element = view.current?.contentDOM;
    if (!element) return;
    for (const [name, value] of Object.entries(JSON.parse(attributesKey) as Record<string, string | null>)) {
      if (value === null) element.removeAttribute(name);
      else element.setAttribute(name, value);
    }
  }, [attributesKey]);

  useImperativeHandle(ref, () => ({
    focus: () => view.current?.focus(),
    caret: () => {
      const main = view.current?.state.selection.main;
      return main?.empty ? main.head : undefined;
    },
    caretOnEdgeRow: direction => {
      const editor = view.current;
      if (!editor) return undefined;
      const head = editor.state.selection.main.head;
      const here = editor.coordsAtPos(head);
      const edge = editor.coordsAtPos(direction === "up" ? 0 : editor.state.doc.length);
      if (!here || !edge || (here.top === 0 && here.bottom === 0)) return undefined;
      return direction === "up" ? here.top <= edge.top + 1 : here.bottom >= edge.bottom - 1;
    },
    replace: (from, to, text) => {
      view.current?.dispatch({ changes: { from, to, insert: text }, selection: EditorSelection.cursor(from + text.length), userEvent: "input.complete" });
    },
    composing: () => !!view.current?.composing,
    text: () => view.current?.state.doc.toString() ?? latest.current.value,
    get dom() { return view.current?.contentDOM ?? null; },
  }), []);

  return <div ref={host} className={`composer-editor${props.className ? ` ${props.className}` : ""}`} />;
});
