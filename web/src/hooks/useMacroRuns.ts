import { useCallback, useEffect, useRef, useState } from 'react'
import { activeMacroRun, fetchMacroRuns, type MacroRun } from '../lib/macroRuns'

/** While a run is active, the panel re-reads the macro's runs this often. */
const ACTIVE_POLL_MS = 5000
/** Otherwise it looks now and then, for a run started by hand through MCP. */
const IDLE_POLL_MS = 30000

/**
 * The skill runs of one macro, read once for every skill button of the panel.
 * onRunEnded is called when the run that was active at the previous read no
 * longer is: what the skill saved can then be read again. With no macro it
 * reads nothing.
 */
export function useMacroRuns(projectId: string, macroKey: string, onRunEnded?: () => void) {
  const [runs, setRuns] = useState<MacroRun[]>([])
  // The macro the latest read was for: an answer for a macro the panel has
  // since left is dropped rather than shown on the new one.
  const current = useRef(`${projectId}/${macroKey}`)
  current.current = `${projectId}/${macroKey}`
  const ended = useRef(onRunEnded)
  useEffect(() => { ended.current = onRunEnded })

  const refresh = useCallback(async () => {
    if (!projectId || !macroKey) return
    const asked = `${projectId}/${macroKey}`
    const answer = await fetchMacroRuns(projectId, macroKey)
    if (current.current === asked) setRuns(answer)
  }, [projectId, macroKey])

  useEffect(() => {
    setRuns([])
    void refresh()
  }, [refresh])

  const active = activeMacroRun(runs)
  useEffect(() => {
    const id = setInterval(() => { void refresh() }, active ? ACTIVE_POLL_MS : IDLE_POLL_MS)
    return () => clearInterval(id)
  }, [active, refresh])

  // Keyed on the macro too, so leaving a macro during its run is not taken for
  // the run's end.
  const seen = useRef<{ macro: string, runId: string } | null>(null)
  useEffect(() => {
    const macro = current.current
    const previous = seen.current
    seen.current = active ? { macro, runId: active.id } : null
    if (previous && previous.macro === macro && previous.runId !== active?.id) ended.current?.()
  }, [active])

  return { runs, refresh }
}
