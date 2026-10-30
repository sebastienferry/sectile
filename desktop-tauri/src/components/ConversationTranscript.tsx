import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { decodeEvents, type ConversationEvent, type ConversationSnapshot } from "../lib/conversation";
import { Markdown } from "./Markdown";

const approvalLabels: Record<string, string> = {
  allow: "Demande autorisée une fois.",
  deny: "Demande refusée.",
  always: "Autorisation mémorisée.",
  answer: "Réponse envoyée.",
};
function eventText(event: ConversationEvent): string {
  if (event.kind === "notice" && event.text === "Prêt pour votre prochain message") return "Réponse terminée";
  if (event.kind === "approval") return approvalLabels[event.text] ?? event.text;
  return event.text;
}

interface ToolEntry { key: number; call?: ConversationEvent; result?: ConversationEvent }
type Row = { key: number; event: ConversationEvent } | { key: number; tools: ToolEntry[] };
export function transcriptRows(events: ConversationEvent[]): Row[] {
  const rows: Row[] = [];
  const calls = new Map<string, ToolEntry[]>();
  for (const [index, event] of events.entries()) {
    if (event.kind === "tool") {
      const entry: ToolEntry = { key: index, call: event };
      const previous = rows[rows.length - 1];
      if (previous && "tools" in previous) previous.tools.push(entry);
      else rows.push({ key: index, tools: [entry] });
      if (event.toolId) calls.set(event.toolId, [...(calls.get(event.toolId) ?? []), entry]);
    } else if (event.kind === "tool_result") {
      // Match by identity, including out-of-order/late results across assistant messages.
      const entry = event.toolId ? calls.get(event.toolId)?.find(item => !item.result) : undefined;
      if (entry) entry.result = event;
      else rows.push({ key: index, tools: [{ key: index, result: event }] });
    } else rows.push({ key: index, event });
  }
  return rows;
}
function Tools({ tools, busy }: { tools: ToolEntry[]; busy: boolean }) {
  const complete = tools.filter(tool => tool.result).length;
  const errors = tools.filter(tool => tool.result?.error).length;
  return <details className="tool-group"><summary>{tools.length === 1 ? `Outil · ${tools[0].call?.tool || tools[0].call?.text || "Résultat"}` : `${tools.length} appels d’outils`}<span>{errors ? `${errors} en erreur` : complete === tools.length ? "Terminé" : busy ? "En cours…" : "Résultat non reçu"}</span></summary>
    {tools.map(({ key, call, result }) => <section className="tool-entry" key={key}><strong>{call?.tool || call?.text || "Résultat d’outil"}</strong>
      {call && <pre>{call.detail || (call.input ? JSON.stringify(call.input, null, 2) : call.text)}</pre>}
      {result && <div className={result.error ? "tool-result tool-failed" : "tool-result"}><small>{result.error ? "Erreur" : "Résultat"}</small><pre>{result.text || "Aucune sortie"}</pre></div>}
      {!result && <p className="muted">{busy ? "En attente du résultat…" : "Résultat non reçu (interruption ou historique incomplet)"}</p>}
    </section>)}
  </details>;
}
export function ConversationTranscript({ snapshot, followRequest = 0 }: { snapshot: ConversationSnapshot; followRequest?: number }) {
  const viewport = useRef<HTMLDivElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const following = useRef(true);
  const lastRequest = useRef(followRequest);
  const [paused, setPaused] = useState(false);
  const rows = useMemo(() => transcriptRows(decodeEvents(snapshot)), [snapshot.events]);
  function toBottom() {
    const element = viewport.current;
    if (element) element.scrollTop = element.scrollHeight;
    following.current = true; setPaused(false);
  }
  useLayoutEffect(() => {
    if (lastRequest.current !== followRequest) { lastRequest.current = followRequest; following.current = true; }
    if (following.current) toBottom();
  }, [rows, snapshot.partial, followRequest]);
  useLayoutEffect(() => {
    if (!content.current || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => { if (following.current) toBottom(); });
    observer.observe(content.current);
    return () => observer.disconnect();
  }, []);
  return <div className="transcript-shell"><div ref={viewport} className="prompt-transcript" aria-label="Historique de conversation" onScroll={() => {
    const element = viewport.current!;
    following.current = element.scrollHeight - element.clientHeight - element.scrollTop < 48;
    setPaused(!following.current);
  }}><div ref={content}>
    {rows.map(row => "tools" in row ? <Tools key={row.key} tools={row.tools} busy={snapshot.busy} /> : <article className={`prompt-event prompt-${row.event.kind}`} key={row.key}>
      <strong>{row.event.interrupted ? "Assistant · réponse interrompue" : ({ user: "Vous", assistant: "Assistant", error: "Erreur", thinking: "Réflexion", approval: "Autorisation" } as Record<string, string>)[row.event.kind] ?? "Activité"}</strong>
      {row.event.kind === "assistant" ? <Markdown text={row.event.text} /> : <p>{eventText(row.event)}</p>}
      {row.event.detail && <details><summary>Détails</summary><pre>{row.event.detail}</pre></details>}
    </article>)}
    {snapshot.partial && <article className="prompt-event"><strong>Assistant</strong><Markdown text={snapshot.partial} /></article>}
  </div></div>{paused && <button className="back-to-bottom" onClick={toBottom}>Aller en bas ↓</button>}</div>;
}
