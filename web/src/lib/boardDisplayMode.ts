export type BoardCardDisplayMode = 'condensed' | 'expanded'

export const BOARD_DISPLAY_MODE_STORAGE_KEY = 'taskflow_board_display_mode'
export const LEGACY_BOARD_DISPLAY_MODE_STORAGE_KEY = 'taskacao_board_display_mode'

export interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

function resolveStorage(customStorage?: StorageLike): StorageLike | null {
  if (customStorage) {
    return customStorage
  }
  if (typeof window !== 'undefined' && window.localStorage) {
    return window.localStorage
  }
  return null
}

/**
 * Loads the board card display mode from storage.
 * Defaults to 'condensed' when no saved preference exists.
 */
export function loadBoardCardDisplayMode(customStorage?: StorageLike): BoardCardDisplayMode {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return 'condensed'
  }

  try {
    const raw =
      storage.getItem(BOARD_DISPLAY_MODE_STORAGE_KEY) ??
      storage.getItem(LEGACY_BOARD_DISPLAY_MODE_STORAGE_KEY)

    if (!raw) {
      return 'condensed'
    }

    const normalized = raw.trim().toLowerCase()
    if (normalized === 'expanded' || normalized === 'false') {
      return 'expanded'
    }
    if (normalized === 'condensed' || normalized === 'true') {
      return 'condensed'
    }

    return 'condensed'
  } catch {
    return 'condensed'
  }
}

/**
 * Persists the board card display mode to storage.
 */
export function saveBoardCardDisplayMode(
  mode: BoardCardDisplayMode,
  customStorage?: StorageLike
): void {
  const storage = resolveStorage(customStorage)
  if (!storage) {
    return
  }

  try {
    storage.setItem(BOARD_DISPLAY_MODE_STORAGE_KEY, mode)
  } catch {
    // Storage unavailable, ignore gracefully
  }
}

/**
 * Toggles the given display mode between 'condensed' and 'expanded'.
 */
export function toggleBoardCardDisplayMode(mode: BoardCardDisplayMode): BoardCardDisplayMode {
  return mode === 'condensed' ? 'expanded' : 'condensed'
}

/**
 * Returns true if the mode corresponds to condensed (single-line) display.
 */
export function isBoardCardCondensed(mode: BoardCardDisplayMode): boolean {
  return mode === 'condensed'
}
