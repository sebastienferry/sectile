// `@` mentions in a composer (#917). A mention the composer inserted is the
// token `@[label](sectile:<kind>/<id>)`, kept in the plain-text draft and
// resolved by the agent on send; a folder is inserted as `@path/` text.

export type MentionTokenKind = "file" | "conversation" | "ticket";
export interface MentionContext { query: string; from: number; to: number }
export interface MentionPart { text?: string; mention?: { kind: MentionTokenKind; label: string; id: string } }
export interface MentionChip { from: number; to: number; kind: MentionTokenKind; label: string }

const TOKEN = /@\[([^\]\n]*)\]\(sectile:(file|conversation|ticket)\/([^)\s]+)\)/g;

/** The token a picked file, conversation or ticket inserts. */
export function mentionToken(kind: MentionTokenKind, id: string, label: string): string {
  const safe = label.replace(/[[\]\n]+/g, " ").trim() || kind;
  // encodeURIComponent leaves ( ) ! ' * as they are; a ")" would end the token early.
  const encoded = encodeURIComponent(id).replace(/[()!'*]/g, char => `%${char.charCodeAt(0).toString(16).toUpperCase()}`);
  return `@[${safe}](sectile:${kind}/${encoded})`;
}

/** The `@` word being typed at the caret: `@` at the start or after whitespace. */
export function mentionContext(value: string, caret: number): MentionContext | undefined {
  let at = caret - 1;
  while (at >= 0 && !/\s/.test(value[at]) && value[at] !== "@") at--;
  if (at < 0 || value[at] !== "@" || (at > 0 && !/\s/.test(value[at - 1]))) return undefined;
  const query = value.slice(at + 1, caret);
  // An inserted token is not a word being typed.
  if (query.startsWith("[")) return undefined;
  let to = caret;
  while (to < value.length && !/\s/.test(value[to])) to++;
  return { query, from: at, to };
}

function decode(id: string) {
  try { return decodeURIComponent(id); } catch { return id; }
}

/** The inserted tokens of a text, as chips. */
export function mentionChips(text: string): MentionChip[] {
  return Array.from(text.matchAll(TOKEN), match => ({ from: match.index!, to: match.index! + match[0].length, kind: match[2] as MentionTokenKind, label: match[1] }));
}

/** A text cut into plain runs and mentions, for the transcript. */
export function splitMentions(text: string): MentionPart[] {
  const parts: MentionPart[] = [];
  let last = 0;
  for (const match of text.matchAll(TOKEN)) {
    if (match.index! > last) parts.push({ text: text.slice(last, match.index) });
    parts.push({ mention: { kind: match[2] as MentionTokenKind, label: match[1], id: decode(match[3]) } });
    last = match.index! + match[0].length;
  }
  if (last < text.length) parts.push({ text: text.slice(last) });
  return parts;
}

export interface MentionConversation { id: string; title: string; archived?: boolean; createdAt?: string }
export interface MentionTicket { id: string; key?: string; title: string }
export interface Ranked<T> { item: T; field: "label" | "detail"; from: number; to: number }

const lower = (value: string | undefined) => (value ?? "").toLowerCase();

/** Conversations whose title holds the query, newest first. */
export function rankConversations(conversations: readonly MentionConversation[], query: string, limit: number): Ranked<MentionConversation>[] {
  const needle = lower(query);
  return conversations
    .map(item => ({ item, at: lower(item.title).indexOf(needle) }))
    .filter(entry => entry.at >= 0)
    .sort((a, b) => (Date.parse(b.item.createdAt ?? "") || 0) - (Date.parse(a.item.createdAt ?? "") || 0))
    .slice(0, limit)
    .map(({ item, at }) => ({ item, field: "label", from: at, to: at + needle.length }));
}

/**
 * Tickets matching on key or title: a key prefix first, then any other match;
 * the board's order (the client holds no update time) within each.
 */
export function rankTickets(tickets: readonly MentionTicket[], query: string, limit: number): Ranked<MentionTicket>[] {
  const needle = lower(query).replace(/^#/, "");
  const keyOf = (ticket: MentionTicket) => lower(ticket.key).replace(/^#/, "");
  const first: Ranked<MentionTicket>[] = [];
  const rest: Ranked<MentionTicket>[] = [];
  for (const item of tickets) {
    const key = keyOf(item);
    const offset = item.key?.startsWith("#") ? 1 : 0;
    if (key.startsWith(needle)) first.push({ item, field: "label", from: offset, to: offset + needle.length });
    else if (key.includes(needle)) rest.push({ item, field: "label", from: offset + key.indexOf(needle), to: offset + key.indexOf(needle) + needle.length });
    else {
      const at = lower(item.title).indexOf(needle);
      if (at >= 0) rest.push({ item, field: "detail", from: at, to: at + needle.length });
    }
  }
  return [...first, ...rest].slice(0, limit);
}
