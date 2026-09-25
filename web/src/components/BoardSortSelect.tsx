import { ArrowDown, ArrowUp } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { BOARD_GROUPING_SIZES, type BoardGroupingToggleSize } from '../lib/boardGrouping'
import { BOARD_SORT_OPTIONS, pickBoardSortField, type BoardSortField } from '../lib/boardSort'

interface BoardSortSelectProps {
  /** `sm` for the dense Backlog toolbar, `md` for the Board one. */
  size?: BoardGroupingToggleSize
}

/**
 * The card order selector, shared by the Board and the Backlog so that both
 * read and write one remembered sort (#402). Sized like the Workflow / Status
 * toggle it sits next to.
 *
 * A native select keeps keyboard, typeahead and screen-reader behaviour, and
 * needs no outside-click handling.
 */
export function BoardSortSelect({ size = 'md' }: BoardSortSelectProps) {
  const { boardSort, setBoardSort, t } = useApp()
  const tokens = BOARD_GROUPING_SIZES[size]
  const directionLabel = boardSort.asc ? t.board.sort.ascending : t.board.sort.descending
  const DirectionIcon = boardSort.asc ? ArrowUp : ArrowDown

  return (
    <div
      role="group"
      aria-label={t.board.sort.label}
      className={`flex items-center gap-0.5 p-0.5 rounded-lg border border-[var(--border-color)] shadow-2xs ${tokens.containerClass}`}
    >
      <select
        value={boardSort.field}
        // Picking a criterion, even the current one, starts it in its natural direction.
        onChange={e => setBoardSort(pickBoardSortField(e.target.value as BoardSortField))}
        aria-label={t.board.sort.label}
        title={t.board.sort.label}
        data-testid="board-sort-field"
        className={`${tokens.buttonPadding} rounded-md text-[11px] font-bold bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] focus:outline-none cursor-pointer`}
      >
        {BOARD_SORT_OPTIONS.map(option => (
          <option key={option.id} value={option.id}>
            {t.board.sort.fields[option.labelKey]}
          </option>
        ))}
      </select>
      <button
        type="button"
        onClick={() => setBoardSort({ ...boardSort, asc: !boardSort.asc })}
        aria-label={directionLabel}
        title={directionLabel}
        data-testid="board-sort-direction"
        className={`flex items-center ${tokens.buttonPadding} rounded-md text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-all cursor-pointer`}
      >
        <DirectionIcon size={tokens.iconSize} />
      </button>
    </div>
  )
}
