import { useEffect, useRef, useState } from "react";
import type { Run } from "../domain/execution";
import { createConversation, formerContextPreview, type FormerContextPreview } from "../lib/conversation";
import { conversationFor } from "../lib/conversation-controller";
import { conversationProfiles, type ProjectProfiles } from "../lib/profiles";

export interface StartOverSkill { id: string; label: string }

interface StartOverDialogProps {
  projectId: string;
  /** The archived conversation the new session starts over from. */
  source: Run;
  agentAvailable: boolean;
  /** Skills that can start on the source's ticket now; empty without a ticket. */
  skills: StartOverSkill[];
  /** Why the skill path is not offered, when it is not. */
  skillUnavailable?: string;
  onStartSkill?: (skillId: string, instructions: string) => Promise<void>;
  onConversationCreated: (runId: string) => void;
  onClose: () => void;
}

type Path = "conversation" | "skill";

function engineKey(source: Run, profiles?: ProjectProfiles): string {
  if (source.engineId && profiles?.profiles.some(profile => profile.id === source.engineId)) return `profile:${source.engineId}`;
  return source.provider === "codex" ? "engine:codex" : "engine:claude";
}

function formerContextSummary(preview: FormerContextPreview): string {
  const kept = `${preview.messages} message${preview.messages === 1 ? "" : "s"} of this conversation will be given as former context.`;
  const omitted = preview.omitted > 0 ? ` ${preview.omitted} earlier message${preview.omitted === 1 ? " is" : "s are"} left out to fit the context limit.` : "";
  const tools = preview.toolsOmitted ? " Tool calls and their results are not included." : "";
  return kept + omitted + tools;
}

/** Starts a new conversation or skill from an archived conversation (#906). */
export default function StartOverDialog({ projectId, source, agentAvailable, skills, skillUnavailable, onStartSkill, onConversationCreated, onClose }: StartOverDialogProps) {
  const [preview, setPreview] = useState<FormerContextPreview>();
  const [previewError, setPreviewError] = useState("");
  const [profiles, setProfiles] = useState<ProjectProfiles>();
  const [path, setPath] = useState<Path>("conversation");
  const [engine, setEngine] = useState(() => engineKey(source));
  const [skillId, setSkillId] = useState(skills[0]?.id ?? "");
  const [instructions, setInstructions] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const engineTouched = useRef(false);
  const canStartSkill = skills.length > 0 && !!onStartSkill;

  useEffect(() => {
    let current = true;
    setPreview(undefined); setPreviewError("");
    formerContextPreview(projectId, source.id)
      .then(result => { if (current) setPreview(result); })
      .catch(cause => { if (current) setPreviewError(String(cause)); });
    void conversationProfiles(projectId).then(data => {
      if (!current || !data || !Array.isArray(data.profiles)) return;
      setProfiles(data);
      if (!engineTouched.current) setEngine(engineKey(source, data));
    }).catch(() => {});
    return () => { current = false; };
  }, [projectId, source]);

  useEffect(() => {
    if (!skills.some(skill => skill.id === skillId)) setSkillId(skills[0]?.id ?? "");
  }, [skills, skillId]);

  const ready = agentAvailable && !!preview && !busy && instructions.trim() !== "" && (path === "conversation" || (canStartSkill && skillId !== ""));

  async function start() {
    if (!ready) return;
    setBusy(true); setError("");
    try {
      if (path === "skill") {
        await onStartSkill!(skillId, instructions.trim());
        onClose();
        return;
      }
      const profileId = engine.startsWith("profile:") ? engine.slice(8) : undefined;
      const profile = profileId ? profiles?.profiles.find(item => item.id === profileId) : undefined;
      const provider = profile ? profile.provider : engine.slice(7);
      const created = await createConversation(projectId, provider, profileId, source.id);
      void conversationFor(created.id).send(instructions.trim(), true, profile ? { model: profile.model || undefined, mode: profile.mode || undefined } : {});
      onConversationCreated(created.id);
      onClose();
    } catch (cause) {
      setError(String(cause instanceof Error ? cause.message : cause));
    } finally {
      setBusy(false);
    }
  }

  const title = source.title?.trim() || "this conversation";
  return (
    <div className="project-delete-backdrop" onMouseDown={event => { if (event.target === event.currentTarget && !busy) onClose(); }}>
      <dialog open className="project-delete-dialog start-over-dialog" aria-label="Start over">
        <h2>Start over from “{title}”</h2>
        <p className="muted">A new session starts with this conversation as background. The archived conversation stays archived and unchanged.</p>
        {!agentAvailable ? (
          <p role="status" className="start-over-status">Agent unavailable. Reconnect to start over.</p>
        ) : previewError ? (
          <p role="alert" className="error">{previewError}</p>
        ) : !preview ? (
          <p role="status" className="start-over-status">Reading the conversation's history…</p>
        ) : (
          <p role="status" className="start-over-status">{formerContextSummary(preview)}</p>
        )}
        <form onSubmit={event => { event.preventDefault(); void start(); }}>
          <fieldset className="start-over-paths" disabled={busy}>
            <legend className="sr-only">Start</legend>
            <label><input type="radio" name="start-over-path" checked={path === "conversation"} onChange={() => setPath("conversation")} />New conversation</label>
            <label title={canStartSkill ? undefined : skillUnavailable}><input type="radio" name="start-over-path" checked={path === "skill"} disabled={!canStartSkill} onChange={() => setPath("skill")} />Skill</label>
          </fieldset>
          {!canStartSkill && skillUnavailable && <p className="muted start-over-hint">{skillUnavailable}</p>}
          {path === "conversation" ? (
            <label className="start-over-field">Profile or engine
              <select value={engine} disabled={busy} onChange={event => { engineTouched.current = true; setEngine(event.target.value); }}>
                {profiles?.profiles.length ? <optgroup label="Project profiles">{profiles.profiles.map(profile => <option key={profile.id} value={`profile:${profile.id}`}>{profile.provider === "codex" ? "Codex" : "Claude Code"} · {profile.name}</option>)}</optgroup> : null}
                <optgroup label="Without profile"><option value="engine:claude">Claude Code</option><option value="engine:codex">Codex</option></optgroup>
              </select>
            </label>
          ) : (
            <label className="start-over-field">Skill
              <select value={skillId} disabled={busy} onChange={event => setSkillId(event.target.value)}>
                {skills.map(skill => <option key={skill.id} value={skill.id}>{skill.label}</option>)}
              </select>
            </label>
          )}
          <label className="start-over-field">New instructions
            <textarea autoFocus required rows={4} maxLength={6000} value={instructions} disabled={busy} onChange={event => setInstructions(event.target.value)} placeholder="What should the new session do?" />
          </label>
          {error && <p role="alert" className="error">{error}</p>}
          <div className="project-delete-actions">
            <button type="button" disabled={busy} onClick={onClose}>Cancel</button>
            <button type="submit" className="primary" disabled={!ready}>{busy ? "Starting…" : "Start"}</button>
          </div>
        </form>
      </dialog>
    </div>
  );
}
