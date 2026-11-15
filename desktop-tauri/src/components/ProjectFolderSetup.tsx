import { useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { Project } from "../domain/board";
import { Icon } from "./Icon";

export function ProjectFolderSetup({ project, onMapped, variant = "prompt" }: {
  project: Project;
  onMapped: (path: string) => void;
  variant?: "prompt" | "settings" | "board";
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const choosing = useRef(false);
  async function chooseFolder() {
    if (choosing.current) return;
    choosing.current = true;
    setPending(true);
    setError("");
    setNotice("");
    try {
      const path = await invoke<string | null>("choose_project_folder", { projectId: project.id });
      if (path) {
        onMapped(path);
        setNotice("Folder mapped to project.");
      }
    } catch {
      setError("Unable to map this folder. Check the folder and agent connection, then try again.");
    } finally {
      choosing.current = false;
      setPending(false);
    }
  }
  if (variant === "board") return <div className="board-folder-action">
    <button className="primary" disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Mapping…" : "Choose folder"}</button>
    {pending && <p role="status">Choose a folder in the picker dialog.</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </div>;
  if (variant === "settings") return <section className="project-folder-settings" aria-label="Project folder">
    <div className="project-folder-settings-heading">
      <h2>Local folder</h2>
      <button disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Mapping…" : project.path ? "Change folder" : "Choose folder"}</button>
    </div>
    {project.path ? <p className="project-folder-path" title={project.path}><Icon name="folder" /><code>{project.path}</code></p> : <p className="muted">No folder mapped to this project.</p>}
    <p className="muted project-folder-hint">Used for new prompts; existing conversations keep their folder.</p>
    {pending && <p role="status">Choose a folder in the picker dialog.</p>}
    {notice && <p role="status">{notice}</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
  return <section className="project-prompt prompt-start folder-prompt-setup">
    <div className="prompt-intro">
      <span className="prompt-symbol"><Icon name="folder" /></span>
      <span className="eyebrow">IN THIS PROJECT</span>
      <h2>Where would you like to work?</h2>
      <p>Choose the local folder for {project.name} to start a conversation.<br />A plain folder is enough, with no Git or tracker required.</p>
    </div>
    <button className="primary" disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Mapping…" : "Choose folder"}</button>
    <p className="muted">The folder will be mapped to this project on your workstation.</p>
    {pending && <p role="status">Choose a folder in the picker dialog.</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
}
