// The editable part of a composer: the CodeMirror editor (#916), its `/` and
// `$` command menu and chips, its `@` mention menu and chips (#917), and the
// Up/Down prompt history of a conversation (#913). The draft is a plain
// string owned by the parent.

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import type { ConversationCommand } from "../lib/conversation";
import { commandChip, commandText, completionContext, isShellDraft, rankCommands, type CompletionContext } from "../domain/command-completion";
import { mentionChips, mentionContext, mentionToken, rankConversations, rankTickets, type MentionContext, type MentionConversation, type MentionTicket, type MentionTokenKind } from "../domain/mentions";
import { isRecalling, navigatePromptHistory, type PromptRecall } from "../domain/prompt-history";
import { useComposerMaxHeight } from "../lib/composer-sizing";
import { mentionFiles, type MentionFile } from "../lib/mentions";
import { ComposerEditor, type ComposerEditorHandle, type EditorChip } from "./ComposerEditor";
import { CompletionMenu, type CompletionEntry } from "./CompletionMenu";
import { iconPath, type IconName } from "./Icon";

export interface PromptInputHandle { focus(): void; resetHistory(): void }

/** What `@` can mention besides files: the project's other conversations and its tickets. */
export interface MentionSources { conversations: MentionConversation[]; tickets: MentionTicket[] }

const SHELL_DESCRIPTION = "Shell mode: this message runs as a shell command in this conversation's folder, and the agent sees its output with your next message.";
// Per-kind caps keep every kind visible within the menu's 50 entries.
const FILE_LIMIT = 25, CONVERSATION_LIMIT = 10, TICKET_LIMIT = 15;
const chipIcons: Record<MentionTokenKind, IconName> = { file: "file", conversation: "chat", ticket: "ticket" };

type MentionPick =
  | { kind: "file" | "folder"; file: MentionFile }
  | { kind: "conversation"; item: MentionConversation }
  | { kind: "ticket"; item: MentionTicket };

function matchIn(text: string, query: string) {
  const at = query ? text.toLowerCase().indexOf(query.toLowerCase()) : -1;
  return at >= 0 ? { from: at, to: at + query.length } : undefined;
}

