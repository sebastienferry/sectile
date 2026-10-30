import { useEffect, useMemo, useRef, useState } from "react";
import type { Project } from "../domain/board";
import { statusLabel, type Run } from "../domain/execution";
import { Icon } from "./Icon";

type PaletteView = "prompt" | "board" | "executions" | "orchestration" | "project-settings";
type Entry = {
  group: "action" | "project" | "execution";
  label: string;
  detail?: string;
  run: () => void;
};

export function CommandPalette({
  open,
  onOpenChange,
  projects,
  runs,
  selectedProjectId,
  onNavigate,
  onOpenFolder,
  onOpenSettings,
  onToggleSidebar,
  sidebarActionLabel,
  shortcutModifier,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projects: Project[];
  runs: Run[];
  selectedProjectId?: string;
  onNavigate: (project: Project, view: PaletteView, runId?: string) => void;
  onOpenFolder: () => void;
  onOpenSettings: () => void;
  onToggleSidebar: () => void;
  sidebarActionLabel: string;
  shortcutModifier: string;
}) {
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const dialog = useRef<HTMLDialogElement>(null);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== "k" || event.altKey) return;
      event.preventDefault();
      setQuery("");
      setActive(0);
      onOpenChange(true);
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onOpenChange]);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
    if (open) requestAnimationFrame(() => input.current?.focus());
  }, [open]);

  const entries = useMemo<Entry[]>(() => {
    const selected = projects.find(project => project.id === selectedProjectId);
    const result: Entry[] = [
      { group: "action", label: "Ouvrir un dossier", detail: "Créer ou retrouver un projet local", run: onOpenFolder },
      { group: "action", label: "Paramètres", detail: `${shortcutModifier}, · Configuration du poste`, run: onOpenSettings },
    ];
    if (sidebarActionLabel) result.push({ group: "action", label: sidebarActionLabel, detail: `${shortcutModifier}B`, run: onToggleSidebar });
    if (selected) {
      result.push({ group: "action", label: "Nouveau prompt", detail: `${shortcutModifier}N · ${selected.name}`, run: () => onNavigate(selected, "prompt") });
      result.push({ group: "action", label: "Ouvrir l’orchestration", detail: selected.name, run: () => onNavigate(selected, "orchestration") });
    }
    for (const project of projects) {
      result.push({ group: "project", label: project.name, detail: "Ouvrir le tableau", run: () => onNavigate(project, "board") });
      if (project.id !== selectedProjectId) {
        result.push({ group: "project", label: `${project.name} · Nouveau prompt`, detail: "Démarrer une conversation", run: () => onNavigate(project, "prompt") });
      }
    }
    for (const run of [...runs].reverse()) {
      const project = projects.find(item => item.id === run.projectId);
      if (!project) continue;
      result.push({
        group: "execution",
        label: run.conversation ? run.title || run.prompt?.trim().split("\n")[0] || "Conversation" : run.taskKey || run.skill || "Exécution",
        detail: [project.name, run.provider, run.conversation ? "Conversation" : statusLabel(run)].filter(Boolean).join(" · "),
        run: () => onNavigate(project, run.conversation ? "prompt" : "executions", run.id),
      });
    }
    return result;
  }, [onNavigate, onOpenFolder, onOpenSettings, onToggleSidebar, projects, runs, selectedProjectId, sidebarActionLabel, shortcutModifier]);

  const shown = useMemo(() => {
    const words = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
    const groupOrder = { action: 0, project: 1, execution: 2 };
    const matches = entries.map((entry, index) => ({ entry, index })).filter(({ entry }) => {
      const text = `${entry.label} ${entry.detail ?? ""}`.toLocaleLowerCase();
      return words.every(word => text.includes(word));
    });
    matches.sort((left, right) => {
      const group = groupOrder[left.entry.group] - groupOrder[right.entry.group];
      if (group) return group;
      const rank = (entry: Entry) => {
        const label = entry.label.toLocaleLowerCase();
        if (label.startsWith(words[0] ?? "")) return 0;
        if (label.split(/[\s·#/-]+/).some(part => part.startsWith(words[0] ?? ""))) return 1;
        return 2;
      };
      return rank(left.entry) - rank(right.entry) || left.index - right.index;
    });
    return matches.slice(0, 30).map(match => match.entry);
  }, [entries, query]);

  function choose(entry: Entry) {
    onOpenChange(false);
    entry.run();
  }

  return <dialog
    ref={dialog}
    className="command-palette"
    aria-label="Navigation rapide"
    onCancel={event => { event.preventDefault(); onOpenChange(false); }}
    onClose={() => onOpenChange(false)}
    onClick={event => { if (event.target === dialog.current) onOpenChange(false); }}
  >
    <label className="command-palette-search">
      <Icon name="search" />
      <input
        ref={input}
        role="combobox"
        aria-label="Rechercher une action, un projet ou une exécution"
        aria-controls="command-palette-results"
        aria-activedescendant={shown[active] ? `command-palette-option-${active}` : undefined}
        aria-expanded="true"
        aria-autocomplete="list"
        placeholder="Rechercher une action, un projet ou une exécution…"
        value={query}
        onChange={event => { setQuery(event.target.value); setActive(0); }}
        onKeyDown={event => {
          if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            if (shown.length) setActive(index => (index + (event.key === "ArrowDown" ? 1 : shown.length - 1)) % shown.length);
          } else if (event.key === "Enter" && shown[active]) {
            event.preventDefault();
            choose(shown[active]);
          }
        }}
      />
      <kbd>Esc</kbd>
    </label>
    <ul id="command-palette-results" role="listbox" aria-label="Résultats">
      {(["action", "project", "execution"] as const).map(group => {
        const items = shown.filter(entry => entry.group === group);
        if (!items.length) return null;
        const label = group === "action" ? "Actions" : group === "project" ? "Projets" : "Exécutions";
        return <li className="command-palette-group" role="presentation" key={group}>
          <span>{label}</span>
          <ul role="presentation">
            {items.map(entry => {
              const index = shown.indexOf(entry);
              return <li role="presentation" key={`${entry.group}:${entry.label}:${index}`}>
                <button
                  id={`command-palette-option-${index}`}
                  role="option"
                  aria-selected={index === active}
                  onMouseMove={() => setActive(index)}
                  onClick={() => choose(entry)}
                >
                  <span>{entry.label}</span>
                  {entry.detail && <small>{entry.detail}</small>}
                </button>
              </li>;
            })}
          </ul>
        </li>;
      })}
      {!shown.length && <li className="command-palette-empty" role="presentation">Aucun résultat</li>}
    </ul>
  </dialog>;
}
