import type { HorizonTab } from './roadmap'
import { resolveStorage, type StorageLike } from './roadmapDisplayMode.ts'

/**
 * How the roadmap was left: its tab, its selected macro, the room its panel
 * takes and the folded sections of the framing.
 *
 * Changing view unmounts the roadmap, and without these it came back on NOW,
 * on its first macro, with the panel shrunk and every section unfolded. They
 * are reading settings, like the row shape, and are kept per browser the same
 * way. The selected macro is the exception, kept per project: a macro key
 * means nothing in another project.
 *
 * Every read tolerates a missing, refused or foreign value and answers the
 * default, and every write to an unavailable storage is a no-op.
 */

export const ROADMAP_TAB_STORAGE_KEY = 'sectile_roadmap_tab'
export const ROADMAP_PANEL_EXPANDED_STORAGE_KEY = 'sectile_roadmap_panel_expanded'
export const ROADMAP_PANEL_HIDDEN_STORAGE_KEY = 'sectile_roadmap_panel_hidden'
export const ROADMAP_DESCRIPTION_OPEN_STORAGE_KEY = 'sectile_roadmap_description_open'
export const ROADMAP_FRAMING_OPEN_STORAGE_KEY = 'sectile_roadmap_framing_open'

/** The key the selected macro of one project is kept under. */
export const roadmapSelectedKeyStorageKey = (projectId: string): string => `sectile_roadmap_selected_key:${projectId}`

const TABS: HorizonTab[] = ['now', 'next', 'later', 'unclassified', 'hidden']

function read(key: string, customStorage?: StorageLike): string | null {
  const storage = resolveStorage(customStorage)
  if (!storage) return null
  try {
    return storage.getItem(key)
  } catch {
    return null
  }
}

function write(key: string, value: string, customStorage?: StorageLike): void {
  const storage = resolveStorage(customStorage)
  if (!storage) return
  try {
    storage.setItem(key, value)
  } catch {
    // Storage unavailable: the preference holds for the page, and no longer.
  }
}

/** The tab the roadmap was left on; NOW when none is known. */
export function loadRoadmapTab(customStorage?: StorageLike): HorizonTab {
  const raw = (read(ROADMAP_TAB_STORAGE_KEY, customStorage) || '').trim().toLowerCase()
  return (TABS as string[]).includes(raw) ? (raw as HorizonTab) : 'now'
}

export function saveRoadmapTab(tab: HorizonTab, customStorage?: StorageLike): void {
  write(ROADMAP_TAB_STORAGE_KEY, tab, customStorage)
}

/** An on or off preference. Anything but a stored "1" or "0" answers the fallback. */
export function loadRoadmapFlag(key: string, fallback: boolean, customStorage?: StorageLike): boolean {
  const raw = read(key, customStorage)
  if (raw === '1') return true
  if (raw === '0') return false
  return fallback
}

export function saveRoadmapFlag(key: string, value: boolean, customStorage?: StorageLike): void {
  write(key, value ? '1' : '0', customStorage)
}

/**
 * The macro selected in a project, or null.
 *
 * Nothing is checked here: the view looks the key up among the macros it
 * shows and falls back to the first one, so a macro since closed, filtered out
 * or deleted behaves as no selection, and is selected again once it shows.
 */
export function loadRoadmapSelectedKey(projectId: string, customStorage?: StorageLike): string | null {
  if (!projectId) return null
  const raw = (read(roadmapSelectedKeyStorageKey(projectId), customStorage) || '').trim()
  return raw || null
}

export function saveRoadmapSelectedKey(projectId: string, key: string | null, customStorage?: StorageLike): void {
  if (!projectId) return
  write(roadmapSelectedKeyStorageKey(projectId), key || '', customStorage)
}
