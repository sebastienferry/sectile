/** A worktree name is a directory name, never a path or a shell argument. */
export const isValidBatchWorktreeName = (value: string): boolean =>
  /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$/.test(value.trim())

export const suggestedBatchWorktreeName = (keys: readonly string[]): string =>
  ('batch-' + keys.map(key => key.replace(/[^a-zA-Z0-9_-]/g, '')).filter(Boolean).join('-'))
    .slice(0, 80).replace(/[-_]+$/, '')

export const buildBatchPickupPrompt = (taskIds: readonly string[], worktreeName: string): string => {
  const name = worktreeName.trim()
  if (!taskIds.length || new Set(taskIds).size !== taskIds.length) throw new Error('A batch needs distinct task IDs')
  if (!isValidBatchWorktreeName(name)) throw new Error('Invalid batch worktree name')
  return `/pickup-issues ${taskIds.join(' ')}\n\n` +
    `Use one dedicated Git worktree named "${name}" at .tasks/worktrees/${name} for this entire batch. ` +
    'This is the workspace explicitly chosen by the user for the batch. ' +
    'Preserve any existing work when preparing it; do not overwrite an unrelated worktree. ' +
    'Process the tickets sequentially in exactly the order listed above and produce one combined pull request.'
}
