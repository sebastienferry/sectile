import { useState } from "react";
import type { MessageQueue, PauseReason } from "../lib/conversation-queue";
import { Icon } from "./Icon";

const pauseLabels: Record<PauseReason, string> = { interrupt: "after an interrupt", turnError: "after an error", sendFailed: "send failed" };

/**
 * Follow-up messages waiting for their own turn. They are not part of the transcript:
 * a message joins it only once the agent has received it.
 */
export function QueuedMessages({ queue, readOnly, onRemove, onResume }: { queue: MessageQueue; readOnly?: boolean; onRemove: (id: string) => void; onResume: () => void }) {
  const [copied, setCopied] = useState("");
  if (!queue.items.length) return null;
  const failed = queue.items.find(item => item.status === "failed");
  const reason = queue.paused === "sendFailed" && failed?.error ? `send failed: ${failed.error}` : queue.paused && pauseLabels[queue.paused];
  return <section className="queued-messages" aria-label="Message queue">
    <header className="queued-messages-header">
      <strong>Queued ({queue.items.length})</strong>
      {readOnly ? <span className="queued-messages-paused">Not sent: this conversation is stopped</span>
        : queue.paused && <>
          <span className="queued-messages-paused" role="status">Queue paused · {reason}</span>
          <button type="button" className="composer-action" onClick={onResume}><Icon name="play" />Resume</button>
        </>}
    </header>
    <ol role="list" aria-label="Queued messages">
      {queue.items.map((item, index) => <li key={item.id} className={`queued-message ${item.status}`}>
        <span className="queued-message-position" aria-hidden="true">{index + 1}</span>
        <p className="queued-message-text" title={item.text}>{item.text}</p>
        <span className="queued-message-status">{item.status === "sending" ? "Sending…" : item.status === "failed" ? `Failed: ${item.error ?? "unknown error"}` : "Queued"}</span>
        <button type="button" className="icon-button" aria-label={`Copy queued message ${index + 1}`} title={copied === item.id ? "Copied" : "Copy"} onClick={() => void navigator.clipboard.writeText(item.text).then(() => { setCopied(item.id); setTimeout(() => setCopied(current => current === item.id ? "" : current), 1800); }, () => setCopied(""))}><Icon name={copied === item.id ? "check" : "copy"} /></button>
        <button type="button" className="icon-button" aria-label={`Remove queued message ${index + 1}`} title="Remove" disabled={item.status === "sending"} onClick={() => onRemove(item.id)}><Icon name="trash" /></button>
      </li>)}
    </ol>
  </section>;
}
