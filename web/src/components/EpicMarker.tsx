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
 * Short coloured bar inside the left edge of the nearest positioned ancestor,
 * for the places that do not show the epic key. It is inset from the top, the
 * bottom and the border so it never meets a rounded corner, and it is a child
 * element rather than a border so the border and the ring, which carry the
 * running, queued and selected states, are left as they are.
 */
export function EpicBar({ parentKey }: { parentKey?: string | null }) {
  const color = epicColorHex(parentKey)
  if (!color) return null
  return (
    <span
      aria-hidden="true"
      data-epic-bar
      className="absolute left-[3px] top-[22%] bottom-[22%] m-0 w-[3px] rounded-full pointer-events-none"
      style={{ backgroundColor: color }}
    />
  )
}
