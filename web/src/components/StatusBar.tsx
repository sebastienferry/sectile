import { useApp } from '../context/AppContext'

export function StatusBar() {
  const { currentProject, activities, setActiveView, setIsProfileOpen, settings } = useApp()
  const running = activities.filter(activity => activity.status === 'running').length
  return <footer className="flex items-center justify-between border-t border-[var(--border-color)] bg-[var(--bg-secondary)] px-4 py-2 text-xs text-[var(--text-muted)]">
    <span>{currentProject?.name || 'All projects'}</span>
    <button type="button" onClick={() => setActiveView('activities')}>{running} active executions</button>
    <button type="button" onClick={() => setIsProfileOpen(true)}>{settings.userName || 'Profile'}</button>
  </footer>
}
