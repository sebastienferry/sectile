export const LAUNCH_MODEL_STORAGE_KEY = 'sectile_launch_models'

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

function readAll(customStorage?: StorageLike): Record<string, string> {
  const storage = resolveStorage(customStorage)
  if (!storage) return {}
  try {
    const raw = storage.getItem(LAUNCH_MODEL_STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    return parsed as Record<string, string>
  } catch {
    // Une sélection illisible ne vaut pas mieux qu'aucune : on repart de zéro
    // plutôt que de faire échouer l'affichage d'une carte.
    return {}
  }
}

/**
 * Le modèle retenu pour les lancements d'une tâche, vide quand l'utilisateur
 * s'en tient au modèle que la configuration résout.
 *
 * La sélection est mémorisée par tâche et survit à un rechargement : elle vaut
 * pour les lancements suivants de cette carte, pas pour une seule action.
 */
export function loadLaunchModel(taskId: string, customStorage?: StorageLike): string {
  const key = (taskId || '').trim()
  if (!key) return ''
  const value = readAll(customStorage)[key]
  return typeof value === 'string' ? value.trim() : ''
}

/** Enregistre la sélection d'une tâche ; une valeur vide efface son entrée. */
export function saveLaunchModel(taskId: string, model: string, customStorage?: StorageLike): void {
  const key = (taskId || '').trim()
  if (!key) return
  const storage = resolveStorage(customStorage)
  if (!storage) return
  const all = readAll(customStorage)
  const value = (model || '').trim()
  if (value) {
    all[key] = value
  } else {
    delete all[key]
  }
  try {
    storage.setItem(LAUNCH_MODEL_STORAGE_KEY, JSON.stringify(all))
  } catch {
    // Stockage indisponible : la sélection ne vivra que le temps de la vue.
  }
}
