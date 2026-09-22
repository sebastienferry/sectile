/**
 * Brand marks vendored from `@lobehub/icons`, which the project no longer depends on.
 *
 * The UI used four of its icons; satisfying the package's non-optional peers installed
 * `@lobehub/ui`, `antd` and `@emoji-mart/react` — the last one demanding React 16-18
 * against the project's React 19, which is what made `npm ci` print an ERESOLVE block
 * on every build. None of that tree ever reached the bundle. See #344.
 */
export { Antigravity } from './Antigravity'
export { Claude } from './Claude'
export { Cursor } from './Cursor'
export { OpenAI } from './OpenAI'
export type { BrandIconProps } from './types'
