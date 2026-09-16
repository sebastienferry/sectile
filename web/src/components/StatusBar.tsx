import { useApp } from '../context/AppContext'
import { McpSessions } from './McpSessions'

export function StatusBar() {
  const { currentProject, activities, setActiveView, setIsProfileOpen, settings } = useApp()
  const running = activities.filter(activity => activity.status === 'running').length
  const queued = activities.filter(activity => activity.status === 'queued' || activity.status === 'pending').length
  const total = running + queued
  const statusLabel =
    total === 0
      ? '0 active executions'
      : queued > 0 && running > 0
      ? `${total} active executions (${running} running, ${queued} queued)`
      : queued > 0
      ? `${queued} queued execution${queued > 1 ? 's' : ''}`
      : `${running} active execution${running > 1 ? 's' : ''}`

  return <footer className="flex items-center justify-between border-t border-[var(--border-color)] bg-[var(--bg-secondary)] px-4 py-2 text-xs text-[var(--text-muted)]">
    <span>{currentProject?.name || 'All projects'}</span>
    <div className="flex items-center gap-4">
      <McpSessions />
      <button type="button" onClick={() => setActiveView('activities')}>{statusLabel}</button>
    </div>
    <button type="button" onClick={() => setIsProfileOpen(true)}>{settings.userName || 'Profile'}</button>
  </footer>
}
