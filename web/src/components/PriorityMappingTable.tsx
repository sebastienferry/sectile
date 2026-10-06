import { useState } from 'react'
import { RefreshCw } from 'lucide-react'
import type { Priority, PriorityMapping } from '../types'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { PRIORITY_LEVELS } from '../lib/priority'
import {
  confirmLine,
  optionFor,
  setLevel,
  setPreferred,
  sureOptionsOf,
  unwritableLevels,
} from '../lib/priorityMapping'

interface PriorityMappingTableProps {
  projectId: string
  mapping: PriorityMapping
  onChange: (mapping: PriorityMapping) => void
}

/**
 * The Jira priority mapping of a saved project (#679): one line per option of
 * its scheme, the guessed lines marked, and per level the option a write
 * sends. Edits are saved with the project; the refresh reads the scheme again
 * and replaces what is shown with what the server stored.
 */
export default function PriorityMappingTable({ projectId, mapping, onChange }: PriorityMappingTableProps) {
  const { refreshPriorityMapping, t } = useApp()
  const ps = t.projectSettings.tracker
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = async () => {
    setRefreshing(true)
    setError(null)
    const outcome = await refreshPriorityMapping(projectId)
    setRefreshing(false)
    if (outcome.error) {
      setError(outcome.error)
      return
    }
    onChange(outcome.project?.priorityMapping ?? {})
  }

  const options = mapping.options ?? []
  const shared = PRIORITY_LEVELS.filter(level => sureOptionsOf(mapping, level).length > 1)
  const unwritable = unwritableLevels(mapping)
  const levelName = (level: Priority) => ps.priorityLevels[level]

  return (
    <div className="col-span-2">
      <div className="flex items-center justify-between mb-1">
        <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
          {ps.priorityMappingTitle}
        </span>
        <button
          type="button"
          onClick={refresh}
          disabled={refreshing}
          className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-lg border border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--accent-color)] disabled:opacity-50"
        >
          <RefreshCw size={11} className={refreshing ? 'animate-spin' : ''} />
          {refreshing ? ps.priorityMappingRefreshing : ps.priorityMappingRefresh}
        </button>
      </div>
      {error && (
        <p role="alert" className="mb-1 text-[10px] text-red-500">{error}</p>
      )}
      {options.length === 0 ? (
        <span className="text-[10px] text-[var(--text-muted)]">{ps.priorityMappingEmpty}</span>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-[10px] text-[var(--text-muted)] text-left">
              <th className="font-normal pb-1">{ps.priorityMappingOption}</th>
              <th className="font-normal pb-1">{ps.priorityMappingLevel}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {options.map(option => (
              <tr key={option.id} data-testid={`priority-mapping-${option.id}`}>
                <td className="py-0.5 pr-2 text-[var(--text-primary)]">{option.name}</td>
                <td className="py-0.5 pr-2">
                  <select
                    aria-label={`${ps.priorityMappingLevel}: ${option.name}`}
                    value={option.level}
                    onChange={e => onChange(setLevel(mapping, option.id, e.target.value as Priority))}
                    className="px-2 py-1 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)]"
                  >
                    {PRIORITY_LEVELS.map(level => (
                      <option key={level} value={level}>{levelName(level)}</option>
                    ))}
                  </select>
                </td>
                <td className="py-0.5">
                  {option.guessed && (
                    <span className="flex items-center gap-2">
                      <span className="px-1.5 py-0.5 text-[9px] rounded bg-amber-500/15 text-amber-600">{ps.priorityMappingGuessed}</span>
                      <button
                        type="button"
                        onClick={() => onChange(confirmLine(mapping, option.id))}
                        className="text-[10px] text-[var(--accent-color)] hover:underline"
                      >
                        {ps.priorityMappingConfirm}
                      </button>
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {shared.map(level => (
        <label key={level} className="flex items-center gap-2 mt-1 text-[10px] text-[var(--text-secondary)]">
          {format(ps.priorityMappingPreferred, { level: levelName(level) })}
          <select
            value={optionFor(mapping, level)?.id}
            onChange={e => onChange(setPreferred(mapping, level, e.target.value))}
            className="px-2 py-0.5 text-[10px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)]"
          >
            {sureOptionsOf(mapping, level).map(o => (
              <option key={o.id} value={o.id}>{o.name}</option>
            ))}
          </select>
        </label>
      ))}
      {unwritable.map(level => (
        <p key={level} className="mt-1 text-[10px] text-amber-600">
          {format(ps.priorityMappingUnwritable, { level: levelName(level) })}
        </p>
      ))}
      <span className="text-[9px] text-[var(--text-muted)] mt-1 block">{ps.priorityMappingHelp}</span>
    </div>
  )
}
