import type { Horizon } from './roadmap'

/**
 * La forme des lignes de la roadmap, et ce que la forme condensée montre.
 *
 * Les deux vivent ensemble parce qu'elles n'ont de sens que l'une par l'autre :
 * l'abréviation d'un horizon et la liste de ceux qu'une ligne dense propose ne
 * servent nulle part ailleurs, et les garder ici les rend vérifiables sans
 * monter un rendu.
 */
export type RoadmapRowDisplayMode = 'condensed' | 'expanded'

/**
 * L'horizon en deux lettres, pour la forme condensée.
 *
 * Une ligne condensée tient sur une ligne, et « NOW NEXT LATER » y prend la
 * place du titre. Deux lettres suffisent à reconnaître ce qu'on connaît déjà, et
 * le libellé entier reste dans l'infobulle et dans la forme dépliée.
 */
export const HORIZON_SHORT: Record<Horizon, string> = {
  now: 'No',
  next: 'Nx',
  later: 'La',
  hidden: 'Ma',
}

/**
 * Les horizons qu'une ligne condensée propose.
 *
 * « masqué » n'y est pas : masquer une macro est un geste qu'on ne veut pas à
 * portée de clic dans une liste dense, et il reste offert par la forme dépliée
 * et par le panneau.
 */
export const CONDENSED_HORIZONS: Horizon[] = ['now', 'next', 'later']

export const ROADMAP_DISPLAY_MODE_STORAGE_KEY = 'sectile_roadmap_display_mode'

export interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

function resolveStorage(customStorage?: StorageLike): StorageLike | null {
  if (customStorage) {
    return customStorage
  }
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return window.localStorage
    }
  } catch {
    return null
  }
  return null
}

/**
 * Lit la forme des lignes de la roadmap.
 *
 * Le défaut est la forme dépliée, contrairement au board : une ligne de macro
 * porte le placement de ses tickets en sprint, et c'est ce que la roadmap est
 * venue vérifier. La forme condensée sert à parcourir beaucoup de macros, ce
 * qu'on demande plutôt qu'on ne subit.
 *
 * Une valeur relue qui n'est plus une forme connue rend le défaut : une
 * préférence écrite par une version antérieure ne doit pas laisser la liste
 * vide.
 */
export function loadRoadmapRowDisplayMode(customStorage?: StorageLike): RoadmapRowDisplayMode {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return 'expanded'
  }

  try {
    const raw = storage.getItem(ROADMAP_DISPLAY_MODE_STORAGE_KEY)
    if (!raw) {
      return 'expanded'
    }
    return raw.trim().toLowerCase() === 'condensed' ? 'condensed' : 'expanded'
  } catch {
    return 'expanded'
  }
}

/** Enregistre la forme des lignes. Un stockage indisponible est sans effet. */
export function saveRoadmapRowDisplayMode(
  mode: RoadmapRowDisplayMode,
  customStorage?: StorageLike
): void {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return
  }

  try {
    storage.setItem(ROADMAP_DISPLAY_MODE_STORAGE_KEY, mode)
  } catch {
    // Stockage indisponible : la forme vaut pour la session, et rien de plus.
  }
}

/** Bascule entre les deux formes. */
export function toggleRoadmapRowDisplayMode(mode: RoadmapRowDisplayMode): RoadmapRowDisplayMode {
  return mode === 'condensed' ? 'expanded' : 'condensed'
}

/** La forme est-elle celle d'une ligne unique. */
export function isRoadmapRowCondensed(mode: RoadmapRowDisplayMode): boolean {
  return mode === 'condensed'
}
