import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Icon, type IconName } from "./Icon";

/** One pill of the tray. Folders today; a per-turn context item can join later (#914). */
export interface ContextTrayItem {
  key: string;
  label: string;
  title: string;
  icon: IconName;
  onClick?: () => void;
}

/** How many pills sit below the first one, i.e. past the first row. */
function overflowCount(row: HTMLElement): number {
  const pills = Array.from(row.children) as HTMLElement[];
  if (!pills.length) return 0;
  const firstTop = pills[0].offsetTop;
  return pills.filter(pill => pill.offsetTop > firstTop).length;
}

/**
 * The context the next message is sent with, as compact pills on one row at the top of
 * the composer box. Pills past the first row hide behind a `+N more` toggle.
 */
export function ContextTray({ items, label = "Message context" }: { items: ContextTrayItem[]; label?: string }) {
  const row = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [hidden, setHidden] = useState(0);
  const signature = items.map(item => item.key).join("\n");
  // A different set of items starts collapsed again.
  useEffect(() => { setExpanded(false); }, [signature]);
  useLayoutEffect(() => {
    const element = row.current;
    if (!element) return;
    const measure = () => setHidden(overflowCount(element));
    measure();
    const Observer = element.ownerDocument.defaultView?.ResizeObserver;
    if (!Observer) return;
    const observer = new Observer(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [signature, expanded]);
  if (!items.length) return null;
  const firstHidden = items.length - hidden;
  return <div className={`context-tray${expanded ? " expanded" : ""}`} role="group" aria-label={label}>
    <div className="context-tray-row" ref={row}>
      {items.map((item, index) => {
        const clipped = !expanded && index >= firstHidden;
        return <button key={item.key} type="button" className="context-tray-pill" title={item.title} aria-label={item.title} aria-hidden={clipped || undefined} tabIndex={clipped ? -1 : undefined} onClick={item.onClick}>
          <Icon name={item.icon} /><span>{item.label}</span>
        </button>;
      })}
    </div>
    {(hidden > 0 || expanded) && <button type="button" className="context-tray-toggle" aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>
      {expanded ? "Show less" : `+${hidden} more`}
    </button>}
  </div>;
}
