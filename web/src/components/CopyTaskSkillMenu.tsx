import { useState } from 'react'
import { useApp } from '../context/AppContext'
import type { Task } from '../types'

const actions = [
  { id: 'pickup', command: '/pickup-issue', label: 'Pick up task' },
  { id: 'clarify', command: '/clarify-issue', label: 'Clarify' },
  { id: 'specify', command: '/specify-issue', label: 'Specify' },
  { id: 'implement', command: '/code-issue', label: 'Implement' },
  { id: 'create_pr', command: '/create-pr', label: 'Review and create PR/MR' },
  { id: 'handoff', command: '/handoff-issue', label: 'Handoff' },
]

export function CopyTaskSkillMenu({ task }: { task: Task }) {
  const { skillCommand } = useApp()
  const [provider, setProvider] = useState('codex')
  const [actionId, setActionId] = useState('pickup')
  const [message, setMessage] = useState('')
  const action = actions.find(item => item.id === actionId) || actions[0]
  const skill = skillCommand(action.id, action.command, task.projectId)
  const prompt = `${skill} ${task.id}. Use TaskFlow MCP to read the task and comments and record workflow transitions. First call taskflow_start_run and save its returned ID. Call taskflow_finish_run with that runId when this entire skill ends, including failure or stopping for user input. Task primary key: ${task.id}.${task.projectId ? ` Project primary key: ${task.projectId}.` : ''}`
  const command = provider + " '" + prompt.replace(/'/g, "'\\''") + "'"
  const fieldClass = 'w-full rounded border border-[var(--border-color)] bg-[var(--bg-primary)] p-2 text-[var(--text-primary)]'

  async function copy() {
    try {
      await navigator.clipboard.writeText(command)
      setMessage('Command copied.')
    } catch {
      setMessage('Copy unavailable. Select and copy the command manually.')
    }
  }

  return (
    <details className="rounded-lg border border-[var(--border-color)] p-2 text-xs" onClick={event => event.stopPropagation()}>
      <summary className="cursor-pointer font-semibold">Copy skill command</summary>
      <div className="mt-3 space-y-2">
        <label className="block">Client
          <select className={fieldClass} value={provider} onChange={event => { setProvider(event.target.value); setMessage('') }}>
            <option value="codex">Codex</option>
            <option value="claude">Claude</option>
          </select>
        </label>
        <label className="block">Action
          <select className={fieldClass} value={actionId} onChange={event => { setActionId(event.target.value); setMessage('') }}>
            {actions.map(item => <option key={item.id} value={item.id}>{item.label}</option>)}
          </select>
        </label>
        <p className="text-[var(--text-muted)]">Run in the local repository with the project skills and TaskFlow MCP configured.</p>
        <pre className="max-h-36 overflow-auto whitespace-pre-wrap break-all rounded bg-[var(--bg-primary)] p-2 select-text"><code>{command}</code></pre>
        <button type="button" className={fieldClass + ' cursor-pointer'} onClick={copy}>Copy command</button>
        <p role="status">{message}</p>
      </div>
    </details>
  )
}
