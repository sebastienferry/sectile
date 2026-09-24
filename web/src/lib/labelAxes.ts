/**
 * Axes de découpe d'une macro, portés par des labels préfixés.
 *
 * Les phases disent l'ordre du travail, les objectifs ce qu'on cherche à
 * obtenir. Un ticket peut servir un objectif sans appartenir à une phase, et
 * l'inverse : ce sont deux axes, pas deux noms pour le même.
 *
 * Le préfixe est ce qui rend la vue possible. Sans lui, il faudrait prendre tout
 * label qui n'est ni une étape de workflow ni une métadonnée d'outillage, et
 * « force-close » se présenterait comme une phase. Ce qui est un axe de découpe
 * le déclare.
 *
 * Ils vivent hors du composant pour qu'un troisième axe s'ajoute ici, en trois
 * lignes, sans toucher à la vue, et pour que la normalisation des noms soit
 * testable sans monter un rendu.
 */

/**
 * Axes connus. Le préfixe est un identifiant, écrit sur le tracker, donc il ne
 * dépend pas de la langue ; les mots affichés, si.
 */
export const LABEL_AXES = {
  phase: { prefix: 'phase:' },
  goal: { prefix: 'goal:' },
} as const

export type LabelAxis = keyof typeof LABEL_AXES

/** Les mots d'un axe, tels que le panneau les emploie. */
export const axisWords = (
  axis: LabelAxis
): { singular: string; plural: string; none: string; placeholder: string } =>
  axis === 'phase'
    ? {
        singular: 'Phase',
        plural: 'Phases',
        none: 'Sans phase',
        placeholder: 'Nommer une phase…',
      }
    : {
        singular: 'Objectif',
        plural: 'Objectifs',
        none: 'Sans objectif',
        placeholder: 'Nommer un objectif…',
      }

/** Le label porte-t-il cet axe ? La casse est ignorée, le tracker n'en garantit aucune. */
export const isAxisLabel = (axis: LabelAxis, label: string): boolean =>
  label.trim().toLowerCase().startsWith(LABEL_AXES[axis].prefix)

/** Nom lisible : le label sans son préfixe. */
export const axisNameOf = (axis: LabelAxis, label: string): string =>
  label.trim().slice(LABEL_AXES[axis].prefix.length)

/**
 * Label à poser sur le tracker, à partir du nom saisi.
 *
 * Jira refuse les espaces dans un label : « migration A » serait rejeté. Ils
 * deviennent des tirets, et l'interface montre le label obtenu avant de
 * l'appliquer, pour que la transformation se voie.
 *
 * Les minuscules sont imposées, et ce n'est pas une coquetterie. Le regroupement
 * compare les labels tels quels, alors que la reconnaissance de l'axe, elle,
 * ignore la casse : « phase:Migration-A » saisi ici et « phase:migration-a »
 * posé depuis le tracker se retrouveraient dans deux groupes distincts, portant
 * le même nom, sans qu'aucun des deux ne soit faux. Un axe n'a qu'une
 * orthographe, et c'est celle-ci.
 *
 * Un nom déjà préfixé n'est pas préfixé deux fois : on saisit aussi bien
 * « migration-a » que « phase:migration-a », et « phase:phase:… » ne serait un
 * choix de personne.
 *
 * Rend une chaîne vide quand il ne reste rien à nommer, ce qui est le signal
 * qu'attend le bouton pour rester désactivé.
 */
export const axisLabelOf = (axis: LabelAxis, name: string): string => {
  const prefix = LABEL_AXES[axis].prefix
  const clean = name
    .trim()
    .toLowerCase()
    .replace(new RegExp('^' + prefix, 'i'), '')
    .trim()
    .replace(/\s+/g, '-')
  return clean ? prefix + clean : ''
}
