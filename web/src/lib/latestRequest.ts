/**
 * Ordering of overlapping reads (#581).
 *
 * The board reloads its tickets from several places at once: a filter change,
 * a project switch, a pushed ticket update, the refresh after a run. Their
 * answers arrive in any order, and applying them as they come let an older
 * answer, with other filters or none, replace the board a newer request had
 * just filled. Each request takes a ticket, and only the holder of the newest
 * ticket may write.
 */

/** A counter that tells whether an answer belongs to the newest request. */
export interface LatestRequest {
  /** Starts a request and returns its ticket. */
  begin(): number
  /**
   * The newest ticket, for a read that joins the newest request rather than
   * superseding it: its answer is kept only if nothing started after it.
   */
  current(): number
  /** True while no request was started after the one holding `ticket`. */
  isLatest(ticket: number): boolean
}

export const createLatestRequest = (): LatestRequest => {
  let latest = 0
  return {
    begin: () => ++latest,
    current: () => latest,
    isLatest: ticket => ticket === latest,
  }
}
