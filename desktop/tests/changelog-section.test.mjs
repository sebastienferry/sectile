import test from 'node:test'
import assert from 'node:assert/strict'
import {spawnSync} from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {fileURLToPath} from 'node:url'
import {changelogSection} from '../../scripts/release/changelog-section.mjs'

const script = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../scripts/release/changelog-section.mjs')

const sample = `# Changelog

## [Unreleased]

### Added

- Not released yet.

## [1.2.0] - 2026-01-31

### Added

- A **bold** thing with \`code\`.

- An entry that runs
  over two lines.

### Fixed

- One fix. (#12)

## [1.1.0] - 2026-01-01

### Fixed

- An older fix.

## [1.0.0] - 2025-12-01

[Unreleased]: https://example.com/compare/v1.2.0...HEAD
[1.2.0]: https://example.com/releases/tag/v1.2.0
`

test('a section in the middle is its body, Markdown and subsections kept', () => {
  assert.equal(changelogSection(sample, '1.2.0'), `### Added

- A **bold** thing with \`code\`.

- An entry that runs
  over two lines.

### Fixed

- One fix. (#12)`)
})

test('the last section with entries stops at the next release heading', () => {
  assert.equal(changelogSection(sample, '1.1.0'), '### Fixed\n\n- An older fix.')
})

test('a missing or empty section is an error naming the version', () => {
  assert.throws(() => changelogSection(sample, '0.9.0'), /CHANGELOG.md has no section for 0.9.0/)
  assert.throws(() => changelogSection(sample, '1.0.0'), /CHANGELOG.md has an empty section for 1.0.0/)
})

test('a version never matches another one or the Unreleased section', () => {
  assert.throws(() => changelogSection(sample, '1.2'), /no section for 1.2/)
  assert.throws(() => changelogSection(sample, '1x2x0'), /no section for 1x2x0/)
  assert.throws(() => changelogSection('## [Unreleased]\n\n- Soon.\n', '0.1.0'), /no section for 0.1.0/)
})

test('the command prints the notes, or fails with the reason on stderr', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-changelog-'))
  try {
    const file = path.join(directory, 'CHANGELOG.md')
    fs.writeFileSync(file, sample)
    const found = spawnSync(process.execPath, [script, '1.1.0', file], {encoding: 'utf8'})
    assert.equal(found.status, 0)
    assert.equal(found.stdout, '### Fixed\n\n- An older fix.\n')
    const missing = spawnSync(process.execPath, [script, '0.9.0', file], {encoding: 'utf8'})
    assert.equal(missing.status, 1)
    assert.equal(missing.stdout, '')
    assert.match(missing.stderr, /CHANGELOG.md has no section for 0.9.0/)
  } finally {
    fs.rmSync(directory, {recursive: true, force: true})
  }
})
