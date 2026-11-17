import { useEffect, useState } from "react";
import type { MessageQueue, PauseReason, QueuedMessage } from "../lib/conversation-queue";
import { Icon } from "./Icon";

const pauseLabels: Record<PauseReason, string> = { interrupt: "after an interrupt", turnError: "after an error", sendFailed: "send failed" };

function pauseReason(queue: MessageQueue) {
  const failed = queue.items.find(item => item.status === "failed");
  return queue.paused === "sendFailed" && failed?.error ? `send failed: ${failed.error}` : queue.paused && pauseLabels[queue.paused];
}
const statusText = (item: QueuedMessage) => item.status === "sending" ? "Sending…" : item.status === "failed" ? `Failed: ${item.error ?? "unknown error"}` : "Queued";

/** Copy feedback shared by every copy button of a queue view. */
function useCopied() {
  const [copied, setCopied] = useState("");
  const copy = (item: QueuedMessage) => void navigator.clipboard.writeText(item.text).then(() => {
    setCopied(item.id);
    setTimeout(() => setCopied(current => current === item.id ? "" : current), 1800);
  }, () => setCopied(""));
  return { copied, copy };
}

interface ItemActions {
  copied: string;
  onCopy: (item: QueuedMessage) => void;
  onEdit?: (id: string) => void;
  onRemove: (id: string) => void;
}
function QueuedItemButtons({ item, position, copied, onCopy, onEdit, onRemove }: ItemActions & { item: QueuedMessage; position: number }) {
  return <>
    <button type="button" className="icon-button" aria-label={`Copy queued message ${position}`} title={copied === item.id ? "Copied" : "Copy"} onClick={() => onCopy(item)}><Icon name={copied === item.id ? "check" : "copy"} /></button>
    {onEdit && <button type="button" className="icon-button" aria-label={`Edit queued message ${position}`} title="Edit: move back into the composer" disabled={item.status === "sending"} onClick={() => onEdit(item.id)}><Icon name="edit" /></button>}
    <button type="button" className="icon-button" aria-label={`Remove queued message ${position}`} title="Remove" disabled={item.status === "sending"} onClick={() => onRemove(item.id)}><Icon name="trash" /></button>
  </>;
}
function QueuedList({ queue, ...actions }: ItemActions & { queue: MessageQueue }) {
  return <ol role="list" aria-label="Queued messages">
    {queue.items.map((item, index) => <li key={item.id} className={`queued-message ${item.status}`}>
      <span className="queued-message-position" aria-hidden="true">{index + 1}</span>
      <p className="queued-message-text" title={item.text}>{item.text}</p>
      <span className="queued-message-status">{statusText(item)}</span>
      <QueuedItemButtons item={item} position={index + 1} {...actions} />
    </li>)}
  </ol>;
}

/**
 * The queue of a stopped conversation: its messages will not be sent, and there is no
 * composer to edit them into, so they can only be copied or removed.
 * Follow-up messages are not part of the transcript: a message joins it only once the
 * agent has received it.
 */
export function QueuedMessages({ queue, onRemove }: { queue: MessageQueue; onRemove: (id: string) => void }) {
  const { copied, copy } = useCopied();
  if (!queue.items.length) return null;
  return <section className="queued-messages" aria-label="Message queue">
    <header className="queued-messages-header">
      <strong>Queued ({queue.items.length})</strong>
      <span className="queued-messages-paused">Not sent: this conversation is stopped</span>
    </header>
    <QueuedList queue={queue} copied={copied} onCopy={copy} onRemove={onRemove} />
  </section>;
}

/**
 * The queue of a writable conversation, at the top of the composer box (#914): the next
 * message to be sent on one line, and the whole list on demand when several wait.
 */
export function QueueStrip({ queue, onRemove, onResume, onEdit }: { queue: MessageQueue; onRemove: (id: string) => void; onResume: () => void; onEdit: (id: string) => void }) {
  const { copied, copy } = useCopied();
  const [expanded, setExpanded] = useState(false);
  const several = queue.items.length > 1;
  useEffect(() => { if (!several) setExpanded(false); }, [several]);
  const head = queue.items[0];
  if (!head) return null;
  const reason = pauseReason(queue);
  const actions = { copied, onCopy: copy, onEdit, onRemove };
  return <section className={`queue-strip${expanded ? " expanded" : ""}`} aria-label="Message queue">
    <div className="queue-strip-header">
      <span className="queue-strip-tag">Queued{several && <span className="queue-strip-count" aria-label={`${queue.items.length} messages`}> · {queue.items.length}</span>}</span>
      {expanded
        ? <span className="queue-strip-spacer" />
        : <>
          <p className="queue-strip-preview" title={head.text}>{head.text}</p>
          {head.status !== "queued" && <span className={`queue-strip-status ${head.status}`}>{statusText(head)}</span>}
        </>}
      {queue.paused && <>
        <span className="queued-messages-paused" role="status">Queue paused · {reason}</span>
        <button type="button" className="composer-action" onClick={onResume}><Icon name="play" />Resume</button>
      </>}
      {!expanded && <QueuedItemButtons item={head} position={1} {...actions} />}
      {several && <button type="button" className="icon-button queue-strip-toggle" aria-expanded={expanded} aria-label={expanded ? "Collapse queued messages" : "Expand queued messages"} title={expanded ? "Collapse" : "Show all queued messages"} onClick={() => setExpanded(value => !value)}><Icon name="chevron" /></button>}
    </div>
    {expanded && <QueuedList queue={queue} {...actions} />}
  </section>;
}
