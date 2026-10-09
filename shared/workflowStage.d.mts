export type WorkflowStageName = 'new' | 'clarified' | 'specified' | 'implemented' | 'reviewed' | 'finished'

export interface StageTrackerColumn {
  name: string
  statuses?: string[]
}

export interface StageBoard {
  trackerColumns?: StageTrackerColumn[]
  stageColumns?: Record<string, string[]>
  trackers?: { trackerId: string; trackerColumns?: StageTrackerColumn[]; stageColumns?: Record<string, string[]> }[]
}

export interface StageTask {
  labels?: string[]
  status?: string
  trackerStatus?: string
  trackerId?: string
}

export interface StageMove<S extends string = string> {
  labels: string[]
  status: S
  trackerStatus: string | undefined
}

export const WORKFLOW_STAGES: WorkflowStageName[]
export const INTERNAL_STATUS_BY_STAGE: Record<WorkflowStageName, string>
export const STAGE_LABELS: string[]
export function trackerBoard(
  project: StageBoard | null | undefined,
  trackerId: string | undefined
): { trackerColumns: StageTrackerColumn[]; stageColumns: Record<string, string[]> }
export function columnOfTask(task: StageTask, project?: StageBoard | null): string | null
export function stageFromColumn(task: StageTask, project?: StageBoard | null): WorkflowStageName | null
export function explicitStage(task: StageTask): WorkflowStageName | null
export function stageMove(task: StageTask, targetStage: WorkflowStageName, project?: StageBoard | null): StageMove
