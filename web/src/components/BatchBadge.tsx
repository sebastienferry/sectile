import { ListOrdered } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { batchIndicator, formatBatchLabel } from '../lib/batchMembership'
import type { Task } from '../types'

// The label takes the colours the card already gives a queued run and a running
// one, so a ticket waiting its turn in a batch reads like a queued ticket, and
// the one the agent is on like a running ticket.
const LABEL_TONES = {
  amber: 'text-amber-400 bg-amber-500/10 border-amber-500/30',
  indigo: 'text-indigo-300 bg-indigo-500/10 border-indigo-500/30',
} as const

/**
 * The batch a ticket belongs to while that batch runs (#522): a badge naming its
 * lead, and next to it where the ticket stands. Renders nothing outside a
 * running batch.
 */
export function BatchBadge({ task }: { task: Pick<Task, 'batch'> }) {
  const { t } = useApp()
  const indicator = batchIndicator(task)
  if (!indicator) return null
  const tooltip = formatBatchLabel(t.batchMember.tooltip, indicator)
  return (
    <span className="inline-flex items-center gap-1 shrink-0 min-w-0">
      <span
        className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded border border-[var(--border-color)] text-[9px] font-mono font-bold text-[var(--text-secondary)] whitespace-nowrap"
        title={tooltip}
      >
        <ListOrdered size={9} aria-hidden="true" />
        {formatBatchLabel(t.batchMember.badge, indicator)}
      </span>
      {indicator.labelKey && indicator.tone && (
        <span
          className={`px-1.5 py-0.5 rounded border text-[9px] font-medium whitespace-nowrap ${LABEL_TONES[indicator.tone]}`}
        >
          {t.batchMember[indicator.labelKey]}
        </span>
      )}
    </span>
  )
}
