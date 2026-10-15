import type { Task, ToastLink } from '../types/index.ts'

export function issueToastLink(
  task: Task,
  open: (task: Task) => void,
  labels: { internal: string; external: string },
): ToastLink | undefined {
  if (task.id) {
    return {
      label: `${labels.internal} ${task.key}`,
      onOpen: () => open(task),
      externalUrl: task.externalUrl || undefined,
    }
  }
  if (task.externalUrl) {
    return { label: `${labels.external} ${task.key}`, externalUrl: task.externalUrl }
  }
  return undefined
}
