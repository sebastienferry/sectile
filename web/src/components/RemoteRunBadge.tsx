import { useState } from 'react'
import { CircleStop } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { deriveRunIndicator, type RunIndicatorState } from '../lib/remoteRunIndicator'
import { RunStateGlyph } from './RunStateGlyph'
import { runState } from '../../../shared/runStates'

// The indicator is a bare glyph: a filled box would read as an action button
// competing with the card's own controls, and the state already carries in the
// colour. The glyph and the colour come from the shared run-state definition,
// which is also what the desktop puts on its notification, so the two cannot
// drift apart.
const LABELS: Record<RunIndicatorState, string> = {
  waiting: 'Remote execution waiting for you',
  running: 'Remote execution running',
  queued: 'Remote execution queued',
  canceled: 'Remote execution canceled',
}

/** The state glyph, at badge size, spinning or pulsing while the state lasts. */
function StateGlyph({ state, spin }: { state: RunIndicatorState; spin: boolean }) {
  return (
    <RunStateGlyph state={state} size={12}
      className={spin ? 'animate-spin' : state === 'waiting' ? 'animate-pulse' : undefined} />
  )
}

/**
 * Renders how long the run has been waiting, in the coarsest unit that still
 * says something. An absent or unparseable timestamp reports nothing at all
 * rather than a wait of zero, which would read as a fresh prompt.
 */
function formatWaited(since?: string): string {
  if (!since) return ''
  const started = Date.parse(since)
  if (Number.isNaN(started)) return ''
  const seconds = Math.max(0, Math.round((Date.now() - started) / 1000))
  if (seconds < 60) return seconds + 's'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return minutes + ' min'
  const hours = Math.floor(minutes / 60)
  return hours + ' h ' + (minutes % 60) + ' min'
}

export function RemoteRunBadge({ taskId }: { taskId: string }) {
  const { activities, fetchActivities, addToast } = useApp()
  const [canceling, setCanceling] = useState(false)
  const [hovered, setHovered] = useState(false)
  const [unreachable, setUnreachable] = useState(false)

  const indicator = deriveRunIndicator(activities, taskId)
  if (!indicator) return null

  const { state, runs, cancelableRunIds, count, waitingSince } = indicator
  const skills = runs.map(run => run.skillName).join(', ')
  const waited = state === 'waiting' ? formatWaited(waitingSince) : ''
  const stateLabel = LABELS[state] + (waited ? ` for ${waited}` : '')
    + (count > 1 ? ` (${count})` : '') + (skills ? ` (${skills})` : '')

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
  const tint = { color: runState(state)?.color }
  // The stop glyph replaces the state glyph only while the control is targeted.
  const showStop = cancelableRunIds.length > 0 && hovered && !canceling
  const glyph = canceling
    ? <StateGlyph state="running" spin />
    : showStop
      ? <CircleStop size={12} aria-hidden="true" />
      : <StateGlyph state={state} spin={state === 'running'} />

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
        <StateGlyph state="canceled" spin={false} />
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