export const PromptInput = forwardRef<PromptInputHandle, {
  ariaLabel: string;
  /** The view whose height caps the composer's (#913). */
  viewSelector: string;
  projectId: string;
  /** The conversation, once there is one: its history, and the folder its files come from. */
  conversationId?: string;
  value: string;
  onChange: (value: string) => void;
  commands?: ConversationCommand[] | null;
  /** Prompts already sent in this conversation, oldest first; history is off without them. */
  prompts?: string[];
  /** Turns `@` mentions on. */
  mentions?: MentionSources;
  placeholder: string;
  readOnly?: boolean;
  autoFocus?: boolean;
  onSubmit: () => void;
  /** Escape with nothing else to close: interrupts a running turn. Returns whether it acted. */
  onEscape?: () => boolean;
}>(function PromptInput({ ariaLabel, viewSelector, projectId, conversationId, value, onChange, commands, prompts, mentions, placeholder, readOnly, autoFocus, onSubmit, onEscape }, ref) {
  const editor = useRef<ComposerEditorHandle>(null);
  const wrapper = useRef<HTMLDivElement>(null);
  useComposerMaxHeight(wrapper, viewSelector);
  const recall = useRef<PromptRecall>(null);
  const [caret, setCaret] = useState<number | undefined>(value.length);
  const [active, setActive] = useState(0);
  // Escape closes the menu until the word being typed changes.
  const [dismissed, setDismissed] = useState<string>();
  const [focused, setFocused] = useState(!!autoFocus);
  const [files, setFiles] = useState<{ query: string; list: MentionFile[] }>({ query: "", list: [] });
  const menuId = `${ariaLabel.replace(/\W+/g, "-").toLowerCase()}-completion`;
  useEffect(() => { recall.current = null; }, [conversationId]);
  useImperativeHandle(ref, () => ({ focus: () => editor.current?.focus(), resetHistory: () => { recall.current = null; } }), []);

  // A recalled prompt is history, not a word being typed: no menu over it.
  const recalled = !!prompts && isRecalling(prompts, value, recall.current);
  const typing = !readOnly && !recalled && caret !== undefined && !isShellDraft(value);
  const command: CompletionContext | undefined = typing && commands ? completionContext(value, caret) : undefined;
  const mention: MentionContext | undefined = typing && !command && mentions ? mentionContext(value, caret) : undefined;
  const contextKey = command ? `/${command.from}:${command.trigger}${command.query}` : mention ? `@${mention.from}:${mention.query}` : "";

  // Files come from the agent, a little after the user stops typing.
  const mentionQuery = mention?.query;
  useEffect(() => {
    if (mentionQuery === undefined) return;
    let current = true;
    const timer = setTimeout(() => {
      void mentionFiles(projectId, conversationId, mentionQuery).then(list => { if (current) setFiles({ query: mentionQuery, list }); });
    }, 120);
    return () => { current = false; clearTimeout(timer); };
  }, [mentionQuery, projectId, conversationId]);

  const ranked = useMemo(() => command ? rankCommands(commands, command.trigger, command.query) : [], [commands, command?.trigger, command?.query]);
  const picks: MentionPick[] = useMemo(() => {
    if (!mention || !mentions) return [];
    const query = mention.query;
    const fileList = files.query === query || query.startsWith(files.query) ? files.list.filter(file => file.path.toLowerCase().includes(query.toLowerCase())) : [];
    return [
      ...fileList.slice(0, FILE_LIMIT).map(file => ({ kind: file.kind, file }) as MentionPick),
      ...rankConversations(mentions.conversations.filter(item => item.id !== conversationId), query, CONVERSATION_LIMIT).map(({ item }) => ({ kind: "conversation", item }) as MentionPick),
      ...rankTickets(mentions.tickets, query, TICKET_LIMIT).map(({ item }) => ({ kind: "ticket", item }) as MentionPick),
    ];
  }, [mention?.query, mentions, files, conversationId]);

  // Another word, or files arriving and shifting the rows, start again from the first entry.
  const entriesKey = `${contextKey}|${files.query}|${files.list.length}`;
  useEffect(() => { setActive(0); }, [entriesKey]);
  // Another project's or conversation's files are never offered.
  useEffect(() => { setFiles({ query: "", list: [] }); }, [projectId, conversationId]);

  const entries: CompletionEntry[] = command
    ? ranked.map(({ command: item, match }, index) => {
      const label = commandText(item, command.trigger);
      const shift = label.length - item.name.length;
      return {
        id: `${index}-${item.name}`, label, hint: item.argumentHint, detail: item.description,
        icon: item.name.startsWith("$") ? "sparkles" : "terminal",
        match: match && (match.field === "name" ? { field: "label", from: match.from + shift, to: match.to + shift } : { field: "detail", from: match.from, to: match.to }),
      };
    })
    : picks.map((pick, index) => {
      const query = mention?.query ?? "";
      if ("file" in pick) {
        const label = pick.kind === "folder" ? `${pick.file.path}/` : pick.file.path;
        const range = matchIn(label, query);
        return { id: `${index}-${pick.file.absolute}`, label, icon: pick.kind, detail: pick.file.rootLabel === "Working folder" ? undefined : pick.file.rootLabel, match: range && { field: "label", ...range } };
      }
      if (pick.kind === "conversation") {
        const range = matchIn(pick.item.title, query);
        return { id: `${index}-c-${pick.item.id}`, label: pick.item.title || "Conversation", icon: "chat", detail: pick.item.archived ? "Archived" : "Conversation", match: range && { field: "label", ...range } };
      }
      const key = pick.item.key ?? pick.item.id;
      const inKey = matchIn(key, query.replace(/^#/, ""));
      const inTitle = inKey ? undefined : matchIn(pick.item.title, query);
      return { id: `${index}-t-${pick.item.id}`, label: key, icon: "ticket", detail: pick.item.title, match: inKey ? { field: "label", ...inKey } : inTitle && { field: "detail", ...inTitle } };
    });
  const open = focused && !!contextKey && entries.length > 0 && dismissed !== contextKey;
  const activeIndex = Math.min(active, Math.max(0, entries.length - 1));

  function replaceWord(from: number, to: number, text: string) {
    // The rest of the word is replaced, so picking mid-word leaves no tail.
    const end = value[to] === " " && text.endsWith(" ") ? to + 1 : to;
    editor.current?.replace(from, end, text);
  }
  function pick(index: number) {
    if (command) {
      const entry = ranked[index];
      if (entry) replaceWord(command.from, command.to, `${commandText(entry.command, command.trigger)} `);
      return;
    }
    const chosen = picks[index];
    if (!mention || !chosen) return;
    if (chosen.kind === "folder") {
      // A folder is text: the menu stays open on its content.
      replaceWord(mention.from, mention.to, `@${chosen.file.path}/`);
      return;
    }
    const token = "file" in chosen
      ? mentionToken("file", chosen.file.absolute, chosen.file.path.split("/").pop() || chosen.file.path)
      : chosen.kind === "conversation"
        ? mentionToken("conversation", chosen.item.id, chosen.item.title || "Conversation")
        : mentionToken("ticket", chosen.item.id, chosen.item.key ?? chosen.item.title);
    replaceWord(mention.from, mention.to, `${token} `);
  }

  const chips = useCallback((doc: string): EditorChip[] => {
    const list: EditorChip[] = mentions ? mentionChips(doc).map(chip => ({ ...chip, icon: iconPath(chipIcons[chip.kind]) })) : [];
    const first = commands ? commandChip(doc, commands) : undefined;
    return first ? [first, ...list] : list;
  }, [commands, !!mentions]);

  function onKey(event: KeyboardEvent): boolean {
    if (event.isComposing || editor.current?.composing()) return false;
    const plain = !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey;
    if (open) {
      if (event.key === "ArrowDown" && plain) { setActive(index => (index + 1) % entries.length); return true; }
      if (event.key === "ArrowUp" && plain) { setActive(index => (index - 1 + entries.length) % entries.length); return true; }
      if ((event.key === "Enter" || event.key === "Tab") && plain) { pick(activeIndex); return true; }
      if (event.key === "Escape") { setDismissed(contextKey); return true; }
    }
    if (prompts) {
      const current = editor.current?.caret();
      const history = navigatePromptHistory({
        prompts, value: editor.current?.text() ?? value, selectionStart: current ?? null, selectionEnd: current ?? null, recall: recall.current,
        event: { key: event.key, shiftKey: event.shiftKey, ctrlKey: event.ctrlKey, altKey: event.altKey, metaKey: event.metaKey },
        caretOnEdgeRow: direction => editor.current?.caretOnEdgeRow(direction),
      });
      recall.current = history.recall;
      // A recalled prompt is cleared by Escape before Escape can interrupt a turn.
      if (history.handled) { onChange(history.value); return true; }
    }
    if (event.key === "Escape") return onEscape?.() ?? false;
    if (event.key === "Enter" && plain) { onSubmit(); return true; }
    return false;
  }

  const shell = !!commands && isShellDraft(value);
  return <div ref={wrapper} className="prompt-input">
    {open && <CompletionMenu id={menuId} label={command ? command.trigger === "$" ? "Skills" : "Slash commands" : "Mentions"} entries={entries} active={activeIndex} onActive={setActive} onPick={pick} />}
    <ComposerEditor
      ref={editor}
      value={value}
      onChange={onChange}
      ariaLabel={ariaLabel}
      placeholder={placeholder}
      maxLength={32000}
      readOnly={readOnly}
      autoFocus={autoFocus}
      chips={chips}
      onKey={onKey}
      onCaret={setCaret}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      attributes={{
        role: "combobox",
        "aria-autocomplete": "list",
        "aria-expanded": open ? "true" : "false",
        "aria-controls": open ? menuId : undefined,
        "aria-activedescendant": open ? `${menuId}-option-${activeIndex}` : undefined,
        "aria-description": shell ? SHELL_DESCRIPTION : undefined,
      }}
    />
  </div>;
});
