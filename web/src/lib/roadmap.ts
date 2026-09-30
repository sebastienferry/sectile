import type { EpicMeta, EpicPriority, EpicReadiness, Priority, Project, Task, TrackerSprint, WorkflowStage } from '../types'
import { suggestReadiness } from './epicAxes.ts'
import { foldForSearch } from './searchFold.ts'
import { WORKFLOW_ORDER, resolveTaskStage } from './workflow.ts'

/**
 * Epic aggregation for the Roadmap view.
 *
 * Sectile does not import epics as cards: they are containers, carried by the
 * tickets as `parentKey` / `parentTitle`. An epic is therefore rebuilt here
 * from its children, and what the view shows about its progress is derived
 * from them: it is the only source available, and it is always up to date
 * after a sync. What a person decides about the epic itself (its horizon, its
 * priority and its quarter) comes from its meta instead.
 *
 * Display text (horizon hints, placement and priority labels, "no sprint")
 * lives in the `planning` catalog; this module only holds keys and colours.
 */

export type Horizon = 'now' | 'next' | 'later' | 'hidden'
/** Onglet de la roadmap : les trois horizons, plus les épics pas encore arbitrés. */
export type HorizonTab = Horizon | 'unclassified'
export type Maturity = 'Draft' | 'Clarified' | 'Specified' | 'Ready'

export interface EpicRow {
  key: string
  title: string
  /** Équipe la plus représentée chez les enfants, faute d'équipe portée par l'épic. */
  squad: string
  /** Classification retenue par l'utilisateur, vide si l'épic n'est pas arbitré. */
  horizon: Horizon | ''
  /** Classification que les données suggèrent, pour proposer un arbitrage. */
  suggested: Horizon
  maturity: Maturity
  /** The epic's own priority, empty when none. Never derived from the children (#627). */
  priority: EpicPriority | ''
  /** The epic's quarter, "2026-Q4", empty when none. */
  quarter: string
  /** The readiness a person decided, empty when nobody did (#633). */
  readiness: EpicReadiness | ''
  /** The readiness the epic's content suggests, shown while nobody decided. */
  suggestedReadiness: EpicReadiness
  tasks: Task[]
  /** Enfants encore ouverts : ceux dont le placement en sprint est à vérifier. */
  open: Task[]
  /** Ouverts dans un sprint actif. */
  inActiveSprint: Task[]
  /** Ouverts dans un sprint futur. */
  inFutureSprint: Task[]
  /** Ouverts dans un sprint clos ou inconnu du board : anomalie à corriger. */
  inStaleSprint: Task[]
  /** Ouverts sans aucun sprint. */
  unscheduled: Task[]
  sprints: string[]
  meta?: EpicMeta
  /** L'épic est terminé côté tracker : hors roadmap par défaut. */
  closed: boolean
  /** The epic's own page on its tracker, empty when there is none. */
  externalUrl: string
}
export type MacroRow = EpicRow

/**
 * What "copy the link" puts on the clipboard.
 *
 * The epic's own page when the tracker gives one; otherwise its key and title,
 * which still name it in a message. The kind tells the toast which of the two
 * was copied, so nobody pastes a reference believing it is a link.
 */
export const macroCopyPayload = (row: Pick<EpicRow, 'key' | 'title' | 'externalUrl'>): { text: string; kind: 'link' | 'ref' } =>
  row.externalUrl ? { text: row.externalUrl, kind: 'link' } : { text: `${row.key}: ${row.title}`, kind: 'ref' }

/**
 * Colours of the view: only the app's global variables, never a hardcoded
 * value. The product theme already lives in index.css (accent, signal
 * colours, light and dark themes, per-project variants), so freezing the
 * design palette here would deprive the view of the chosen theme and accent.
 *
 * The accent carries NOW, since it is the brand orange by default and a
 * project may legitimately change it. The labels are product vocabulary and
 * read the same in every language; the hints come from the catalog.
 */
