import type { BoardGroupingMode } from '../types'

/**
 * L'ordre, les libellés et les icônes du sélecteur de regroupement sont décrits
 * ici, et non dans les vues : le Board et le Backlog rendaient chacun leur copie
 * du contrôle, qui divergeait à chaque évolution (issue #290).
 */
export type BoardGroupingIcon = 'sparkles' | 'kanban'

export interface BoardGroupingOption {
  id: BoardGroupingMode
  icon: BoardGroupingIcon
  /** Couleur de l'icône lorsque l'option n'est pas active. */
  inactiveIconClass: string
  /** Clés dans `translations.board.grouping`. */
  labelKey: 'workflow' | 'status'
  tooltipKey: 'workflowTooltip' | 'statusTooltip'
}

/** Le workflow agentique vient en premier : c'est le regroupement principal du produit. */
export const BOARD_GROUPING_OPTIONS: readonly BoardGroupingOption[] = [
  {
    id: 'workflow',
    icon: 'sparkles',
    inactiveIconClass: 'text-amber-400',
    labelKey: 'workflow',
    tooltipKey: 'workflowTooltip',
  },
  {
    id: 'status',
    icon: 'kanban',
    inactiveIconClass: 'text-cyan-400',
    labelKey: 'status',
    tooltipKey: 'statusTooltip',
  },
]

export type BoardGroupingToggleSize = 'sm' | 'md'

export interface BoardGroupingSizeTokens {
  iconSize: number
  buttonPadding: string
  containerClass: string
}

/**
 * Seule la densité distingue les deux toolbars : le Backlog est plus compact que
 * le Board. Tout le reste (ordre, libellés, icônes, tooltips) est commun.
 */
export const BOARD_GROUPING_SIZES: Record<BoardGroupingToggleSize, BoardGroupingSizeTokens> = {
  sm: {
    iconSize: 12,
    buttonPadding: 'px-2 py-1',
    containerClass: 'bg-[var(--bg-secondary)]',
  },
  md: {
    iconSize: 15,
    buttonPadding: 'px-2 py-1.5',
    containerClass: 'bg-[var(--bg-tertiary)]',
  },
}
