import type { SkillEditorEntry, SkillOverrideKind } from '../types'

type KindEntry = Pick<SkillEditorEntry, 'isCustom' | 'overrideKind' | 'content' | 'defaultContent' | 'defaultWorkContent'>

/**
 * What the skills editor holds once the override kind switches to next (#732):
 * the stored override when it is of that kind, else the built-in work sections
 * for work only, or the complete built-in skill for a full replacement.
 */
export const kindStartContent = (entry: KindEntry, next: SkillOverrideKind): string => {
  if (entry.isCustom && (entry.overrideKind || '') === next) return entry.content
  return next === 'work' ? entry.defaultWorkContent ?? '' : entry.defaultContent
}
