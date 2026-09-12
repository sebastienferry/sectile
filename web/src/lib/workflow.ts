import type { Project, Status, Task, WorkflowStage } from '../types'

/**
 * Étape du workflow agentique d'une tâche, et skill qui en découle.
 *
 * Deux sources possibles. Les labels du ticket, historiquement la seule, restent
 * la trace écrite dans le tracker. Et depuis que les colonnes du board portent
 * une affectation d'étapes, la colonne où se trouve le ticket décide : c'est
 * elle qui reflète l'état réel du travail côté équipe, alors qu'un label peut
 * n'avoir jamais été posé.
 */

export const WORKFLOW_ORDER: WorkflowStage[] = [
  'new',
  'clarified',
  'specified',
  'implemented',
  'reviewed',
  'finished',
]

/** Étape déduite des labels, puis du statut interne en repli. */
export const stageFromLabels = (task: Task): WorkflowStage => {
  const labels = (task.labels || []).map(l => l.toLowerCase().replace(/^#+/, ''))
  if (labels.includes('finished') || labels.includes('closed') || labels.includes('done')) return 'finished'
  if (labels.includes('reviewed')) return 'reviewed'
  if (labels.includes('implemented')) return 'implemented'
  if (labels.includes('specified')) return 'specified'
  if (labels.includes('clarified')) return 'clarified'
  if (labels.includes('new') || labels.includes('untouched')) return 'new'

  // Match labels that contain keywords
  if (labels.some(l => l.includes('progress') || l.includes('code') || l.includes('implement'))) return 'specified'
  if (labels.some(l => l.includes('test') || l.includes('validat') || l.includes('qa'))) return 'implemented'
  if (labels.some(l => l.includes('review') || l.includes('pr') || l.includes('mr'))) return 'reviewed'

  const trackerSt = (task.trackerStatus || '').toLowerCase()
  if (trackerSt === 'closed' || trackerSt === 'done' || trackerSt === 'terminé' || trackerSt === 'finished') return 'finished'
  if (trackerSt.includes('review') || trackerSt.includes('pr')) return 'reviewed'
  if (trackerSt.includes('test') || trackerSt.includes('validat')) return 'implemented'
  if (trackerSt.includes('progress') || trackerSt.includes('code') || trackerSt.includes('implement')) return 'specified'
  if (trackerSt.includes('specify') || trackerSt.includes('spec')) return 'clarified'
  if (trackerSt === 'open' && (task.status === 'to_clarify' || !task.status)) return 'new'

  if (task.status === 'finished' || task.status === 'done') return 'finished'
  if (task.status === 'to_close') return 'reviewed'
  if (task.status === 'to_test' || task.status === 'to_validate') return 'implemented'
  if (task.status === 'clarified') return 'clarified'
  if (task.status === 'to_implement' || task.status === 'in_progress' || task.status === 'specified') return 'specified'
  return 'new'
}

/** Colonne du board contenant la tâche, d'après son statut de tracker. */
export const columnOfTask = (task: Task, project?: Project | null): string | null => {
  const status = (task.trackerStatus || '').toLowerCase()
  if (!status || !project?.trackerColumns?.length) return null
  const column = project.trackerColumns.find(col =>
    col.name.toLowerCase() === status ||
    (col.statuses && col.statuses.some(st => st.toLowerCase() === status))
  )
  return column?.name || null
}

/**
 * Étape affectée à la colonne de la tâche. Quand une colonne en porte
 * plusieurs, la moins avancée gagne : c'est l'étape encore à faire dans cette
 * colonne, donc celle qui doit être proposée.
 */
export const stageFromColumn = (task: Task, project?: Project | null): WorkflowStage | null => {
  const column = columnOfTask(task, project)
  if (!column) return null
  const mapping = project?.stageColumns || {}
  for (const stage of WORKFLOW_ORDER) {
    if ((mapping[stage] || []).includes(column)) return stage
  }
  return null
}

/**
 * Étape retenue : les labels de workflow d'abord quand un label explicite est présent,
 * puis la colonne du board quand le projet est configuré, puis le statut/mots-clés en repli.
 */
export const resolveTaskStage = (task: Task, project?: Project | null): WorkflowStage => {
  // 1. Les labels de workflow explicites ont priorité absolue dans la vue agentique
  const labels = (task.labels || []).map(l => l.toLowerCase().replace(/^#+/, ''))
  if (labels.includes('finished') || labels.includes('closed') || labels.includes('done')) return 'finished'
  if (labels.includes('reviewed')) return 'reviewed'
  if (labels.includes('implemented')) return 'implemented'
  if (labels.includes('specified')) return 'specified'
  if (labels.includes('clarified')) return 'clarified'
  if (labels.includes('new') || labels.includes('untouched')) return 'new'

  // 2. Colonne du board configurée pour le projet si pas de label explicite
  const colStage = stageFromColumn(task, project)
  if (colStage) return colStage

  // 3. Repli sur les mots-clés de labels et le statut
  return stageFromLabels(task)
}

/**
 * Skill à proposer pour une étape : l'étape nomme ce qui est déjà acquis, donc
 * la skill est l'étape suivante. Une tâche terminée n'en propose aucune.
 */
export const skillForStage = (stage: WorkflowStage): string | null => {
  switch (stage) {
    case 'new':
      return 'clarify'
    case 'clarified':
      return 'specify'
    case 'specified':
      return 'implement'
    case 'implemented':
    case 'reviewed':
      return 'create_pr'
    case 'finished':
    default:
      return null
  }
}

/**
 * Étape du workflow et statut interne se répondent un pour un : c'est le même
 * découpage, nommé par le label côté tracker et par le statut côté application.
 * Le serveur tient la même table (internal/db/board.go).
 */
export const INTERNAL_STATUS_BY_STAGE: Record<WorkflowStage, Status> = {
  new: 'to_clarify',
  clarified: 'clarified',
  specified: 'to_implement',
  implemented: 'to_test',
  reviewed: 'to_close',
  finished: 'finished',
}

/** Étape correspondant à un statut interne, alias historiques compris. */
export const stageForInternalStatus = (status: Status): WorkflowStage => {
  if (status === 'finished' || status === 'done') return 'finished'
  if (status === 'to_close') return 'reviewed'
  if (status === 'to_test' || status === 'to_validate') return 'implemented'
  if (status === 'clarified') return 'clarified'
  if (status === 'to_implement' || status === 'in_progress' || status === 'specified') return 'specified'
  return 'new'
}

/**
 * Statuts du tracker qu'une étape recouvre, en passant par les colonnes que le
 * projet lui a affectées. Vide quand le projet n'a pas de mapping : il n'y a
 * alors rien à convertir, et forcer une correspondance approximative vaudrait
 * moins que de ne pas filtrer.
 */
export const trackerStatusesForStage = (project: Project | null | undefined, stage: WorkflowStage): string[] => {
  const columns = project?.stageColumns?.[stage] || []
  if (columns.length === 0 || !project?.trackerColumns?.length) return []
  const out: string[] = []
  columns.forEach(name => {
    const column = project.trackerColumns?.find(col => col.name === name)
    ;(column?.statuses || []).forEach(status => {
      if (!out.includes(status)) out.push(status)
    })
  })
  return out
}

/**
 * Étape que recouvre une sélection de statuts du tracker. La moins avancée
 * gagne, comme pour la colonne d'une tâche : c'est l'étape qui reste à faire.
 */
export const stageForTrackerStatuses = (
  project: Project | null | undefined,
  statuses: string[]
): WorkflowStage | null => {
  if (statuses.length === 0 || !project?.trackerColumns?.length) return null
  const lowered = statuses.map(st => st.toLowerCase())
  const columns = project.trackerColumns
    .filter(col => (col.statuses || []).some(st => lowered.includes(st.toLowerCase())))
    .map(col => col.name)
  if (columns.length === 0) return null

  const mapping = project.stageColumns || {}
  for (const stage of WORKFLOW_ORDER) {
    if ((mapping[stage] || []).some(name => columns.includes(name))) return stage
  }
  return null
}

export interface NextStepInfo {
  currentStage: WorkflowStage
  nextSkillId: string | null
  stepLabel: string
  stepDescription: string
  stepTooltip: string
  autoTooltip: string
  remainingSteps: string[]
}

/**
 * Informations descriptives et tooltips détaillés pour avancer d'un pas
 * ou en autonomie dans le workflow agentique.
 */
export const getNextStepInfo = (task: Task, project?: Project | null): NextStepInfo => {
  const currentStage = resolveTaskStage(task, project)

  switch (currentStage) {
    case 'new':
      return {
        currentStage: 'new',
        nextSkillId: 'clarify',
        stepLabel: 'Clarifier',
        stepDescription: 'Clarifier les exigences et questions non tranchées',
        stepTooltip: "Avancer d'un pas : Clarifier les exigences (clarify-issue)",
        autoTooltip: "Avancer en autonomie : Clarifier ➔ Spécifier ➔ Coder ➔ Créer PR",
        remainingSteps: ['Clarifier', 'Spécifier', 'Coder', 'Créer PR'],
      }
    case 'clarified':
      return {
        currentStage: 'clarified',
        nextSkillId: 'specify',
        stepLabel: 'Spécifier',
        stepDescription: 'Rédiger la spécification technique (Spec Kit / OpenSpec)',
        stepTooltip: "Avancer d'un pas : Spécifier la solution technique (specify-issue)",
        autoTooltip: "Avancer en autonomie : Spécifier ➔ Coder ➔ Créer PR",
        remainingSteps: ['Spécifier', 'Coder', 'Créer PR'],
      }
    case 'specified':
      return {
        currentStage: 'specified',
        nextSkillId: 'implement',
        stepLabel: 'Coder',
        stepDescription: 'Implémenter le code sur la branche et passer les tests',
        stepTooltip: "Avancer d'un pas : Implémenter le code et tests (code-issue)",
        autoTooltip: "Avancer en autonomie : Coder ➔ Créer PR",
        remainingSteps: ['Coder', 'Créer PR'],
      }
    case 'implemented':
      return {
        currentStage: 'implemented',
        nextSkillId: 'create_pr',
        stepLabel: 'Créer PR',
        stepDescription: 'Revue de code peer-review et ouverture de la Pull Request',
        stepTooltip: "Avancer d'un pas : Revue de code & Création de PR (create-pr)",
        autoTooltip: "Avancer en autonomie : Revue & Création de PR",
        remainingSteps: ['Créer PR'],
      }
    case 'reviewed':
      return {
        currentStage: 'reviewed',
        nextSkillId: null,
        stepLabel: 'Merge',
        stepDescription: 'Fusionner la Pull Request / branche et finaliser (#finished)',
        stepTooltip: "Avancer d'un pas : Fusionner la PR et finaliser le ticket (#finished)",
        autoTooltip: "PR prête : la fusion finale reste manuelle (#finished)",
        remainingSteps: ['Merge / Finaliser'],
      }
    case 'finished':
    default:
      return {
        currentStage: 'finished',
        nextSkillId: null,
        stepLabel: 'Terminé',
        stepDescription: 'Ticket finalisé et validé',
        stepTooltip: 'Ticket terminé (#finished)',
        autoTooltip: 'Ticket déjà terminé',
        remainingSteps: [],
      }
  }
}
