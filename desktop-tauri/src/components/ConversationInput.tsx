// The editable part of the conversation composer: the CodeMirror editor
// (#916), its `/` and `$` completion menu and chips, and the Up/Down prompt
// history (#913). The draft is a plain string owned by the parent.

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import type { ConversationCommand } from "../lib/conversation";
import { commandChip, commandText, completionContext, isShellDraft, rankCommands, type CompletionContext } from "../domain/command-completion";
import { isRecalling, navigatePromptHistory, type PromptRecall } from "../domain/prompt-history";
import { useComposerMaxHeight } from "../lib/composer-sizing";
import { ComposerEditor, type ComposerEditorHandle, type EditorChip } from "./ComposerEditor";
import { CompletionMenu, type CompletionEntry } from "./CompletionMenu";

export interface ConversationInputHandle { focus(): void; resetHistory(): void }

const MENU_ID = "conversation-completion";
const SHELL_DESCRIPTION = "Shell mode: this message runs as a shell command in this conversation's folder, and the agent sees its output with your next message.";

export const ConversationInput = forwardRef<ConversationInputHandle, {
  conversationId: string;
  value: string;
  onChange: (value: string) => void;
  commands?: ConversationCommand[] | null;
  /** Prompts already sent in this conversation, oldest first. */
  prompts: string[];
  placeholder: string;
  readOnly?: boolean;
  onSubmit: () => void;
  /** Escape with nothing else to close: interrupts a running turn. Returns whether it acted. */
  onEscape: () => boolean;
}>(function ConversationInput({ conversationId, value, onChange, commands, prompts, placeholder, readOnly, onSubmit, onEscape }, ref) {
  const editor = useRef<ComposerEditorHandle>(null);
  const wrapper = useRef<HTMLDivElement>(null);
  useComposerMaxHeight(wrapper, ".conversation-main-column");
  const recall = useRef<PromptRecall>(null);
  const [caret, setCaret] = useState<number | undefined>(value.length);
  const [active, setActive] = useState(0);
  // Escape closes the menu until the token being typed changes.
  const [dismissed, setDismissed] = useState<string>();
  const [focused, setFocused] = useState(false);
  useEffect(() => { recall.current = null; }, [conversationId]);
  useImperativeHandle(ref, () => ({ focus: () => editor.current?.focus(), resetHistory: () => { recall.current = null; } }), []);

  // A recalled prompt is history, not a command being typed: no menu over it.
  const recalled = isRecalling(prompts, value, recall.current);
  const context: CompletionContext | undefined = !readOnly && !recalled && caret !== undefined && !isShellDraft(value) ? completionContext(value, caret) : undefined;
  const contextKey = context ? `${context.from}:${context.trigger}${context.query}` : "";
  const ranked = useMemo(() => context ? rankCommands(commands, context.trigger, context.query) : [], [commands, context?.trigger, context?.query]);
  const open = focused && !!context && ranked.length > 0 && dismissed !== contextKey;
  useEffect(() => { setActive(0); }, [contextKey]);
  const entries: CompletionEntry[] = ranked.map(({ command, match }, index) => ({
    id: `${index}-${command.name}`,
    label: commandText(command, context?.trigger ?? "/"),
    hint: command.argumentHint,
    detail: command.description,
    icon: command.name.startsWith("$") ? "sparkles" : "terminal",
    match: match && { field: match.field === "name" ? "label" : "detail", from: match.field === "name" ? match.from + (context?.trigger === "/" && !command.name.startsWith("/") ? 1 : 0) : match.from, to: match.field === "name" ? match.to + (context?.trigger === "/" && !command.name.startsWith("/") ? 1 : 0) : match.to },
  }));

  function pick(index: number) {
    const entry = ranked[index];
    if (!entry || !context) return;
    const text = `${commandText(entry.command, context.trigger)} `;
    // The rest of the first token is replaced, so picking mid-word does not leave a tail.
    const to = value[context.to] === " " ? context.to + 1 : context.to;
    editor.current?.replace(context.from, to, text);
  }

  const chips = useCallback((doc: string): EditorChip[] => {
    const chip = commandChip(doc, commands);
    return chip ? [chip] : [];
  }, [commands]);

  function onKey(event: KeyboardEvent): boolean {
    if (event.isComposing || editor.current?.composing()) return false;
    const plain = !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey;
    if (open) {
      if (event.key === "ArrowDown" && plain) { setActive(index => (index + 1) % entries.length); return true; }
      if (event.key === "ArrowUp" && plain) { setActive(index => (index - 1 + entries.length) % entries.length); return true; }
      if ((event.key === "Enter" || event.key === "Tab") && plain) { pick(active); return true; }
      if (event.key === "Escape") { setDismissed(contextKey); return true; }
    }
    const current = editor.current?.caret();
    const history = navigatePromptHistory({
      prompts, value: editor.current?.text() ?? value, selectionStart: current ?? null, selectionEnd: current ?? null, recall: recall.current,
      event: { key: event.key, shiftKey: event.shiftKey, ctrlKey: event.ctrlKey, altKey: event.altKey, metaKey: event.metaKey },
      caretOnEdgeRow: direction => editor.current?.caretOnEdgeRow(direction),
    });
    recall.current = history.recall;
    // A recalled prompt is cleared by Escape before Escape can interrupt a turn.
    if (history.handled) { onChange(history.value); return true; }
    if (event.key === "Escape") return onEscape();
    if (event.key === "Enter" && plain) { onSubmit(); return true; }
    return false;
  }

  const shell = isShellDraft(value);
  return <div ref={wrapper} className="conversation-input">
    {open && <CompletionMenu id={MENU_ID} label={context?.trigger === "$" ? "Skills" : "Slash commands"} entries={entries} active={Math.min(active, entries.length - 1)} onActive={setActive} onPick={pick} />}
    <ComposerEditor
      ref={editor}
      value={value}
      onChange={onChange}
      ariaLabel="Next message"
      placeholder={placeholder}
      maxLength={32000}
      readOnly={readOnly}
      chips={chips}
      onKey={onKey}
      onCaret={setCaret}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      attributes={{
        role: "combobox",
        "aria-autocomplete": "list",
        "aria-expanded": open ? "true" : "false",
        "aria-controls": open ? MENU_ID : undefined,
        "aria-activedescendant": open ? `${MENU_ID}-option-${Math.min(active, entries.length - 1)}` : undefined,
        "aria-description": shell ? SHELL_DESCRIPTION : undefined,
      }}
    />
  </div>;
});
