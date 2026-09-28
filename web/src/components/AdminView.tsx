import React from 'react'
import { Activity, RefreshCw, Shield, Users, UserCheck } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { useAdminStats } from '../hooks/useAdminStats'
import { fill, RUN_STATUSES, windowMinutes } from '../lib/adminStats'
import { formatTime } from '../lib/i18n'
import { UsersPanel } from './UsersPanel'
import { ServerTrackerCredentialsPanel } from './ServerTrackerCredentialsPanel'

interface StatCardProps {
  icon: React.ReactNode
  label: string
  value: number | string
  detail?: React.ReactNode
}

function StatCard({ icon, label, value, detail }: StatCardProps) {
  return (
    <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4">
      <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
        {icon}
        <span>{label}</span>
      </div>
      <div className="mt-2 text-3xl font-bold tabular-nums text-[var(--text-primary)]">{value}</div>
      {detail && <div className="mt-1 text-[11px] text-[var(--text-muted)]">{detail}</div>}
    </div>
  )
}

/**
 * The admin page: what the board is doing right now, the roster, then the
 * credentials the server reaches its trackers with. It
 * replaced a modal that only held the roster, because watching the board is
 * something one keeps open, not something one opens and closes.
 *
 * Every figure comes from the database, so on a deployment with several
 * replicas the page reads the same whichever one answers.
 */
export const AdminView: React.FC = () => {
  const { t, settings } = useApp()
  const { user: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const { stats, error, isLoading, revision, refresh } = useAdminStats(isAdmin)

  if (!isAdmin || !currentUser) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-center px-6">
        <Shield size={28} className="text-[var(--text-muted)]" />
        <p className="text-xs text-[var(--text-secondary)]">{t.admin.adminOnly}</p>
      </div>
    )
  }

  const placeholder = isLoading ? '…' : '-'
  const runLabels: Record<(typeof RUN_STATUSES)[number], string> = {
    running: t.admin.running, queued: t.admin.queued, pending: t.admin.pending,
  }

  return (
    <div className="flex flex-col h-full overflow-hidden" data-admin-view>
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5 border-b border-[var(--border-color)] shrink-0">
        <div className="w-7 h-7 rounded-xl bg-amber-500/20 text-amber-500 flex items-center justify-center">
          <Shield size={14} />
        </div>
        <div className="min-w-0">
          <h2 className="text-sm font-bold text-[var(--text-primary)]">{t.admin.title}</h2>
          <p className="text-[11px] text-[var(--text-muted)]">{t.admin.subtitle}</p>
        </div>
        <div className="ml-auto flex items-center gap-2 text-[11px] text-[var(--text-muted)]">
          {error
            ? <span className="text-amber-400">{t.admin.statsUnavailable} ({error})</span>
            : stats && <span>{t.admin.updatedAt} {formatTime(settings.language, stats.generatedAt, { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>}
          <button
            type="button"
            onClick={() => void refresh()}
            className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer"
          >
            <RefreshCw size={12} className={isLoading ? 'animate-spin' : ''} />
            <span>{t.admin.refresh}</span>
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-6 text-xs">
        <section className="grid grid-cols-1 gap-3 sm:grid-cols-3" aria-label={t.admin.title}>
          <StatCard
            icon={<UserCheck size={13} className="text-emerald-400" />}
            label={t.admin.activeUsers}
            value={stats?.users.active ?? placeholder}
            detail={fill(t.admin.activeUsersHint, { minutes: windowMinutes(stats) })}
          />
          <StatCard
            icon={<Activity size={13} className="text-cyan-400" />}
            label={t.admin.activeRuns}
            value={stats?.runs.active ?? placeholder}
            detail={stats && RUN_STATUSES
              .map(status => `${stats.runs.byStatus[status] ?? 0} ${runLabels[status]}`)
              .join(' · ')}
          />
          <StatCard
            icon={<Users size={13} className="text-violet-400" />}
            label={t.admin.totalUsers}
            value={stats?.users.total ?? placeholder}
            detail={stats && `${stats.users.admins} ${t.admin.admins} · ${stats.users.blocked} ${t.admin.blocked}`}
          />
        </section>

        <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="admin-users-title">
          <h3 id="admin-users-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
            <Users size={14} /> {t.admin.users}
          </h3>
          {/* The roster reloads with the figures, so a person who just signed
              in shows as online at the same moment the count goes up. */}
          <UsersPanel currentUserId={currentUser.userId} embedded reloadKey={revision} onChange={refresh} />
        </section>

        <ServerTrackerCredentialsPanel />
      </div>
    </div>
  )
}
