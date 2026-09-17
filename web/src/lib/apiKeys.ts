// Wording for the API keys panel. Kept free of React so the rules can be
// tested as plain functions.

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

// One line a person can act on: how long is left, or that nothing is.
export function describeExpiry(expiresAt: string | null, now: Date = new Date()): string {
  const state = expiryState(expiresAt, now)
  if (state === 'none') return 'No expiry'
  const date = new Date(expiresAt as string)
  if (state === 'expired') return `Expired ${date.toLocaleDateString()}`
  const days = Math.ceil((date.getTime() - now.getTime()) / DAY)
  const left = days === 1 ? '1 day' : `${days} days`
  return state === 'soon' ? `Expires in ${left}, renew it` : `Expires ${date.toLocaleDateString()} (${left})`
}
