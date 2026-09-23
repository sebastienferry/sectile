import type { WorkflowStage } from '../types'

/**
 * The rules of the board's multi-selection (issue #109), kept out of
 * `BoardView` so that they can be tested without a DOM. The board selects
 * cards to launch `/pickup-issues` on them, so a card is selectable only while
 * the batch would start its workflow from the beginning.
 */
export const SELECTABLE_STAGES: readonly WorkflowStage[] = ['new', 'clarified']

export const isSelectableStage = (stage: WorkflowStage): boolean => SELECTABLE_STAGES.includes(stage)

/** Ctrl+click, or Cmd+click on macOS, toggles a card instead of opening it. */
export const isSelectionClick = (e: { ctrlKey: boolean; metaKey: boolean }): boolean => e.ctrlKey || e.metaKey

export const toggleSelected = (selected: ReadonlySet<string>, id: string): Set<string> => {
  const next = new Set(selected)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  return next
}

/**
 * Keeps the selected ids that are still selectable on screen. The same Set
 * comes back when nothing drops out, so the effect that prunes on every render
 * of the board settles instead of setting a fresh Set each time.
 */
export const pruneSelection = (selected: ReadonlySet<string>, selectableIds: readonly string[]): ReadonlySet<string> => {
  const onScreen = new Set(selectableIds)
  if ([...selected].every(id => onScreen.has(id))) return selected
  return new Set([...selected].filter(id => onScreen.has(id)))
}

/**
 * The selection in board order, whatever order the cards were clicked in:
 * `pickup-issues` processes its tickets in the order it receives them.
 */
export const orderSelection = (selected: ReadonlySet<string>, boardOrder: readonly string[]): string[] =>
  boardOrder.filter(id => selected.has(id))

export interface EscapeContext {
  key: string
  defaultPrevented: boolean
  /** A surface the `AppContext` handler closes on `Escape` is open, or a search is typed. */
  appSurfaceOpen: boolean
  /** The focus is in a field or a terminal, where `Escape` belongs to the widget. */
  inputFocused: boolean
  /** An `aria-modal` dialog is in the document. */
  modalOpen: boolean
}

/**
 * `Escape` clears the selection only when nothing else would take the key. A
 * card menu spends it with `preventDefault()` before it reaches `window`, and
 * the surfaces `AppContext` ranks are excluded by state, so the two `window`
 * handlers never act on the same press, whatever order they run in.
 */
export const shouldEscapeClearSelection = (ctx: EscapeContext): boolean =>
  ctx.key === 'Escape'
  && !ctx.defaultPrevented
  && !ctx.appSurfaceOpen
  && !ctx.inputFocused
  && !ctx.modalOpen
