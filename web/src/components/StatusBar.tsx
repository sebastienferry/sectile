import { useState } from 'react'
import { useApp } from '../context/AppContext'
import { McpSessions } from './McpSessions'
import { ChangelogModal } from './ChangelogModal'
import { useVersion } from '../hooks/useVersion'
import { DisplayScaleMenu } from './DisplayScaleMenu'

export function StatusBar() {
  const { currentProject, activities, setActiveView, setIsProfileOpen, settings } = useApp()
  const { version } = useVersion()
  const [isChangelogOpen, setIsChangelogOpen] = useState(false)
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

  return <>
    <footer className="flex items-center justify-between border-t border-[var(--border-color)] bg-[var(--bg-secondary)] px-4 py-2 text-xs text-[var(--text-muted)]">
      <span>{currentProject?.name || 'All projects'}</span>
      <div className="flex items-center gap-4">
        <McpSessions />
        {/* Le zoom et la densité, à portée du regard qui trouve l'écran trop
            petit plutôt qu'à quatre clics dans les réglages. */}
        <DisplayScaleMenu />
        <button type="button" onClick={() => setActiveView('activities')}>{statusLabel}</button>
        {/* The server's version, and the way into the release notes: knowing
            what you are running and knowing what changed are the same
            question, asked a second apart. */}
        {version && (
          <button
            type="button"
            onClick={() => setIsChangelogOpen(true)}
            title={`Sectile ${version.version}${version.commit ? ` · ${version.commit.slice(0, 12)}` : ''} — release notes`}
            className="font-mono hover:text-[var(--text-primary)] transition-colors cursor-pointer"
          >
            {version.version}
          </button>
        )}
      </div>
      <button type="button" onClick={() => setIsProfileOpen(true)}>{settings.userName}</button>
    </footer>
    <ChangelogModal open={isChangelogOpen} onClose={() => setIsChangelogOpen(false)} version={version} />
  </>
}
