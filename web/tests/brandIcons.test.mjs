import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// The brand marks are vendored SVG, not behaviour: what needs pinning is the shape
// of each component and the absence of the dependency they replaced. Both are read
// as text, so this suite runs without a build step, like the other ones here.
const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

const NAMES = ['Antigravity', 'Claude', 'OpenAI', 'Cursor']

const manifest = await read('../package.json')
const lockfile = await read('../package-lock.json')
const barrel = await read('../src/components/icons/index.ts')
const icons = Object.fromEntries(
  await Promise.all(NAMES.map(async (name) => [name, await read(`../src/components/icons/${name}.tsx`)])),
)

test('the manifest no longer depends on @lobehub/icons', () => {
  const pkg = JSON.parse(manifest)
  assert.equal(pkg.dependencies['@lobehub/icons'], undefined)
  // Nothing took its place: the four marks live in the repository now.
  assert.ok(!Object.keys(pkg.dependencies).some((name) => name.startsWith('@lobehub/')))
  // The release procedure owns this field (AGENTS.md §4), not a dependency change.
  assert.equal(pkg.version, '0.1.0')
})

test('the removed peer tree stays out of the lockfile', () => {
  // This is the regression guard on the ERESOLVE block itself: npm printed it because
  // @lobehub/icons' non-optional peers pulled @lobehub/ui, which depends on
  // @emoji-mart/react, which wants React 16-18 against this project's React 19. The
  // warning cannot come back while these packages are absent.
  for (const absent of ['@lobehub/icons', '@lobehub/ui', '@lobehub/streamdown', '@lobehub/fluent-emoji', '@emoji-mart/react', 'node_modules/antd']) {
    assert.ok(!lockfile.includes(absent), `${absent} is back in the lockfile`)
  }
})

test('every vendored mark keeps the contract of the package it replaces', () => {
  for (const name of NAMES) {
    const source = icons[name]
    assert.match(source, /viewBox="0 0 24 24"/, `${name} lost its viewBox`)
    assert.match(source, /fill="currentColor"/, `${name} stopped following the text colour`)
    assert.match(source, /fillRule="evenodd"/, `${name} lost its fill rule`)
    // One size prop drives both dimensions, defaulting to the surrounding font size.
    assert.match(source, /size = '1em'/, `${name} lost its default size`)
    assert.match(source, /height=\{size\}/, `${name} stopped binding its height`)
    assert.match(source, /width=\{size\}/, `${name} stopped binding its width`)
    // A caller's style spreads last, so it overrides the two defaults.
    assert.match(source, /style=\{\{ flex: 'none', lineHeight: 1, \.\.\.style \}\}/, `${name} lost its style defaults`)
    // The rest of the props reach the svg, which is how className keeps working.
    assert.match(source, /\{\.\.\.rest\}/, `${name} stopped forwarding its props`)
    assert.match(source, new RegExp(`<title>${name}</title>`), `${name} lost its accessible title`)
    assert.match(source, /export const \w+ = memo</, `${name} is no longer memoised`)
    // One path per mark, copied verbatim from @lobehub/icons@5.18.0.
    assert.equal(source.match(/<path d="/g).length, 1, `${name} should render exactly one path`)
  }
})

test('the barrel exports the four marks and their props type', () => {
  for (const name of NAMES) {
    assert.match(barrel, new RegExp(`export \\{ ${name} \\} from './${name}'`), `${name} is not re-exported`)
  }
  assert.match(barrel, /export type \{ BrandIconProps \} from '\.\/types'/)
})

test('every call site imports the marks from the local barrel', async () => {
  const callSites = {
    'ProfileModal.tsx': ['Antigravity', 'Claude', 'OpenAI'],
  }
  for (const [file, names] of Object.entries(callSites)) {
    const source = await read(`../src/components/${file}`)
    const line = source.match(/^import \{[^}]*\} from '\.\/icons'$/m)
    assert.ok(line, `${file} does not import from the local barrel`)
    for (const name of names) {
      assert.ok(line[0].includes(name), `${file} stopped importing ${name}`)
    }
    // No call site may reach for the package again.
    assert.ok(!source.includes('@lobehub'), `${file} still mentions @lobehub`)
  }
})
