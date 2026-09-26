import type { TrackerBoard, TrackerColumn } from '../types'

/**
 * La reprise des colonnes du board côté éditeur. Le serveur applique la même
 * fusion à la synchro : ces fonctions sont là pour que « Détecter » donne le
 * même résultat avant même d'enregistrer.
 */

/** Une colonne telle que le tracker la décrit : un nom et les statuts qu'elle regroupe. */
export interface DetectedColumn {
  name: string
  statuses: string[]
}

/**
 * Le tracker donne les noms et l'ordre ; l'utilisateur garde ses statuts affectés
 * à la main que le tracker ne revendique nulle part, ses colonnes masquées et ses
 * colonnes à lui, tant qu'elles portent encore un statut libre.
 */
export const mergeDetectedColumns = (
  current: TrackerColumn[],
  detected: DetectedColumn[]
): TrackerColumn[] => {
  const claimed = new Set(detected.flatMap(c => c.statuses.map(s => s.toLowerCase())))
  const previousByName = new Map(current.map(c => [c.name.toLowerCase(), c]))

  const merged: TrackerColumn[] = detected.map(col => {
    const previous = previousByName.get(col.name.toLowerCase())
    const statuses = [...col.statuses]
    const seen = new Set(statuses.map(s => s.toLowerCase()))
    for (const st of previous?.statuses || []) {
      const key = st.toLowerCase()
      if (seen.has(key) || claimed.has(key)) continue
      seen.add(key)
      statuses.push(st)
    }
    return { name: col.name, statuses, hidden: previous?.hidden }
  })

  const detectedNames = new Set(detected.map(c => c.name.toLowerCase()))
  for (const col of current) {
    if (detectedNames.has(col.name.toLowerCase())) continue
    const kept = col.statuses.filter(st => !claimed.has(st.toLowerCase()))
    if (kept.length > 0) merged.push({ ...col, statuses: kept })
  }
  return merged
}

/** Les étapes visant une colonne disparue sont retirées ; les autres ne bougent pas. */
export const pruneStageColumns = (
  stageColumns: Record<string, string[]>,
  columns: TrackerColumn[]
): Record<string, string[]> => {
  const names = new Set(columns.map(c => c.name))
  const pruned: Record<string, string[]> = {}
  Object.entries(stageColumns).forEach(([stage, cols]) => {
    const kept = (cols || []).filter(c => names.has(c))
    if (kept.length > 0) pruned[stage] = kept
  })
  return pruned
}

/**
 * Le board retenu pour le projet, sinon son premier board scrum — celui qui porte
 * les colonnes qu'on veut refléter — sinon son premier board tout court.
 */
export const pickBoardId = (boards: TrackerBoard[], current?: string): string => {
  if (current && boards.some(b => b.id === current)) return current
  return boards.find(b => b.type?.toLowerCase() === 'scrum')?.id || boards[0]?.id || ''
}

/** The board recorded on the project, or '' when none is, or when the tracker no longer lists it. */
export const recordedBoardId = (boards: TrackerBoard[], current?: string): string =>
  current && boards.some(b => b.id === current) ? current : ''

/** The board offered as the default while none is recorded; '' once one is. */
export const suggestedBoardId = (boards: TrackerBoard[], recorded: string): string =>
  recorded ? '' : pickBoardId(boards)

/**
 * A choice is imported unless it is the board already recorded: comparing with the
 * displayed selection would ignore the suggested board while nothing is recorded.
 */
export const shouldImportBoard = (next: string, recorded: string): boolean =>
  next !== '' && next !== recorded
