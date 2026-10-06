import { useState } from 'react'
import { Copy } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { resolveTaskStage, skillForStage } from '../lib/workflow'
import { format } from '../lib/i18n'
import { PICKUP_COMMAND, PICKUP_SKILL, SKILL_COMMANDS, taskSkillPrompt } from '../lib/skillPrompt'
import type { Task } from '../types'

export function CopyTaskSkillMenu({ task }: { task: Task }) {
  const { skillCommand, projects, currentProject, t } = useApp()
  const strings = t.taskDetail.copySkill
  const [copied, setCopied] = useState('')
  // What the clipboard refused, so it can still be selected by hand.
  const [fallback, setFallback] = useState('')

  const project = projects.find(item => item.id === task.projectId) || currentProject
  // The column the task sits in names the step still to do, so it names the
  // command to offer next to the autonomous one.
  const stage = resolveTaskStage(task, project)
  const stageSkill = skillForStage(stage)

  function promptFor(skillId: string, command: string): string {
    return taskSkillPrompt(skillCommand(skillId, command, task.projectId), task.id, task.projectId)
  }

  async function copy(label: string, skillId: string, command: string) {
    const prompt = promptFor(skillId, command)
    try {
      await navigator.clipboard.writeText(prompt)
      setFallback('')
      setCopied(label)
    } catch {
      setCopied('')
      setFallback(prompt)
    }
  }

  const itemClass = 'w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer'

  return (
    <div onClick={event => event.stopPropagation()}>
      {stageSkill && (
        <button
          type="button"
          className={itemClass}
          title={format(strings.copyStageTitle, { command: skillCommand(stageSkill, SKILL_COMMANDS[stageSkill], task.projectId) })}
          onClick={() => copy('column', stageSkill, SKILL_COMMANDS[stageSkill])}
        >
          <Copy size={12} />
          <span>{format(strings.copyCommand, { command: skillCommand(stageSkill, SKILL_COMMANDS[stageSkill], task.projectId) })}</span>
        </button>
      )}
      <button
        type="button"
        className={itemClass}
        title={strings.copyPickupTitle}
        onClick={() => copy('pickup', PICKUP_SKILL, PICKUP_COMMAND)}
      >
        <Copy size={12} />
        <span>{format(strings.copyCommand, { command: skillCommand(PICKUP_SKILL, PICKUP_COMMAND, task.projectId) })}</span>
      </button>
      <p role="status" className="px-2.5 text-[10px] text-[var(--text-muted)]">
        {fallback ? strings.clipboardBlocked : copied ? strings.copied : ''}
      </p>
      {fallback && (
        <pre className="mx-2.5 max-h-32 overflow-auto whitespace-pre-wrap break-all rounded bg-[var(--bg-primary)] p-2 text-[10px] select-text"><code>{fallback}</code></pre>
      )}
    </div>
  )
}
