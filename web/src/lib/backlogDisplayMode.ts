/**
 * The shape of a backlog row, and what the condensed shape gives up.
 *
 * The backlog is the view one scrolls when looking for something rather than
 * reading: the description excerpt and the macro title are what a row carries
 * for the reader who stops on it, and exactly what costs the reader who does
 * not. The condensed shape drops both and keeps the macro key, which is the
 * part one scans for.
 */
export type BacklogRowDisplayMode = 'condensed' | 'expanded'

export const BACKLOG_DISPLAY_MODE_STORAGE_KEY = 'sectile_backlog_display_mode'

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
 * Reads the shape of the backlog rows.
 *
 * The default is the unfolded shape, like the roadmap and unlike the board: a
 * backlog row is where one reads what a ticket is about before opening it, and
 * a reader who never asked for density should not have to discover that the
 * excerpt exists.
 *
 * A stored value that is no longer a known shape answers the default rather
 * than an empty list: a preference written by an earlier version must not
 * leave the view unreadable.
 */
export function loadBacklogRowDisplayMode(customStorage?: StorageLike): BacklogRowDisplayMode {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return 'expanded'
  }

  try {
    const raw = storage.getItem(BACKLOG_DISPLAY_MODE_STORAGE_KEY)
    if (!raw) {
      return 'expanded'
    }
    return raw.trim().toLowerCase() === 'condensed' ? 'condensed' : 'expanded'
  } catch {
    return 'expanded'
  }
}

/** Stores the shape of the rows. An unavailable storage is a no-op. */
export function saveBacklogRowDisplayMode(
  mode: BacklogRowDisplayMode,
  customStorage?: StorageLike
): void {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return
  }

  try {
    storage.setItem(BACKLOG_DISPLAY_MODE_STORAGE_KEY, mode)
  } catch {
    // Storage unavailable: the shape holds for the session, and no longer.
  }
}

/** Toggles between the two shapes. */
export function toggleBacklogRowDisplayMode(mode: BacklogRowDisplayMode): BacklogRowDisplayMode {
  return mode === 'condensed' ? 'expanded' : 'condensed'
}

/** Whether the shape is the single-line one. */
export function isBacklogRowCondensed(mode: BacklogRowDisplayMode): boolean {
  return mode === 'condensed'
}
