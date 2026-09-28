/**
 * Renders the server's activity text in the viewer's language.
 *
 * The server writes an activity's `action`, `summary` and `steps` in French,
 * from a fixed set of templates, and stores the resulting sentence. The web
 * recognizes those templates and renders the English one with the same
 * parameters; anything it does not recognize (agent output, external errors,
 * custom command lines, a template added on the server since) is shown exactly
 * as stored. See ADR 0035.
 *
 * The French catalog value of each template is the server's wording with
 * `{0}`, `{1}`… in place of its parameters, and it is the only source of the
 * pattern: the two cannot drift apart. Parameters are never translated.
 */
import { format, type Locale } from './i18n.ts'
import { operations } from '../locales/operations.ts'

type TemplateKey = keyof typeof operations.fr.activityTemplates

export interface ActivityTemplate {
  key: TemplateKey
  pattern: RegExp
}

const PLACEHOLDER = /\{(\d+)\}/g

const escapeRegExp = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

/**
 * The anchored pattern of a template: its literal text escaped, each `{n}`
 * turned into a lazy capture. `s` lets a parameter span lines, as an error
 * quoted by the server sometimes does.
 */
export function templatePattern(template: string): RegExp {
  const parts = template.split(PLACEHOLDER)
  // split with a capture group alternates literal text and placeholder digits.
  const source = parts.map((part, index) => (index % 2 === 0 ? escapeRegExp(part) : '(.*?)')).join('')
  return new RegExp(`^${source}$`, 's')
}

const literalLength = (template: string): number => template.replace(PLACEHOLDER, '').length

/**
 * Every template, the most specific first: a text matching both
 * `Cible : {0} ➔ aucun épic` and `Cible : {0}` is the former. The template
 * with more literal text is the narrower one, so it is tried first.
 */
export const ACTIVITY_TEMPLATES: readonly ActivityTemplate[] = (Object.keys(operations.fr.activityTemplates) as TemplateKey[])
  .map(key => ({ key, pattern: templatePattern(operations.fr.activityTemplates[key]) }))
  .sort((a, b) => literalLength(operations.fr.activityTemplates[b.key]) - literalLength(operations.fr.activityTemplates[a.key]))

/** The template a server text was written from, with its parameters, if any. */
export function matchActivityTemplate(text: string): { key: TemplateKey; params: string[] } | null {
  for (const template of ACTIVITY_TEMPLATES) {
    const match = template.pattern.exec(text)
    if (match) return { key: template.key, params: match.slice(1) }
  }
  return null
}

/**
 * The activity text in `locale`. French is the server's own language and is
 * returned as is; English renders the recognized template with its parameters
 * verbatim, and leaves unrecognized text untouched.
 */
export function localizeActivityText(text: string, locale: Locale): string {
  if (locale === 'fr' || !text) return text
  const match = matchActivityTemplate(text)
  if (!match) return text
  const params: Record<string, string> = {}
  match.params.forEach((value, index) => {
    params[String(index)] = value
  })
  return format(operations[locale].activityTemplates[match.key], params)
}
