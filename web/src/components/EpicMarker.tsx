import { useCallback } from 'react'
import { useApp } from '../context/AppContext'
import { epicColorHex, epicColorsEnabled } from '../lib/epicColor'

/**
 * Returns whether a task of the given project shows its epic's colour, so a
 * view listing tasks of several projects paints each one by its own setting.
 */
export function useEpicColors(): (projectId?: string | null) => boolean {
  const { projects, currentProject } = useApp()
  return useCallback(
    (projectId?: string | null) => epicColorsEnabled(projects, projectId, currentProject),
    [projects, currentProject]
  )
}

/**
 * Coloured bar along the left edge of the nearest positioned ancestor, over its
 * full height.
 *
 * It is a transparent layer covering the ancestor, with the ancestor's own
 * corner radius, that paints an inset shadow on its left side. The browser
 * clips an inset shadow to the rounded shape of its box, so the bar follows the
 * card's corners whatever their radius; a plain 3px-wide strip would instead
 * clamp that radius to its own width and overflow the curve. The layer carries
 * its own box-shadow, so the border, the ring and the hover shadow of the card,
 * which express the running, queued and selected states, are left as they are.
 */
export function EpicBar({ parentKey }: { parentKey?: string | null }) {
  const color = epicColorHex(parentKey)
  if (!color) return null
  return (
    <span
      aria-hidden="true"
      data-epic-bar
      className="absolute inset-0 m-0 rounded-[inherit] pointer-events-none"
      style={{ boxShadow: `inset 3px 0 0 ${color}` }}
    />
  )
}
