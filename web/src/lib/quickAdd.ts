/**
 * Quick add (#445).
 *
 * The rules the quick add dialog applies to the macro it offers and to what
 * happens once the ticket exists. The dialog itself only renders them.
 */

import type { MacroMeta } from '../types'

/**
 * What the dialog does once the ticket is created: nothing, open it and
 * rewrite it as a user story, or launch its clarification. Exclusive, and
 * reset to `none` at every opening, so launching an agent stays a gesture.
 */
export type QuickAddFollowUp = 'none' | 'rewrite' | 'clarify'

export const QUICK_ADD_FOLLOW_UPS: QuickAddFollowUp[] = ['none', 'rewrite', 'clarify']

/**
 * The macros a new ticket can be attached to: the open ones, in the order of
 * their keys, M-2 before M-10.
 */
export const quickAddMacroOptions = (macros: MacroMeta[]): MacroMeta[] =>
  macros
    .filter(m => m.key && !m.closed)
    .sort((a, b) => a.key.localeCompare(b.key, undefined, { numeric: true, sensitivity: 'base' }))

/**
 * The macro pre-selected when the board is filtered on one. The board filter
 * holds a key or a title; anything that names none of the offered macros, the
 * "no macro" filter included, selects none.
 */
export const initialQuickAddMacro = (filter: string | null | undefined, options: MacroMeta[]): string => {
  const wanted = (filter || '').trim()
  if (!wanted) return ''
  const match = options.find(m => m.key === wanted) || options.find(m => (m.title || '').trim() === wanted)
  return match ? match.key : ''
}
