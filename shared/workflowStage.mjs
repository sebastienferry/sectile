// The workflow stage of a task on a board, and the change a move to another
// stage makes, decided in one place so that the web board and the desktop
// board place a task in the same column and move it alike (#806).

// The six stages, in board order.
export const WORKFLOW_STAGES = ['new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished']

// Stage and internal status are the same six-way split, named by the label on
// the tracker side and by the status on the application side. The server holds
// the same table (internal/db/board.go).
export const INTERNAL_STATUS_BY_STAGE = {
 new: 'to_clarify',
 clarified: 'clarified',
 specified: 'to_implement',
 implemented: 'to_test',
 reviewed: 'to_close',
 finished: 'finished',
}

// The workflow labels a move replaces, without their leading '#'.
export const STAGE_LABELS = ['untouched', 'new', 'clarified', 'specified', 'implemented', 'reviewed', 'finished', 'closed']

const bareLabels = task => (task?.labels || []).map(label => String(label).toLowerCase().replace(/^#+/, ''))

// The board a ticket of a tracker reads in a project: that tracker's columns
// and the mapping that applies in the project. The project's own fields, its
// default tracker's, when the tracker is not one it lists (#741).
export function trackerBoard(project, trackerId) {
 const ref = trackerId ? project?.trackers?.find(item => item.trackerId === trackerId) : undefined
 if (ref) return { trackerColumns: ref.trackerColumns || [], stageColumns: ref.stageColumns || {} }
 return { trackerColumns: project?.trackerColumns || [], stageColumns: project?.stageColumns || {} }
}

// The board column holding the task, from its tracker status, or null.
export function columnOfTask(task, project) {
 const status = (task?.trackerStatus || '').toLowerCase()
 const { trackerColumns } = trackerBoard(project, task?.trackerId)
 if (!status || !trackerColumns.length) return null
 const column = trackerColumns.find(col =>
  col.name.toLowerCase() === status ||
  (col.statuses && col.statuses.some(st => st.toLowerCase() === status))
 )
 return column?.name || null
}

// The stage mapped to the task's column. When a column carries several, the
// least advanced wins: it is the stage still to do in that column.
export function stageFromColumn(task, project) {
 const column = columnOfTask(task, project)
 if (!column) return null
 const mapping = trackerBoard(project, task?.trackerId).stageColumns
 for (const stage of WORKFLOW_STAGES) {
  if ((mapping[stage] || []).includes(column)) return stage
 }
 return null
}

// The stage an explicit workflow label names, or null. It wins over the column
// and the status on both boards.
export function explicitStage(task) {
 const labels = bareLabels(task)
 if (labels.includes('finished') || labels.includes('closed') || labels.includes('done')) return 'finished'
 if (labels.includes('reviewed')) return 'reviewed'
 if (labels.includes('implemented')) return 'implemented'
 if (labels.includes('specified')) return 'specified'
 if (labels.includes('clarified')) return 'clarified'
 if (labels.includes('new') || labels.includes('untouched')) return 'new'
 return null
}

// What a move of the task to targetStage writes: its workflow label replaced by
// the target one, the target's internal status, and the tracker status of the
// first column the project maps to the stage (that column's first status, else
// its name), the task's own when the project maps none. `project` is the one
// the ticket's tracker reads, as trackerBoard gives it.
export function stageMove(task, targetStage, project) {
 const labels = (task?.labels || []).filter(label => !STAGE_LABELS.includes(String(label).toLowerCase().replace(/^#+/, '')))
 labels.push(`#${targetStage.replace(/^#+/, '')}`)
 const status = INTERNAL_STATUS_BY_STAGE[targetStage] ?? task?.status
 let trackerStatus = task?.trackerStatus
 const mapped = project?.stageColumns?.[targetStage]
 if (mapped?.length) {
  const name = mapped[0]
  const column = project.trackerColumns?.find(col => col.name === name)
  if (column?.statuses?.length) trackerStatus = column.statuses[0]
  else if (name) trackerStatus = name
 }
 return { labels, status, trackerStatus }
}
