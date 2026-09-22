import React, { useEffect, useMemo, useState } from 'react'
import {
  Bot,
  Check,
  FileCode2,
  Loader2,
  Package,
  RefreshCw,
  RotateCcw,
  Save,
  Terminal,
  X,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type {
  MarketplaceCatalog,
  SkillEditorEntry,
  SkillMarketplace,
  SkillMode,
  SkillPackPin,
  SkillPackPreview,
} from '../types'
import { isFromMarketplace, isReproducible, originLabel, pinLabel, previewSummary } from '../lib/skillPack'

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
 *
 * Le corps d'une skill peut aussi venir d'une marketplace. La précédence est
 * alors « modèle intégré → pack → édition du projet », l'édition gagnant
 * toujours, et un pack ne devient effectif qu'après que son diff a été vu.
 */
export const SkillsView: React.FC = () => {
  const {
    currentProject,
    fetchSkillEditor,
    saveSkillContent,
    resetSkillContent,
    saveSkillMode,
    fetchSkillMarketplaces,
    fetchMarketplaceCatalog,
    fetchSkillPackPin,
    previewSkillPack,
    applySkillPack,
    unpinSkillPack,
    t,
  } = useApp()

  const [entries, setEntries] = useState<SkillEditorEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [selectedId, setSelectedId] = useState<string>('')
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState<string | null>(null)

  const [pin, setPin] = useState<SkillPackPin | null>(null)
  const [marketplaces, setMarketplaces] = useState<SkillMarketplace[] | null>(null)
  const [marketplace, setMarketplace] = useState('')
  const [catalog, setCatalog] = useState<MarketplaceCatalog | null>(null)
  const [preview, setPreview] = useState<SkillPackPreview | null>(null)
  const [packBusy, setPackBusy] = useState<string | null>(null)

  const load = async () => {
    setIsLoading(true)
    const [list, pinned] = await Promise.all([fetchSkillEditor(), fetchSkillPackPin()])
    setEntries(list)
    setPin(pinned)
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

  const closePack = () => {
    setMarketplaces(null)
    setMarketplace('')
    setCatalog(null)
    setPreview(null)
  }

  // Ouvrir le sélecteur ne lit que le registre : rien n'est résolu tant qu'une
  // marketplace n'est pas choisie, et rien n'est écrit tant que le pack n'est
  // pas appliqué.
  const openPicker = async () => {
    setPackBusy('picker')
    setPreview(null)
    setCatalog(null)
    setMarketplaces(await fetchSkillMarketplaces())
    setPackBusy(null)
  }

  const openCatalog = async (name: string) => {
    setPackBusy('catalog')
    setMarketplace(name)
    setCatalog(await fetchMarketplaceCatalog(name))
    setPackBusy(null)
  }

  const openPreview = async (source: string, plugin: string, commit?: string) => {
    setPackBusy('preview')
    setPreview(await previewSkillPack(source, plugin, commit))
    setPackBusy(null)
  }

  const applyPack = async () => {
    if (!preview) return
    setPackBusy('apply')
    const applied = await applySkillPack(preview.pack.marketplace, preview.pack.plugin, preview.pack.commit)
    setPackBusy(null)
    if (!applied) return
    closePack()
    await load()
  }

  const unpin = async () => {
    setPackBusy('unpin')
    const done = await unpinSkillPack()
    setPackBusy(null)
    if (done) await load()
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

  const summary = previewSummary(preview)

  return (
    <div className="flex-1 flex flex-col min-h-0">
      {/* Le pack de skills du projet : d'où viennent les corps que les agents exécutent */}
      <div className="px-3 py-2 border-b border-[var(--border-color)] flex items-center gap-2 flex-wrap bg-[var(--bg-secondary)]">
        <Package size={13} className="text-[var(--accent-color)] shrink-0" />
        <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-secondary)]">
          {t.skillPack.title}
        </span>
        {pin ? (
          <span className="flex items-center gap-2 min-w-0">
            <code className="text-[10px] font-mono text-[var(--text-primary)] truncate">{pinLabel(pin)}</code>
            {pin.appliedAt && (
              <span className="text-[9px] text-[var(--text-muted)]">
                {t.skillPack.appliedAt} {new Date(pin.appliedAt).toLocaleString()}
              </span>
            )}
            {pin.orphaned && (
              <span role="alert" className="text-[9px] font-bold text-amber-400" title={t.skillPack.orphaned}>
                {t.skillPack.orphaned}
              </span>
            )}
          </span>
        ) : (
          <span className="text-[10px] text-[var(--text-muted)]">{t.skillPack.none}</span>
        )}

        <div className="ml-auto flex items-center gap-1.5">
          <button
            type="button"
            onClick={openPicker}
            disabled={packBusy !== null}
            className="flex items-center gap-1 px-2.5 py-1 rounded-xl text-[10px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
          >
            {packBusy === 'picker' ? <Loader2 size={10} className="animate-spin" /> : <Package size={10} />}
            <span>{t.skillPack.choose}</span>
          </button>
          {pin && !pin.orphaned && (
            <button
              type="button"
              onClick={() => openPreview(pin.marketplace, pin.plugin)}
              disabled={packBusy !== null}
              className="flex items-center gap-1 px-2.5 py-1 rounded-xl text-[10px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
              title="Relire la marketplace à sa tête et montrer le diff"
            >
              {packBusy === 'preview' ? <Loader2 size={10} className="animate-spin" /> : <RefreshCw size={10} />}
              <span>{t.skillPack.update}</span>
            </button>
          )}
          {pin && (
            <button
              type="button"
              onClick={unpin}
              disabled={packBusy !== null}
              className="flex items-center gap-1 px-2.5 py-1 rounded-xl text-[10px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
            >
              {packBusy === 'unpin' ? <Loader2 size={10} className="animate-spin" /> : <RotateCcw size={10} />}
              <span>{t.skillPack.unpin}</span>
            </button>
          )}
        </div>
      </div>

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
                      {isFromMarketplace(entry) && (
                        <span
                          className="text-[8px] font-bold px-1 rounded text-sky-400 bg-sky-500/10 border border-sky-500/30"
                          title={originLabel(entry)}
                        >
                          {t.skillPack.badge}
                        </span>
                      )}
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

        {/* Le sélecteur de pack, son aperçu, ou l'éditeur */}
        {marketplaces !== null || preview ? (
          <div className="flex-1 flex flex-col min-h-0">
            <div className="px-4 py-2.5 border-b border-[var(--border-color)] flex items-center gap-2">
              <h3 className="text-[13px] font-bold text-[var(--text-primary)]">
                {preview ? t.skillPack.previewTitle : t.skillPack.choose}
              </h3>
              <p className="text-[10px] text-[var(--text-muted)]">{t.skillPack.previewIntro}</p>
              <button
                type="button"
                onClick={closePack}
                className="ml-auto flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-[11px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                <X size={11} />
                <span>{t.skillPack.close}</span>
              </button>
              {preview && (
                <button
                  type="button"
                  onClick={applyPack}
                  disabled={packBusy !== null}
                  className="flex items-center gap-1 px-3 py-1.5 rounded-xl text-[11px] font-bold text-white accent-bg hover:opacity-90 disabled:opacity-40 cursor-pointer"
                >
                  {packBusy === 'apply' ? <Loader2 size={11} className="animate-spin" /> : <Check size={11} />}
                  <span>{t.skillPack.apply}</span>
                </button>
              )}
            </div>

            <div className="flex-1 overflow-y-auto p-4 space-y-3 text-[11px] text-[var(--text-secondary)]">
              {packBusy === 'catalog' || packBusy === 'preview' ? (
                <div className="flex items-center gap-2">
                  <Loader2 size={12} className="animate-spin text-[var(--accent-color)]" />
                  <span>{t.skillPack.loading}</span>
                </div>
              ) : null}

              {!preview && marketplaces && (
                <div className="space-y-1.5">
                  <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{t.skillPack.marketplace}</div>
                  {marketplaces.length === 0 && (
                    <p className="text-[var(--text-muted)]">Aucune marketplace enregistrée pour ce déploiement.</p>
                  )}
                  {marketplaces.map(entry => (
                    <button
                      key={entry.name}
                      type="button"
                      onClick={() => openCatalog(entry.name)}
                      className={`w-full text-left px-2.5 py-2 rounded-xl border cursor-pointer ${
                        entry.name === marketplace
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)]/40'
                          : 'bg-[var(--bg-secondary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/30'
                      }`}
                    >
                      <span className="font-bold text-[var(--text-primary)]">{entry.name}</span>
                      <code className="ml-2 text-[9px] font-mono text-[var(--text-muted)]">{entry.kind} · {entry.locator}</code>
                      {entry.description && <div className="text-[10px] text-[var(--text-muted)]">{entry.description}</div>}
                    </button>
                  ))}
                </div>
              )}

              {!preview && catalog && (
                <div className="space-y-1.5 pt-2">
                  <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{t.skillPack.plugin}</div>
                  {catalog.plugins.filter(plugin => plugin.skills.length > 0).length === 0 && (
                    <p className="text-[var(--text-muted)]">{t.skillPack.noPlugin}</p>
                  )}
                  {catalog.plugins.map(plugin => (
                    <button
                      key={plugin.name}
                      type="button"
                      disabled={plugin.skills.length === 0}
                      onClick={() => openPreview(marketplace, plugin.name, catalog.commit)}
                      className="w-full text-left px-2.5 py-2 rounded-xl border bg-[var(--bg-secondary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/30 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                    >
                      <span className="font-bold text-[var(--text-primary)]">{plugin.name}</span>
                      {plugin.version && <code className="ml-2 text-[9px] font-mono text-[var(--text-muted)]">@{plugin.version}</code>}
                      {plugin.description && <div className="text-[10px] text-[var(--text-muted)]">{plugin.description}</div>}
                      <div className="text-[9px] font-mono text-[var(--text-muted)]">
                        {plugin.skills.length > 0 ? plugin.skills.join(', ') : plugin.error}
                      </div>
                      {plugin.ignored && plugin.ignored.length > 0 && (
                        <div className="text-[9px] font-mono text-amber-400">
                          {t.skillPack.ignored} : {plugin.ignored.join(', ')}
                        </div>
                      )}
                    </button>
                  ))}
                </div>
              )}

              {preview && (
                <>
                  <div className="font-mono text-[10px] text-[var(--text-muted)]">
                    {preview.pack.marketplace} / {preview.pack.plugin}
                    {preview.pack.version ? ` @ ${preview.pack.version}` : ''}
                    {preview.pack.commit ? ` · ${preview.pack.commit.slice(0, 7)}` : ''}
                  </div>
                  {!isReproducible(preview) && (
                    <p role="alert" className="text-amber-400">{t.skillPack.notReproducible}</p>
                  )}
                  {summary.warnings.map(warning => (
                    <p key={warning} className="text-amber-400">{warning}</p>
                  ))}

                  {summary.rejected.length > 0 && (
                    <div>
                      <div className="text-[10px] font-bold uppercase tracking-wider text-rose-400">{t.skillPack.rejected}</div>
                      <ul className="list-disc ml-4">
                        {summary.rejected.map(item => (
                          <li key={item.dir} className="font-mono text-[10px]">{item.dir} — {item.reason}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {summary.ignored.length > 0 && (
                    <p className="font-mono text-[10px] text-[var(--text-muted)]">
                      {t.skillPack.ignored} : {summary.ignored.join(', ')}
                    </p>
                  )}
                  {summary.missing.length > 0 && (
                    <p className="font-mono text-[10px] text-[var(--text-muted)]">
                      {t.skillPack.missing} : {summary.missing.join(', ')}
                    </p>
                  )}

                  <div className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    {t.skillPack.changed} ({summary.changed.length}) · {t.skillPack.unchanged} ({summary.unchanged.length})
                  </div>
                  {preview.entries.map(entry => (
                    <details key={entry.skillId} className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] px-2.5 py-2">
                      <summary className="cursor-pointer text-[11px] font-bold text-[var(--text-primary)]">
                        {entry.name} <code className="font-mono text-[9px] text-[var(--text-muted)]">{entry.dirName}</code>
                        {entry.changed ? '' : ' — identique'}
                      </summary>
                      <div className="grid grid-cols-2 gap-2 mt-2">
                        <pre className="whitespace-pre-wrap font-mono text-[9.5px] text-[var(--text-muted)] max-h-64 overflow-y-auto">{entry.current}</pre>
                        <pre className="whitespace-pre-wrap font-mono text-[9.5px] text-[var(--text-primary)] max-h-64 overflow-y-auto">{entry.proposed}</pre>
                      </div>
                    </details>
                  ))}
                </>
              )}
            </div>
          </div>
        ) : (
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
                        title={isFromMarketplace(selected) ? 'Revenir au corps fourni par le pack' : 'Revenir au modèle intégré de Sectile'}
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
                    {selected.isCustom
                      ? 'contenu propre à ce projet'
                      : isFromMarketplace(selected)
                        ? originLabel(selected)
                        : 'modèle intégré de Sectile'}
                  </span>
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
