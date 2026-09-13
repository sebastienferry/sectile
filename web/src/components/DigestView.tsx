import React, { useEffect, useMemo, useState } from 'react'
import {
  CalendarDays,
  Flame,
  Clock,
  CalendarClock,
  Eye,
  ListChecks,
  CheckCircle2,
  Layers,
  Loader2,
  RefreshCw,
  Sparkles,
  Copy,
  Info,
  ExternalLink,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { MarkdownView } from './Markdown'
import type { DigestTaskRef } from '../types'

/** Today in the local timezone, as YYYY-MM-DD. */
const todayISO = (): string => {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

const priorityDot = (p: string): string => {
  switch (p) {
    case 'urgent': return 'bg-rose-500'
    case 'high': return 'bg-amber-500'
    case 'medium': return 'bg-blue-500'
    default: return 'bg-slate-500'
  }
}

/**
 * The flags a task carries in the digest: staleness, due-date pressure, an open
 * PR. A task whose tracker dates were unavailable shows no age at all rather
 * than a fabricated zero.
 */
const RefFlags: React.FC<{ r: DigestTaskRef }> = ({ r }) => {
  const flags: { label: string; cls: string }[] = []

  if (r.isStale) {
    flags.push({ label: `${r.ageDays}j ouvert`, cls: 'text-rose-300 bg-rose-500/10 border-rose-500/25' })
  }
  if (typeof r.daysToDue === 'number') {
    if (r.daysToDue < 0) {
      flags.push({ label: `retard ${-r.daysToDue}j`, cls: 'text-rose-300 bg-rose-500/10 border-rose-500/25' })
    } else if (r.daysToDue === 0) {
      flags.push({ label: "échéance aujourd'hui", cls: 'text-amber-300 bg-amber-500/10 border-amber-500/25' })
    } else {
      flags.push({ label: `dans ${r.daysToDue}j`, cls: 'text-blue-300 bg-blue-500/10 border-blue-500/25' })
    }
  }
  if (r.prUrl) {
    flags.push({ label: 'PR', cls: 'text-purple-300 bg-purple-500/10 border-purple-500/25' })
  }
  if (r.issueType) {
    flags.push({ label: r.issueType, cls: 'text-[var(--text-muted)] bg-[var(--bg-tertiary)] border-[var(--border-color)]' })
  }

  if (flags.length === 0) return null
  return (
    <>
      {flags.map(f => (
        <span key={f.label} className={`text-[9px] font-mono px-1.5 py-0.2 rounded-full border ${f.cls}`}>
          {f.label}
        </span>
      ))}
    </>
  )
}

const DigestSection: React.FC<{
  title: string
  icon: React.ReactNode
  refs: DigestTaskRef[]
  emptyText: string
  accent: string
}> = ({ title, icon, refs, emptyText, accent }) => {
  const [expanded, setExpanded] = useState(false)
  const cap = 8
  const shown = expanded ? refs : refs.slice(0, cap)

  return (
    <section className="rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] overflow-hidden">
      <header className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
        <div className="flex items-center gap-2">
          <span className={accent}>{icon}</span>
          <h3 className="text-xs font-bold text-[var(--text-primary)]">{title}</h3>
        </div>
        <span className="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded-full bg-[var(--bg-primary)] text-[var(--text-muted)] border border-[var(--border-color)]">
          {refs.length}
        </span>
      </header>

      <div className="p-3">
        {refs.length === 0 ? (
          <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">{emptyText}</p>
        ) : (
          <>
            <ul className="space-y-1.5">
              {shown.map(r => (
                <li key={r.key} className="flex items-start gap-2 text-xs">
                  <span className={`w-1.5 h-1.5 rounded-full shrink-0 mt-1.5 ${priorityDot(r.priority)}`} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5 flex-wrap">
                      {r.externalUrl ? (
                        <a
                          href={r.externalUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="font-mono font-bold text-[11px] text-[var(--accent-color)] hover:underline inline-flex items-center gap-0.5"
                        >
                          {r.key}
                          <ExternalLink size={9} className="opacity-70" />
                        </a>
                      ) : (
                        <span className="font-mono font-bold text-[11px] text-[var(--accent-color)]">{r.key}</span>
                      )}
                      <RefFlags r={r} />
                    </div>
                    <p className="text-[var(--text-secondary)] leading-snug">{r.title}</p>
                    {r.parentKey && (
                      <span className="text-[10px] text-violet-300/80 inline-flex items-center gap-1 mt-0.5">
                        <Layers size={9} />
                        <span className="font-mono">{r.parentKey}</span>
                        {r.parentTitle && <span className="opacity-80 truncate">{r.parentTitle}</span>}
                      </span>
                    )}
                  </div>
                </li>
              ))}
            </ul>
            {refs.length > cap && (
              <button
                type="button"
                onClick={() => setExpanded(v => !v)}
                className="mt-2 text-[10px] font-semibold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {expanded ? 'Réduire' : `Afficher les ${refs.length - cap} autres`}
              </button>
            )}
          </>
        )}
      </div>
    </section>
  )
}

export const DigestView: React.FC = () => {
  const {
    dailyDigest,
    isDigestLoading,
    isDigestEnriching,
    fetchDailyDigest,
    generateDailyDigest,
    currentProject,
    isDigestAvailable,
    addToast,
    t,
  } = useApp()

  const [date, setDate] = useState<string>(todayISO())
  const [assignee, setAssignee] = useState<string>('')

  useEffect(() => {
    if (!isDigestAvailable) return
    void fetchDailyDigest(date, assignee)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [date, assignee, currentProject?.id, isDigestAvailable])

  // The assignee list comes from the digest itself, so it always uses the
  // tracker's own spelling of each name.
  const assigneeOptions = dailyDigest?.assignees || []

  const stats = dailyDigest?.stats
  const statTiles = useMemo(() => {
    if (!stats) return []
    return [
      { label: 'Ouvertes', value: stats.totalOpen, cls: 'text-[var(--text-primary)]' },
      { label: 'Urgentes', value: stats.urgent, cls: 'text-rose-400' },
      { label: 'Hautes', value: stats.high, cls: 'text-amber-400' },
      { label: 'Stale', value: stats.stale, cls: 'text-rose-300' },
      { label: 'En retard', value: stats.overdue, cls: 'text-rose-400' },
      { label: 'En revue', value: stats.awaitingReview, cls: 'text-purple-300' },
      { label: 'Terminées 7j', value: stats.doneLast7Days, cls: 'text-emerald-400' },
    ]
  }, [stats])

  if (!currentProject) {
    return (
      <div className="p-6 text-xs text-[var(--text-muted)]">
        Sélectionnez un projet dans la barre latérale : un digest quotidien est toujours propre à un projet.
      </div>
    )
  }

  if (!isDigestAvailable) {
    return (
      <div className="p-6 text-xs text-[var(--text-muted)] space-y-1.5">
        <p className="text-[var(--text-primary)] font-semibold">
          Le digest quotidien est réservé aux projets personnels.
        </p>
        <p>
          « {currentProject.name} » est un projet de delivery. Passez-le en type « Projet personnel »
          dans ses paramètres pour activer son digest.
        </p>
      </div>
    )
  }

  return (
    <div className="p-4 sm:p-6 space-y-4 overflow-y-auto h-full">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-10 h-10 rounded-xl bg-[var(--accent-light)] accent-text border border-[var(--accent-color)]/30 flex items-center justify-center shrink-0">
            <CalendarDays size={18} />
          </div>
          <div className="min-w-0">
            <h2 className="text-sm font-bold text-[var(--text-primary)] truncate">
              Daily Digest — {currentProject.name}
            </h2>
            <p className="text-[11px] text-[var(--text-muted)]">
              {dailyDigest
                ? `Généré le ${new Date(dailyDigest.generatedAt).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
                : 'Chargement…'}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <input
            type="date"
            value={date}
            onChange={e => setDate(e.target.value)}
            className="px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
          />
          <select
            value={assignee}
            onChange={e => setAssignee(e.target.value)}
            className="px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] max-w-[190px]"
            title="Restreindre le digest à une personne : sur un backlog d'équipe, un digest non filtré n'est plus un brief"
          >
            <option value="">Tout le projet</option>
            {assigneeOptions.map(a => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => void generateDailyDigest({ date, assignee })}
            disabled={isDigestLoading || isDigestEnriching}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-50 cursor-pointer"
            title="Recalculer les sections issues des tâches et enregistrer le digest"
          >
            {isDigestLoading ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
            <span>Recalculer</span>
          </button>
          <button
            type="button"
            onClick={() => void generateDailyDigest({ date, assignee, enrich: true })}
            disabled={isDigestEnriching || isDigestLoading}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 active:scale-95 disabled:opacity-50 cursor-pointer"
            title="Demander le brief du jour à l'agent du projet : réunions, annonces, tout ce que Sectile ne voit pas dans les tickets"
          >
            {isDigestEnriching ? <Loader2 size={13} className="animate-spin" /> : <Sparkles size={13} />}
            <span>{isDigestEnriching ? t.nav.dailyBriefRunning : t.nav.dailyBrief}</span>
          </button>
          <button
            type="button"
            onClick={() => {
              if (!dailyDigest?.markdown) return
              navigator.clipboard?.writeText(dailyDigest.markdown)
              addToast({ type: 'success', title: 'Digest copié', description: 'Markdown complet dans le presse-papier.' })
            }}
            disabled={!dailyDigest?.markdown}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-50 cursor-pointer"
            title="Copier le digest en Markdown"
          >
            <Copy size={13} />
            <span>Copier</span>
          </button>
        </div>
      </div>

      {/* Stat tiles */}
      {statTiles.length > 0 && (
        <div className="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-7 gap-2">
          {statTiles.map(t => (
            <div key={t.label} className="rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] px-3 py-2">
              <div className={`text-lg font-bold leading-none ${t.cls}`}>{t.value}</div>
              <div className="text-[10px] text-[var(--text-muted)] mt-0.5">{t.label}</div>
            </div>
          ))}
        </div>
      )}

      {/* An unfiltered team backlog is not a brief: say so rather than showing 300 lines. */}
      {dailyDigest && !dailyDigest.assignee && dailyDigest.focus.length > 40 && (
        <div className="flex items-start gap-2 p-3 rounded-xl bg-amber-500/10 border border-amber-500/25 text-[11px] text-amber-200 leading-relaxed">
          <Info size={14} className="shrink-0 mt-0.5 text-amber-400" />
          <span>
            {dailyDigest.focus.length} tâches sont en priorité haute ou urgente sur l'ensemble du projet.
            Sur un backlog d'équipe, la priorité par défaut du tracker (« Major » chez Jira) se mappe sur
            « haute », ce qui rend le digest illisible. Choisissez une personne dans le sélecteur pour
            obtenir un vrai brief quotidien.
          </span>
        </div>
      )}

      {/* Agenda — the only AI-produced section */}
      <section className="rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] overflow-hidden">
        <header className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <div className="flex items-center gap-2">
            <CalendarClock size={15} className="text-cyan-400" />
            <h3 className="text-xs font-bold text-[var(--text-primary)]">Agenda du jour</h3>
          </div>
          {dailyDigest && (
            <span className={`text-[9px] font-mono font-bold px-1.5 py-0.5 rounded-full border ${
              dailyDigest.aiStatus === 'completed'
                ? 'text-emerald-300 bg-emerald-500/10 border-emerald-500/30'
                : dailyDigest.aiStatus === 'failed'
                ? 'text-rose-300 bg-rose-500/10 border-rose-500/30'
                : 'text-[var(--text-muted)] bg-[var(--bg-primary)] border-[var(--border-color)]'
            }`}>
              {dailyDigest.aiStatus === 'completed' ? 'agent OK' : dailyDigest.aiStatus === 'failed' ? 'agent en échec' : 'non récupéré'}
            </span>
          )}
        </header>
        <div className="p-4">
          {dailyDigest?.agenda ? (
            <MarkdownView compact>{dailyDigest.agenda}</MarkdownView>
          ) : (
            <div className="flex items-start gap-2 text-[11px] text-[var(--text-muted)] leading-relaxed">
              <Info size={13} className="shrink-0 mt-0.5 text-amber-400" />
              <span>
                {dailyDigest?.aiStatus === 'failed'
                  ? `Agenda indisponible : ${dailyDigest.aiError}`
                  : `Sectile ne voit pas votre calendrier. Lancez « ${t.nav.dailyBrief} » pour que l'agent du projet remonte les réunions du jour avec ses propres connecteurs.`}
              </span>
            </div>
          )}
        </div>
      </section>

      {/* Task-derived sections */}
      {dailyDigest && (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          <DigestSection
            title="À traiter aujourd'hui"
            icon={<Flame size={15} />}
            accent="text-rose-400"
            refs={dailyDigest.focus}
            emptyText="Rien d'urgent ou de haute priorité en attente."
          />
          <DigestSection
            title="Trop longtemps ouvertes"
            icon={<Clock size={15} />}
            accent="text-amber-400"
            refs={dailyDigest.stale}
            emptyText={
              dailyDigest.stats.openDateUnknown > 0
                ? `Ancienneté indisponible pour ${dailyDigest.stats.openDateUnknown} tâches ouvertes : la CLI Atlassian n'expose pas les champs created / updated, la synchro ne connaît donc pas la date réelle du ticket.`
                : 'Aucune tâche prioritaire ouverte depuis plus de 7 jours.'
            }
          />
          <DigestSection
            title="Échéances"
            icon={<CalendarClock size={15} />}
            accent="text-blue-400"
            refs={dailyDigest.dueSoon}
            emptyText="Aucune échéance dans les 7 prochains jours."
          />
          <DigestSection
            title="En attente de revue"
            icon={<Eye size={15} />}
            accent="text-purple-400"
            refs={dailyDigest.awaitingReview}
            emptyText="Rien en attente de revue."
          />
          <DigestSection
            title="À ne pas oublier cette semaine"
            icon={<ListChecks size={15} />}
            accent="text-indigo-400"
            refs={dailyDigest.watch}
            emptyText="Rien d'autre en cours."
          />
          <DigestSection
            title="Terminées récemment"
            icon={<CheckCircle2 size={15} />}
            accent="text-emerald-400"
            refs={dailyDigest.recentlyDone}
            emptyText={
              dailyDigest.stats.closedDateUnknown > 0
                ? `${dailyDigest.stats.closedDateUnknown} tâches terminées, mais la date de clôture est indisponible (champ updated non exposé par la CLI Atlassian) : aucune fenêtre de 7 jours ne peut être calculée.`
                : 'Aucune tâche terminée sur les 7 derniers jours.'
            }
          />
        </div>
      )}

      {/* Load per macro */}
      {dailyDigest && ((dailyDigest.byMacro && dailyDigest.byMacro.length > 0) || (dailyDigest.byEpic && dailyDigest.byEpic.length > 0)) && (
        <section className="rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] overflow-hidden">
          <header className="flex items-center gap-2 px-4 py-2.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
            <Layers size={15} className="text-violet-400" />
            <h3 className="text-xs font-bold text-[var(--text-primary)]">Charge par macro</h3>
          </header>
          <div className="p-3 overflow-x-auto">
            <table className="w-full text-xs min-w-[420px]">
              <thead>
                <tr className="text-[10px] uppercase tracking-wider text-[var(--text-muted)]">
                  <th className="text-left font-bold py-1 px-2">Macro</th>
                  <th className="text-left font-bold py-1 px-2">Titre</th>
                  <th className="text-right font-bold py-1 px-2">Ouvertes</th>
                  <th className="text-right font-bold py-1 px-2">Terminées</th>
                </tr>
              </thead>
              <tbody>
                {(dailyDigest.byMacro || dailyDigest.byEpic || []).slice(0, 15).map(g => (
                  <tr key={g.parentKey} className="border-t border-[var(--border-color)]">
                    <td className="py-1.5 px-2 font-mono font-bold text-violet-300">{g.parentKey}</td>
                    <td className="py-1.5 px-2 text-[var(--text-secondary)] truncate max-w-[280px]">{g.parentTitle || '—'}</td>
                    <td className="py-1.5 px-2 text-right font-mono text-[var(--text-primary)]">{g.openCount}</td>
                    <td className="py-1.5 px-2 text-right font-mono text-[var(--text-muted)]">{g.doneCount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  )
}
