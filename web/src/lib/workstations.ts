import type { TaskActivity } from '../types'

export interface Workstation { deviceId: string; label?: string }

/** Intersect by identity, never by a possibly duplicated human-readable label. */
export function commonWorkstations(groups: Workstation[][]): Workstation[] {
  if (!groups.length) return []
  return groups[0].filter((device, index, all) =>
    all.findIndex(item => item.deviceId === device.deviceId) === index &&
    groups.every(group => group.some(item => item.deviceId === device.deviceId)))
}

export function workstationName(device: Workstation, candidates: Workstation[] = []): string {
  const label = device.label?.trim() || device.deviceId
  return candidates.filter(item => (item.label?.trim() || item.deviceId) === label).length > 1
    ? `${label} (${device.deviceId})` : label
}

export function mayCancelExecution(run: TaskActivity, viewer?: { userId?: string; role?: string } | null): boolean {
  return ['queued', 'pending', 'running'].includes(run.status) &&
    (viewer?.role === 'admin' || Boolean(viewer?.userId && run.userId === viewer.userId))
}