export const HORIZON_META: Record<Horizon, { label: string; color: string; bg: string; border: string }> = {
  now: {
    label: 'NOW',
    color: 'var(--accent-color)',
    bg: 'var(--accent-light)',
    border: 'rgb(var(--accent-rgb) / 0.45)',
  },
  next: {
    label: 'NEXT',
    color: 'var(--status-info)',
    bg: 'rgb(var(--status-info-rgb) / 0.13)',
    border: 'rgb(var(--status-info-rgb) / 0.4)',
  },
  later: {
    label: 'LATER',
    color: 'var(--status-warn)',
    bg: 'rgb(var(--status-warn-rgb) / 0.12)',
    border: 'rgb(var(--status-warn-rgb) / 0.32)',
  },
  hidden: {
    label: 'HIDDEN',
    color: 'var(--text-muted)',
    bg: 'var(--bg-tertiary)',
    border: 'var(--border-color)',
  },
}

export const MATURITY_META: Record<Maturity, { pct: number; color: string; bg: string; border: string }> = {
  Draft: { pct: 20, color: 'var(--text-muted)', bg: 'var(--bg-tertiary)', border: 'var(--border-color)' },
  Clarified: { pct: 50, color: 'var(--status-warn)', bg: 'rgb(var(--status-warn-rgb) / 0.14)', border: 'rgb(var(--status-warn-rgb) / 0.34)' },
  Specified: { pct: 78, color: 'var(--status-info)', bg: 'rgb(var(--status-info-rgb) / 0.12)', border: 'rgb(var(--status-info-rgb) / 0.32)' },
  Ready: { pct: 100, color: 'var(--status-ok)', bg: 'rgb(var(--status-ok-rgb) / 0.13)', border: 'rgb(var(--status-ok-rgb) / 0.32)' },
}

/**
 * The readiness levels (#633) borrow the maturity palette: muted for an idea,
 * warn while shaping, ok once ready.
 */
export const READINESS_META: Record<EpicReadiness, { color: string; bg: string; border: string }> = {
  idea: MATURITY_META.Draft,
  shaping: MATURITY_META.Clarified,
  ready: MATURITY_META.Ready,
}

export const PRIORITY_META: Record<Priority, { color: string; bg: string }> = {
  urgent: { color: 'var(--status-danger)', bg: 'rgb(var(--status-danger-rgb) / 0.13)' },
  high: { color: 'var(--accent-color)', bg: 'var(--accent-light)' },
  medium: { color: 'var(--status-info)', bg: 'rgb(var(--status-info-rgb) / 0.12)' },
  low: { color: 'var(--text-muted)', bg: 'var(--bg-tertiary)' },
}

const isOpen = (task: Task): boolean => task.status !== 'finished' && task.status !== 'done'

/**
 * Maturité de l'épic = étape la moins avancée parmi ses enfants ouverts. Un épic
 * n'est « Ready » que si aucun de ses tickets n'attend encore un cadrage ou une
 * spécification.
 */
const maturityOf = (open: Task[], total: number, project?: Project | null): Maturity => {
  // Aucun enfant du tout : l'épic n'est pas prêt, il est vide. Le distinguer de
  // « tous les enfants terminés » évite d'afficher Ready sur une coquille.
  if (total === 0) return 'Draft'
  if (open.length === 0) return 'Ready'
  let lowest = WORKFLOW_ORDER.length - 1
  open.forEach(task => {
    const index = WORKFLOW_ORDER.indexOf(resolveTaskStage(task, project))
    if (index >= 0 && index < lowest) lowest = index
  })
  const stage: WorkflowStage = WORKFLOW_ORDER[lowest]
  if (stage === 'new') return 'Draft'
  if (stage === 'clarified') return 'Clarified'
  if (stage === 'specified') return 'Specified'
  return 'Ready'
}

