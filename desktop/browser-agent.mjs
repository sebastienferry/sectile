import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

const readable = new Set([
  '/desktop/status', '/desktop/version', '/desktop/projects', '/desktop/project',
  '/desktop/runs', '/desktop/tasks', '/desktop/tasks/detail', '/desktop/tasks/assignable',
  '/desktop/tasks/issue-types', '/desktop/workstation', '/desktop/engines',
  '/desktop/task-engines', '/desktop/repositories', '/desktop/folders',
  '/desktop/conversation', '/desktop/run-result', '/desktop/git-diff',
  '/desktop/codex-settings', '/desktop/agy-settings', '/desktop/workstation/sandbox',
])

// This is a development-only, read-only bridge. Credentials never enter the
// renderer, and other web origins cannot use it as a local-agent gateway.
export function browserAgent({ readConnection = async () => JSON.parse(await fs.readFile(
  path.join(os.homedir(), '.taskflow', 'agent-connection.json'), 'utf8')),
  request = fetch } = {}) {
  return async (req, res, next) => {
    if (!req.url.startsWith('/__desktop/')) return next()
    res.setHeader('Content-Type', 'application/json')
    res.setHeader('Cache-Control', 'no-store')
    const fail = (status, message) => { res.statusCode = status; res.end(JSON.stringify({ error: message })) }
    const host = req.headers.host || ''
    if (!/^(localhost|127\.0\.0\.1):\d+$/.test(host) ||
        req.headers['x-sectile-preview'] !== '1' ||
        (req.headers.origin && req.headers.origin !== 'http://' + host) ||
        req.headers['sec-fetch-site'] === 'cross-site') return fail(403, 'Forbidden')
    if (req.method !== 'GET') return fail(405, 'The canvas connection is read-only. Use Sectile Desktop to make changes.')
    const route = new URL(req.url.replace('/__desktop/', '/desktop/'), 'http://localhost')
    if (!readable.has(route.pathname)) return fail(404, 'This operation is not available in the canvas.')
    try {
      const connection = await readConnection()
      const target = new URL(connection.url)
      if (target.protocol !== 'http:' || target.hostname !== '127.0.0.1' ||
          target.username || target.password || !connection.token) throw Error('Invalid connection')
      const response = await request(new URL(route.pathname + route.search, target), {
        headers: { Authorization: 'Bearer ' + connection.token },
        redirect: 'error', signal: AbortSignal.timeout(15000),
      })
      res.statusCode = response.status
      res.end(await response.text())
    } catch {
      fail(503, 'Cannot reach the local Sectile agent. Open Sectile Desktop and connect it, then retry here.')
    }
  }
}
