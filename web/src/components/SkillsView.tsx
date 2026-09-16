import React, { useEffect, useMemo, useState } from 'react'
import {
  Bot,
  FileCode2,
  Loader2,
  RotateCcw,
  Save,
  Terminal,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { SkillEditorEntry, SkillMode } from '../types'

/**
 * Les trois valeurs du réglage par skill. « Défaut du projet » est le troisième
 * état dont la précédence a besoin : sans lui, aucune skill ne peut dire « pas
 * d'avis » et retomber sur le réglage du projet.
 */
const SKILL_MODE_OPTIONS: { value: SkillMode; label: string; title: string }[] = [
  { value: '', label: 'Défaut du projet', title: "La skill ne fixe rien : le défaut du projet décide" },
  { value: 'interactive', label: 'Interactif', title: 'Ouvre un terminal que tu réponds, et tu confirmes la transition' },
  { value: 'autonomous', label: 'Autonome', title: "Lance la CLI en headless, sans terminal ; le worker pose la transition" },
]

/**
 * Éditeur des skills du workflow agentique.
 *
 * Les cinq pas du workflow ont une skill et une seule, et c'est le même contenu
 * qui est rendu dans chaque répertoire d'agent du dépôt. Éditer ici régénère les
 * fichiers ; un SKILL.md retouché à la main dans le dépôt n'est pas écrasé en
 * silence, il est signalé comme divergent et peut être réimporté.
 */
export const SkillsView: React.FC = () => {
  const { currentProject, fetchSkillEditor, saveSkillContent, resetSkillContent, saveSkillMode } = useApp()

  const [entries, setEntries] = useState<SkillEditorEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [selectedId, setSelectedId] = useState<string>('')
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const load = async () => {
    setIsLoading(true)
    const list = await fetchSkillEditor()
    setEntries(list)
    setIsLoading(false)
    if (list.length && !list.some(e => e.id === selectedId)) {
      setSelectedId(list[0].id)
      setDraft(list[0].content)
    }
  }

  useEffect(() => {
    load()
    // Rechargé au changement de projet : les skills sont propres au projet.
  }, [currentProject?.id])

  const selected = useMemo(() => entries.find(e => e.id === selectedId) || null, [entries, selectedId])

  const select = (entry: SkillEditorEntry) => {
    setSelectedId(entry.id)
    setDraft(entry.content)
  }

  const isDirty = Boolean(selected && (draft !== selected.content || selected.requiresReconciliation))

  const applyEntry = (entry: SkillEditorEntry | null) => {
    if (!entry) return
    setEntries(prev => prev.map(e => (e.id === entry.id ? entry : e)))
    setDraft(entry.content)
  }

  const run = async (action: string, fn: () => Promise<SkillEditorEntry | null>) => {
    if (busy) return
    setBusy(action)
    applyEntry(await fn())
    setBusy(null)
  }

  if (!currentProject) {
    return (
      <div className="flex-1 flex items-center justify-center p-8 text-center">
        <p className="text-xs text-[var(--text-muted)] max-w-sm">
          Sélectionne un projet : les skills sont éditées par projet, et régénérées dans le dépôt de ce
          projet.
        </p>
      </div>
    )
  }

  return (
    <div className="flex-1 flex min-h-0">
      {/* Les pas du workflow, dans l'ordre */}
      <div className="w-72 shrink-0 border-r border-[var(--border-color)] flex flex-col min-h-0">
        <div className="px-3 py-2.5 border-b border-[var(--border-color)]">
          <h2 className="text-[11px] font-bold uppercase tracking-wider text-[var(--text-secondary)] flex items-center gap-1.5">
            <FileCode2 size={13} className="text-[var(--accent-color)]" />
            <span>Workflow skills</span>
          </h2>
          <p className="text-[10px] text-[var(--text-muted)] mt-1 leading-snug">
            Clarify → Specify → Implement → Adjust → Handoff.
          </p>
        </div>

        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {isLoading && entries.length === 0 ? (
            <div className="flex items-center gap-2 px-2 py-3 text-[var(--text-muted)]">
              <Loader2 size={13} className="animate-spin text-[var(--accent-color)]" />
              <span className="text-[11px]">Lecture des skills…</span>
            </div>
          ) : (
            entries.map((entry, index) => (
              <React.Fragment key={entry.id}>
                {index === 5 && <h3 className="px-2 pt-4 pb-1 text-[10px] font-bold uppercase text-[var(--text-muted)]">Additional skills</h3>}
                <button
                  type="button"
                  onClick={() => select(entry)}
                  className={`w-full text-left px-2.5 py-2 rounded-xl border transition-colors cursor-pointer ${
                    entry.id === selectedId
                      ? 'bg-[var(--accent-light)] border-[var(--accent-color)]/40'
                      : 'bg-[var(--bg-secondary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/30'
                  }`}
                >
                  <div className="flex items-center gap-1.5">
                    {index < 5 && <span className="text-[9px] font-mono font-bold text-[var(--text-muted)]">{index + 1}</span>}
                    <span className="text-[11px] font-bold text-[var(--text-primary)] truncate">{entry.name}</span>
                    {entry.scope === 'macro' && (
                      <span
                        className="ml-auto text-[8px] font-bold px-1 rounded text-orange-400 bg-orange-500/10 border border-orange-500/30 shrink-0"
                        title="Skill de cadrage et raffinage Macro"
                      >
                        MACRO
                      </span>
                    )}
                    {entry.isCustom && (
                      <span
                        className={`${entry.scope === 'macro' ? '' : 'ml-auto'} text-[8px] font-bold px-1 rounded text-[var(--accent-color)] bg-[var(--accent-light)] border border-[var(--accent-color)]/30 shrink-0`}
                        title="Contenu propre à ce projet"
                      >
                        PERSO
                      </span>
                    )}
                  </div>
                  <div className="mt-1 flex items-center gap-1 text-[9px] font-mono text-[var(--text-muted)]">
                    {entry.scope === 'macro' ? (
                      <span className="text-orange-400 font-bold">Raffinage Macro</span>
                    ) : entry.fromStage && entry.toStage ? (
                      <>
                        <span>#{entry.fromStage}</span>
                        <span className="text-[var(--accent-color)]">➔</span>
                        <span>#{entry.toStage}</span>
                      </>
                    ) : <span>Additional skill</span>}
                    {entry.mode && (
                      <span
                        className="ml-1 flex items-center gap-0.5 text-[var(--text-secondary)]"
                        title={entry.mode === 'autonomous' ? 'Exécution autonome (headless)' : 'Session interactive'}
                      >
                        {entry.mode === 'autonomous' ? <Bot size={8} /> : <Terminal size={8} />}
                        {entry.mode === 'autonomous' ? 'Autonome' : 'Interactif'}
                      </span>
                    )}
                  </div>
                  <div className="mt-1 flex items-center gap-1.5">
                    <code className="text-[9px] text-[var(--text-secondary)]">{entry.command}</code>
                    {!entry.installed && (
                      <span className="text-[8px] font-bold text-amber-400" title="Aucun SKILL.md dans le dépôt">
                        NON INSTALLÉE
                      </span>
                    )}
                    {entry.diverged && (
                      <span className="text-[8px] font-bold text-rose-400" title="Le fichier du dépôt diffère">
                        DIVERGENTE
                      </span>
                    )}
                  </div>
                </button>
              </React.Fragment>
            ))
          )}
        </div>
      </div>

      {/* L'éditeur */}
      <div className="flex-1 flex flex-col min-h-0">
        {!selected ? (
          <div className="flex-1 flex items-center justify-center text-[11px] text-[var(--text-muted)]">
            Choisis une skill à gauche.
          </div>
        ) : (
          <>
            <div className="px-4 py-2.5 border-b border-[var(--border-color)] flex items-center gap-2 flex-wrap">
              <div className="min-w-0">
                {selected.requiresReconciliation && <p role="alert" className="text-amber-400 text-xs">Legacy customization requires reconciliation. Review the complete content and save under Adjust, or reset to the default. Automatic adjustment is blocked.</p>}
                {selected.overrideOrigin && <p className="text-xs">Source: {selected.overrideOrigin}. Other saved entries: {selected.legacyConflicts?.join(', ') || 'none'}</p>}
                <h3 className="text-[13px] font-bold text-[var(--text-primary)] truncate">{selected.name}</h3>
                <p className="text-[10px] text-[var(--text-muted)] truncate">{selected.description}</p>
              </div>

              <div className="ml-auto flex items-center gap-1.5">
                <label className="flex items-center gap-1 text-[10px] text-[var(--text-secondary)]">
                  <span>Mode</span>
                  <select
                    value={selected.mode || ''}
                    disabled={busy !== null}
                    onChange={e => run('mode', () => saveSkillMode(selected.id, e.target.value as SkillMode))}
                    className="px-1.5 py-1 rounded-lg text-[10px] bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] disabled:opacity-40 cursor-pointer"
                    title="Mode d'exécution de cette skill. Une surcharge au lancement le remplace pour ce lancement seulement."
                  >
                    {SKILL_MODE_OPTIONS.map(option => (
                      <option key={option.value} value={option.value} title={option.title}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>

                {selected.isCustom && (
                  <button
                    type="button"
                    onClick={() => run('reset', () => resetSkillContent(selected.id))}
                    disabled={busy !== null}
                    className="flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-[11px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
                    title="Revenir au modèle intégré de Sectile"
                  >
                    {busy === 'reset' ? <Loader2 size={11} className="animate-spin" /> : <RotateCcw size={11} />}
                    <span>Réinitialiser</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => run('save', () => saveSkillContent(selected.id, draft))}
                  disabled={busy !== null || !isDirty}
                  className="flex items-center gap-1 px-3 py-1.5 rounded-xl text-[11px] font-bold text-white accent-bg hover:opacity-90 disabled:opacity-40 cursor-pointer"
                  title="Save skill instructions for the local agent"
                >
                  {busy === 'save' ? <Loader2 size={11} className="animate-spin" /> : <Save size={11} />}
                  <span>{isDirty ? 'Enregistrer' : 'À jour'}</span>
                </button>
              </div>
            </div>



            {Object.entries(selected.legacyContents || {}).map(([id, content]) => <details key={id} className="px-4 text-xs"><summary>Preserved customization: {id}</summary><pre className="whitespace-pre-wrap">{content}</pre></details>)}
            <textarea
              value={draft}
              onChange={e => setDraft(e.target.value)}
              spellCheck={false}
              className="flex-1 min-h-0 w-full px-4 py-3 font-mono text-[11.5px] leading-relaxed bg-[var(--bg-primary)] text-[var(--text-primary)] border-0 focus:outline-none resize-none"
            />

            <div className="px-4 py-1.5 border-t border-[var(--border-color)] flex items-center gap-3 text-[9px] font-mono text-[var(--text-muted)] flex-wrap">
              <span>{draft.split('\n').length} lignes</span>

              {selected.updatedAt && <span>modifiée le {new Date(selected.updatedAt).toLocaleString()}</span>}
              <span className="ml-auto">
                {selected.isCustom ? 'contenu propre à ce projet' : 'modèle intégré de Sectile'}
              </span>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
