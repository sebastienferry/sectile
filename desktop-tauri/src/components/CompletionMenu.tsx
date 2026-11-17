import { useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Icon, type IconName } from "./Icon";

/** One entry of a composer completion menu: commands and skills (#916), mentions (#917). */
export interface CompletionEntry {
  id: string;
  label: string;
  /** The part of the label or the detail that matched the query. */
  match?: { field: "label" | "detail"; from: number; to: number };
  detail?: string;
  hint?: string;
  icon?: IconName;
}

function highlighted(text: string, range?: { from: number; to: number }): ReactNode {
  if (!range || range.from >= range.to) return text;
  return <>{text.slice(0, range.from)}<mark>{text.slice(range.from, range.to)}</mark>{text.slice(range.to)}</>;
}

/**
 * A floating listbox anchored to the composer box: above it, or below when the
 * viewport leaves too little room above. The editor keeps the focus; it names
 * the active option through aria-activedescendant.
 */
export function CompletionMenu({ id, label, entries, active, onActive, onPick }: {
  id: string;
  label: string;
  entries: CompletionEntry[];
  active: number;
  onActive: (index: number) => void;
  onPick: (index: number) => void;
}) {
  const list = useRef<HTMLUListElement>(null);
  const [below, setBelow] = useState(false);
  useLayoutEffect(() => {
    const element = list.current;
    const anchor = (element?.closest(".composer-box") ?? element?.parentElement)?.getBoundingClientRect();
    if (!element || !anchor) return;
    setBelow(anchor.top < element.offsetHeight + 8);
  }, [entries.length]);
  useLayoutEffect(() => {
    list.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView?.({ block: "nearest" });
  }, [active]);
  return <ul ref={list} id={id} role="listbox" aria-label={label} className={`completion-menu${below ? " below" : ""}`}>
    {entries.map((entry, index) => <li
      key={entry.id}
      id={`${id}-option-${index}`}
      data-index={index}
      role="option"
      aria-selected={index === active}
      className={index === active ? "active" : undefined}
      // Keep the editor focused: a mousedown on the menu would blur it first.
      onMouseDown={event => { event.preventDefault(); onPick(index); }}
      onMouseEnter={() => onActive(index)}
    >
      {entry.icon && <Icon name={entry.icon} size={14} />}
      <span className="completion-menu-label">{highlighted(entry.label, entry.match?.field === "label" ? entry.match : undefined)}</span>
      {entry.hint && <span className="completion-menu-hint">{entry.hint}</span>}
      {entry.detail && <span className="completion-menu-detail">{highlighted(entry.detail, entry.match?.field === "detail" ? entry.match : undefined)}</span>}
    </li>)}
  </ul>;
}
