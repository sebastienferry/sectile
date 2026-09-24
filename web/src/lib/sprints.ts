import type { TrackerSprint } from '../types'

export interface SprintTimelineConfig {
  durationDays: number
  startDate: string
}

/** Formate une date en YYYY-MM-DD */
export const formatDateISO = (d: Date): string => {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

/** Récupère le lundi de la semaine d'une date */
export const getMonday = (d: Date): Date => {
  const date = new Date(d)
  const day = date.getDay()
  const diff = date.getDate() - day + (day === 0 ? -6 : 1)
  date.setDate(diff)
  date.setHours(0, 0, 0, 0)
  return date
}

/** Formate une date en français lisible (ex: 1 sept. 2026) */
export const formatDateFR = (dateStr?: string): string => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString('fr-FR', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

/** Formate une date pour input type="date" */
export const formatDateInput = (dateStr?: string): string => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return formatDateISO(d)
}

/** Calcule les dates consécutives pour une liste de sprints */
export const calculateSprintDates = (
  sprints: TrackerSprint[],
  startDateStr: string,
  durationDays: number
): TrackerSprint[] => {
  let currentStart = new Date(startDateStr)
  if (isNaN(currentStart.getTime())) {
    currentStart = getMonday(new Date())
  }

  const today = new Date()
  today.setHours(0, 0, 0, 0)

  return sprints.map((sprint, idx) => {
    const start = new Date(currentStart)
    const end = new Date(start)
    end.setDate(end.getDate() + Math.max(1, durationDays) - 1)
    end.setHours(23, 59, 59, 999)

    // Si le sprint a été explicitement clôturé, on conserve son état
    let state = sprint.state || 'future'
    if (sprint.state !== 'closed') {
      if (today > end) {
        state = 'closed'
      } else if (today >= start && today <= end) {
        state = 'active'
      } else {
        state = 'future'
      }
    }

    const nextStart = new Date(end)
    nextStart.setDate(nextStart.getDate() + 1)
    nextStart.setHours(0, 0, 0, 0)
    currentStart = nextStart

    return {
      ...sprint,
      name: sprint.name || `Sprint ${idx + 1}`,
      startDate: formatDateISO(start),
      endDate: formatDateISO(end),
      state,
    }
  })
}

/** Décale consécutivement les sprints suivants après modification de date */
export const shiftSubsequentSprintDates = (
  sprints: TrackerSprint[],
  changedIndex: number,
  defaultDurationDays: number = 14
): TrackerSprint[] => {
  const result = [...sprints]
  if (changedIndex < 0 || changedIndex >= result.length) return result

  const baseSprint = result[changedIndex]
  if (!baseSprint.endDate) return result

  let prevEnd = new Date(baseSprint.endDate)

  for (let i = changedIndex + 1; i < result.length; i++) {
    const nextStart = new Date(prevEnd)
    nextStart.setDate(nextStart.getDate() + 1)
    nextStart.setHours(0, 0, 0, 0)

    // Calcul de la durée précédente du sprint i, ou durée par défaut
    let currDuration = defaultDurationDays
    if (result[i].startDate && result[i].endDate) {
      const s = new Date(result[i].startDate!)
      const e = new Date(result[i].endDate!)
      const diffDays = Math.round((e.getTime() - s.getTime()) / (1000 * 60 * 60 * 24)) + 1
      if (diffDays > 0) currDuration = diffDays
    }

    const nextEnd = new Date(nextStart)
    nextEnd.setDate(nextEnd.getDate() + Math.max(1, currDuration) - 1)
    nextEnd.setHours(23, 59, 59, 999)

    result[i] = {
      ...result[i],
      startDate: formatDateISO(nextStart),
      endDate: formatDateISO(nextEnd),
    }

    prevEnd = nextEnd
  }

  return result
}

/** Génère une liste de sprints par défaut */
export const generateDefaultSprints = (
  count: number = 4,
  startDateStr?: string,
  durationDays: number = 14
): TrackerSprint[] => {
  const start = startDateStr ? new Date(startDateStr) : getMonday(new Date())
  const placeholders: TrackerSprint[] = Array.from({ length: count }, (_, i) => ({
    id: `sprint-${i + 1}`,
    name: `Sprint ${i + 1}`,
    state: i === 0 ? 'active' : 'future',
  }))
  return calculateSprintDates(placeholders, formatDateISO(start), durationDays)
}

/** Calcule les jours restants ou le délai jusqu'à un sprint */
export const getSprintRelativeInfo = (sprint: TrackerSprint): { label: string; type: 'current' | 'future' | 'past' } => {
  if (sprint.state === 'closed') {
    return {
      label: 'Sprint clôturé',
      type: 'past',
    }
  }

  if (!sprint.startDate || !sprint.endDate) {
    return {
      label: sprint.state === 'active' ? 'En cours' : 'À venir',
      type: sprint.state === 'active' ? 'current' : 'future',
    }
  }

  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const start = new Date(sprint.startDate)
  const end = new Date(sprint.endDate)
  end.setHours(23, 59, 59, 999)

  if (sprint.state === 'active' || (today >= start && today <= end)) {
    const diffMs = end.getTime() - today.getTime()
    const daysLeft = Math.ceil(diffMs / (1000 * 60 * 60 * 24))
    if (daysLeft < 0) {
      return { label: `En cours (dépassé de ${Math.abs(daysLeft)}j)`, type: 'current' }
    }
    return {
      label: daysLeft <= 1 ? "Dernier jour du sprint !" : `En cours (${daysLeft} jours restants)`,
      type: 'current',
    }
  }

  if (today < start) {
    const diffMs = start.getTime() - today.getTime()
    const daysUntil = Math.ceil(diffMs / (1000 * 60 * 60 * 24))
    return {
      label: daysUntil === 1 ? "Débute demain" : `Débute dans ${daysUntil} jours`,
      type: 'future',
    }
  }

  return {
    label: "Échéance dépassée",
    type: 'past',
  }
}

/**
 * Who owns a project's sprints, which decides what the timeline may do:
 * - `tracker`: the tracker (Jira) holds them; every change is written there
 *   first and the timeline shows the tracker's answer;
 * - `readonly`: the tracker has no sprints (GitHub); sprints stored locally
 *   are shown, nothing is created, changed or moved into them;
 * - `local`: the local board, which keeps the timeline's own sprints.
 */
export type SprintManagement = 'tracker' | 'readonly' | 'local'

export const sprintManagementOf = (project?: { issueTracker?: string; githubRepo?: string } | null): SprintManagement => {
  const tracker = (project?.issueTracker || '').toLowerCase().trim()
  if (tracker === 'jira') return 'tracker'
  if (tracker === 'github' || ((tracker === '' || tracker === 'local') && project?.githubRepo?.trim())) return 'readonly'
  return 'local'
}

/**
 * The id and name to move work items to, from a sprint named or identified by
 * value. A tracker moves work items by sprint id, never by name; the local
 * board keeps using the name as both, as it always has. An empty value is the
 * backlog.
 */
export const sprintTarget = (sprints: TrackerSprint[], value: string, management: SprintManagement): { id: string; name: string } => {
  const wanted = value.trim()
  if (!wanted) return { id: '', name: '' }
  const sprint = sprints.find(sp => sp.id === wanted) || sprints.find(sp => sp.name.toLowerCase().trim() === wanted.toLowerCase())
  const name = sprint?.name || wanted
  return { id: management === 'tracker' && sprint?.id ? sprint.id : name, name }
}

/** The day after the last sprint ends, where a new batch starts by default. */
export const nextBatchStart = (sprints: TrackerSprint[], fallback: string): string => {
  const ends = sprints.map(sp => (sp.endDate ? new Date(sp.endDate) : null)).filter((d): d is Date => d !== null && !isNaN(d.getTime()))
  if (ends.length === 0) return fallback
  const last = new Date(Math.max(...ends.map(d => d.getTime())))
  last.setDate(last.getDate() + 1)
  return formatDateISO(last)
}
