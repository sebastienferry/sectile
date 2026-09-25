/**
 * Live MCP clients, as the status bar tells them.
 *
 * A session is a connection the server observes, not work someone declared: it
 * says who is attached right now and what each of them would leave behind. The
 * runs a session owns matter because they are the ones the server closes when
 * that client goes away.
 */

/** One live session, as returned by GET /api/mcp/sessions. */
export interface McpSession {
  id: string
  /** The server instance holding the session, when several serve the deployment. */
  instance?: string
  client: string
  title?: string
  version?: string
  connectedAt: string
  runs: string[]
}

/**
 * The answer of GET /api/mcp/sessions: the sessions of every live server
 * instance, and the instances that did not answer in time.
 */
export interface McpSessionsResponse {
  sessions: McpSession[]
  unreachable?: string[]
}

/** What the status bar counts: attached clients and the runs they hold. */
export interface McpSessionSummary {
  clients: number
  runs: number
}

export const summarizeSessions = (sessions: McpSession[]): McpSessionSummary => ({
  clients: sessions.length,
  runs: sessions.reduce((total, session) => total + (session.runs?.length || 0), 0),
})

/**
 * A client's display name. Clients describe themselves twice: a programmatic
 * name shared by every instance, and a title naming this one. The title is what
 * tells two windows of the same client apart, so it wins when present.
 */
export const sessionLabel = (session: McpSession): string => {
  const title = (session.title || '').trim()
  const client = (session.client || '').trim()
  if (title && client && title !== client) return `${client} · ${title}`
  return title || client || 'unknown client'
}

/**
 * How long a client has been attached, in a compact form that needs no
 * translation. Below a minute the exact count is noise, so it reads as seconds
 * only until the first minute.
 */
export const connectedFor = (connectedAt: string, now: number = Date.now()): string => {
  const start = new Date(connectedAt)
  if (!connectedAt || Number.isNaN(start.getTime())) return ''
  const seconds = Math.max(0, Math.floor((now - start.getTime()) / 1000))
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  return `${Math.floor(hours / 24)}d`
}

/** Most recently connected first, which is the order the server already sends. */
export const sortSessions = (sessions: McpSession[]): McpSession[] =>
  [...sessions].sort((left, right) => {
    const difference = new Date(right.connectedAt).getTime() - new Date(left.connectedAt).getTime()
    return difference !== 0 && !Number.isNaN(difference) ? difference : left.id.localeCompare(right.id)
  })
