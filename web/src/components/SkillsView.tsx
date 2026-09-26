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
import { format, formatDateTime, plural } from '../lib/i18n'
import type { SkillsEditorStrings } from '../locales/skillsEditor'
import type { SkillEditorEntry, SkillMode } from '../types'

/**
 * The three values of the per-skill setting. "Project default" is the third
 * state the precedence needs: without it, no skill could say "no opinion" and
 * fall back to the project setting. The stored values never change with the
 * UI language, only their labels do.
 */
const skillModeOptions = (modes: SkillsEditorStrings['modes']): { value: SkillMode; label: string; title: string }[] => [
  { value: '', label: modes.projectDefault, title: modes.projectDefaultHelp },
  { value: 'interactive', label: modes.interactive, title: modes.interactiveHelp },
  { value: 'autonomous', label: modes.autonomous, title: modes.autonomousHelp },
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
  const { t, settings, currentProject, fetchSkillEditor, saveSkillContent, resetSkillContent, saveSkillMode } = useApp()
  const { modes, list, indicators, editor } = t.skillsEditor

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
    // Reloaded when the project changes: skills belong to the project.
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
        <p className="text-xs text-[var(--text-muted)] max-w-sm">{list.noProject}</p>
      </div>
    )
  }

  return (
    <div className="flex-1 flex min-h-0">
      {/* The workflow steps, in order */}
      <div className="w-72 shrink-0 border-r border-[var(--border-color)] flex flex-col min-h-0">
        <div className="px-3 py-2.5 border-b border-[var(--border-color)]">
          <h2 className="text-[11px] font-bold uppercase tracking-wider text-[var(--text-secondary)] flex items-center gap-1.5">
            <FileCode2 size={13} className="text-[var(--accent-color)]" />
            <span>{list.title}</span>
          </h2>
          <p className="text-[10px] text-[var(--text-muted)] mt-1 leading-snug">{list.pipeline}</p>
        </div>

        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {isLoading && entries.length === 0 ? (
            <div className="flex items-center gap-2 px-2 py-3 text-[var(--text-muted)]">
              <Loader2 size={13} className="animate-spin text-[var(--accent-color)]" />
              <span className="text-[11px]">{list.loading}</span>
            </div>
          ) : (
            entries.map((entry, index) => (
              <React.Fragment key={entry.id}>
                {index === 5 && <h3 className="px-2 pt-4 pb-1 text-[10px] font-bold uppercase text-[var(--text-muted)]">{list.additionalSkills}</h3>}
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
                        title={indicators.macroTitle}
                      >
                        {indicators.macro}
                      </span>
                    )}
                    {entry.isCustom && (
                      <span
                        className={`${entry.scope === 'macro' ? '' : 'ml-auto'} text-[8px] font-bold px-1 rounded text-[var(--accent-color)] bg-[var(--accent-light)] border border-[var(--accent-color)]/30 shrink-0`}
                        title={indicators.customTitle}
                      >
                        {indicators.custom}
                      </span>
                    )}
                  </div>
                  <div className="mt-1 flex items-center gap-1 text-[9px] font-mono text-[var(--text-muted)]">
                    {entry.scope === 'macro' ? (
                      <span className="text-orange-400 font-bold">{entry.id === 'realign_macro' ? list.macroRealignment : list.macroRefinement}</span>
                    ) : entry.fromStage && entry.toStage ? (
                      <>
                        <span>#{entry.fromStage}</span>
                        <span className="text-[var(--accent-color)]">➔</span>
                        <span>#{entry.toStage}</span>
                      </>
                    ) : <span>{list.additionalSkill}</span>}
                    {entry.mode && (
                      <span
                        className="ml-1 flex items-center gap-0.5 text-[var(--text-secondary)]"
                        title={entry.mode === 'autonomous' ? modes.autonomousRun : modes.interactiveSession}
                      >
                        {entry.mode === 'autonomous' ? <Bot size={8} /> : <Terminal size={8} />}
                        {entry.mode === 'autonomous' ? modes.autonomous : modes.interactive}
                      </span>
                    )}
                  </div>
                  <div className="mt-1 flex items-center gap-1.5">
                    <code className="text-[9px] text-[var(--text-secondary)]">{entry.command}</code>
                    {!entry.installed && (
                      <span className="text-[8px] font-bold text-amber-400" title={indicators.notInstalledTitle}>
                        {indicators.notInstalled}
                      </span>
                    )}
                    {entry.diverged && (
                      <span className="text-[8px] font-bold text-rose-400" title={indicators.divergedTitle}>
                        {indicators.diverged}
                      </span>
                    )}
                  </div>
                </button>
              </React.Fragment>
            ))
          )}
        </div>
      </div>

      {/* The editor */}
      <div className="flex-1 flex flex-col min-h-0">
        {!selected ? (
          <div className="flex-1 flex items-center justify-center text-[11px] text-[var(--text-muted)]">
            {list.noSelection}
          </div>
        ) : (
          <>
            <div className="px-4 py-2.5 border-b border-[var(--border-color)] flex items-center gap-2 flex-wrap">
              <div className="min-w-0">
                {selected.requiresReconciliation && <p role="alert" className="text-amber-400 text-xs">{editor.reconciliationRequired}</p>}
                {selected.overrideOrigin && (
                  <p className="text-xs">
                    {format(editor.overrideOrigin, {
                      origin: selected.overrideOrigin,
                      entries: selected.legacyConflicts?.join(', ') || editor.noOtherEntries,
                    })}
                  </p>
                )}
                <h3 className="text-[13px] font-bold text-[var(--text-primary)] truncate">{selected.name}</h3>
                <p className="text-[10px] text-[var(--text-muted)] truncate">{selected.description}</p>
              </div>

              <div className="ml-auto flex items-center gap-1.5">
                <label className="flex items-center gap-1 text-[10px] text-[var(--text-secondary)]">
                  <span>{modes.label}</span>
                  <select
                    value={selected.mode || ''}
                    disabled={busy !== null}
                    onChange={e => run('mode', () => saveSkillMode(selected.id, e.target.value as SkillMode))}
                    className="px-1.5 py-1 rounded-lg text-[10px] bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] disabled:opacity-40 cursor-pointer"
                    title={modes.selectTitle}
                  >
                    {skillModeOptions(modes).map(option => (
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
                    title={editor.resetTitle}
                  >
                    {busy === 'reset' ? <Loader2 size={11} className="animate-spin" /> : <RotateCcw size={11} />}
                    <span>{editor.reset}</span>
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => run('save', () => saveSkillContent(selected.id, draft))}
                  disabled={busy !== null || !isDirty}
                  className="flex items-center gap-1 px-3 py-1.5 rounded-xl text-[11px] font-bold text-white accent-bg hover:opacity-90 disabled:opacity-40 cursor-pointer"
                  title={editor.saveTitle}
                >
                  {busy === 'save' ? <Loader2 size={11} className="animate-spin" /> : <Save size={11} />}
                  <span>{isDirty ? editor.save : editor.upToDate}</span>
                </button>
              </div>
            </div>



            {Object.entries(selected.legacyContents || {}).map(([id, content]) => <details key={id} className="px-4 text-xs"><summary>{format(editor.preservedCustomization, { id })}</summary><pre className="whitespace-pre-wrap">{content}</pre></details>)}
            <textarea
              value={draft}
              onChange={e => setDraft(e.target.value)}
              spellCheck={false}
              className="flex-1 min-h-0 w-full px-4 py-3 font-mono text-[11.5px] leading-relaxed bg-[var(--bg-primary)] text-[var(--text-primary)] border-0 focus:outline-none resize-none"
            />

            <div className="px-4 py-1.5 border-t border-[var(--border-color)] flex items-center gap-3 text-[9px] font-mono text-[var(--text-muted)] flex-wrap">
              <span>{plural(settings.language, draft.split('\n').length, editor.lines)}</span>

              {selected.updatedAt && (
                <span>{format(editor.updatedAt, { date: formatDateTime(settings.language, selected.updatedAt) })}</span>
              )}
              <span className="ml-auto">
                {selected.isCustom ? editor.customContent : editor.builtInTemplate}
              </span>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
