/** Default command of each workflow skill, before any project override. */
export const SKILL_COMMANDS: Record<string, string> = {
  clarify: '/clarify-issue',
  specify: '/specify-issue',
  implement: '/implement-issue',
  adjust: '/adjust-issue',
  handoff: '/handoff-issue',
}

export const PICKUP_SKILL = 'pickup'
export const PICKUP_COMMAND = '/pickup-issue'

/**
 * The prompt that runs `command` on a task, pasted into whichever assistant
 * the user works in rather than run through a shell. Every engine receives the
 * same text: the local agent launches them all with it.
 */
export function taskSkillPrompt(command: string, taskId: string, projectId?: string): string {
  return `${command} ${taskId}. Use Sectile MCP to read the task and comments and record workflow transitions. First call start_run and save its returned ID. Call finish_run with that runId when this entire skill ends, including failure or stopping for user input. Task primary key: ${taskId}.${projectId ? ` Project primary key: ${projectId}.` : ''}`
}
