// `/` command and `$` skill completion for the conversation composer (#916).
//
// Only the first token of a message is a command for the agent: Claude and
// Antigravity read a slash command at the start, and Codex attaches a skill
// only when it is the first field. Completion and chips follow the same rule.

import type { ConversationCommand } from "../lib/conversation.ts";

export type CommandTrigger = "/" | "$";
export interface CompletionContext { trigger: CommandTrigger; query: string; from: number; to: number }
export interface MatchRange { field: "name" | "description"; from: number; to: number }
export interface RankedCommand { command: ConversationCommand; match?: MatchRange }
export interface ChipRange { from: number; to: number; kind: "command" | "skill"; label: string }

/** The first token, its bounds, and whether whitespace follows it. */
function firstToken(value: string) {
  const from = value.length - value.trimStart().length;
  let to = from;
  while (to < value.length && !/\s/.test(value[to])) to++;
  return { from, to, text: value.slice(from, to) };
}

/** The `/` or `$` token being typed, when the caret sits inside the first token. */
export function completionContext(value: string, caret: number): CompletionContext | undefined {
  const token = firstToken(value);
  const trigger = token.text[0];
  if (trigger !== "/" && trigger !== "$") return undefined;
  if (caret <= token.from || caret > token.to) return undefined;
  return { trigger, query: value.slice(token.from + 1, caret), from: token.from, to: token.to };
}

const isSkill = (name: string) => name.startsWith("$");
/** The name as typed after its trigger: `/review` and `$code-issue` both match on the bare word. */
const bare = (name: string) => name.replace(/^[/$]/, "");

/** Exact name, then prefix, then substring, then description; stable within a tier. */
export function rankCommands(commands: readonly ConversationCommand[] | null | undefined, trigger: CommandTrigger, query: string, limit = 50): RankedCommand[] {
  const needle = query.toLowerCase();
  const tiers: RankedCommand[][] = [[], [], [], []];
  for (const command of commands ?? []) {
    if (!command?.name || isSkill(command.name) !== (trigger === "$")) continue;
    const name = bare(command.name).toLowerCase();
    const offset = command.name.length - bare(command.name).length;
    if (!needle) { tiers[1].push({ command }); continue; }
    const at = name.indexOf(needle);
    if (name === needle) tiers[0].push({ command, match: { field: "name", from: offset, to: offset + needle.length } });
    else if (at === 0) tiers[1].push({ command, match: { field: "name", from: offset, to: offset + needle.length } });
    else if (at > 0) tiers[2].push({ command, match: { field: "name", from: offset + at, to: offset + at + needle.length } });
    else {
      const inDescription = command.description?.toLowerCase().indexOf(needle) ?? -1;
      if (inDescription >= 0) tiers[3].push({ command, match: { field: "description", from: inDescription, to: inDescription + needle.length } });
    }
  }
  return tiers.flat().slice(0, limit);
}

/** The text a menu entry inserts: the command as the agent expects it. */
export function commandText(command: ConversationCommand, trigger: CommandTrigger): string {
  return trigger === "$" ? command.name : `/${bare(command.name)}`;
}

/** The chip of a known first command or skill, once whitespace follows it. */
export function commandChip(value: string, commands: readonly ConversationCommand[] | null | undefined): ChipRange | undefined {
  const token = firstToken(value);
  if (token.to >= value.length || !/^[/$]./.test(token.text)) return undefined;
  const trigger = token.text[0] as CommandTrigger;
  const known = (commands ?? []).some(command => command?.name && isSkill(command.name) === (trigger === "$") && commandText(command, trigger) === token.text);
  return known ? { from: token.from, to: token.to, kind: trigger === "$" ? "skill" : "command", label: token.text } : undefined;
}

/** A draft starting with `!` runs as a shell command on the workstation. */
export const isShellDraft = (value: string) => value.trimStart().startsWith("!");
