import { useState } from 'react'
import { RefreshCw } from 'lucide-react'
import type { EpicAxisField, EpicAxisFieldDiscovery, EpicAxisFields } from '../types'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import {
  EPIC_PRIORITY_VALUES,
  choiceLabel,
  normalizeQuarterValue,
  optionChoices,
  pickField,
  quarterRows,
  setOption,
  type EpicFieldAxis,
} from '../lib/epicAxisFields'

interface EpicAxisFieldsEditorProps {
  projectId: string
  fields: EpicAxisFields
  onChange: (fields: EpicAxisFields) => void
}

const AXES: EpicFieldAxis[] = ['priority', 'quarter']

const selectClass =
  'px-2 py-1 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)]'

/**
 * The Jira custom fields of the epic priority and quarter of a saved project
 * (#680). The candidates are read on demand from one epic's edit screen;
 * picking a field prefills its map with the server's deductions, and every
 * line stays editable. Edits are saved with the project, which marks the
 * lines a person changed as set by hand.
 */
export default function EpicAxisFieldsEditor({ projectId, fields, onChange }: EpicAxisFieldsEditorProps) {
  const { loadEpicAxisFieldCandidates, t } = useApp()
  const ps = t.projectSettings.tracker
  const [discovery, setDiscovery] = useState<EpicAxisFieldDiscovery | null>(null)
  const [reading, setReading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [addedQuarters, setAddedQuarters] = useState<string[]>([])
  const [newQuarter, setNewQuarter] = useState('')

  const read = async () => {
    setReading(true)
    setError(null)
    const outcome = await loadEpicAxisFieldCandidates(projectId)
    setReading(false)
    if (outcome.error) {
      setError(outcome.error)
      return
    }
    setDiscovery(outcome.discovery ?? null)
  }

  const setAxis = (axis: EpicFieldAxis, field: EpicAxisField | undefined) => {
    const next = { ...fields }
    if (field) next[axis] = field
    else delete next[axis]
    onChange(next)
  }

  const candidates = discovery?.candidates ?? []
  const loaded = candidates.length > 0

  const addQuarter = () => {
    const value = normalizeQuarterValue(newQuarter)
    if (!value) return
    setAddedQuarters(prev => (prev.includes(value) ? prev : [...prev, value]))
    setNewQuarter('')
  }

  return (
    <div className="col-span-2" data-testid="epic-axis-fields">
      <div className="flex items-center justify-between mb-1">
        <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
          {ps.epicAxisFieldsTitle}
        </span>
        <button
          type="button"
          onClick={read}
          disabled={reading}
          className="flex items-center gap-1 px-2 py-1 text-[10px] rounded-lg border border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--accent-color)] disabled:opacity-50"
        >
          <RefreshCw size={11} className={reading ? 'animate-spin' : ''} />
          {reading ? ps.epicAxisFieldsReading : ps.epicAxisFieldsRead}
        </button>
      </div>
      {error && (
        <p role="alert" className="mb-1 text-[10px] text-red-500">{error}</p>
      )}
      {discovery?.noEpic && (
        <p className="mb-1 text-[10px] text-amber-600">{ps.epicAxisFieldsNoEpic}</p>
      )}
      {discovery?.epicKey && (
        <p className="mb-1 text-[10px] text-[var(--text-muted)]">{format(ps.epicAxisFieldsReadFrom, { key: discovery.epicKey })}</p>
      )}
      <div className="grid grid-cols-2 gap-3">
        {AXES.map(axis => {
          const field = fields[axis]
          const candidate = candidates.find(c => c.id === field?.id)
          const choices = optionChoices(candidate)
          const values = axis === 'priority' ? [...EPIC_PRIORITY_VALUES] : quarterRows(field, addedQuarters)
          const known = field && !candidates.some(c => c.id === field.id)
          return (
            <div key={axis}>
              <label htmlFor={`project-epic-field-${axis}`} className="block text-[10px] text-[var(--text-muted)] mb-1">
                {ps.epicAxisPrefixAxes[axis]}
              </label>
              <select
                id={`project-epic-field-${axis}`}
                value={field?.id ?? ''}
                disabled={!loaded && !field}
                onChange={e => {
                  const id = e.target.value
                  if (!id) return setAxis(axis, undefined)
                  if (id === field?.id) return
                  const picked = candidates.find(c => c.id === id)
                  if (picked) setAxis(axis, pickField(picked, axis))
                }}
                className={`w-full ${selectClass}`}
              >
                <option value="">{ps.epicAxisFieldsNone}</option>
                {known && <option value={field.id}>{field.name || field.id}</option>}
                {candidates.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
              {field && (
                <table className="w-full text-xs mt-1">
                  <thead>
                    <tr className="text-[10px] text-[var(--text-muted)] text-left">
                      <th className="font-normal pb-1">{ps.epicAxisFieldsValue}</th>
                      <th className="font-normal pb-1">{ps.epicAxisFieldsOption}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {values.map(value => {
                      const path = field.options?.[value] ?? ''
                      return (
                        <tr key={value} data-testid={`epic-field-${axis}-${value}`}>
                          <td className="py-0.5 pr-2 font-mono text-[var(--text-primary)]">
                            {value.toUpperCase()}
                            {field.manual?.includes(value) && (
                              <span className="ml-1 px-1 py-0.5 text-[9px] rounded bg-[var(--bg-tertiary)] text-[var(--text-muted)]">{ps.epicAxisFieldsManual}</span>
                            )}
                          </td>
                          <td className="py-0.5">
                            {candidate ? (
                              <select
                                aria-label={`${ps.epicAxisFieldsOption}: ${value}`}
                                value={path}
                                onChange={e => setAxis(axis, setOption(field, value, e.target.value))}
                                className={selectClass}
                              >
                                <option value="">{ps.epicAxisFieldsNoOption}</option>
                                {path && !choices.some(c => c.path === path) && <option value={path}>{path}</option>}
                                {choices.map(c => (
                                  <option key={c.path} value={c.path}>{c.label}</option>
                                ))}
                              </select>
                            ) : (
                              <span className="text-[var(--text-secondary)]">{choiceLabel(choices, path) || ps.epicAxisFieldsNoOption}</span>
                            )}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
              {field && axis === 'quarter' && candidate && (
                <div className="flex items-center gap-2 mt-1">
                  <input
                    type="text"
                    value={newQuarter}
                    onChange={e => setNewQuarter(e.target.value)}
                    onKeyDown={e => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        addQuarter()
                      }
                    }}
                    placeholder={ps.epicAxisFieldsQuarterPlaceholder}
                    aria-label={ps.epicAxisFieldsAddQuarter}
                    className="w-24 px-2 py-1 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono"
                  />
                  <button
                    type="button"
                    onClick={addQuarter}
                    disabled={!normalizeQuarterValue(newQuarter)}
                    className="text-[10px] text-[var(--accent-color)] hover:underline disabled:opacity-50"
                  >
                    {ps.epicAxisFieldsAddQuarter}
                  </button>
                </div>
              )}
              {field && !candidate && (
                <span className="text-[9px] text-[var(--text-muted)] mt-1 block">{ps.epicAxisFieldsReadToEdit}</span>
              )}
            </div>
          )
        })}
      </div>
      <span className="text-[9px] text-[var(--text-muted)] mt-1 block">{ps.epicAxisFieldsHelp}</span>
    </div>
  )
}
