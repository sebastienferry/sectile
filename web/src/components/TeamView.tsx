import React, { useEffect, useMemo, useState } from 'react'
import { RefreshCw, Users, CircleSlash, ExternalLink } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format, formatDate, formatTime, parseDateInput, plural } from '../lib/i18n'
import { Avatar } from './Avatar'
import type { Task, TeamMemberLoad, TeamWorkload } from '../types'

/**
 * Charge d'une équipe, personne par personne. L'équipe vient du champ Team du
 * tracker, porté par les tickets : elle est facultative, donc un projet qui ne
 * l'utilise pas voit un écran qui le dit plutôt qu'un tableau vide.
 *
 * Les membres sans aucun ticket restent affichés : une colonne vide est le sujet
 * de la vue, pas un trou dedans. Les tickets assignés à quelqu'un qui n'est pas
 * dans l'équipe sont montrés à part, parce que c'est justement ce qu'on cherche
 * quand la charge ne tombe pas juste.
 */
export const TeamView: React.FC = () => {
  const {
    teams,
    teamFilter,
    setTeamFilter,
    fetchTeamWorkload,
    refreshTeamMembers,
    setSelectedTask,
    activeJobCount,
    currentProject,
    t,
    settings,
  } = useApp()
  const strings = t.planning.team
  const language = settings.language

  const [selectedTeam, setSelectedTeam] = useState<string>('')
  const [workload, setWorkload] = useState<TeamWorkload | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)

  // L'équipe affichée suit le filtre global quand il y en a un : passer du board
  // à cette vue ne doit pas perdre le contexte de travail.
  useEffect(() => {
    const fallback = teamFilter || teams[0]?.name || ''
    setSelectedTeam(prev => (prev && teams.some(t => t.name === prev) ? prev : fallback))
  }, [teams, teamFilter])

  useEffect(() => {
    if (!selectedTeam) {
      setWorkload(null)
      return
    }
    let alive = true
    setIsLoading(true)
    fetchTeamWorkload(selectedTeam).then(data => {
      if (!alive) return
      setWorkload(data)
      setIsLoading(false)
    })
    return () => {
      alive = false
    }
    // activeJobCount : une assignation part dans la file, la charge n'est à jour
    // qu'une fois la file vidée.
  }, [selectedTeam, fetchTeamWorkload, activeJobCount])

  const team = teams.find(t => t.name === selectedTeam)

  const totals = useMemo(() => {
    if (!workload) return { assigned: 0, people: 0, unassigned: 0, outside: 0 }
    return {
      assigned: workload.members.reduce((sum, m) => sum + m.total, 0),
      people: workload.members.filter(m => m.total > 0).length,
      unassigned: workload.unassigned.length,
      outside: workload.outside.reduce((sum, m) => sum + m.total, 0),
    }
  }, [workload])

  const syncedLabel = (iso?: string): string => {
    const date = parseDateInput(iso)
    if (!date) return strings.membersNeverRead
    return format(strings.membersReadAt, {
      date: formatDate(language, date, { day: '2-digit', month: '2-digit', year: 'numeric' }),
      time: formatTime(language, date),
    })
  }

  if (teams.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6">
        <Users size={28} className="text-[var(--text-muted)]" />
        <p className="text-sm font-bold text-[var(--text-primary)]">{strings.emptyTitle}</p>
        <p className="text-xs text-[var(--text-secondary)] max-w-md leading-relaxed">{strings.emptyBody}</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      <div className="flex flex-wrap items-center gap-2 px-4 py-2.5 border-b border-[var(--border-color)] shrink-0">
        <Users size={14} className="text-violet-400" />
        <select
          value={selectedTeam}
          onChange={e => {
            setSelectedTeam(e.target.value)
            setTeamFilter(e.target.value || null)
          }}
          className="px-2 py-1.5 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none cursor-pointer max-w-[240px]"
        >
          {teams.map(t => (
            <option key={t.id || t.name} value={t.name}>
              {t.name} ({t.taskCount})
            </option>
          ))}
        </select>

        <span className="text-[10px] text-[var(--text-muted)]">
          {plural(language, team?.memberCount || 0, strings.people)} · {syncedLabel(team?.syncedAt)}
        </span>

        {team?.id && (
          <button
            type="button"
            disabled={isRefreshing}
            onClick={async () => {
              setIsRefreshing(true)
              await refreshTeamMembers(team.id)
              setIsRefreshing(false)
            }}
            className="ml-auto flex items-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] cursor-pointer disabled:opacity-50"
            title={strings.refreshTitle}
          >
            <RefreshCw size={12} className={isRefreshing ? 'animate-spin' : ''} />
            {strings.refresh}
          </button>
        )}
      </div>

      <div className="flex flex-wrap gap-3 px-4 py-2 text-[10px] text-[var(--text-secondary)] border-b border-[var(--border-color)] shrink-0">
        <span>
          {plural(language, totals.assigned, strings.assignedSummary, {
            people: plural(language, totals.people, strings.people),
          })}
        </span>
        <span>{plural(language, totals.unassigned, strings.unassignedCount)}</span>
        {totals.outside > 0 && <span>{plural(language, totals.outside, strings.outsideCount)}</span>}
        {currentProject && (
          <span className="text-[var(--text-muted)]">{format(strings.project, { name: currentProject.name })}</span>
        )}
      </div>

      <div className="flex-1 overflow-auto p-4">
        {isLoading && !workload ? (
          <p className="text-xs text-[var(--text-muted)]">{strings.loading}</p>
        ) : (
          <div className="grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(260px,1fr))]">
            {workload?.members.map(load => (
              <MemberColumn
                key={load.member.accountId || load.member.displayName}
                load={load}
                onOpen={setSelectedTask}
                emptyLabel={strings.noTicket}
              />
            ))}

            {workload?.outside.map(load => (
              <MemberColumn
                key={`outside-${load.member.displayName}`}
                load={load}
                onOpen={setSelectedTask}
                badge={strings.outsideBadge}
                emptyLabel={strings.noTicket}
              />
            ))}

            {workload && workload.unassigned.length > 0 && (
              <MemberColumn
                load={{
                  member: { teamId: '', accountId: '', displayName: strings.unassigned, active: true },
                  tasks: workload.unassigned,
                  byStatus: {},
                  total: workload.unassigned.length,
                }}
                onOpen={setSelectedTask}
                emptyLabel={strings.noTicket}
                unassigned
              />
            )}
          </div>
        )}
      </div>
    </div>
  )
}

