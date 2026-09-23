// Vite root for the browser tests (*.browser.mjs).
//
// Sectile names task worktrees after the ticket key, e.g. .tasks/worktrees/#387,
// and Vite reads a '#' in an absolute path as the start of a URL fragment: the
// dependency scan fails and every module id is cut short, so the page never
// renders and each test times out as if a component were broken. A checkout
// whose path holds '#' is therefore served through a symbolic link to it from a
// fresh temporary directory, and Vite is told to keep that link instead of
// resolving it back to the real path.
import { mkdtempSync, rmSync, symlinkSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

/**
 * Returns the root the Vite server of the test at `testUrl` (its
 * `import.meta.url`) must use, and the `resolve.preserveSymlinks` value that
 * goes with it. The root is `web/`, in forward slashes and without a trailing
 * slash, because the fixtures match Vite's module ids against it.
 *
 * The link names the checkout, not `web/`, so that `web/`'s imports of
 * `../../../shared` still land in the same checkout. It is removed when the
 * process exits; removing a link never touches what it points to.
 */
export function browserRoot(testUrl, { tempDir = tmpdir(), onExit = callback => process.once('exit', callback) } = {}) {
  const web = fileURLToPath(new URL('..', testUrl)).replace(/\\/g, '/').replace(/\/$/, '')
  if (!web.includes('#')) return { root: web, preserveSymlinks: false }
  const checkout = web.slice(0, web.lastIndexOf('/'))
  const dir = mkdtempSync(join(tempDir, 'sectile-browser-'))
  if (dir.includes('#')) {
    rmSync(dir, { recursive: true, force: true })
    throw new Error(`browser tests: the temporary directory ${dir} also contains '#'; set TMPDIR to a path without it`)
  }
  const link = join(dir, 'checkout')
  symlinkSync(checkout, link, 'dir')
  onExit(() => rmSync(dir, { recursive: true, force: true }))
  return { root: join(link, 'web').replace(/\\/g, '/'), preserveSymlinks: true }
}
