/**
 * Vues optionnelles de l'espace de travail.
 *
 * Triage, Roadmap et Timeline répondent à un besoin de planification (trier ce
 * qui n'est classé nulle part, poser les macros sur des horizons, lire le
 * calendrier des sprints) qu'un projet suivant un seul flux de tickets n'a
 * jamais. Les laisser en permanence dans la barre latérale revient à proposer
 * trois écrans vides ; chaque projet dit donc lesquelles il affiche, et aucune
 * n'est active par défaut.
 *
 * Le serveur normalise la liste stockée, mais l'interface ne peut pas s'y fier :
 * une réponse d'un binaire plus ancien, ou un projet pas encore chargé, valent
 * « aucune vue » plutôt qu'une barre incohérente.
 */

import type { OptionalViewMode, Project, ViewMode } from '../types'

/** Ordre canonique des vues optionnelles, celui de la barre latérale. */
export const OPTIONAL_VIEWS: OptionalViewMode[] = ['triage', 'roadmap', 'timeline']

/** Vraie si `view` est l'une des vues qu'un projet active à la demande. */
export const isOptionalView = (view: ViewMode): view is OptionalViewMode =>
  (OPTIONAL_VIEWS as ViewMode[]).includes(view)

/**
 * Vues optionnelles réellement affichables pour ce projet, dans l'ordre
 * canonique et sans doublon. Sans projet, la réponse est vide : mieux vaut une
 * barre sans ces entrées qu'une entrée menant à un écran sans données.
 */
export const enabledOptionalViews = (project: Project | null | undefined): OptionalViewMode[] => {
  const asked = new Set(
    (project?.enabledViews ?? []).map(view => String(view).trim().toLowerCase())
  )
  return OPTIONAL_VIEWS.filter(view => asked.has(view))
}

/**
 * Vraie si le projet affiche cette vue. Une vue qui n'est pas optionnelle est
 * toujours disponible : seules les trois vues de planification se configurent.
 */
export const isViewAvailable = (project: Project | null | undefined, view: ViewMode): boolean =>
  !isOptionalView(view) || enabledOptionalViews(project).includes(view)

/**
 * Vue à afficher quand celle demandée n'est pas disponible sur ce projet. Le
 * cas se produit en changeant de projet alors qu'une vue optionnelle est
 * ouverte : la vue mémorisée ne doit pas laisser un écran mort.
 */
export const resolveAvailableView = (
  project: Project | null | undefined,
  view: ViewMode,
  fallback: ViewMode = 'board'
): ViewMode => (isViewAvailable(project, view) ? view : fallback)
