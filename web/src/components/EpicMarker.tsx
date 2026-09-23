import { epicColorHex } from '../lib/epicColor'

interface EpicMarkerProps {
  parentKey?: string | null
}

/**
 * Coloured bar on the left edge of the nearest positioned ancestor. It is a
 * child element rather than a border or a box-shadow so that the border and
 * the ring, which already carry the running, queued and selected states, are
 * left as they are.
 */
export function EpicBar({ parentKey }: EpicMarkerProps) {
  const color = epicColorHex(parentKey)
  if (!color) return null
  return (
    <span
      aria-hidden="true"
      data-epic-bar
      className="absolute left-0 top-0 bottom-0 m-0 w-[3px] rounded-l-[inherit] pointer-events-none"
      style={{ backgroundColor: color }}
    />
  )
}

/** Coloured dot placed in front of the epic key, or of the task key when the epic is not shown. */
export function EpicDot({ parentKey, className = '' }: EpicMarkerProps & { className?: string }) {
  const color = epicColorHex(parentKey)
  if (!color) return null
  return (
    <span
      role="img"
      aria-label={`Épic ${parentKey!.trim()}`}
      title={`Épic ${parentKey!.trim()}`}
      data-epic-dot
      className={`inline-block shrink-0 w-1.5 h-1.5 rounded-full self-center ${className}`}
      style={{ backgroundColor: color }}
    />
  )
}
