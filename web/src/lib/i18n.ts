/**
 * Locale helpers shared by every web surface: interpolation, plural forms,
 * dates and numbers in the UI language, and the document language.
 *
 * The UI language is Sectile's setting, never the browser's: a German browser
 * with Sectile in French shows French dates. The catalog itself stays in
 * `locales/translations.ts`; these helpers only format what it holds.
 */

export type Locale = 'fr' | 'en'

/** What a formatter shows for a missing or unreadable date. */
export const EMPTY_VALUE = '-'

/** The key under which the browser remembers the last UI language. */
export const LOCALE_STORAGE_KEY = 'sectile.language'

export type DateInput = string | number | Date | null | undefined

export interface PluralForms {
  one: string
  other: string
}

export const isLocale = (value: unknown): value is Locale => value === 'fr' || value === 'en'

/** The BCP 47 tag the Intl formatters get for a UI language. */
export const intlLocale = (locale: Locale): string => (locale === 'en' ? 'en-US' : 'fr-FR')

/**
 * Replaces every `{name}` of the template with its parameter. A placeholder
 * without a parameter stays visible, so a missing value shows in review rather
 * than silently disappearing.
 */
export function format(template: string, params: Record<string, string | number> = {}): string {
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : match,
  )
}

/**
 * Picks the singular or plural form for `count` under the language's rules,
 * then interpolates `{count}` and the other parameters. French treats 0 and 1
 * as singular, English only 1.
 */
export function plural(
  locale: Locale,
  count: number,
  forms: PluralForms,
  params: Record<string, string | number> = {},
): string {
  const category = new Intl.PluralRules(intlLocale(locale)).select(count)
  const template = category === 'one' ? forms.one : forms.other
  return format(template, { count: formatNumber(locale, count), ...params })
}

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/

/**
 * Reads a date input. A `YYYY-MM-DD` string is a calendar day and becomes
 * local midnight, so it never shows as the day before in a time zone west of
 * UTC; anything else is an instant.
 */
export function parseDateInput(value: DateInput): Date | null {
  if (value === null || value === undefined || value === '') return null
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  if (typeof value === 'string') {
    const day = DATE_ONLY.exec(value)
    if (day) {
      const date = new Date(Number(day[1]), Number(day[2]) - 1, Number(day[3]))
      return Number.isNaN(date.getTime()) ? null : date
    }
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

const DEFAULT_DATE: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric' }
const DEFAULT_TIME: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' }

function formatWith(locale: Locale, value: DateInput, options: Intl.DateTimeFormatOptions): string {
  const date = parseDateInput(value)
  if (!date) return EMPTY_VALUE
  return new Intl.DateTimeFormat(intlLocale(locale), options).format(date)
}

/** A calendar date, by default "1 sept. 2026" or "Sep 1, 2026". */
export function formatDate(locale: Locale, value: DateInput, options: Intl.DateTimeFormatOptions = DEFAULT_DATE): string {
  return formatWith(locale, value, options)
}

/** A date with its time, in the viewer's time zone. */
export function formatDateTime(locale: Locale, value: DateInput, options?: Intl.DateTimeFormatOptions): string {
  return formatWith(locale, value, options ?? { ...DEFAULT_DATE, ...DEFAULT_TIME })
}

/** A time of day, in the viewer's time zone. */
export function formatTime(locale: Locale, value: DateInput, options: Intl.DateTimeFormatOptions = DEFAULT_TIME): string {
  return formatWith(locale, value, options)
}

export function formatNumber(locale: Locale, value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(intlLocale(locale), options).format(value)
}

function storage(): Storage | null {
  try {
    return typeof window !== 'undefined' ? window.localStorage : null
  } catch {
    return null
  }
}

/** Remembers the UI language for the next signed-out visit. */
export function rememberLocale(locale: Locale): void {
  try {
    storage()?.setItem(LOCALE_STORAGE_KEY, locale)
  } catch {
    // A browser that refuses storage simply falls back to its own language.
  }
}

/**
 * The language to use before any personal setting is known: the one this
 * browser last used, else the browser's language (French when it starts with
 * `fr`), else English.
 */
export function resolveInitialLocale(
  remembered: string | null = readRememberedLocale(),
  browserLanguages: readonly string[] = typeof navigator !== 'undefined' ? navigator.languages ?? [navigator.language] : [],
): Locale {
  if (isLocale(remembered)) return remembered
  const first = browserLanguages.find(Boolean)
  return first?.toLowerCase().startsWith('fr') ? 'fr' : 'en'
}

function readRememberedLocale(): string | null {
  try {
    return storage()?.getItem(LOCALE_STORAGE_KEY) ?? null
  } catch {
    return null
  }
}

/** Sets the page language and title, so assistive technologies and the tab speak the UI language. */
export function applyDocumentLocale(locale: Locale, title: string): void {
  if (typeof document === 'undefined') return
  document.documentElement.lang = locale
  document.title = title
}
