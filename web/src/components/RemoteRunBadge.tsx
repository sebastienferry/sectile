import { useState } from 'react'
import { Loader2, Clock } from 'lucide-react'
import { useApp } from '../context/AppContext'

export function RemoteRunBadge({ taskId }: { taskId: string }) {
  const { activities, fetchActivities, addToast } = useApp()
  const [canceling, setCanceling] = useState<string | null>(null)
  async function cancelRun(runId: string) {
    setCanceling(runId)
    try {
      const response = await fetch('/api/tasks/' + encodeURIComponent(taskId) + '/cancel-run', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ runId }),
      })
      if (!response.ok) {
        const error = await response.json()
        throw new Error(error.error || 'Could not confirm execution stopped')
      }
      await fetchActivities()
    } catch (error) {
      addToast({ type: 'error', title: 'Cancellation failed', description: error instanceof Error ? error.message : String(error) })
    } finally { setCanceling(null) }
  }
  const runs = activities.filter(activity => activity.taskId === taskId && activity.skillId === 'remote_run' && (activity.status === 'running' || activity.status === 'queued'))
  if (runs.length === 0) return null

  const runningRuns = runs.filter(r => r.status === 'running')
  const queuedRuns = runs.filter(r => r.status === 'queued')

  return (
    <span className="inline-flex shrink-0 items-center gap-1">
      {runningRuns.length > 0 && (
        <span role="status" title={runningRuns.map(run => run.skillName).join(', ')}
          className="inline-flex shrink-0 items-center gap-1 rounded border border-cyan-500/30 bg-cyan-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-cyan-400">
          <Loader2 size={10} className="animate-spin" aria-hidden="true" />
          Remote execution{runningRuns.length > 1 ? ` (${runningRuns.length})` : ''}
          {runningRuns.filter(run => run.action === 'Agent-owned remote execution').map(run => (
            <button key={run.id} type="button" disabled={canceling !== null}
              onClick={event => { event.stopPropagation(); void cancelRun(run.id) }}
              title={'Stop ' + run.skillName}
              className="ml-1 rounded border border-cyan-500/30 px-1 hover:bg-cyan-500/20 disabled:opacity-50">
              {canceling === run.id ? 'Stopping…' : 'Stop'}
            </button>
          ))}
        </span>
      )}
      {queuedRuns.length > 0 && (
        <span role="status" title={queuedRuns.map(run => run.skillName).join(', ')}
          className="inline-flex shrink-0 items-center gap-1 rounded border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-semibold text-amber-400">
          <Clock size={10} aria-hidden="true" />
          Queued{queuedRuns.length > 1 ? ` (${queuedRuns.length})` : ''}
          {queuedRuns.filter(run => run.action === 'Agent-owned remote execution').map(run => (
            <button key={run.id} type="button" disabled={canceling !== null}
              onClick={event => { event.stopPropagation(); void cancelRun(run.id) }}
              title={'Cancel ' + run.skillName}
              className="ml-1 rounded border border-amber-500/30 px-1 hover:bg-amber-500/20 disabled:opacity-50">
              {canceling === run.id ? 'Stopping…' : 'Stop'}
            </button>
          ))}
        </span>
      )}
    </span>
  )
}