const MemberColumn: React.FC<{
  load: TeamMemberLoad
  onOpen: (task: Task) => void
  badge?: string
  /** What an empty column says, in the UI language. */
  emptyLabel: string
  unassigned?: boolean
}> = ({ load, onOpen, badge, emptyLabel, unassigned }) => (
  <div className="flex flex-col rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] overflow-hidden">
    <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--border-color)]">
      {unassigned ? (
        <CircleSlash size={13} className="text-[var(--text-muted)] shrink-0" />
      ) : (
        <Avatar name={load.member.displayName} url={load.member.avatarUrl} size={20} title={load.member.email || load.member.displayName} />
      )}
      <span className="text-xs font-bold text-[var(--text-primary)] truncate flex-1" title={load.member.email || undefined}>
        {load.member.displayName}
      </span>
      {badge && (
        <span className="text-[8.5px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wide text-amber-300 bg-amber-400/10 border border-amber-400/30 shrink-0">
          {badge}
        </span>
      )}
      <span className="text-[10px] font-mono font-bold text-[var(--text-secondary)] shrink-0">{load.total}</span>
    </div>

    {load.tasks.length === 0 ? (
      <p className="px-3 py-3 text-[10.5px] text-[var(--text-muted)]">{emptyLabel}</p>
    ) : (
      <div className="flex flex-col divide-y divide-[var(--border-color)] max-h-[320px] overflow-auto">
        {load.tasks.map(task => (
          <button
            key={task.id}
            type="button"
            onClick={() => onOpen(task)}
            className="flex items-start gap-2 px-3 py-2 text-left hover:bg-[var(--bg-tertiary)] cursor-pointer"
          >
            <span className="text-[9.5px] font-mono font-bold shrink-0 mt-0.5" style={{ color: 'var(--status-info)' }}>
              {task.key}
            </span>
            <span className="flex-1 min-w-0">
              <span className="block text-[11px] leading-snug text-[var(--text-secondary)] truncate">{task.title}</span>
              {task.trackerStatus && (
                <span className="block text-[9px] mt-0.5 text-[var(--text-muted)]">{task.trackerStatus}</span>
              )}
            </span>
            <ExternalLink size={10} className="text-[var(--text-muted)] shrink-0 mt-0.5" />
          </button>
        ))}
      </div>
    )}
  </div>
)
