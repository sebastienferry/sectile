import React, { useState } from 'react'
import { Loader2, Square, type LucideIcon } from 'lucide-react'
import { useAgentStatus } from '../hooks/useAgentStatus'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { activeMacroRun, cancelMacroRun, launchMacroSkill, macroLaunchBlocker, macroRunLabel, macroSkillId, type MacroRun } from '../lib/macroRuns'
import { RunStateGlyph } from './RunStateGlyph'

/** The strings of one macro skill, such as planning.macro.realign. */
export interface MacroSkillStrings {
  noAgent: string
  alreadyRunning: string
  launchRefused: string
  stopRefused: string
  launched: string
  forceClose: string
  title: string
  running: string
  waiting: string
  action: string
  stopTitle: string
  stopLabel: string
}

/** The colour classes of a button, written out so Tailwind keeps them. */
const TONES = {
  sky: { button: 'text-sky-300 bg-sky-500/10 border-sky-500/30 hover:bg-sky-500/20', icon: 'text-sky-400' },
  orange: { button: 'text-orange-300 bg-orange-500/10 border-orange-500/30 hover:bg-orange-500/20', icon: 'text-orange-400' },
}

interface Props {
  projectId: string
  macroKey: string
  /** The catalog ID of the skill, such as realign_macro. */
  skillId: string
  strings: MacroSkillStrings
  icon: LucideIcon
  tone: keyof typeof TONES
  /** The macro's runs, shared by every skill button of the panel. */
  runs: MacroRun[]
  refresh: () => Promise<void>
  /** Whether a run of a skill no button launches shows here, to stay stoppable. */
  showsForeignRuns?: boolean
  /** The other skill IDs the panel has a button for. */
  otherSkillIds?: string[]
  /** Checked before the launch; false cancels it. */
  beforeLaunch?: () => Promise<boolean>
  onError: (message: string) => void
  onLaunched: (message: string) => void
}

/**
 * Launches a macro skill on the local agent, through the same Run the desktop
 * app shows for a task skill, shows the run while it is active, and stops it.
 * A macro runs one skill at a time: a run of another skill disables the button.
 */
export const MacroSkillButton: React.FC<Props> = ({
  projectId, macroKey, skillId, strings, icon: Icon, tone, runs, refresh,
  showsForeignRuns = false, otherSkillIds = [], beforeLaunch, onError, onLaunched,
}) => {
  const { agents } = useAgentStatus()
  const { user } = useCurrentUser()
  const { t } = useApp()
  const shared = t.planning.macro
  const [launching, setLaunching] = useState(false)
  const [stopping, setStopping] = useState(false)

  const running = activeMacroRun(runs)
  const runSkill = running ? macroSkillId(running.skillName) : ''
  const foreign = running !== null && runSkill !== skillId && !otherSkillIds.includes(runSkill)
  // The run this button shows and can stop: its own skill's, or one no button owns.
  const active = running && (runSkill === skillId || (foreign && showsForeignRuns)) ? running : null

  const blocker = macroLaunchBlocker(agents, projectId, runs, { ...strings, otherRunning: shared.otherSkillRunning }, user?.userId || '', skillId)
  const launch = async () => {
    setLaunching(true)
    try {
      if (beforeLaunch && !(await beforeLaunch())) return
      await launchMacroSkill(projectId, macroKey, skillId, strings.launchRefused)
      onLaunched(format(strings.launched, { key: macroKey }))
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
        await cancelMacroRun(projectId, macroKey, active.id, strings.stopRefused)
      } catch (err) {
        // An agent that cannot be reached cannot confirm the stop; closing
        // anyway is the person's explicit choice.
        const message = err instanceof Error ? err.message : String(err)
        if (!window.confirm(format(strings.forceClose, { message }))) throw err
        await cancelMacroRun(projectId, macroKey, active.id, strings.stopRefused, true)
      }
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err))
    } finally {
      setStopping(false)
      void refresh()
    }
  }

  const label = !active
    ? strings.action
    : foreign && !active.waitingSince
      ? format(shared.foreignRunning, { skill: active.skillName })
      : macroRunLabel(active, strings)
  const colours = TONES[tone]
  return (
    <>
      <button
        type="button"
        disabled={launching || blocker !== null}
        onClick={launch}
        className={`flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-bold border disabled:opacity-50 cursor-pointer ${colours.button}`}
        title={blocker || strings.title}
        data-macro-skill={skillId}
        data-macro-run={active ? (active.waitingSince ? 'waiting' : active.status) : undefined}
      >
        {/* A run waiting on its user shows the glyph a task card shows (#648). */}
        {active?.waitingSince
          ? <RunStateGlyph state="waiting" size={10} className="animate-pulse" />
          : launching || active ? <Loader2 size={10} className={`animate-spin ${colours.icon}`} /> : <Icon size={10} className={colours.icon} />}
        <span>{label}</span>
      </button>
      {active && (
        <button
          type="button"
          disabled={stopping}
          onClick={stop}
          className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-bold text-rose-300 bg-rose-500/10 border border-rose-500/30 hover:bg-rose-500/20 disabled:opacity-50 cursor-pointer"
          title={strings.stopTitle}
          aria-label={strings.stopLabel}
        >
          <Square size={9} />
        </button>
      )}
    </>
  )
}
