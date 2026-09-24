import { useApp } from '../context/AppContext'
import { PRIORITY_LEVELS, priorityColor } from '../lib/priority'
import type { Priority } from '../types'

interface PrioritySelectProps {
  value: Priority
  onChange: (priority: Priority) => void
  /** The form's own select classes, so the field matches its neighbours. */
  className?: string
}

/**
 * Priority field of the edit forms, showing the same colour dot as the card.
 *
 * A native select stays in charge of the keyboard, typeahead and mobile
 * pickers; its options cannot draw a coloured shape everywhere (macOS ignores
 * option styling), so the dot is laid over the closed field instead.
 */
export function PrioritySelect({ value, onChange, className = '' }: PrioritySelectProps) {
  const { t } = useApp()
  return (
    <div className="relative">
      <span
        aria-hidden="true"
        data-priority-dot
        className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 w-2 h-2 rounded-full ring-1 ring-black/10"
        style={{ backgroundColor: priorityColor(value) }}
      />
      <select
        value={value}
        onChange={e => onChange(e.target.value as Priority)}
        className={className}
        style={{ paddingLeft: '1.5rem' }}
      >
        {PRIORITY_LEVELS.map(level => (
          <option key={level} value={level}>{t.priority[level]}</option>
        ))}
      </select>
    </div>
  )
}
