/** The answer of GET /api/admin/stats. */
export interface AdminStats {
  users: {
    total: number
    admins: number
    blocked: number
    /** Accounts whose session reached the server within the active window. */
    active: number
  }
  runs: {
    active: number
    byStatus: Record<string, number>
  }
  activeWindowSeconds: number
  generatedAt: string
}

/** The run statuses the page breaks the active runs into, in reading order. */
export const RUN_STATUSES = ['running', 'queued', 'pending'] as const

/** Replaces `{name}` placeholders in a translated string. */
export function fill(template: string, values: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (match, key: string) => (key in values ? String(values[key]) : match))
}

/** The active window in whole minutes, never below one. */
export function windowMinutes(stats: Pick<AdminStats, 'activeWindowSeconds'> | null | undefined): number {
  const seconds = stats?.activeWindowSeconds ?? 0
  return Math.max(1, Math.round(seconds / 60))
}

/**
 * How long ago an instant was, in the interface's language: "3 minutes ago",
 * "il y a 2 heures". An unparseable or missing instant answers null, so the
 * caller says "never" rather than "Invalid Date".
 */
export function relativeTime(iso: string | undefined | null, now: number, locale: string): string | null {
  if (!iso) return null
  const at = Date.parse(iso)
  if (Number.isNaN(at)) return null
  const seconds = Math.round((at - now) / 1000)
  const format = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ['second', 60], ['minute', 60], ['hour', 24], ['day', 30], ['month', 12],
  ]
  let value = seconds
  for (const [unit, size] of steps) {
    if (Math.abs(value) < size) return format.format(value, unit)
    value = Math.round(value / size)
  }
  return format.format(value, 'year')
}
