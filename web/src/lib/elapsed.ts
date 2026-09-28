import { format } from './i18n.ts'
import { shell } from '../locales/shell.ts'

/**
 * Temps écoulé, dit comme on le dirait à l'oral.
 *
 * Sur un board, ce qui compte n'est pas la date mais la durée : un ticket en
 * cours depuis trois jours et un autre depuis deux mois ne demandent pas la même
 * attention, et la date brute oblige à faire le calcul de tête.
 */

/** The units of a short elapsed time, from `t.shell.elapsed`. */
export interface ElapsedUnits {
  lessThanHour: string
  hours: string
  days: string
  weeks: string
  months: string
}

/**
 * Short duration for a label: "3 j", "2 sem.", "5 mois" in French. Callers that
 * do not pass the catalog get the French units.
 */
export const shortElapsed = (since?: string, units: ElapsedUnits = shell.fr.elapsed): string => {
  if (!since) return ''
  const start = new Date(since)
  if (Number.isNaN(start.getTime())) return ''

  const hours = Math.max(0, (Date.now() - start.getTime()) / 3_600_000)
  if (hours < 1) return units.lessThanHour
  if (hours < 24) return format(units.hours, { count: Math.floor(hours) })

  const days = Math.floor(hours / 24)
  if (days < 14) return format(units.days, { count: days })
  if (days < 60) return format(units.weeks, { count: Math.floor(days / 7) })
  return format(units.months, { count: Math.floor(days / 30) })
}

/**
 * Seuil au delà duquel une durée mérite d'être signalée en couleur. Deux
 * semaines dans la même catégorie de statut, c'est un ticket qui n'avance plus.
 */
export const STALE_ELAPSED_DAYS = 14

export const isElapsedStale = (since?: string): boolean => {
  if (!since) return false
  const start = new Date(since)
  if (Number.isNaN(start.getTime())) return false
  return (Date.now() - start.getTime()) / 86_400_000 >= STALE_ELAPSED_DAYS
}
