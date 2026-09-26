#!/usr/bin/env node
//
// Print the release notes of one version, read from CHANGELOG.md.
//
// Usage: node scripts/release/changelog-section.mjs <X.Y.Z> [CHANGELOG.md]
//
// The GitHub release workflow uses it for the notes of the release it creates:
// the body of `## [X.Y.Z] - date`, heading excluded, up to the next release
// heading or the link definitions at the bottom of the file. The Markdown is
// kept as written, since GitHub renders it. A version with no section, or an
// empty one, is an error: a release is not published without its notes.

import fs from 'node:fs'
import path from 'node:path'
import {fileURLToPath} from 'node:url'

export function changelogSection(markdown, version) {
  const heading = new RegExp(`^##\\s+\\[${version.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\](\\s|$)`)
  const lines = String(markdown).split('\n')
  const start = lines.findIndex(line => heading.test(line))
  if (start === -1) throw Error(`CHANGELOG.md has no section for ${version}`)
  const body = []
  for (const line of lines.slice(start + 1)) {
    if (/^##\s/.test(line) || /^\[[^\]]+\]:\s/.test(line)) break
    body.push(line)
  }
  const notes = body.join('\n').trim()
  if (!notes) throw Error(`CHANGELOG.md has an empty section for ${version}`)
  return notes
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [version, file = 'CHANGELOG.md'] = process.argv.slice(2)
  if (!version) {
    console.error('usage: changelog-section.mjs <X.Y.Z> [CHANGELOG.md]')
    process.exit(2)
  }
  try {
    process.stdout.write(changelogSection(fs.readFileSync(file, 'utf8'), version) + '\n')
  } catch (error) {
    console.error(error.message)
    process.exit(1)
  }
}
