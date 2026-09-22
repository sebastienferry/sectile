// The repository changelog, read for display.
//
// The desktop app embeds CHANGELOG.md at build time rather than asking the
// server for it: the settings pane has to answer with the agent stopped, and
// "nothing is running" is precisely when somebody comes to check which version
// they have installed.
//
// The file follows Keep a Changelog, whose structure is strict enough to read
// without a Markdown engine: `## [version] - date`, then `### Section`, then
// bullets. Parsing it yields data, and the pane builds DOM nodes from that
// data — never HTML injected from a text file.

/** Strips the inline markup the pane does not render: bold, code, links. */
export function plainText(line) {
  return String(line)
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/\*\*([^*]+)\*\*/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * parseChangelog returns the releases in file order, most recent first. A
 * release with no entry — the empty `## [Unreleased]` of a repository that has
 * just published — is kept as it is: that is information, not a defect to hide.
 */
export function parseChangelog(markdown) {
  const releases = []
  let release = null
  let section = null
  let item = null

  const flushItem = () => {
    if (item && section) section.items.push(plainText(item))
    item = null
  }

  for (const raw of String(markdown || '').split('\n')) {
    const line = raw.replace(/\s+$/, '')

    const heading = /^##\s+\[?([^\]]+?)\]?(?:\s+-\s+(.+))?$/.exec(line)
    if (heading && !line.startsWith('###')) {
      flushItem()
      release = { version: heading[1].trim(), date: (heading[2] || '').trim(), sections: [] }
      section = null
      releases.push(release)
      continue
    }

    const subheading = /^###\s+(.+)$/.exec(line)
    if (subheading && release) {
      flushItem()
      section = { title: subheading[1].trim(), items: [] }
      release.sections.push(section)
      continue
    }

    const bullet = /^[-*]\s+(.+)$/.exec(line)
    if (bullet && section) {
      flushItem()
      item = bullet[1]
      continue
    }

    // A bullet continuing on the next line, indented. Keep a Changelog allows
    // multi-line entries, and cutting them would make them unreadable.
    if (item && /^\s+\S/.test(raw)) {
      item += ' ' + raw.trim()
      continue
    }

    flushItem()
  }
  flushItem()

  return releases
}

/**
 * isPublished says whether a release has actually been cut: it carries entries
 * and a version number rather than the `Unreleased` heading Keep a Changelog
 * reserves for work that has shipped to nobody.
 *
 * The distinction only started to matter once `Unreleased` held entries. Until
 * then "the first release with entries" and "the most recent published
 * release" named the same thing, and the fallback below read as correct while
 * being one entry away from answering `Unreleased` to somebody asking which
 * version they have installed.
 */
export function isPublished(release) {
  if (!release || String(release.version).trim().toLowerCase() === 'unreleased') return false
  return release.sections.some(section => section.items.length > 0)
}

/**
 * releaseNotesFor returns the requested release, or the most recent published
 * one when the installed version does not appear in the file — which is the
 * case of every build made outside a tag.
 */
export function releaseNotesFor(releases, version) {
  const wanted = String(version || '').replace(/^v/, '')
  const match = releases.find(entry => entry.version.replace(/^v/, '') === wanted)
  if (match) return match
  return releases.find(isPublished) || null
}
