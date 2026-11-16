import { useEffect, useRef, useState } from "react";
import {
  getAgySettings,
  saveProviderPermissions,
  type ProviderPermissions as PermissionInput,
  type Readiness,
} from "../lib/workspace";

export function ProviderPermissions({
  provider,
  view,
  onSaved,
  inline = false,
}: {
  provider: string;
  view?: Readiness;
  onSaved: () => Promise<void>;
  inline?: boolean;
}) {
  const [choice, setChoice] = useState("");
  const [agyLoaded, setAgyLoaded] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const acting = useRef(false);
  const edited = useRef(false);

  const stored =
    provider === "claude"
      ? view?.permissions?.mode
      : provider === "codex"
      ? view?.permissions?.approvalsReviewer
      : view?.permissions?.mode;

  useEffect(() => {
    if (!edited.current && typeof stored === "string" && stored) {
      setChoice(stored);
    }
  }, [stored]);

  useEffect(() => {
    if (provider !== "agy") return;
    let active = true;
    void getAgySettings()
      .then((res) => {
        if (!active) return;
        setAgyLoaded(true);
        if (!edited.current) {
          setChoice(res.conversationMode || "accept-edits");
        }
      })
      .catch(() => {
        if (active) setAgyLoaded(false);
      });
    return () => {
      active = false;
    };
  }, [provider]);

  if (provider !== "claude" && provider !== "codex" && provider !== "agy") {
    return null;
  }

  const ready = provider === "agy" ? agyLoaded || !!view : !!view;
  const labelText = inline
    ? provider === "codex"
      ? "Approval reviewer"
      : "Conversation permission mode"
    : provider === "claude"
    ? "Default Claude mode"
    : provider === "codex"
    ? "Codex permission review"
    : "Antigravity permission mode";
  const saveLabel = inline ? "Save permissions" : "Save Sectile permissions";

  const hintText =
    provider === "claude"
      ? "What Claude may do without asking in the first message of every new conversation, including the skills and prompts you launch in it. The composer changes it from the next message."
      : provider === "codex"
      ? "Applies to Codex conversations from the next message. Automatic review may approve or deny eligible requests; the sandbox stays active."
      : "What Antigravity may do in the first message of every new conversation, including the skills and prompts you launch in it. Choose Auto-approve all tools when skills run shell commands or tools that headless mode cannot prompt for.";

  const content = (
    <div className="provider-permissions-body">
      <p className="muted">{hintText}</p>
      <div className="provider-permissions-row">
        <label>
          <span>{labelText}</span>
          <select
            disabled={pending || !ready}
            value={choice}
            onChange={(e) => {
              edited.current = true;
              setChoice(e.target.value);
              setNotice("");
            }}
          >
            <option value="">Choose…</option>
            {provider === "claude" ? (
              <>
                <option value="default">Ask before edits</option>
                <option value="acceptEdits">Accept edits</option>
                <option value="plan">Plan mode</option>
                <option value="auto">Auto mode</option>
              </>
            ) : provider === "codex" ? (
              <>
                <option value="user">Ask me</option>
                <option value="auto_review">Approve on my behalf</option>
              </>
            ) : (
              <>
                <option value="accept-edits">Accept edits</option>
                <option value="default">Ask / follow rules</option>
                <option value="auto">Auto-approve all tools</option>
                <option value="plan">Plan mode</option>
              </>
            )}
          </select>
        </label>
        <button
          disabled={pending || !choice || !ready}
          onClick={() => {
            if (acting.current) return;
            acting.current = true;
            setPending(true);
            setError("");
            setNotice("");
            const input =
              provider === "claude"
                ? { provider, mode: choice }
                : provider === "codex"
                ? { provider, approvalsReviewer: choice }
                : { provider: "agy", mode: choice };
            void saveProviderPermissions(input as PermissionInput)
              .then(async () => {
                await onSaved();
                edited.current = false;
                setNotice("Preference saved for future runs of this engine.");
              })
              .catch(() =>
                setError("Save not confirmed. Check services and reload values."),
              )
              .finally(() => {
                acting.current = false;
                setPending(false);
              });
          }}
        >
          {saveLabel}
        </button>
      </div>
      {notice && <p role="status">{notice}</p>}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
    </div>
  );

  if (inline) {
    return (
      <section className="provider-permissions-inline" aria-label={labelText}>
        <h4>{labelText}</h4>
        {content}
      </section>
    );
  }

  return (
    <details className="provider-permissions">
      <summary>Conversation permissions</summary>
      {content}
    </details>
  );
}
