import React, { useCallback, useEffect, useRef, useState } from 'react'
import { GitCompareArrows, Loader2, Square } from 'lucide-react'
import { useAgentStatus } from '../hooks/useAgentStatus'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { activeMacroRun, cancelMacroRun, fetchMacroRuns, launchMacroSkill, macroLaunchBlocker, type MacroRun } from '../lib/macroRuns'

interface Props {
  projectId: string
  macroKey: string
  onError: (message: string) => void
  onLaunched: (message: string) => void
}

/** While a run is active, the panel re-reads the macro's runs this often. */
const ACTIVE_POLL_MS = 5000
/** Otherwise it looks now and then, for a run started by hand through MCP. */
const IDLE_POLL_MS = 30000

/**
 * Launches realign-macro for one macro on the local agent, through the same
 * Run the desktop app shows for a task skill, shows the run while it is
 * active, and stops it.
 */
export const MacroRealignButton: React.FC<Props> = ({ projectId, macroKey, onError, onLaunched }) => {
  const { agents } = useAgentStatus()
  const { user } = useCurrentUser()
  const [runs, setRuns] = useState<MacroRun[]>([])
  const [launching, setLaunching] = useState(false)
  const [stopping, setStopping] = useState(false)
  // The macro the latest read was for: an answer for a macro the panel has
  // since left is dropped rather than shown on the new one.
  const current = useRef(`${projectId}/${macroKey}`)
  current.current = `${projectId}/${macroKey}`

  const refresh = useCallback(async () => {
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

  const blocker = macroLaunchBlocker(agents, projectId, runs, user?.userId || '')
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
  const stop = async () => {
    if (!active) return
    setStopping(true)
    try {
      try {
        await cancelMacroRun(projectId, macroKey, active.id)
      } catch (err) {
        // An agent that cannot be reached cannot confirm the stop; closing
        // anyway is the person's explicit choice.
        const message = err instanceof Error ? err.message : String(err)
        if (!window.confirm(`${message}\n\nFermer quand même cette exécution ? Aucun processus local ne sera arrêté.`)) throw err
        await cancelMacroRun(projectId, macroKey, active.id, true)
      }
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err))
    } finally {
      setStopping(false)
      void refresh()
    }
  }

  return (
    <>
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
      {active && (
        <button
          type="button"
          disabled={stopping}
          onClick={stop}
          className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-bold text-rose-300 bg-rose-500/10 border border-rose-500/30 hover:bg-rose-500/20 disabled:opacity-50 cursor-pointer"
          title="Arrêter le réalignement en cours"
          aria-label="Arrêter le réalignement"
        >
          <Square size={9} />
        </button>
      )}
    </>
  )
}
