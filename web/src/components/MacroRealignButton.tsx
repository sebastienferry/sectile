import React, { useCallback, useEffect, useState } from 'react'
import { GitCompareArrows, Loader2 } from 'lucide-react'
import { useAgentStatus } from '../hooks/useAgentStatus'
import { activeMacroRun, fetchMacroRuns, launchMacroSkill, macroLaunchBlocker, type MacroRun } from '../lib/macroRuns'

interface Props {
  projectId: string
  macroKey: string
  onError: (message: string) => void
  onLaunched: (message: string) => void
}

/** While a run is active, the panel re-reads the macro's runs this often. */
const ACTIVE_POLL_MS = 5000

/**
 * Launches realign-macro for one macro on the local agent, through the same
 * Run the desktop app shows for a task skill, and shows the run while it is
 * active.
 */
export const MacroRealignButton: React.FC<Props> = ({ projectId, macroKey, onError, onLaunched }) => {
  const { agents } = useAgentStatus()
  const [runs, setRuns] = useState<MacroRun[]>([])
  const [launching, setLaunching] = useState(false)

  const refresh = useCallback(async () => {
    setRuns(await fetchMacroRuns(projectId, macroKey))
  }, [projectId, macroKey])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const active = activeMacroRun(runs)
  useEffect(() => {
    if (!active) return
    const id = setInterval(() => { void refresh() }, ACTIVE_POLL_MS)
    return () => clearInterval(id)
  }, [active, refresh])

  const blocker = macroLaunchBlocker(agents, projectId, runs)
  const launch = async () => {
    setLaunching(true)
    try {
      await launchMacroSkill(projectId, macroKey, 'realign_macro')
      onLaunched(`Réalignement de ${macroKey} lancé sur l'agent local.`)
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err))
    } finally {
      setLaunching(false)
      void refresh()
    }
  }

  return (
    <button
      type="button"
      disabled={launching || blocker !== null}
      onClick={launch}
      className="flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold text-sky-300 bg-sky-500/10 border border-sky-500/30 hover:bg-sky-500/20 disabled:opacity-50 cursor-pointer"
      title={blocker || 'Réaligner la spécification sur la découpe, dans le worktree de la macro'}
      data-macro-run={active ? active.status : undefined}
    >
      {launching || active ? <Loader2 size={10} className="animate-spin text-sky-400" /> : <GitCompareArrows size={10} className="text-sky-400" />}
      <span>{active ? 'Réalignement en cours' : 'Réaligner la spec'}</span>
    </button>
  )
}
