import type { BatchMemberState, Task } from '../types'

/**
 * The tone a batch member takes on its card: the amber of a queued run while it
 * waits its turn, the indigo of a running run while the agent works on it, and
 * none once it is done.
 */
export type BatchTone = 'amber' | 'indigo' | null

export interface BatchIndicator {
  leadKey: string
  position: number
  size: number
  state: BatchMemberState
  tone: BatchTone
  /** The key of the state label in the batchMember locale group, null for none. */
  labelKey: 'waiting' | 'processing' | null
}

const TONES: Record<BatchMemberState, BatchTone> = { waiting: 'amber', processing: 'indigo', done: null }
const LABELS: Record<BatchMemberState, BatchIndicator['labelKey']> = { waiting: 'waiting', processing: 'processing', done: null }

/**
 * What a ticket shows of the running batch it belongs to (#522), or null when it
 * is in none. The card, the list row and the detail panel all read it, so the
 * three cannot disagree. An unknown state is shown as done: the badge alone.
 */
export function batchIndicator(task: Pick<Task, 'batch'> | null | undefined): BatchIndicator | null {
  const batch = task?.batch
  if (!batch || !batch.leadKey) return null
  const state: BatchMemberState = batch.state in TONES ? batch.state : 'done'
  return {
    leadKey: batch.leadKey,
    position: batch.position,
    size: batch.size,
    state,
    tone: TONES[state],
    labelKey: LABELS[state],
  }
}

/** Fills the {key}, {position} and {size} placeholders of a batch label. */
export function formatBatchLabel(template: string, indicator: Pick<BatchIndicator, 'leadKey' | 'position' | 'size'>): string {
  return template
    .replace('{key}', indicator.leadKey)
    .replace('{position}', String(indicator.position))
    .replace('{size}', String(indicator.size))
}
