import { Kanban, Sparkles } from 'lucide-react'
import { useApp } from '../context/AppContext'
import {
  BOARD_GROUPING_OPTIONS,
  BOARD_GROUPING_SIZES,
  type BoardGroupingIcon,
  type BoardGroupingToggleSize,
} from '../lib/boardGrouping'

const ICONS: Record<BoardGroupingIcon, typeof Sparkles> = {
  sparkles: Sparkles,
  kanban: Kanban,
}

interface BoardGroupingToggleProps {
  /** `sm` pour la toolbar dense du Backlog, `md` pour celle du Board. */
  size?: BoardGroupingToggleSize
}

/**
 * Sélecteur unique « Workflow / Statuts », partagé par le Board et le Backlog
 * pour qu'ils ne puissent plus diverger (issue #290).
 */
export function BoardGroupingToggle({ size = 'md' }: BoardGroupingToggleProps) {
  const { boardGrouping, setBoardGrouping, t } = useApp()
  const tokens = BOARD_GROUPING_SIZES[size]

  return (
    <div
      role="group"
      aria-label={t.board.grouping.workflow + ' / ' + t.board.grouping.status}
      className={`flex items-center gap-0.5 p-0.5 rounded-lg border border-[var(--border-color)] shadow-2xs ${tokens.containerClass}`}
    >
      {BOARD_GROUPING_OPTIONS.map(option => {
        const Icon = ICONS[option.icon]
        const isActive = boardGrouping === option.id
        return (
          <button
            key={option.id}
            type="button"
            onClick={() => setBoardGrouping(option.id)}
            aria-pressed={isActive}
            title={t.board.grouping[option.tooltipKey]}
            className={`flex items-center gap-1.5 ${tokens.buttonPadding} rounded-md text-[11px] font-bold transition-all cursor-pointer ${
              isActive
                ? 'bg-[var(--accent-color)] text-white shadow-xs'
                : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)]'
            }`}
          >
            <Icon size={tokens.iconSize} className={isActive ? 'text-white' : option.inactiveIconClass} />
            <span className="hidden md:inline">{t.board.grouping[option.labelKey]}</span>
          </button>
        )
      })}
    </div>
  )
}
