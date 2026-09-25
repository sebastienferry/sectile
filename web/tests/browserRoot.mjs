// Vite root for the browser tests (*.browser.mjs).
//
// Sectile names task worktrees after the ticket key, e.g. .tasks/worktrees/#387,
// and Vite reads a '#' in an absolute path as the start of a URL fragment: the
// dependency scan fails and every module id is cut short, so the page never
// renders and each test times out as if a component were broken.
//
// Serving the checkout through a '#'-free symbolic link is not enough on its
// own: Vite also serves files from where it is installed (its client, at
// /@vite/client), and Node resolves the test's imports to their real paths, so
// a worktree with its own node_modules still hands Vite a path holding '#'.
// A test started from such a checkout is therefore run again through the link,
// with Node told to keep symbolic links, so that the whole process, Vite
// included, only ever sees the '#'-free path.
import { spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync, symlinkSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const RELAUNCHED = 'SECTILE_BROWSER_TEST_RELAUNCHED'

/**
 * Returns the root the Vite server of the test at `testUrl` (its
 * `import.meta.url`) must use, and the `resolve.preserveSymlinks` value that
 * goes with it. The root is `web/`, in forward slashes and without a trailing
 * slash, because the fixtures match Vite's module ids against it.
 *
 * From a checkout whose path holds '#', it does not return: it links the
 * checkout from a fresh temporary directory, runs the same test through the
 * link and exits with its status. The link names the checkout, not `web/`, so
 * that `web/`'s imports of `../../../shared` still land in the same checkout.
 * It is removed when the process exits; removing a link never touches what it
 * points to.
 */
export function browserRoot(testUrl, {
  tempDir = tmpdir(),
  env = process.env,
  onExit = callback => process.once('exit', callback),
  relaunch = relaunchThrough,
} = {}) {
  const web = fileURLToPath(new URL('..', testUrl)).replace(/\\/g, '/').replace(/\/$/, '')
  if (!web.includes('#')) return { root: web, preserveSymlinks: env[RELAUNCHED] === '1' }
  if (env[RELAUNCHED] === '1') throw new Error(`browser tests: ${web} still contains '#' after the relaunch through a link`)
  const checkout = web.slice(0, web.lastIndexOf('/'))
  const dir = mkdtempSync(join(tempDir, 'sectile-browser-'))
  if (dir.includes('#')) {
    rmSync(dir, { recursive: true, force: true })
    throw new Error(`browser tests: the temporary directory ${dir} also contains '#'; set TMPDIR to a path without it`)
  }
  const link = join(dir, 'checkout')
  symlinkSync(checkout, link, 'dir')
  onExit(() => rmSync(dir, { recursive: true, force: true }))
  relaunch(join(link, relative(checkout, fileURLToPath(testUrl))), { ...env, [RELAUNCHED]: '1' })
}

function relaunchThrough(script, env) {
  const { status, signal, error } = spawnSync(process.execPath,
    [...process.execArgv, '--preserve-symlinks', '--preserve-symlinks-main', script, ...process.argv.slice(2)],
    { stdio: 'inherit', env })
  if (error) throw error
  process.exit(signal ? 1 : status)
}
