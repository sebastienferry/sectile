import { useState } from 'react'
import { useApp } from '../context/AppContext'
import { useViewer } from '../hooks/useViewer'
import { mayCancelExecution } from '../lib/workstations'
import type { TaskActivity } from '../types'

/** One permission and recovery flow for task and activity surfaces. */
export function ExecutionRecovery({ run }: { run: TaskActivity }) {
  const { t, fetchActivities, fetchActivityStats, addToast } = useApp()
  const viewer = useViewer()
  const [busy, setBusy] = useState(false)
  const [unconfirmed, setUnconfirmed] = useState(false)
  if (!mayCancelExecution(run, viewer)) return null
  const cancel = async (force = false) => {
    if (force && !window.confirm(t.workstationPicker.forceConfirm)) return
    setBusy(true)
    try {
      const response = await fetch(`/api/activities/${encodeURIComponent(run.id)}/cancel`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ force }),
      })
      if (!response.ok) {
        if (response.status === 502 && run.action === 'Agent-owned remote execution') setUnconfirmed(true)
        throw new Error((await response.json()).error || t.shell.remoteRun.cancelUnconfirmed)
      }
      setUnconfirmed(false)
      await fetchActivities()
      await fetchActivityStats()
    } catch (error) {
      addToast({ type: 'error', title: t.shell.remoteRun.cancelFailed, description: error instanceof Error ? error.message : String(error) })
    } finally { setBusy(false) }
  }
  return <span className="inline-flex gap-2" onClick={event => event.stopPropagation()}>
    <button type="button" disabled={busy} onClick={() => void cancel()} className="text-xs text-rose-400">
      {t.activities.detail.cancel}
    </button>
    {unconfirmed && <button type="button" disabled={busy} onClick={() => void cancel(true)} className="text-xs text-rose-400">
      {t.workstationPicker.force}
    </button>}
  </span>
}
