import React from 'react'
import { RotateCcw } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { TrackerColumn, WorkflowStage } from '../types'
import { cleanStageMapping, toggleStageColumn, type StageMapping } from '../lib/stageMapping'

/**
 * A project's mapping of the workflow stages onto one tracker's columns (#741),
 * in the project settings, for a member as for an admin. The columns are the
 * tracker's, imported by an admin; only the stage each one holds is the
 * project's. Without a mapping of its own (own null) the project reads the
 * tracker's, shown here as inherited; the first click starts from it.
 */

const STAGES: WorkflowStage[] = ['new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished']

interface Props {
  trackerId: string
  trackerName: string
  columns: TrackerColumn[]
  /** The tracker's own mapping, the default. */
  inherited: StageMapping | undefined
  /** The project's own mapping, null when it reads the tracker's. */
  own: StageMapping | null
  onChange: (own: StageMapping | null) => void
}

export const StageColumnsEditor: React.FC<Props> = ({ trackerId, trackerName, columns, inherited, own, onChange }) => {
  const { t } = useApp()
  const ps = t.projectSettings.tracker
  const effective = cleanStageMapping(own ?? inherited)
  const isOwn = own !== null

  return (
    <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)]" data-stage-mapping={trackerId}>
      <div className="flex items-center gap-2 mb-2">
        <span className="text-[11px] font-semibold text-[var(--text-primary)] truncate">{trackerName}</span>
        <span
          className={`text-[9px] px-1.5 py-0.5 rounded border ${
            isOwn
              ? 'bg-[var(--accent-light)] border-[var(--accent-color)] text-[var(--accent-color)]'
              : 'bg-[var(--bg-primary)] border-[var(--border-color)] text-[var(--text-muted)]'
          }`}
          data-stage-mapping-origin={isOwn ? 'project' : 'tracker'}
        >
          {isOwn ? ps.stageMappingOwn : ps.stageMappingInherited}
        </span>
        {isOwn && (
          <button
            type="button"
            onClick={() => onChange(null)}
            className="ml-auto inline-flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            <RotateCcw size={11} />
            {ps.stageMappingReset}
          </button>
        )}
      </div>
      {columns.length === 0 ? (
        <p className="text-[10px] text-[var(--text-muted)]">{ps.stageMappingNoColumns}</p>
      ) : (
        <div className="space-y-1.5">
          {STAGES.map(stage => (
            <div key={stage} className="flex items-start gap-2">
              <span className="w-20 shrink-0 pt-0.5 text-[10px] font-mono text-[var(--text-secondary)]">#{stage}</span>
              <div className="flex flex-wrap gap-1">
                {columns.map(column => {
                  const active = (effective[stage] || []).includes(column.name)
                  return (
                    <button
                      key={column.name}
                      type="button"
                      aria-pressed={active}
                      onClick={() => {
                        // A mapping left with no column reads the tracker's, as the server does.
                        const next = toggleStageColumn(own, inherited, stage, column.name)
                        onChange(Object.keys(cleanStageMapping(next)).length > 0 ? next : null)
                      }}
                      className={`text-[10px] px-2 py-0.5 rounded-lg border cursor-pointer transition-colors ${
                        active
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] text-[var(--text-primary)]'
                          : 'bg-[var(--bg-primary)] border-[var(--border-color)] text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      {column.name}
                    </button>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
