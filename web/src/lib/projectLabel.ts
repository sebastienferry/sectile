/**
 * Le label d'appartenance attribue un ticket du tracker à un projet Sectile
 * quand plusieurs projets partagent un projet du tracker. La synchro continue
 * d'importer tout le board : c'est l'affichage qui se restreint, et ces deux
 * fonctions sont la partie testable de ce filtre.
 */

/** Deux labels se comparent sans casse et sans le « # » des labels d'étape. */
export const cleanLabel = (label: string): string =>
  label.trim().replace(/^#/, '').trim().toLowerCase()

/**
 * Paramètre `membership` de `GET /api/tasks`. Absent vaut « ce projet
 * seulement » : c'est le défaut, donc rien n'est envoyé dans ce cas.
 */
export const membershipParam = (showAllTickets: boolean): string | null =>
  showAllTickets ? 'all' : null

export interface ProjectLabelState {
  /** Le label du projet, vide quand il n'y en a pas. */
  label: string
  /** Le projet filtre sur un label : les entrées de menu ont un sens. */
  applicable: boolean
  /** Le ticket porte déjà le label. */
  carries: boolean
}

/**
 * Ce que le menu d'une carte doit proposer : rien si le projet n'a pas de label,
 * « retirer du projet » si le ticket le porte, « ajouter au projet » sinon.
 */
export const projectLabelState = (
  taskLabels: string[] | undefined,
  projectLabel: string | undefined,
): ProjectLabelState => {
  const label = (projectLabel || '').trim()
  if (label === '') {
    return { label: '', applicable: false, carries: false }
  }
  const want = cleanLabel(label)
  const carries = (taskLabels || []).some(l => cleanLabel(l) === want)
  return { label, applicable: true, carries }
}

/** Les labels du ticket une fois ajouté au projet, sans doublon. */
export const withProjectLabel = (taskLabels: string[] | undefined, projectLabel: string): string[] => {
  const labels = taskLabels || []
  const want = cleanLabel(projectLabel)
  if (want === '' || labels.some(l => cleanLabel(l) === want)) return labels
  return [...labels, projectLabel.trim()]
}

/** Les labels du ticket une fois retiré du projet, quelle que soit la casse écrite. */
export const withoutProjectLabel = (taskLabels: string[] | undefined, projectLabel: string): string[] => {
  const want = cleanLabel(projectLabel)
  return (taskLabels || []).filter(l => cleanLabel(l) !== want)
}
