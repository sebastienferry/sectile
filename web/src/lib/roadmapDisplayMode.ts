import type { Horizon } from './roadmap'

/**
 * The shape of a roadmap row, and what the condensed shape shows.
 *
 * The two live together because neither means anything without the other: the
 * two-letter form of a horizon and the list of the ones a dense row offers
 * serve nowhere else, and keeping them here makes them testable without
 * mounting a render.
 */
export type RoadmapRowDisplayMode = 'condensed' | 'expanded'

/**
 * A horizon in two letters, for the condensed shape.
 *
 * A condensed row fits on one line, and "NOW NEXT LATER" takes the title's
 * place on it. Two letters are enough to recognise what one already knows, and
 * the whole label stays in the tooltip and in the unfolded shape.
 */
export const HORIZON_SHORT: Record<Horizon, string> = {
  now: 'No',
  next: 'Nx',
  later: 'La',
  hidden: 'Ma',
}

/**
 * The horizons a condensed row offers.
 *
 * "hidden" is not among them: hiding a macro is not a gesture one wants within
 * a click's reach in a dense list, and it stays offered by the unfolded shape
 * and by the panel.
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
 * Reads the shape of the roadmap rows.
 *
 * The default is the unfolded shape, unlike the board: a macro row carries the
 * sprint placement of its work items, and that is what one came to the roadmap
 * to check. The condensed shape is for browsing many macros, which is something
 * one asks for rather than something one is given.
 *
 * A stored value that is no longer a known shape answers the default: a
 * preference written by an earlier version must not leave the list empty.
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

/** Stores the shape of the rows. An unavailable storage is a no-op. */
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
    // Storage unavailable: the shape holds for the session, and no longer.
  }
}

/** Toggles between the two shapes. */
export function toggleRoadmapRowDisplayMode(mode: RoadmapRowDisplayMode): RoadmapRowDisplayMode {
  return mode === 'condensed' ? 'expanded' : 'condensed'
}

/** Whether the shape is the single-line one. */
export function isRoadmapRowCondensed(mode: RoadmapRowDisplayMode): boolean {
  return mode === 'condensed'
}
