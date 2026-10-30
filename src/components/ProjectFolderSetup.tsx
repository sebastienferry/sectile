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
        setNotice("Dossier associé au projet.");
      }
    } catch {
      setError("Impossible d’associer ce dossier. Vérifiez le dossier et la connexion de l’agent, puis réessayez.");
    } finally {
      choosing.current = false;
      setPending(false);
    }
  }
  if (variant === "board") return <div className="board-folder-action">
    <button className="primary" disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Association en cours…" : "Choisir un dossier"}</button>
    {pending && <p role="status">Choisissez un dossier dans la fenêtre de sélection.</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </div>;
  if (variant === "settings") return <section className="project-folder-settings" aria-label="Dossier du projet">
    <div className="project-folder-settings-heading">
      <h2>Dossier local</h2>
      <button disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Association en cours…" : project.path ? "Changer de dossier" : "Choisir un dossier"}</button>
    </div>
    {project.path ? <p className="project-folder-path" title={project.path}><Icon name="folder" /><code>{project.path}</code></p> : <p className="muted">Aucun dossier associé à ce projet.</p>}
    <p className="muted project-folder-hint">Utilisé pour les nouveaux prompts ; les conversations existantes gardent leur dossier.</p>
    {pending && <p role="status">Choisissez un dossier dans la fenêtre de sélection.</p>}
    {notice && <p role="status">{notice}</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
  return <section className="project-prompt prompt-start folder-prompt-setup">
    <div className="prompt-intro">
      <span className="prompt-symbol"><Icon name="folder" /></span>
      <span className="eyebrow">DANS CE PROJET</span>
      <h2>Où voulez-vous travailler ?</h2>
      <p>Choisissez le dossier local de {project.name} pour commencer une conversation.<br />Un dossier simple suffit, sans Git ni tracker.</p>
    </div>
    <button className="primary" disabled={pending} onClick={() => void chooseFolder()}><Icon name="folder" />{pending ? "Association en cours…" : "Choisir un dossier"}</button>
    <p className="muted">Le dossier sera associé à ce projet sur votre poste.</p>
    {pending && <p role="status">Choisissez un dossier dans la fenêtre de sélection.</p>}
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
}
