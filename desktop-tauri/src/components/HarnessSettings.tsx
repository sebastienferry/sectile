import { useEffect, useRef, useState } from "react";
import {
  getWorkstationHarness,
  saveWorkstationHarness,
  type HarnessPermissionMode,
} from "../lib/workspace";

const COMMON_WRITABLE_PRESETS = [
  "~/.cargo",
  "~/go/pkg/mod",
  "~/.npm",
  "~/.pinchtab",
];

function effectiveMode(res?: {
  permissionMode?: HarnessPermissionMode;
  autoApprove?: boolean;
}): HarnessPermissionMode {
  if (res?.permissionMode) return res.permissionMode;
  if (res?.autoApprove) return "auto";
  return "";
}

export function HarnessSettings() {
  const [loaded, setLoaded] = useState(false);
  const [permissionMode, setPermissionMode] =
    useState<HarnessPermissionMode>("");
  const [networkAccess, setNetworkAccess] = useState(false);
  const [writablePaths, setWritablePaths] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const saving = useRef(false);

  useEffect(() => {
    let active = true;
    void getWorkstationHarness()
      .then((res) => {
        if (!active) return;
        setPermissionMode(effectiveMode(res));
        setNetworkAccess(Boolean(res?.networkAccess));
        setWritablePaths(res?.writablePaths ?? []);
        setLoaded(true);
      })
      .catch(() => {
        if (active) setLoaded(false);
      });
    return () => {
      active = false;
    };
  }, []);

  function addPath(raw: string) {
    const entry = raw.trim();
    if (!entry || /[\r\n]/.test(entry)) return;
    if (!writablePaths.includes(entry)) {
      setWritablePaths((prev) => [...prev, entry]);
      setNotice("");
    }
    setDraft("");
  }

  async function save() {
    if (saving.current) return;
    saving.current = true;
    setPending(true);
    setError("");
    setNotice("");
    try {
      const saved = await saveWorkstationHarness({
        permissionMode,
        autoApprove: permissionMode === "auto",
        networkAccess,
        writablePaths,
      });
      setPermissionMode(effectiveMode(saved));
      setNetworkAccess(Boolean(saved?.networkAccess));
      setWritablePaths(saved?.writablePaths ?? []);
      setNotice(
        "Global harness saved for future Claude, Codex, and Antigravity turns.",
      );
    } catch {
      setError("Save not confirmed. Check local services and try again.");
    } finally {
      saving.current = false;
      setPending(false);
    }
  }

  return (
    <section
      className="claude-sandbox-settings harness-settings"
      aria-label="Global harness"
    >
      <div className="sandbox-section-header">
        <h4>Global harness (all AI engines)</h4>
        <p className="muted">
          Single permission and sandbox policy translated transparently into
          Claude Code, Codex, and Antigravity launches.
        </p>
      </div>

      <div className="provider-permissions-row">
        <label>
          <span>Global permission mode</span>
          <select
            disabled={!loaded || pending}
            value={permissionMode}
            onChange={(e) => {
              setPermissionMode(e.target.value as HarnessPermissionMode);
              setNotice("");
            }}
          >
            <option value="">Engine default</option>
            <option value="auto">
              Auto-approve (Claude Auto, Codex Auto-review, Antigravity Auto)
            </option>
            <option value="accept-edits">Accept edits</option>
            <option value="default">Ask before edits</option>
            <option value="plan">Plan mode (read-only)</option>
          </select>
        </label>
      </div>

      <label className="sandbox-checkbox">
        <input
          type="checkbox"
          disabled={!loaded || pending}
          checked={networkAccess}
          onChange={(e) => {
            setNetworkAccess(e.target.checked);
            setNotice("");
          }}
        />
        <span>
          <strong>Allow network &amp; local test servers</strong> — permit
          loopback binding (<code>127.0.0.1</code> for Rust/Go/Node tests) and{" "}
          <code>git fetch</code> / <code>push</code> / <code>gh</code> inside
          the sandbox.
        </span>
      </label>

      <div className="sandbox-list">
        <span className="sandbox-list-label">Shared writable directories</span>
        <ul className="sandbox-entries" aria-label="Shared writable directories">
          {writablePaths.length === 0 && (
            <li className="sandbox-empty">None</li>
          )}
          {writablePaths.map((entry) => (
            <li className="sandbox-entry" key={entry}>
              <code>{entry}</code>
              <button
                type="button"
                className="sandbox-chip-button"
                disabled={!loaded || pending}
                aria-label={`Remove ${entry} from Shared writable directories`}
                onClick={() => {
                  setWritablePaths((prev) =>
                    prev.filter((item) => item !== entry),
                  );
                  setNotice("");
                }}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
        <div className="sandbox-add-row">
          <input
            type="text"
            value={draft}
            disabled={!loaded || pending}
            placeholder="e.g. ~/.cargo or ~/go/pkg/mod"
            aria-label="New shared writable directory"
            onChange={(e) => {
              setDraft(e.target.value);
              setNotice("");
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                addPath(draft);
              }
            }}
          />
          <button
            type="button"
            disabled={!loaded || pending || !draft.trim()}
            onClick={() => addPath(draft)}
          >
            Add
          </button>
        </div>
        <div className="harness-presets">
          {COMMON_WRITABLE_PRESETS.filter(
            (preset) => !writablePaths.includes(preset),
          ).map((preset) => (
            <button
              key={preset}
              type="button"
              className="harness-preset-button"
              disabled={!loaded || pending}
              onClick={() => addPath(preset)}
            >
              + {preset}
            </button>
          ))}
        </div>
      </div>

      <div className="sandbox-save-row">
        <button
          type="button"
          className="primary"
          disabled={!loaded || pending}
          onClick={() => void save()}
        >
          {pending ? "Saving…" : "Save global harness"}
        </button>
        {notice && <p role="status">{notice}</p>}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
      </div>
    </section>
  );
}