/**
 * Classification suggérée, jamais imposée : du travail dans un sprint actif
 * ressemble à du NOW, dans un sprint futur à du NEXT, et le reste à du cadrage.
 * L'utilisateur tranche, la suggestion ne sert qu'à proposer un arbitrage en un
 * clic sur les épics encore non classés.
 */
// La suggestion ne propose jamais « hidden » : décider qu'un épic est du
// tout-venant est un jugement, aucune donnée ne le dit.
const suggestHorizon = (inActive: Task[], inFuture: Task[]): Horizon => {
  if (inActive.length > 0) return 'now'
  if (inFuture.length > 0) return 'next'
  return 'later'
}

const sprintStateIndex = (sprints?: TrackerSprint[]): Map<string, string> => {
  const map = new Map<string, string>()
  ;(sprints || []).forEach(sp => {
    const name = (sp.name || '').trim().toLowerCase()
    if (name) map.set(name, (sp.state || '').toLowerCase())
  })
  return map
}

export const buildEpicRows = (
  tasks: Task[],
  project?: Project | null,
  epicMeta?: EpicMeta[]
): EpicRow[] => {
  const metaByKey = new Map<string, EpicMeta>()
  ;(epicMeta || []).forEach(m => metaByKey.set(m.key, m))
  const states = sprintStateIndex(project?.sprints)

  const byKey = new Map<string, Task[]>()
  tasks.forEach(task => {
    const key = (task.parentKey || '').trim()
    if (!key) return
    const list = byKey.get(key)
    if (list) list.push(task)
    else byKey.set(key, [task])
  })

  // Les épics connus du tracker mais sans aucun ticket doivent apparaître :
  // sinon un épic fraîchement créé est invisible, donc inutilisable comme cible
  // pour découper un épic trop gros.
  ;(epicMeta || []).forEach(m => {
    if (!byKey.has(m.key)) byKey.set(m.key, [])
  })

  const rows: EpicRow[] = []
  byKey.forEach((children, key) => {
    const open = children.filter(isOpen)

    const inActiveSprint: Task[] = []
    const inFutureSprint: Task[] = []
    const inStaleSprint: Task[] = []
    const unscheduled: Task[] = []
    open.forEach(task => {
      const sprint = (task.sprint || '').trim()
      if (!sprint) {
        unscheduled.push(task)
        return
      }
      // Un sprint que le board ne connaît plus (clos, ou hors des sprints
      // rapatriés) est une anomalie autant qu'un ticket sans sprint.
      switch (states.get(sprint.toLowerCase())) {
        case 'active':
          inActiveSprint.push(task)
          break
        case 'future':
          inFutureSprint.push(task)
          break
        default:
          inStaleSprint.push(task)
      }
    })

    const teamCounts = new Map<string, number>()
    children.forEach(t => {
      const team = (t.team || '').trim()
      if (team) teamCounts.set(team, (teamCounts.get(team) || 0) + 1)
    })
    const squad = Array.from(teamCounts.entries()).sort((a, b) => b[1] - a[1])[0]?.[0] || '-'
    // An empty epic has no team; the priority and the quarter are the epic's
    // own, read from its meta and never deduced from its children.
    const meta = metaByKey.get(key)
    rows.push({
      key,
      // Le titre du ticket épic quand la synchro l'a lu, sinon celui que
      // portent ses enfants.
      title: meta?.title || children.find(t => (t.parentTitle || '').trim())?.parentTitle || key,
      squad,
      horizon: (meta?.horizon as Horizon | '') || '',
      suggested: suggestHorizon(inActiveSprint, inFutureSprint),
      maturity: maturityOf(open, children.length, project),
      priority: meta?.priority || '',
      quarter: meta?.quarter || '',
      readiness: meta?.readiness || '',
      suggestedReadiness: suggestReadiness({ childCount: children.length, description: meta?.description, todos: meta?.todos }),
      tasks: children,
      open,
      inActiveSprint,
      inFutureSprint,
      inStaleSprint,
      unscheduled,
      sprints: Array.from(
        new Set([...inActiveSprint, ...inFutureSprint, ...inStaleSprint].map(t => (t.sprint || '').trim()).filter(Boolean))
      ).sort(),
      meta,
      closed: Boolean(meta?.closed),
      externalUrl: meta?.externalUrl || '',
    })
  })

  // Le plus gros chantier ouvert en premier : c'est celui qui demande le plus
  // d'attention à la revue de sprint.
  return rows.sort((a, b) => {
    if (b.open.length !== a.open.length) return b.open.length - a.open.length
    return a.key.localeCompare(b.key, undefined, { numeric: true })
  })
}

