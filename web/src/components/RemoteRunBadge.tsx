import { useState } from 'react'
import { Loader2, Clock, CircleSlash, CircleStop } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { deriveRunIndicator, type RunIndicatorState } from '../lib/remoteRunIndicator'

// The indicator is a bare glyph: a filled box would read as an action button
// competing with the card's own controls, and the state already carries in the
// colour. Theme variables keep that colour in step with the rest of the UI.
const PRESENTATION: Record<RunIndicatorState, { label: string; color: string }> = {
  running: { label: 'Remote execution running', color: 'var(--run-active)' },
  queued: { label: 'Remote execution queued', color: 'var(--status-warn)' },
  canceled: { label: 'Remote execution canceled', color: 'var(--text-muted)' },
}

export function RemoteRunBadge({ taskId }: { taskId: string }) {
  const { activities, fetchActivities, addToast } = useApp()
  const [canceling, setCanceling] = useState(false)
  const [hovered, setHovered] = useState(false)
  const [unreachable, setUnreachable] = useState(false)

  const indicator = deriveRunIndicator(activities, taskId)
  if (!indicator) return null

  const { state, runs, cancelableRunIds, count } = indicator
  const presentation = PRESENTATION[state]
  const skills = runs.map(run => run.skillName).join(', ')
  const stateLabel = presentation.label + (count > 1 ? ` (${count})` : '') + (skills ? ` (${skills})` : '')

  async function cancelRuns(runIds: string[], force = false) {
    setCanceling(true)
    try {
      for (const runId of runIds) {
        const response = await fetch('/api/tasks/' + encodeURIComponent(taskId) + '/cancel-run', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ runId, force }),
        })
        if (!response.ok) {
          const error = await response.json()
          throw new Error(error.error || 'Could not confirm execution stopped')
        }
      }
      await fetchActivities()
      setUnreachable(false)
    } catch (error) {
      // An unreachable agent leaves the run open for good unless it can be
      // closed on purpose, so offer that rather than repeating the failure.
      if (!force) setUnreachable(true)
      addToast({ type: 'error', title: 'Cancellation failed', description: error instanceof Error ? error.message : String(error) })
    } finally { setCanceling(false) }
  }

  const shape = 'inline-flex shrink-0 items-center justify-center rounded p-0.5 bg-transparent border-0'
  const interactive = ' cursor-pointer hover:brightness-125 disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]'
  const tint = { color: presentation.color }
  // The stop glyph replaces the state glyph only while the control is targeted.
  const showStop = cancelableRunIds.length > 0 && hovered && !canceling
  const StateIcon = state === 'running' ? Loader2 : state === 'queued' ? Clock : CircleSlash
  const glyph = canceling
    ? <Loader2 size={12} className="animate-spin" aria-hidden="true" />
    : showStop
      ? <CircleStop size={12} aria-hidden="true" />
      : <StateIcon size={12} className={state === 'running' ? 'animate-spin' : undefined} aria-hidden="true" />

  if (cancelableRunIds.length === 0) {
    return <span role="status" title={stateLabel} aria-label={stateLabel} className={shape} style={tint}>{glyph}</span>
  }

  const actionLabel = canceling ? 'Stopping ' + skills : 'Stop ' + skills
  if (unreachable) {
    return (
      <button type="button" disabled={canceling}
        title="The agent could not be reached. Close this run without stopping any local process."
        aria-label={'Force close ' + skills}
        onClick={event => { event.stopPropagation(); void cancelRuns(cancelableRunIds, true) }}
        className={shape + interactive} style={tint}>
        <CircleSlash size={12} aria-hidden="true" />
      </button>
    )
  }
  return (
    <button type="button" disabled={canceling} title={showStop || canceling ? actionLabel : stateLabel} aria-label={actionLabel}
      onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
      onFocus={() => setHovered(true)} onBlur={() => setHovered(false)}
      onClick={event => { event.stopPropagation(); void cancelRuns(cancelableRunIds) }}
      className={shape + interactive} style={tint}>
      {glyph}
    </button>
  )
}
