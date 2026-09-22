import type { SVGProps } from 'react'

/**
 * Props shared by the vendored brand marks.
 *
 * `size` drives both `width` and `height`, so a caller sets one number instead of
 * two. SVG has no `size` attribute, so it is removed from the SVG props before
 * being redeclared — without the `Omit`, a caller passing `size={12}` would be
 * checked against whatever the DOM typings happen to carry under that name.
 * Everything else an `<svg>` accepts stays available and is spread as-is.
 */
export type BrandIconProps = Omit<SVGProps<SVGSVGElement>, 'size'> & {
  size?: string | number
}
