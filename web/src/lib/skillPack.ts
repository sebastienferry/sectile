import type { SkillEditorEntry, SkillPackPin, SkillPackPreview } from '../types'

/**
 * Lecture d'un pack de skills venu d'une marketplace.
 *
 * Les corps d'un pack sont des prompts qu'une CLI exécute : ce qui compte à
 * l'écran, c'est de dire d'où vient ce qui tourne, et ce qu'un pack changerait
 * avant qu'il ne change quoi que ce soit. Ces fonctions ne font que ça, sans
 * rien décider : l'application reste une action explicite.
 */

/** Coordonnées lisibles d'un pack épinglé. Vide quand le projet n'en a pas. */
export function pinLabel(pin: SkillPackPin | null | undefined): string {
  if (!pin || !pin.marketplace || !pin.plugin) return ''
  let label = `${pin.marketplace} / ${pin.plugin}`
  if (pin.version) label += ` @ ${pin.version}`
  if (pin.commit) label += ` · ${pin.commit.slice(0, 7)}`
  return label
}

/** Une skill dont la ligne de base vient d'une marketplace. */
export function isFromMarketplace(entry: Pick<SkillEditorEntry, 'origin'> | null | undefined): boolean {
  return Boolean(entry && entry.origin === 'marketplace')
}

/**
 * Ce que l'entrée affiche sous le corps : le pack quand il y en a un, sinon le
 * modèle intégré. Une édition du projet passe devant les deux, et c'est le
 * badge « PERSO » qui le dit.
 */
export function originLabel(entry: Pick<SkillEditorEntry, 'origin' | 'packOrigin'> | null | undefined): string {
  if (!entry) return ''
  if (entry.origin === 'marketplace') return entry.packOrigin || 'marketplace'
  return ''
}

export interface PreviewSummary {
  /** Skills que le pack fournit et qui changeraient. */
  changed: string[]
  /** Skills que le pack fournit à l'identique. */
  unchanged: string[]
  /** Répertoires livrés par le plugin que Sectile n'installe pas. */
  ignored: string[]
  /** Répertoires refusés, avec la raison. */
  rejected: { dir: string; reason: string }[]
  /** Skills du workflow que le pack ne fournit pas. */
  missing: string[]
  warnings: string[]
}

/** Résumé d'un aperçu : rien n'est écrit, tout est dit. */
export function previewSummary(preview: SkillPackPreview | null | undefined): PreviewSummary {
  const summary: PreviewSummary = { changed: [], unchanged: [], ignored: [], rejected: [], missing: [], warnings: [] }
  if (!preview) return summary

  for (const entry of preview.entries || []) {
    if (entry.changed) summary.changed.push(entry.dirName)
    else summary.unchanged.push(entry.dirName)
  }
  summary.missing = [...(preview.missing || [])]
  summary.ignored = [...(preview.pack?.ignored || [])]
  summary.warnings = [...(preview.pack?.warnings || [])]
  for (const [dir, reason] of Object.entries(preview.pack?.rejected || {})) {
    summary.rejected.push({ dir, reason })
  }
  return summary
}

/**
 * Un pack sans révision résolue ne se rejoue pas à l'identique : un répertoire
 * local n'épingle rien, et l'aperçu doit le dire avant l'application.
 */
export function isReproducible(preview: SkillPackPreview | null | undefined): boolean {
  return Boolean(preview?.pack?.commit)
}
