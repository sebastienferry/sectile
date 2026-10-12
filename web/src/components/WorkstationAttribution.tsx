import type { TaskActivity } from '../types'
import { useAgentStatus } from '../hooks/useAgentStatus'
import { useViewer } from '../hooks/useViewer'
import { useApp } from '../context/AppContext'

export function WorkstationAttribution({ run }: { run: TaskActivity }) {
  const { agents, isLoading, error } = useAgentStatus()
  const { t } = useApp()
  const viewer = useViewer()
  if (!run.deviceId) return null
  const unavailable = run.userId === viewer?.userId && ['queued', 'pending', 'running'].includes(run.status) && !isLoading && !error &&
    !agents.some(agent => agent.deviceId === run.deviceId && agent.userId === run.userId)
  return <span className="max-w-40 truncate text-xs text-[var(--text-secondary)]" data-workstation-attribution={run.id}>
    {run.deviceLabel || run.deviceId}{unavailable ? ` · ${t.workstationPicker.unavailable}` : ''}
  </span>
}
