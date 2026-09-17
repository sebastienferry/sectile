/**
 * The one definition of what a run state looks like.
 *
 * Every surface that shows a run state reads it here: the badge on a task row
 * in the web UI, and the icon on the notification the desktop application
 * raises. That is the point — a banner whose glyph says something the list does
 * not is one more thing to decode. Adding a state means adding one entry, and
 * neither surface can show a state the other does not know.
 *
 * The geometry is lucide's, so these glyphs are the ones used everywhere else
 * in the interface rather than a second icon set drawn beside it.
 */

/** An SVG element of a glyph: its tag and its attributes. */
export type IconNode = [string, Record<string, string | number>]

export interface RunState {
  /** Stable identifier, used as the key on both surfaces. */
  id: string
  /** What the state is called, in the interface's own terms. */
  label: string
  /** How a notification says it, after the session name. */
  announcement: string
  /** The state's colour, as a literal so a notification icon can carry it. */
  color: string
  /** Whether the glyph turns while the state lasts. Ignored on a notification. */
  spins?: boolean
  icon: IconNode[]
}

const CIRCLE: IconNode = ['circle', { cx: 12, cy: 12, r: 10 }]

export const RUN_STATES = {
  waiting: {
    id: 'waiting',
    label: 'Waiting for you',
    announcement: 'is waiting for you',
    color: '#f59e0b',
    icon: [
      ['path', { d: 'M18 11V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2' }],
      ['path', { d: 'M14 10V4a2 2 0 0 0-2-2a2 2 0 0 0-2 2v2' }],
      ['path', { d: 'M10 10.5V6a2 2 0 0 0-2-2a2 2 0 0 0-2 2v8' }],
      ['path', { d: 'M18 8a2 2 0 1 1 4 0v6a8 8 0 0 1-8 8h-2c-2.8 0-4.5-.86-5.99-2.34l-3.6-3.6a2 2 0 0 1 2.83-2.82L7 15' }],
    ],
  },
  running: {
    id: 'running',
    label: 'Running',
    announcement: 'is running',
    color: '#60a5fa',
    spins: true,
    icon: [['path', { d: 'M21 12a9 9 0 1 1-6.219-8.56' }]],
  },
  queued: {
    id: 'queued',
    label: 'Queued',
    announcement: 'is queued',
    color: '#fbbf24',
    icon: [CIRCLE, ['path', { d: 'M12 6v6l4 2' }]],
  },
  completed: {
    id: 'completed',
    label: 'Finished',
    announcement: 'finished its turn',
    color: '#34d399',
    icon: [CIRCLE, ['path', { d: 'm9 12 2 2 4-4' }]],
  },
  failed: {
    id: 'failed',
    label: 'Failed',
    announcement: 'failed',
    color: '#fb7185',
    icon: [
      ['path', { d: 'm21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3' }],
      ['path', { d: 'M12 9v4' }],
      ['path', { d: 'M12 17h.01' }],
    ],
  },
  canceled: {
    id: 'canceled',
    label: 'Cancelled',
    announcement: 'was cancelled',
    color: '#94a3b8',
    icon: [CIRCLE, ['line', { x1: 9, x2: 15, y1: 15, y2: 9 }]],
  },
} as const satisfies Record<string, RunState>

export type RunStateId = keyof typeof RUN_STATES

/** Reads a state by id, or nothing when the id names no defined state. */
export function runState(id: string): RunState | undefined {
  return (RUN_STATES as Record<string, RunState>)[id]
}

/**
 * Serialises a state's glyph as standalone SVG markup, so a surface that cannot
 * render a component — a notification icon — still draws the same glyph.
 * `size` is in pixels; notifications want a larger canvas than a badge.
 */
export function runStateSvg(id: string, size = 64): string {
  const state = runState(id)
  if (!state) return ''
  const elements = state.icon
    .map(([tag, attributes]) => {
      const pairs = Object.entries(attributes).map(([name, value]) => `${name}="${value}"`)
      return `<${tag} ${pairs.join(' ')}/>`
    })
    .join('')
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24"`
    + ` fill="none" stroke="${state.color}" stroke-width="2" stroke-linecap="round"`
    + ` stroke-linejoin="round">${elements}</svg>`
}

/**
 * The same glyph as a data URL, which is the form an Electron notification icon
 * is built from. Base64 rather than percent-encoding: the markup carries quotes
 * and `#`, which a URL would have to escape one by one. `btoa` is global in the
 * browser and in Node, and the markup is ASCII, which is all it accepts.
 */
export function runStateIconDataUrl(id: string, size = 64): string {
  const svg = runStateSvg(id, size)
  if (!svg) return ''
  return 'data:image/svg+xml;base64,' + btoa(svg)
}
