import { useState } from 'react'
import { Copy } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { resolveTaskStage, skillForStage } from '../lib/workflow'
import type { Task } from '../types'

/** Default command of each workflow skill, before any project override. */
const SKILL_COMMANDS: Record<string, string> = {
  clarify: '/clarify-issue',
  specify: '/specify-issue',
  implement: '/code-issue',
  adjust: '/adjust-issue',
  handoff: '/handoff-issue',
}

const PICKUP_SKILL = 'pickup'
const PICKUP_COMMAND = '/pickup-issue'

export function CopyTaskSkillMenu({ task }: { task: Task }) {
  const { skillCommand, projects, currentProject } = useApp()
  const [copied, setCopied] = useState('')
  // What the clipboard refused, so it can still be selected by hand.
  const [fallback, setFallback] = useState('')

  const project = projects.find(item => item.id === task.projectId) || currentProject
  // The column the task sits in names the step still to do, so it names the
  // command to offer next to the autonomous one.
  const stage = resolveTaskStage(task, project)
  const stageSkill = skillForStage(stage)

  // The clipboard receives the prompt itself: it is pasted into whichever
  // assistant the user works in, not run through a shell.
  function promptFor(skillId: string, command: string): string {
    const skill = skillCommand(skillId, command, task.projectId)
    return `${skill} ${task.id}. Use Sectile MCP to read the task and comments and record workflow transitions. First call start_run and save its returned ID. Call finish_run with that runId when this entire skill ends, including failure or stopping for user input. Task primary key: ${task.id}.${task.projectId ? ` Project primary key: ${task.projectId}.` : ''}`
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
          title={`Copy ${skillCommand(stageSkill, SKILL_COMMANDS[stageSkill], task.projectId)} for this task`}
          onClick={() => copy('column', stageSkill, SKILL_COMMANDS[stageSkill])}
        >
          <Copy size={12} />
          <span>Copy {skillCommand(stageSkill, SKILL_COMMANDS[stageSkill], task.projectId)}</span>
        </button>
      )}
      <button
        type="button"
        className={itemClass}
        title="Copy the autonomous chain command for this task"
        onClick={() => copy('pickup', PICKUP_SKILL, PICKUP_COMMAND)}
      >
        <Copy size={12} />
        <span>Copy {skillCommand(PICKUP_SKILL, PICKUP_COMMAND, task.projectId)}</span>
      </button>
      <p role="status" className="px-2.5 text-[10px] text-[var(--text-muted)]">
        {fallback ? 'Clipboard blocked. Select the command below.' : copied ? 'Copied. Paste it into your assistant.' : ''}
      </p>
      {fallback && (
        <pre className="mx-2.5 max-h-32 overflow-auto whitespace-pre-wrap break-all rounded bg-[var(--bg-primary)] p-2 text-[10px] select-text"><code>{fallback}</code></pre>
      )}
    </div>
  )
}
