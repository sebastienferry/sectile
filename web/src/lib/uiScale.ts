/**
 * L'échelle de l'interface : les crans offerts, et comment on passe de l'un à
 * l'autre.
 *
 * La densité ne bouge que la taille de police racine, ce qui laisse intactes
 * toutes les tailles fixées en pixels. L'échelle, elle, zoome tout le document.
 * Ce sont deux réglages, pas deux noms pour le même : on peut vouloir une
 * interface dense et grande, ou aérée et petite.
 *
 * Les crans vivent ici plutôt que dans le composant pour que le pas et le
 * bornage soient testables sans monter un rendu, et pour que la même liste serve
 * la barre d'état et l'écran de réglages. La liste côté serveur, dans
 * `internal/db/db.go`, doit rester identique : c'est elle qui borne ce qui est
 * enregistré.
 */

/**
 * Les crans de zoom, dans l'ordre croissant.
 *
 * Les quatre premiers niveaux historiques sont conservés tels quels, 112 compris
 * plutôt qu'arrondi à 110 : un réglage déjà choisi par quelqu'un ne se déplace
 * pas pour faire une plus jolie suite. Les crans ajoutés vont vers le bas, pour
 * qui veut tenir plus de tickets à l'écran, et surtout vers le haut, où s'arrêter
 * à 125 laissait sans réponse « c'est trop petit ».
 */
export const UI_SCALE_OPTIONS = [80, 90, 100, 112, 125, 150, 175]

export const DEFAULT_UI_SCALE = 100

/**
 * Ramène une valeur sur un cran offert.
 *
 * Une valeur absente ou nulle, celle d'une base plus ancienne, vaut cent. Une
 * valeur hors liste s'accroche au cran le plus proche plutôt que d'être refusée :
 * un réglage écrit par une autre version ne doit pas rendre l'interface
 * inutilisable, et le refus n'aurait rien à proposer à la place.
 */
export function normalizeUIScale(scale: number | undefined | null): number {
  if (!scale || scale <= 0) {
    return DEFAULT_UI_SCALE
  }
  let best = UI_SCALE_OPTIONS[0]
  let bestGap = -1
  for (const option of UI_SCALE_OPTIONS) {
    const gap = Math.abs(option - scale)
    if (bestGap < 0 || gap < bestGap) {
      best = option
      bestGap = gap
    }
  }
  return best
}

/**
 * Le cran suivant, vers le haut ou vers le bas.
 *
 * Aux extrémités, rend le cran courant : un bouton qui ne peut plus rien faire
 * se désactive, il ne boucle pas de 175 % à 80 %. Repartir du plus petit alors
 * qu'on demandait plus grand serait exactement le contraire du geste.
 */
export function stepUIScale(scale: number | undefined | null, direction: 'up' | 'down'): number {
  const current = normalizeUIScale(scale)
  const index = UI_SCALE_OPTIONS.indexOf(current)
  const next = direction === 'up' ? index + 1 : index - 1
  if (next < 0 || next >= UI_SCALE_OPTIONS.length) {
    return current
  }
  return UI_SCALE_OPTIONS[next]
}

/** Peut-on encore aller dans cette direction ? Ce qu'attend le bouton. */
export function canStepUIScale(scale: number | undefined | null, direction: 'up' | 'down'): boolean {
  return stepUIScale(scale, direction) !== normalizeUIScale(scale)
}

/** L'étiquette d'un cran, la seule forme sous laquelle il s'affiche. */
export const uiScaleLabel = (scale: number): string => `${scale} %`
