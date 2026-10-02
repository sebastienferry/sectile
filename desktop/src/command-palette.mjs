// The command palette searches actions, projects and executions in one list.
// An entry matches when every word of the query appears in its label or its
// detail; it ranks higher when the label starts with the query, then when a
// word of the label does. Groups keep their order: actions, then projects,
// then executions.

const GROUPS = ['action', 'project', 'execution']

function score(entry, query) {
  const label = entry.label.toLowerCase()
  if (label.startsWith(query)) return 0
  if (label.split(/[\s·#/-]+/).some(word => word.startsWith(query))) return 1
  return 2
}

export function paletteMatches(entries, text, limit = 30) {
  const query = String(text || '').trim().toLowerCase()
  const words = query.split(/\s+/).filter(Boolean)
  const found = []
  for (const [index, entry] of entries.entries()) {
    const haystack = (entry.label + ' ' + (entry.detail || '')).toLowerCase()
    if (words.every(word => haystack.includes(word))) found.push({ entry, index, rank: query ? score(entry, query) : 0 })
  }
  found.sort((a, b) => GROUPS.indexOf(a.entry.group) - GROUPS.indexOf(b.entry.group) || a.rank - b.rank || a.index - b.index)
  return found.slice(0, limit).map(item => item.entry)
}