/**
 * Anomalies de placement pour un horizon opérationnel : dans NOW on attend un
 * sprint actif, dans NEXT un sprint futur. Tout le reste doit se voir.
 */
/**
 * Recherche d'épic, pour la barre de recherche en vue roadmap.
 *
 * Ici on cherche un épic, pas un ticket. La clé, le titre et l'équipe portante
 * répondent à « où est passé cet épic ». Les clés et les titres des enfants
 * répondent à « dans quel épic se trouve ce ticket », qui est la question qu'on
 * se pose réellement devant une liste d'épics, et à laquelle rien ne répondait.
 *
 * Chaque mot doit se retrouver quelque part, dans n'importe quel ordre : taper
 * « paiement latence » trouve l'épic dont le titre porte les deux, sans exiger
 * qu'ils soient collés.
 */
export const matchesEpicSearch = (row: EpicRow, query: string): boolean => {
  const terms = foldForSearch(query).split(/\s+/).filter(Boolean)
  if (terms.length === 0) return true
  const haystack = foldForSearch(
    [row.key, row.title, row.squad, ...row.tasks.map(t => `${t.key} ${t.title}`)].join(' ')
  )
  return terms.every(term => haystack.includes(term))
}

export const matchesMacroSearch = matchesEpicSearch
export const buildMacroRows = buildEpicRows

/**
 * Anomalies de placement d'un épic : les tickets non terminés qui n'ont aucun
 * sprint, ou qui restent dans un sprint passé.
 *
 * La règle est la même pour NOW et pour NEXT. Un ticket déjà rangé dans un
 * sprint à venir alors que l'épic est en NOW n'est pas en faute, il est en
 * avance : c'est de l'information, pas une correction à faire, et le compter ici
 * remplissait la colonne d'alertes qui ne demandaient aucune action.
 *
 * LATER et les épics masqués n'ont pas d'anomalie : on y cadre un épic avant
 * qu'il ait des tickets, donc l'absence de sprint y est l'état normal.
 */
export const placementIssues = (row: EpicRow, horizon: Horizon): Task[] => {
  if (horizon === 'now' || horizon === 'next') {
    return [...row.unscheduled, ...row.inStaleSprint]
  }
  return []
}

/** État du placement d'un ticket au regard de l'horizon visé. */
export type PlacementState = 'ok' | 'other-horizon' | 'stale' | 'missing'

export const placementOf = (task: Task, row: EpicRow, horizon: Horizon): PlacementState => {
  if (row.unscheduled.includes(task)) return 'missing'
  if (row.inStaleSprint.includes(task)) return 'stale'
  if (horizon === 'now') return row.inActiveSprint.includes(task) ? 'ok' : 'other-horizon'
  if (horizon === 'next') return row.inFutureSprint.includes(task) ? 'ok' : 'other-horizon'
  return 'ok'
}

export const PLACEMENT_META: Record<PlacementState, { color: string; bg: string; border: string }> = {
  ok: {
    color: 'var(--status-ok)',
    bg: 'rgb(var(--status-ok-rgb) / 0.13)',
    border: 'rgb(var(--status-ok-rgb) / 0.32)',
  },
  'other-horizon': {
    color: 'var(--status-info)',
    bg: 'rgb(var(--status-info-rgb) / 0.12)',
    border: 'rgb(var(--status-info-rgb) / 0.32)',
  },
  stale: {
    color: 'var(--status-warn)',
    bg: 'rgb(var(--status-warn-rgb) / 0.14)',
    border: 'rgb(var(--status-warn-rgb) / 0.34)',
  },
  missing: {
    color: 'var(--status-danger)',
    bg: 'rgb(var(--status-danger-rgb) / 0.13)',
    border: 'rgb(var(--status-danger-rgb) / 0.32)',
  },
}

