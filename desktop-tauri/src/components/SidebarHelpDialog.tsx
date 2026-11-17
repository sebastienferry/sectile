import { useEffect, useId, useRef, useState } from "react";

const skillEntries = [
  ["working", "Working", "Work is queued, preparing or running. No answer is implied."],
  ["waiting", "Needs your answer", "Respond to the question or approval in the console, or choose the required repository."],
  ["done", "Done", "The skill outcome is recorded; a workflow skill has reached its expected stage. The whole ticket is not necessarily finished."],
  ["verify", "Needs verification", "Completion is unconfirmed, the expected stage was not reached, or the session ended without a result. Check the result and ticket."],
  ["failed", "Could not finish", "Inspect the console and recorded reason before deciding whether to relaunch."],
  ["stopping", "Stopping", "Stop requested; waiting for the session to exit."],
  ["stopped", "Stopped", "Execution stopped. Completion is not implied, and the recorded reason may be unavailable."],
];
const conversationEntries = [
  ["live", "Working conversation or active execution", "The conversation is busy or a fallback execution is active."],
  ["session", "Active session", "The session may be ready between turns. A pulse alone does not prove work is running."],
  ["needs-input", "Needs input", "A question, approval or other input needs your response."],
  ["idle", "Idle", "The conversation or execution is inactive. This dot alone does not certify success or explain why work stopped."],
];

/** Local help owns no conversation or execution state. Native modality contains focus. */
export default function SidebarHelp() {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    if (!open) return;
    const element = dialog.current!;
    element.showModal();
    element.querySelector<HTMLButtonElement>("button")?.focus();
    return () => {
      element.close();
      if (trigger.current?.isConnected) trigger.current.focus();
      else document.getElementById("settings")?.focus();
    };
  }, [open]);

  return <>
    <button ref={trigger} type="button" className="sidebar-help-button" aria-label="Sidebar status help" aria-haspopup="dialog" onClick={() => setOpen(true)} title="Sidebar status help">?</button>
    {open && <dialog ref={dialog} className="sidebar-help-dialog" aria-labelledby={titleId} onCancel={event => { event.preventDefault(); event.stopPropagation(); setOpen(false); }}>
      <header className="sidebar-help-header">
        <h2 id={titleId}>Sidebar status help</h2>
        <button type="button" onClick={() => setOpen(false)}>Close</button>
      </header>
      <div className="sidebar-help-body" tabIndex={0} role="region" aria-label="Status legend">
        <p>Row labels and tooltips identify the skill being performed and give details. The symbol describes its current state or outcome. This legend explains symbols; it does not diagnose or change a run.</p>
        <h3>Ticket skills</h3>
        <ul>{skillEntries.map(([kind, label, description]) => <li key={kind}>
          <span aria-hidden="true" className={`skill-status-mark skill-${kind}`} />
          <div><strong>{label}</strong><p>{description}</p></div>
        </li>)}</ul>
        <h3>Conversations and fallback executions</h3>
        <ul>{conversationEntries.map(([kind, label, description]) => <li key={kind}>
          <span aria-hidden="true" className={`conversation-indicator ${kind}`} />
          <div><strong>{label}</strong><p>{description}</p></div>
        </li>)}</ul>
      </div>
    </dialog>}
  </>;
}
