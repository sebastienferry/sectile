import type { TaskActivity } from '../types'

/**
 * Le moteur d'un run, tel qu'il a réellement tourné : « claude · claude-opus-5 »,
 * ou le seul moteur quand aucun modèle n'a atteint la ligne de commande.
 *
 * Vide quand le run n'en porte aucun, ce qui est le cas de tous ceux enregistrés
 * avant que Sectile ne le retienne : on n'affiche alors rien plutôt que de
 * laisser croire à un moteur par défaut.
 */
export function runEngineLabel(activity: Pick<TaskActivity, 'provider' | 'model'> | undefined): string {
  const provider = (activity?.provider || '').trim()
  const model = (activity?.model || '').trim()
  if (!provider && !model) return ''
  if (!model) return provider
  if (!provider) return model
  return `${provider} · ${model}`
}
