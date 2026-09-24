/**
 * Roadmap projects: other Jira project keys whose story keys a project's
 * slicing attaches to a line. The field is typed as a comma-separated list;
 * the server normalises it the same way (upper case, no duplicates, without
 * the project's own key).
 */

/** Renders the stored list for the input. */
export const formatProjectKeyList = (keys: string[] | undefined): string => (keys || []).join(', ')

/** Reads the input: split on commas, semicolons and blanks, upper-cased, deduplicated, own key dropped. */
export const parseProjectKeyList = (raw: string, ownKey = ''): string[] => {
  const own = ownKey.trim().toUpperCase()
  const out: string[] = []
  for (const part of raw.split(/[\s,;]+/)) {
    const key = part.trim().toUpperCase()
    if (key && key !== own && !out.includes(key)) out.push(key)
  }
  return out
}
