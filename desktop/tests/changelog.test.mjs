import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {fileURLToPath} from 'node:url'
import {parseChangelog, plainText, releaseNotesFor} from '../src/changelog.mjs'

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')

const sample = `# Changelog

Some preamble nobody shows.

## [Unreleased]

## [1.2.0] - 2026-01-31

### Added

- A **bold** thing with \`code\` and a [link](https://example.com).
- An entry that runs
  over two lines.

### Fixed

- One fix.

## [1.1.0] - 2026-01-01

### Fixed

- An older fix.
`

test('inline markup is removed rather than shown as punctuation', () => {
  assert.equal(plainText('A **bold** thing with `code` and a [link](https://x)'), 'A bold thing with code and a link')
})

test('releases are read in file order with their sections and entries', () => {
  const releases = parseChangelog(sample)
  assert.deepEqual(releases.map(release => release.version), ['Unreleased', '1.2.0', '1.1.0'])
  assert.equal(releases[1].date, '2026-01-31')
  assert.deepEqual(releases[1].sections.map(section => section.title), ['Added', 'Fixed'])
  assert.equal(releases[1].sections[0].items.length, 2)
})

test('an entry spanning several lines stays one entry', () => {
  const [, current] = parseChangelog(sample)
  assert.equal(current.sections[0].items[1], 'An entry that runs over two lines.')
})

test('an empty Unreleased section is kept, not hidden', () => {
  const [unreleased] = parseChangelog(sample)
  assert.deepEqual(unreleased.sections, [])
})

test('the installed version selects its own notes', () => {
  const releases = parseChangelog(sample)
  assert.equal(releaseNotesFor(releases, 'v1.1.0').version, '1.1.0')
  assert.equal(releaseNotesFor(releases, '1.2.0').version, '1.2.0')
})

// A development build carries no release number, and showing it nothing at all
// would read as "this app has no history".
test('an unknown version falls back to the latest release that has entries', () => {
  const releases = parseChangelog(sample)
  assert.equal(releaseNotesFor(releases, 'dev').version, '1.2.0')
})

test('parsing survives an empty or missing changelog', () => {
  assert.deepEqual(parseChangelog(''), [])
  assert.deepEqual(parseChangelog(undefined), [])
  assert.equal(releaseNotesFor([], 'v1.0.0'), null)
})

// The file the app embeds is the repository's own; a rename or a format change
// that this parser cannot read must fail here rather than in a settings panel.
test('the repository changelog parses into at least one release with entries', () => {
  const releases = parseChangelog(fs.readFileSync(path.join(repositoryRoot, 'CHANGELOG.md'), 'utf8'))
  assert.ok(releases.length > 0, 'no release read from CHANGELOG.md')
  const published = releases.find(release => release.sections.some(section => section.items.length > 0))
  assert.ok(published, 'no release with entries in CHANGELOG.md')
  assert.match(published.version, /^\d+\.\d+\.\d+$/)
})
