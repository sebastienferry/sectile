import { runState, type IconNode } from '../../../shared/runStates'

/**
 * Renders a run state's glyph from the shared definition, as an inline SVG.
 *
 * The icon nodes are data — a tag and its attributes — and turning them into
 * elements is exactly the code that drifts once it exists twice, which is why
 * every web surface showing a run state draws it here. The glyph inherits the
 * colour of its container: the badge around it already carries the state's own,
 * and a stroke set here would fight it.
 */
export function RunStateGlyph({ state, size = 12, className }: {
  state: string
  size?: number
  className?: string
}) {
  const definition = runState(state)
  if (!definition) return null
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"
      className={className}>
      {definition.icon.map(([tag, attributes]: IconNode, index: number) =>
        tag === 'circle' ? <circle key={index} {...attributes} />
          : tag === 'line' ? <line key={index} {...attributes} />
            : <path key={index} {...attributes} />)}
    </svg>
  )
}