/**
 * Un épic appartient au projet quand sa clé porte le préfixe du projet. Les
 * autres viennent de tickets rattachés à un épic d'un autre projet Jira : utile
 * à savoir, encombrant dans une roadmap d'équipe.
 */
export const belongsToProjectKey = (row: EpicRow, projectKey: string): boolean => {
  const prefix = projectKey.trim().toUpperCase()
  if (!prefix) return true
  return row.key.toUpperCase().startsWith(prefix + '-')
}

/**
 * Label prefixes the roadmap owns on an epic. A label under one of them is set
 * by its own control (the horizon tabs for `roadmap:`, the panel's priority and
 * quarter fields for the axes of #627) and is never shown, filtered or edited
 * as a free label. Mirrors `macroAxisPrefixes` in `internal/db/macrolabels.go`,
 * which refuses them on the server too.
 */
export const EPIC_AXIS_LABEL_PREFIXES = ['roadmap:', 'priority:', 'quarter:', 'readiness:']

/** A bare quarter, "2026-Q3", which the import reads as the epic's quarter. */
const BARE_QUARTER = /^\d{4}[.\- ]q[1-4]$/

/** The match ignores case and a leading `#`, as the server's does. */
export const isEpicAxisLabel = (label: string): boolean => {
  const clean = label.trim().replace(/^#/, '').trim().toLowerCase()
  return EPIC_AXIS_LABEL_PREFIXES.some(prefix => clean.startsWith(prefix)) || BARE_QUARTER.test(clean)
}

/** An epic's free labels, in the order the tracker returned them. */
export const freeEpicLabels = (meta?: EpicMeta | null): string[] =>
  (meta?.labels || []).filter(label => label.trim() !== '' && !isEpicAxisLabel(label))

export interface EpicLabelCount {
  label: string
  count: number
}

/**
 * The free labels the given epics carry, with how many carry each. Two
 * spellings differing only by case are one label, shown as first met; the list
 * is sorted by label.
 */
export const epicLabelInventory = (rows: EpicRow[]): EpicLabelCount[] => {
  const byKey = new Map<string, EpicLabelCount>()
  rows.forEach(row => {
    const seen = new Set<string>()
    freeEpicLabels(row.meta).forEach(label => {
      const key = label.toLowerCase()
      if (seen.has(key)) return
      seen.add(key)
      const entry = byKey.get(key)
      if (entry) entry.count++
      else byKey.set(key, { label, count: 1 })
    })
  })
  return [...byKey.values()].sort((a, b) => a.label.localeCompare(b.label))
}

/**
 * Whether an epic passes the label filter. The filter is an OR: the labels of
 * one axis are exclusive on an epic, so an AND would empty the view as soon as
 * two were picked. No selected label filters nothing.
 */
export const matchesEpicLabels = (row: EpicRow, selected: string[]): boolean => {
  if (selected.length === 0) return true
  const carried = new Set(freeEpicLabels(row.meta).map(label => label.toLowerCase()))
  return selected.some(label => carried.has(label.toLowerCase()))
}

/**
 * Drops the selected labels no epic of the view carries any more, so that
 * changing another filter never leaves the list empty for a reason nobody sees.
 * Returns the same array when nothing changes, which lets a state setter bail.
 */
export const pruneSelectedLabels = (selected: string[], inventory: EpicLabelCount[]): string[] => {
  const offered = new Set(inventory.map(entry => entry.label.toLowerCase()))
  const kept = selected.filter(label => offered.has(label.toLowerCase()))
  return kept.length === selected.length ? selected : kept
}

/**
 * Whether the roadmap offers to edit an epic's labels: only on Jira, the one
 * tracker whose epics are read, and only on an epic of the project itself, not
 * a milestone-shaped or local `M-<n>` key. The server says so in
 * `labelsWritable` (#627); a server that does not send it gets the same rule
 * applied here. A hint for the view: the server refuses the rest anyway, with
 * the reason.
 */
export const canEditEpicLabels = (project: Project | null | undefined, row: EpicRow): boolean => {
  if (!project || project.issueTracker !== 'jira') return false
  if (typeof row.meta?.labelsWritable === 'boolean') return row.meta.labelsWritable
  if (/^M-\d+$/i.test(row.key.trim())) return false
  return belongsToProjectKey(row, project.jiraProject || '')
}

/**
 * Rang chronologique des sprints du projet.
 *
 * La date de début du board fait référence quand elle est là. À défaut, on lit
 * le nom : la convention `2026-Q3-04` se trie toute seule, et ce qui n'y répond
 * pas garde son rang d'apparition sur le board. Un ticket sans sprint passe en
 * dernier, parce qu'il n'est pas encore placé dans le temps.
 */
export const sprintRank = (project?: Project | null): Map<string, number> => {
  const sprints = project?.sprints || []
  const keyed = sprints.map((sp, index) => {
    const start = (sp.startDate || '').trim()
    const parsed = start ? Date.parse(start) : NaN
    return {
      name: (sp.name || '').trim(),
      index,
      time: Number.isFinite(parsed) ? parsed : NaN,
      nameKey: sprintNameKey(sp.name || ''),
    }
  })

  const sorted = [...keyed].sort((a, b) => {
    if (Number.isFinite(a.time) && Number.isFinite(b.time)) return a.time - b.time
    if (Number.isFinite(a.time)) return -1
    if (Number.isFinite(b.time)) return 1
    if (a.nameKey && b.nameKey) return a.nameKey.localeCompare(b.nameKey)
    return a.index - b.index
  })

  const ranks = new Map<string, number>()
  sorted.forEach((sp, rank) => {
    if (sp.name) ranks.set(sp.name, rank)
  })
  return ranks
}

/** Clé triable extraite d'un nom de sprint du type `2026-Q3-04 PE`. */
const sprintNameKey = (name: string): string => {
  const m = name.match(/(\d{4})[-\s]*Q?(\d)[-\s]*(\d+)?/)
  if (!m) return ''
  const [, year, quarter, num] = m
  return `${year}-${quarter}-${(num || '0').padStart(3, '0')}`
}

/**
 * Tickets d'un épic dans l'ordre chronologique de leur sprint, puis par clé.
 * C'est cet ordre que le curseur de coupe utilise : couper à un rang veut dire
 * « tout ce qui vient après ce sprint part ailleurs ».
 */
export const tasksBySprintOrder = (tasks: Task[], project?: Project | null): Task[] => {
  const ranks = sprintRank(project)
  const rankOf = (task: Task): number => {
    const sprint = (task.sprint || '').trim()
    if (!sprint) return Number.MAX_SAFE_INTEGER
    const known = ranks.get(sprint)
    if (known !== undefined) return known
    // Sprint inconnu du board : placé après les sprints connus, mais avant les
    // tickets sans sprint, et trié entre eux par nom.
    return Number.MAX_SAFE_INTEGER - 1
  }

  return [...tasks].sort((a, b) => {
    const ra = rankOf(a)
    const rb = rankOf(b)
    if (ra !== rb) return ra - rb
    const sa = (a.sprint || '').trim()
    const sb = (b.sprint || '').trim()
    if (sa !== sb) return sa.localeCompare(sb)
    return a.key.localeCompare(b.key, undefined, { numeric: true })
  })
}

/** A ticket's sprint label for the grouped display; `noSprint` names the missing one. */
export const sprintLabelOf = (task: Task, noSprint: string): string => (task.sprint || '').trim() || noSprint
