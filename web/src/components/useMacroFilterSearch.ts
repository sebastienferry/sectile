import React from 'react'
import { useApp } from '../context/AppContext'
import type { LookupOption } from './LookupField'
import { matchesSearch } from '../lib/searchFold'
import { plural } from '../lib/i18n'

/** The value of `parentFilter` that keeps the tickets with no macro. */
export const NO_MACRO = '__no_macro__'

export interface MacroCandidate {
  key: string
  title?: string
  count?: number
}

/**
 * The macro filter's values, the same in every view that offers it (#866): the
 * macros the board's facets carry, then the known parents, then whatever the
 * view adds, each key once; "No macro" first while some ticket has none.
 *
 * Returns the lookup's search, and the label of the current `parentFilter`.
 */
export function useMacroFilterSearch(extra: MacroCandidate[] = []) {
  const { taskFacets, availableParents, parentFilter, t, settings } = useApp()
  const F = t.shell.filters
  const lang = settings.language

  const macros = React.useMemo(() => {
    const list: Array<{ id: string; label: string; sublabel?: string }> = []
    const seen = new Set<string>()
    const add = (m: MacroCandidate) => {
      if (!m.key || seen.has(m.key)) return
      seen.add(m.key)
      list.push({
        id: m.key,
        label: m.title && m.title !== m.key ? `${m.key} · ${m.title}` : m.key,
        sublabel: m.count ? plural(lang, m.count, F.ticketCount) : undefined,
      })
    }
    taskFacets.macros?.forEach(add)
    availableParents.forEach(add)
    extra.forEach(add)
    return list
  }, [taskFacets.macros, availableParents, extra, F, lang])

  const search = React.useCallback(
    async (query: string): Promise<LookupOption[]> => {
      const filtered = macros.filter(m => matchesSearch(query, m.label, m.id))
      if (taskFacets.noMacroCount > 0 && matchesSearch(query, F.noMacro, F.noMilestone)) {
        return [{ id: NO_MACRO, label: F.noMacro, sublabel: plural(lang, taskFacets.noMacroCount, F.ticketCount) }, ...filtered]
      }
      return filtered
    },
    [macros, taskFacets.noMacroCount, F, lang]
  )

  const label = React.useMemo(() => {
    if (!parentFilter) return ''
    if (parentFilter === NO_MACRO || parentFilter === 'none') return F.noMacro
    return macros.find(m => m.id === parentFilter)?.label || parentFilter
  }, [parentFilter, macros, F])

  return { search, label }
}
