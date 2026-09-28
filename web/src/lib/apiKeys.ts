// Expiry rules for the API keys panel. Kept free of React so the rules can be
// tested as plain functions.

import { format, formatDate, plural, type Locale, type PluralForms } from './i18n.ts'

export type ApiKey = {
  ID: string
  UserID: string
  Label: string
  CreatedAt: string
  LastSeen: string
  ExpiresAt: string | null
}

export const DEFAULT_KEY_TTL_DAYS = 90

// Keys expiring within this window are flagged, matching the agent's own log
// warning so both surfaces agree on when a key is "about to expire".
export const EXPIRY_WARNING_DAYS = 10

const DAY = 24 * 60 * 60 * 1000

export type ExpiryState = 'none' | 'valid' | 'soon' | 'expired'

export function expiryState(expiresAt: string | null, now: Date = new Date()): ExpiryState {
  if (!expiresAt) return 'none'
  const date = new Date(expiresAt)
  if (Number.isNaN(date.getTime())) return 'none'
  const remaining = date.getTime() - now.getTime()
  if (remaining <= 0) return 'expired'
  if (remaining <= EXPIRY_WARNING_DAYS * DAY) return 'soon'
  return 'valid'
}

/** What the expiry of a key amounts to, for the panel to put into words. */
export type ExpiryStatus =
  | { state: 'none' }
  | { state: 'expired'; date: Date }
  | { state: 'soon' | 'valid'; date: Date; days: number }

export function expiryStatus(expiresAt: string | null, now: Date = new Date()): ExpiryStatus {
  const state = expiryState(expiresAt, now)
  if (state === 'none') return { state }
  const date = new Date(expiresAt as string)
  if (state === 'expired') return { state, date }
  return { state, date, days: Math.ceil((date.getTime() - now.getTime()) / DAY) }
}

/** The catalog strings `describeExpiry` words a status with. */
export interface ExpiryStrings {
  noExpiry: string
  /** With `{date}`. */
  expired: string
  /** With `{count}`. */
  expiresSoon: PluralForms
  /** With `{date}` and `{count}`. */
  expiresOn: PluralForms
}

// One line a person can act on: how long is left, or that nothing is. The
// words come from the caller's catalog, so the rule is tested in both
// languages without React.
export function describeExpiry(
  expiresAt: string | null,
  strings: ExpiryStrings,
  locale: Locale,
  now: Date = new Date(),
): string {
  const status = expiryStatus(expiresAt, now)
  switch (status.state) {
    case 'none': return strings.noExpiry
    case 'expired': return format(strings.expired, { date: formatDate(locale, status.date) })
    case 'soon': return plural(locale, status.days, strings.expiresSoon)
    case 'valid': return plural(locale, status.days, strings.expiresOn, { date: formatDate(locale, status.date) })
  }
}
