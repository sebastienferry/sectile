import { useEffect, useRef, useState } from "react";
import {
  getWorkstationSandbox,
  saveWorkstationSandbox,
  type ClaudeSandbox,
} from "../lib/workspace";

type TriState = "inherited" | "on" | "off";

function toTriState(value?: boolean | null): TriState {
  return value === true ? "on" : value === false ? "off" : "inherited";
}

function fromTriState(state: TriState): boolean | null {
  return state === "on" ? true : state === "off" ? false : null;
}

function ListEditor({
  label,
  placeholder,
  entries,
  disabled,
  onChange,
}: {
  label: string;
  placeholder: string;
  entries: string[];
  disabled: boolean;
  onChange: (next: string[]) => void;
}) {
  const [draft, setDraft] = useState("");
  const [status, setStatus] = useState("");

  function add() {
    const entry = draft.trim();
    if (!entry) {
      setStatus("Enter a value before adding it.");
      return;
    }
    if (/[\r\n]/.test(entry)) {
      setStatus("An entry is a single line.");
      return;
    }
    if (entries.includes(entry)) {
      setStatus(`${entry} is already in the list.`);
      return;
    }
    onChange([...entries, entry]);
    setDraft("");
    setStatus("");
  }

  return (
    <div className="sandbox-list">
      <span className="sandbox-list-label">{label}</span>
      <ul className="sandbox-entries" aria-label={label}>
        {entries.length === 0 && <li className="sandbox-empty">None</li>}
        {entries.map((entry) => (
          <li className="sandbox-entry" key={entry}>
            <code>{entry}</code>
            <button
              type="button"
              className="sandbox-chip-button"
              disabled={disabled}
              aria-label={`Remove ${entry} from ${label}`}
              onClick={() => onChange(entries.filter((item) => item !== entry))}
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
          disabled={disabled}
          placeholder={placeholder}
          aria-label={`New entry for ${label}`}
          onChange={(e) => {
            setDraft(e.target.value);
            setStatus("");
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
        />
        <button type="button" disabled={disabled} onClick={add}>
          Add
        </button>
      </div>
      {status && (
        <p className="muted" role="status">
          {status}
        </p>
      )}
    </div>
  );
}

export function ClaudeSandboxSettings({
  projects,
}: {
  projects: { id: string; name: string }[];
}) {
  const [loaded, setLoaded] = useState(false);
  const [base, setBase] = useState<ClaudeSandbox>({});
  const [sandboxState, setSandboxState] = useState<TriState>("inherited");
  const [sandboxedCmds, setSandboxedCmds] = useState<TriState>("inherited");
  const [unsandboxedRetries, setUnsandboxedRetries] =
    useState<TriState>("inherited");
  const [onlySelected, setOnlySelected] = useState(false);
  const [selectedProjects, setSelectedProjects] = useState<string[]>([]);
  const [allowedDomains, setAllowedDomains] = useState<string[]>([]);
  const [excludedCommands, setExcludedCommands] = useState<string[]>([]);
  const [allowRules, setAllowRules] = useState<string[]>([]);
  const [denyRules, setDenyRules] = useState<string[]>([]);
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const saving = useRef(false);

  function applyPayload(sandbox: ClaudeSandbox = {}, coveredProjects: string[] = []) {
    setBase(sandbox);
    setSandboxState(toTriState(sandbox.enabled));
    setSandboxedCmds(toTriState(sandbox.autoAllowBashIfSandboxed));
    setUnsandboxedRetries(toTriState(sandbox.allowUnsandboxedCommands));
    setOnlySelected(coveredProjects.length > 0);
    setSelectedProjects(coveredProjects);
    setAllowedDomains(sandbox.allowedDomains ?? []);
    setExcludedCommands(sandbox.excludedCommands ?? []);
    setAllowRules(sandbox.allow ?? []);
    setDenyRules(sandbox.deny ?? []);
  }

  useEffect(() => {
    let active = true;
    void getWorkstationSandbox()
      .then((payload) => {
        if (!active) return;
        applyPayload(payload.claudeSandbox ?? {}, payload.projects ?? []);
        setLoaded(true);
      })
      .catch(() => {
        if (active) setError("Claude sandbox settings unavailable.");
      });
    return () => {
      active = false;
    };
  }, []);

  const autonomyActive =
    sandboxState === "on" &&
    sandboxedCmds === "on" &&
    unsandboxedRetries === "off";

  async function save() {
    if (saving.current) return;
    saving.current = true;
    setPending(true);
    setError("");
    setNotice("");
    const claudeSandbox: ClaudeSandbox = {
      enabled: fromTriState(sandboxState),
      autoAllowBashIfSandboxed: fromTriState(sandboxedCmds),
      allowUnsandboxedCommands: fromTriState(unsandboxedRetries),
      allowedDomains,
      excludedCommands,
      allow: allowRules,
      deny: denyRules,
    };
    const projectIds = onlySelected
      ? projects.map((p) => p.id).filter((id) => selectedProjects.includes(id))
      : [];
    try {
      const updated = await saveWorkstationSandbox({
        claudeSandbox,
        claudeSandboxBase: base,
        projects: projectIds,
      });
      applyPayload(updated.claudeSandbox ?? {}, updated.projects ?? []);
      setNotice("Claude sandbox settings saved for future runs.");
    } catch {
      setError("Save not confirmed. Check local services and try again.");
    } finally {
      saving.current = false;
      setPending(false);
    }
  }

  return (
    <section className="claude-sandbox-settings" aria-label="Sandbox">
      <div className="sandbox-section-header">
        <h4>Sandbox</h4>
        <p className="muted">
          OS-level security boundaries, network access, and tool permissions for
          Claude Code.
        </p>
      </div>

      <div className="sandbox-row">
        <div>
          <strong>Claude Code sandbox</strong>
          <p className="muted">
            {sandboxState === "on"
              ? "Enabled · Shell commands run inside Claude's sandbox."
              : sandboxState === "off"
              ? "Off · Claude's sandbox is disabled by Sectile."
              : "Inherited · Claude Code's own settings decide."}
          </p>
        </div>
        <div
          className="sandbox-segmented"
          role="group"
          aria-label="Claude Code sandbox"
        >
          {(
            [
              ["inherited", "Inherited"],
              ["on", "On"],
              ["off", "Off"],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              disabled={!loaded || pending}
              className={sandboxState === value ? "active" : ""}
              aria-pressed={sandboxState === value}
              onClick={() => {
                setSandboxState(value);
                setNotice("");
              }}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <div className="sandbox-block">
        <strong>Applies to</strong>
        <p className="muted">
          {!onlySelected
            ? "Every project, including the ones you add later."
            : selectedProjects.length > 0
            ? "Applies only to the checked projects."
            : "Check at least one project: with none checked, settings still apply to every project."}
        </p>
        <div
          className="sandbox-whitelist-modes"
          role="radiogroup"
          aria-label="Projects the Claude settings cover"
        >
          <label className="sandbox-radio">
            <input
              type="radio"
              name="claude-sandbox-coverage"
              disabled={!loaded || pending}
              checked={!onlySelected}
              onChange={() => {
                setOnlySelected(false);
                setNotice("");
              }}
            />
            All projects
          </label>
          <label className="sandbox-radio">
            <input
              type="radio"
              name="claude-sandbox-coverage"
              disabled={!loaded || pending}
              checked={onlySelected}
              onChange={() => {
                setOnlySelected(true);
                setNotice("");
              }}
            />
            Only these projects
          </label>
        </div>
        {onlySelected && (
          <div
            className="sandbox-whitelist-projects"
            role="group"
            aria-label="Projects the Claude settings apply to"
          >
            {projects.length === 0 ? (
              <p className="muted">No project is added to this workstation yet.</p>
            ) : (
              projects.map((project) => (
                <label className="sandbox-checkbox" key={project.id}>
                  <input
                    type="checkbox"
                    disabled={!loaded || pending}
                    checked={selectedProjects.includes(project.id)}
                    onChange={(e) => {
                      setSelectedProjects((prev) =>
                        e.target.checked
                          ? [...prev, project.id]
                          : prev.filter((id) => id !== project.id),
                      );
                      setNotice("");
                    }}
                  />
                  {project.name}
                </label>
              ))
            )}
          </div>
        )}
      </div>

      <div className="sandbox-block">
        <strong>Execution policy</strong>
        <p className="muted">
          Autonomy runs shell commands inside the sandbox and blocks unsandboxed
          retries. Save and start the next message to apply.
        </p>
        <div className="sandbox-policy-controls">
          <button
            type="button"
            disabled={!loaded || pending}
            className={autonomyActive ? "active" : ""}
            aria-pressed={autonomyActive}
            onClick={() => {
              if (autonomyActive) {
                setSandboxState("inherited");
                setSandboxedCmds("inherited");
                setUnsandboxedRetries("inherited");
              } else {
                setSandboxState("on");
                setSandboxedCmds("on");
                setUnsandboxedRetries("off");
              }
              setNotice("");
            }}
          >
            Autonomy in sandbox
          </button>
          <label>
            Sandboxed commands
            <select
              disabled={!loaded || pending}
              value={sandboxedCmds}
              onChange={(e) => {
                setSandboxedCmds(e.target.value as TriState);
                setNotice("");
              }}
            >
              <option value="inherited">Inherited</option>
              <option value="on">Auto-allow</option>
              <option value="off">Ask</option>
            </select>
          </label>
          <label>
            Unsandboxed retries
            <select
              disabled={!loaded || pending}
              value={unsandboxedRetries}
              onChange={(e) => {
                setUnsandboxedRetries(e.target.value as TriState);
                setNotice("");
              }}
            >
              <option value="inherited">Inherited</option>
              <option value="off">Blocked</option>
              <option value="on">Allowed</option>
            </select>
          </label>
        </div>
      </div>

      <details className="sandbox-rules-details">
        <summary>Network domains, excluded commands, and tool rules</summary>
        <div className="sandbox-lists-grid">
          <ListEditor
            label="Allowed domains"
            placeholder="e.g. registry.npmjs.org"
            entries={allowedDomains}
            disabled={!loaded || pending}
            onChange={(next) => {
              setAllowedDomains(next);
              setNotice("");
            }}
          />
          <ListEditor
            label="Excluded commands"
            placeholder="e.g. git"
            entries={excludedCommands}
            disabled={!loaded || pending}
            onChange={(next) => {
              setExcludedCommands(next);
              setNotice("");
            }}
          />
          <ListEditor
            label="Allowed tool rules"
            placeholder="e.g. Bash(npm test)"
            entries={allowRules}
            disabled={!loaded || pending}
            onChange={(next) => {
              setAllowRules(next);
              setNotice("");
            }}
          />
          <ListEditor
            label="Denied tool rules"
            placeholder="e.g. Bash(rm -rf *)"
            entries={denyRules}
            disabled={!loaded || pending}
            onChange={(next) => {
              setDenyRules(next);
              setNotice("");
            }}
          />
        </div>
      </details>

      <div className="sandbox-save-row">
        <button
          type="button"
          className="primary"
          disabled={!loaded || pending}
          onClick={() => void save()}
        >
          {pending ? "Saving…" : "Save Claude sandbox"}
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
