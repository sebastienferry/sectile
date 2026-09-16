import { RemoteRunBadge } from './RemoteRunBadge'
import { CopyTaskSkillMenu } from './CopyTaskSkillMenu'
import React, { useState, useRef, useEffect } from 'react'
import { createPortal } from 'react-dom'
import {
  Flame,
  ListFilter,
  Calendar,
  Clock,
  Sparkles,
  Loader2,
  GitPullRequest,
  ExternalLink,
  MoreHorizontal,
  ChevronsRight,
  ChevronRight,
  FileCode,
  CheckCircle2,
  Eye,
  MessageCircle,
  Trash2,
  Copy,
  CopyPlus,
  Pin,
  X,
} from 'lucide-react'
import type { Task, Priority, SkillMode } from '../types'
import { useApp } from '../context/AppContext'
import { issueTypeStyle } from '../lib/issueTypes'
import { Avatar } from './Avatar'
import { shortElapsed, isElapsedStale } from '../lib/elapsed'
import { resolveTaskStage, getNextStepInfo, prRecoverySkill } from '../lib/workflow'

interface TaskCardProps {
  task: Task
  isDragging?: boolean
  onDragStart?: (e: React.DragEvent) => void
  compact?: boolean
}

export const TaskCard: React.FC<TaskCardProps> = ({ task, isDragging, onDragStart, compact = false }) => {
  const {
    setSelectedTask,
    advanceTask,
    isPinned,
    togglePin,
    runSkill,
    isSkillRunning,
    runningSkillId,
    activities,
    openCloneModal,
    deleteTask,
    projects,
    settings,
    parentFilter,
    setParentFilter,
    skillLabel,
    t,
    addToast,
    setTaskSprint,
  } = useApp()

  // Le menu est rendu dans un portail avec un positionnement fixe : les colonnes
  // du board défilent en overflow-y-auto, ce qui découpait un menu en position
  // absolue et le faisait passer sous l'en-tête de colonne pour les cartes du
  // haut. Le portail sort de ce conteneur, et l'ouverture bascule vers le bas
  // quand il n'y a pas la place au-dessus.
  const [advancing, setAdvancing] = useState<'step' | 'auto' | null>(null)
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  const [menuPos, setMenuPos] = useState<{ left: number; top?: number; bottom?: number; maxHeight: number } | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const menuNodeRef = useRef<HTMLDivElement>(null)
  const menuButtonRef = useRef<HTMLButtonElement>(null)

  const MENU_WIDTH = 214
  const MENU_MAX_HEIGHT = 380
  const MENU_GAP = 6
  const MARGIN = 8
  const BOTTOM_RESERVE = 36 // Reserve space for global bottom StatusBar

  const openMenuAt = () => {
    const btn = menuButtonRef.current
    if (!btn) return
    const rect = btn.getBoundingClientRect()

    // Annuler l'effet de zoom de l'interface car le portail subit le zoom à son tour
    const zoomRaw = getComputedStyle(document.documentElement).getPropertyValue('--ui-zoom')
    const zoom = parseFloat(zoomRaw) || 1

    const btnTop = rect.top / zoom
    const btnBottom = rect.bottom / zoom
    const btnRight = rect.right / zoom

    const vpHeight = window.innerHeight / zoom
    const vpWidth = window.innerWidth / zoom

    const spaceAbove = btnTop - MARGIN
    const spaceBelow = vpHeight - btnBottom - BOTTOM_RESERVE

    // Aligner à droite du bouton tout en restant dans les limites horizontales de l'écran
    const left = Math.max(MARGIN, Math.min(btnRight - MENU_WIDTH, vpWidth - MENU_WIDTH - MARGIN))

    // Préférer l'ouverture vers le bas si l'espace est suffisant ou plus grand que vers le haut
    if (spaceBelow >= 220 || spaceBelow >= spaceAbove) {
      const maxHeight = Math.max(140, Math.min(MENU_MAX_HEIGHT, spaceBelow - MENU_GAP))
      setMenuPos({
        left,
        top: btnBottom + MENU_GAP,
        maxHeight,
      })
    } else {
      const maxHeight = Math.max(140, Math.min(MENU_MAX_HEIGHT, spaceAbove - MENU_GAP))
      setMenuPos({
        left,
        bottom: vpHeight - btnTop + MENU_GAP,
        maxHeight,
      })
    }
    setIsMenuOpen(true)
  }

  const taskProject = projects.find(p => p.id === task.projectId)
  const targetGithubRepo = (taskProject?.githubRepo || settings.githubRepo || '').replace(/^https?:\/\/github\.com\//, '').replace(/\.git$/, '')
  const externalUrl = task.externalUrl || (
    task.source === 'github' && targetGithubRepo && task.key?.startsWith('#')
      ? `https://github.com/${targetGithubRepo}/issues/${task.key.replace('#', '')}`
      : undefined
  )

  useEffect(() => {
    if (!isMenuOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node
      // Le menu vit dans un portail : il faut tester les deux racines.
      if (menuRef.current?.contains(target) || menuNodeRef.current?.contains(target)) return
      setIsMenuOpen(false)
    }
    // Le menu est en position fixe : plutôt que de le faire suivre le défilement,
    // on le referme, ce qui reste prévisible et évite un menu qui flotte loin de
    // sa carte.
    const close = (e: Event) => {
      if (e.target instanceof Node && menuNodeRef.current?.contains(e.target)) return
      setIsMenuOpen(false)
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setIsMenuOpen(false)
        menuButtonRef.current?.focus()
      }
    }
    menuNodeRef.current?.querySelector<HTMLElement>('button:not(:disabled), a[href]')?.focus({ preventScroll: true })
    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('mousedown', handleClickOutside)
    window.addEventListener('scroll', close, true)
    window.addEventListener('resize', close)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('mousedown', handleClickOutside)
      window.removeEventListener('scroll', close, true)
      window.removeEventListener('resize', close)
    }
  }, [isMenuOpen])

  const latestActivity = React.useMemo(() => {
    if (!activities || activities.length === 0) return null
    const taskActs = activities.filter(a => a.taskId === task.id)
    if (taskActs.length === 0) return null
    const running = taskActs.find(a => a.status === 'running' || a.status === 'queued' || a.status === 'pending')
    if (running) return running
    return taskActs[0]
  }, [activities, task.id])

  // Priorité : simple pastille de couleur (le libellé reste en infobulle)
  const PRIORITY_DOTS: Record<Priority, { color: string; label: string }> = {
    urgent: { color: 'var(--status-danger)', label: t.priority.urgent },
    high: { color: 'var(--status-warn)', label: t.priority.high },
    medium: { color: 'var(--status-info)', label: t.priority.medium },
    low: { color: 'var(--text-muted)', label: t.priority.low },
  }

  const getPriorityBadge = (priority: Priority) => {
    const dot = PRIORITY_DOTS[priority]
    if (!dot) return null
    return (
      <span
        className="w-2 h-2 rounded-full shrink-0 ring-1 ring-black/10"
        style={{ backgroundColor: dot.color }}
        title={`${t.taskModal.priority} : ${dot.label}`}
      />
    )
  }

  const handleDragStartInternal = (e: React.DragEvent) => {
    e.dataTransfer.setData('text/plain', task.id)
    e.dataTransfer.effectAllowed = 'move'
    if (onDragStart) onDragStart(e)
  }

  // Skill par identifiant, pour que l'étape résolue et la déduction historique
  // produisent exactement la même action.
  const skillAction = (id: string) => {
    switch (id) {
      case 'clarify':
        return {
          id: 'clarify',
          label: skillLabel('clarify', 'Clarifier'),
          icon: <Sparkles size={11} className="text-amber-400" />,
          title: 'Clarifier les exigences et cadrer la tâche',
          action: async (e: React.MouseEvent) => {
            e.stopPropagation()
            if (isSkillRunning) return
            await runSkill(task.id, 'clarify')
          },
        }
      case 'specify':
        return {
          id: 'specify',
          label: skillLabel('specify', 'Spécifier'),
          icon: <FileCode size={11} className="text-blue-400" />,
          title: 'Rédiger la spécification technique (Spec Kit / OpenSpec)',
          action: async (e: React.MouseEvent) => {
            e.stopPropagation()
            if (isSkillRunning) return
            await runSkill(task.id, 'specify')
          },
        }
      case 'implement':
        return {
          id: 'implement',
          label: skillLabel('implement', 'Coder'),
          icon: <Flame size={11} className="text-indigo-400" />,
          title: "Lancer l'implémentation du code par l'agent IA",
          action: async (e: React.MouseEvent) => {
            e.stopPropagation()
            if (isSkillRunning) return
            await runSkill(task.id, 'implement')
          },
        }
      case 'adjust':
        return {
          id: 'adjust',
          label: skillLabel('adjust', 'Adjust'),
          icon: <GitPullRequest size={11} className="text-purple-400" />,
          title: 'Review the complete branch and adjust the existing PR',
          action: async (e: React.MouseEvent) => {
            e.stopPropagation()
            if (isSkillRunning) return
            await runSkill(task.id, 'adjust')
          },
        }
      default:
        return null
    }
  }

  const nextStepInfo = getNextStepInfo(task, taskProject)
  const isFinishedTask = nextStepInfo.currentStage === 'finished'

  // Un pas du workflow. Le serveur lance l'étape en pleine autonomie en arrière-plan.
  // Une session TTY interactive peut être ouverte manuellement via le bouton terminal.
  const handleAdvance = async (auto: boolean, modeOverride?: SkillMode) => {
    if (advancing || isFinishedTask) return
    setAdvancing(auto ? 'auto' : 'step')
    await advanceTask(task.id, auto, modeOverride)
    setAdvancing(null)
  }

  // Determine current workflow stage action (Clarifier ➔ Spécifier ➔ Coder ➔ Adjust ➔ Merge ➔ #finished)
  const getWorkflowAction = () => {
    const stage = resolveTaskStage(task, taskProject)
    if (stage === 'finished') return null

    // If PR is already created or task is in reviewed stage -> Action is "Merge"
    if (
      stage === 'reviewed' || task.status === 'to_close'
    ) {
      return {
        id: 'handoff',
        label: 'Handoff',
        icon: <CheckCircle2 size={11} className="text-emerald-400" />,
        title: 'Verify human merge and hand off the task',
        action: async (e: React.MouseEvent) => {
          e.stopPropagation()
          await runSkill(task.id, 'handoff')
        },
      }
    }

    // If code is implemented / to_test (and no PR created yet) -> Action is "Adjust"
    if (stage === 'implemented' || task.status === 'to_test' || task.status === 'to_validate') {
      return skillAction('adjust')
    }

    // If spec is ready / to_implement -> Action is "Coder"
    if (stage === 'specified' || task.status === 'to_implement' || task.status === 'in_progress') {
      return skillAction('implement')
    }

    // A clarified task is ready for specification.
    if (stage === 'clarified') {
      return skillAction('specify')
    }

    // Default: Backlog / to_clarify -> Action is "Clarifier"
    return skillAction('clarify')
  }

  const workflowAction = getWorkflowAction()
  const isRunning = latestActivity?.status === 'running'
  const isQueued = latestActivity?.status === 'queued' || latestActivity?.status === 'pending'

  const isCondensed = compact
  const compactActionClass = 'w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] focus-visible:outline-2 focus-visible:outline-[var(--accent-color)] transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed'
  const actionsMenu = (
    <div className="flex items-center gap-1 relative" ref={menuRef} onClick={e => e.stopPropagation()}>
      {/* Menu (...) Button */}
      <button
        type="button"
        ref={menuButtonRef}
        onClick={e => {
          e.stopPropagation()
          if (isMenuOpen) {
            setIsMenuOpen(false)
          } else {
            openMenuAt()
          }
        }}
        className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-transparent hover:border-[var(--border-color)]/60 transition-colors cursor-pointer"
        title="Actions"
        aria-label="Actions"
        aria-expanded={isMenuOpen}
      >
        <MoreHorizontal size={14} />
      </button>

      {/* Contextual Dropdown Menu */}
      {isMenuOpen && menuPos && createPortal(
        <div
          ref={menuNodeRef}
          style={{
            position: 'fixed',
            left: menuPos.left,
            top: menuPos.top,
            bottom: menuPos.bottom,
            width: MENU_WIDTH,
            maxHeight: menuPos.maxHeight || MENU_MAX_HEIGHT,
          }}
          className="overflow-y-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl p-1 z-[100] animate-in fade-in-0 zoom-in-95 duration-100 text-xs">
          {isCondensed && (
            <>
              <button type="button" className={compactActionClass} onClick={() => { setIsMenuOpen(false); togglePin(task.id) }}>
                <Pin size={12} /><span>{isPinned(task.id) ? t.compactCard.unpin : t.compactCard.pin}</span>
              </button>
              <button type="button" className={compactActionClass} disabled={advancing !== null || isFinishedTask} onClick={() => { setIsMenuOpen(false); handleAdvance(false) }}>
                <ChevronRight size={12} /><span>{t.compactCard.advance}</span>
              </button>
              <button type="button" className={compactActionClass} disabled={advancing !== null || isFinishedTask} onClick={() => { setIsMenuOpen(false); handleAdvance(true) }}>
                <ChevronsRight size={12} /><span>{t.compactCard.advanceAuto}</span>
              </button>
              {/* Surcharge ponctuelle du mode : ce lancement seulement, rien n'est enregistré. */}
              <button type="button" className={compactActionClass} disabled={advancing !== null || isFinishedTask} onClick={() => { setIsMenuOpen(false); handleAdvance(false, 'interactive') }}>
                <ChevronRight size={12} /><span>Avancer en interactif</span>
              </button>
              <button type="button" className={compactActionClass} disabled={advancing !== null || isFinishedTask} onClick={() => { setIsMenuOpen(false); handleAdvance(false, 'non_interactive') }}>
                <ChevronRight size={12} /><span>Avancer en non interactif</span>
              </button>
              {task.parentKey && (
                <button type="button" className={compactActionClass} onClick={() => { setIsMenuOpen(false); setParentFilter(parentFilter === task.parentKey ? null : task.parentKey!) }}>
                  <ListFilter size={12} /><span>{parentFilter === task.parentKey ? t.compactCard.clearParent : t.compactCard.filterParent} {task.parentKey}</span>
                </button>
              )}
              {task.prUrl && (
                <a className={compactActionClass} href={task.prUrl} target="_blank" rel="noreferrer" onClick={() => setIsMenuOpen(false)}>
                  <GitPullRequest size={12} /><span>{t.compactCard.openPr}</span>
                </a>
              )}
              <div className="h-px bg-[var(--border-color)] my-1" />
            </>
          )}
          <CopyTaskSkillMenu task={task} />
          {/* Action de l'étape courante du workflow (nom du skill) */}
          {workflowAction && (
            <>
              <button
                type="button"
                onClick={async e => {
                  setIsMenuOpen(false)
                  await workflowAction.action(e)
                }}
                disabled={isSkillRunning}
                className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] font-semibold text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                title={workflowAction.title}
              >
                {isSkillRunning && runningSkillId === workflowAction.id ? (
                  <Loader2 size={12} className="animate-spin" />
                ) : (
                  workflowAction.icon
                )}
                <span className="truncate">{workflowAction.label}</span>
              </button>
              <div className="h-px bg-[var(--border-color)] my-1" />
            </>
          )}

          <button
            type="button"
            onClick={() => {
              setIsMenuOpen(false)
              setSelectedTask(task)
            }}
            className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <Eye size={12} className="text-blue-400" />
            <span>Voir les détails</span>
          </button>

          {/* Discuter : ouvre l'agent en session interactive, sans lancer de skill. */}
          {!isFinishedTask && (
            <button
              type="button"
              onClick={async () => {
                setIsMenuOpen(false)
                await runSkill(task.id, 'discuss')
              }}
              disabled={isSkillRunning}
              title="Ouvrir une session avec l'agent sur cette tâche, sans lancer de skill"
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSkillRunning && runningSkillId === 'discuss' ? <Loader2 size={12} className="animate-spin" /> : <MessageCircle size={12} className="text-cyan-400" />}
              <span>Discuter</span>
            </button>
          )}

          {/* Adjust : masqué quand c'est déjà l'action de l'étape courante */}
          {!task.prUrl && resolveTaskStage(task, taskProject) === 'implemented' && (
            <button
              type="button"
              onClick={async () => {
                setIsMenuOpen(false)
                await runSkill(task.id, prRecoverySkill(taskProject), 'PR recovery: preserve accepted work and attained stage; create/reuse/link the PR after owner checks. Do not advance to reviewed.')
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-purple-400 hover:bg-purple-500/10 transition-colors cursor-pointer"
            >
              <GitPullRequest size={12} />
              <span>Complete PR setup in the earlier stage</span>
            </button>
          )}

          {task.prUrl && resolveTaskStage(task, taskProject) === 'reviewed' && <button type="button" onClick={async () => { setIsMenuOpen(false); await runSkill(task.id, 'adjust') }} className="w-full px-2.5 py-1.5 text-purple-400 text-left text-xs">Adjust again</button>}
          {/* Merge / Finaliser : masqué quand c'est déjà l'action de l'étape courante */}
          {task.status !== 'finished' && workflowAction?.id !== 'handoff' && (
            <button
              type="button"
              onClick={async () => {
                setIsMenuOpen(false)
                await runSkill(task.id, 'handoff')
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-emerald-400 hover:bg-emerald-500/10 transition-colors cursor-pointer"
            >
              <CheckCircle2 size={12} />
              <span>Handoff after human merge</span>
            </button>
          )}


          {externalUrl && (
            <a
              href={externalUrl}
              target="_blank"
              rel="noreferrer"
              onClick={() => setIsMenuOpen(false)}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            >
              <ExternalLink size={12} className="text-amber-400" />
              <span>Ouvrir sur le tracker</span>
            </a>
          )}

          <button
            type="button"
            onClick={() => {
              setIsMenuOpen(false)
              openCloneModal(task)
            }}
            className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            title="Créer une copie de cette story"
          >
            <CopyPlus size={12} className="text-cyan-400" />
            <span>Cloner la story</span>
          </button>

          <button
            type="button"
            onClick={() => {
              setIsMenuOpen(false)
              navigator.clipboard.writeText(`${task.key}: ${task.title}`)
              addToast({ type: 'info', title: 'Copié', description: `${task.key} copié dans le presse-papier` })
            }}
            className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <Copy size={12} className="text-slate-400" />
            <span>Copier la référence</span>
          </button>

          {Boolean(task.sprint) && (
            <button
              type="button"
              onClick={async () => {
                setIsMenuOpen(false)
                await setTaskSprint(task.id, '', '')
              }}
              className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-amber-400 hover:bg-amber-500/10 transition-colors cursor-pointer"
              title="Retirer la tâche du sprint et la renvoyer au backlog"
            >
              <X size={12} />
              <span>Retirer du sprint</span>
            </button>
          )}

          <div className="h-px bg-[var(--border-color)] my-1" />

          <button
            type="button"
            onClick={() => {
              setIsMenuOpen(false)
              if (window.confirm(`Supprimer la tâche ${task.key} ?`)) {
                deleteTask(task.id)
              }
            }}
            className="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-lg text-[11px] text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
          >
            <Trash2 size={12} />
            <span>Supprimer la tâche</span>
          </button>
        </div>,
        document.body
      )}
    </div>
  )

  return (
    <div
      draggable
      onDragStart={handleDragStartInternal}
      onClick={() => setSelectedTask(task)}
      className={`group relative border bg-[var(--bg-secondary)] ${isCondensed ? 'rounded-none px-1.5 py-1' : 'rounded-2xl p-3'} hover:shadow-md transition-all duration-150 cursor-grab active:cursor-grabbing select-none ${
        isRunning
          ? 'border-indigo-500/60 shadow-md shadow-indigo-500/10 ring-1 ring-indigo-500/20'
          : isQueued
          ? 'border-amber-500/50 shadow-md shadow-amber-500/10 ring-1 ring-amber-500/20'
          : 'border-[var(--border-color)] hover:border-[var(--accent-color)]/60'
      } ${
        isDragging ? 'opacity-40 scale-95 ring-2 ring-[var(--accent-color)] ring-dashed' : ''
      }`}
    >
      {isCondensed ? (
        <div className="flex items-center gap-1 min-w-0">
          {externalUrl ? (
            <a href={externalUrl} target="_blank" rel="noreferrer" onClick={e => e.stopPropagation()} className="shrink-0 whitespace-nowrap text-[10px] font-mono font-bold text-[var(--accent-color)] hover:underline focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]">
              {task.key}
            </a>
          ) : <span className="shrink-0 whitespace-nowrap text-[10px] font-mono font-bold text-[var(--accent-color)]">{task.key}</span>}
          <button type="button" title={task.title} onClick={e => { e.stopPropagation(); setSelectedTask(task) }} className="min-w-0 flex-1 truncate text-left text-[11px] font-semibold text-[var(--text-primary)] leading-none cursor-pointer focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]">
            {task.title}
          </button>
          <RemoteRunBadge taskId={task.id} />
          {actionsMenu}
        </div>
      ) : (
        <>
      {/* Ligne 1 : Référence (Parent / Tâche) + pastille de priorité */}
      <div className="flex items-center justify-between gap-2 mb-1">
        {/* Référence : ParentID / TaskID */}
        <span className="inline-flex items-baseline text-[11px] font-mono font-bold min-w-0">
          {task.parentKey && (
            <>
              <button
                type="button"
                onClick={e => {
                  e.stopPropagation()
                  setParentFilter(parentFilter === task.parentKey ? null : task.parentKey!)
                }}
                className={`hover:underline cursor-pointer transition-colors ${
                  parentFilter === task.parentKey
                    ? 'text-violet-300'
                    : 'text-[var(--text-muted)] hover:text-violet-300'
                }`}
                title={`${task.parentType || 'Parent'} ${task.parentKey}${task.parentTitle ? ` — ${task.parentTitle}` : ''} (cliquer pour filtrer)`}
              >
                {task.parentKey}
              </button>
              <span className="mx-0.5 text-[var(--text-muted)] opacity-50">/</span>
            </>
          )}
          {externalUrl ? (
            <a
              href={externalUrl}
              target="_blank"
              rel="noreferrer"
              onClick={e => e.stopPropagation()}
              className="inline-flex items-center gap-0.5 text-[var(--accent-color)] hover:underline"
              title={`Ouvrir ${task.key} sur le tracker externe`}
            >
              <span>{task.key}</span>
              <ExternalLink size={9} className="opacity-70" />
            </a>
          ) : (
            <span className="text-[var(--accent-color)]">{task.key}</span>
          )}
        </span>

        {/* Pastille de priorité */}
        {getPriorityBadge(task.priority)}
      </div>

      {/* Ligne 1b : Titre, sous la référence */}
      <h4 className="text-xs font-semibold text-[var(--text-primary)] leading-snug line-clamp-2 mb-1.5">
        {task.issueType && (
          <span
            className="inline-flex items-center align-middle mr-1.5 px-1.5 py-0.5 rounded text-[9px] font-bold uppercase tracking-wide"
            style={{
              color: issueTypeStyle(task.issueType).color,
              background: issueTypeStyle(task.issueType).background,
              border: `1px solid ${issueTypeStyle(task.issueType).border}`,
            }}
            title={`Type de ticket : ${task.issueType}`}
          >
            {issueTypeStyle(task.issueType).short}
          </span>
        )}
        {task.title}
      </h4>

      {/* Ligne 2 : Description tronquée */}
      {task.description && (
        <p className="text-[11px] text-[var(--text-muted)] line-clamp-2 mb-2 leading-relaxed">
          {task.description}
        </p>
      )}

      {/* Ligne 3 : Métadonnées / Liens : Branche Git + Icône PR + Labels */}
      <div className="flex items-center justify-between gap-1.5 mb-2.5 flex-wrap">
        <div className="flex items-center gap-1.5 flex-wrap min-w-0" onClick={e => e.stopPropagation()}>
          {/* Lien Branche Git, sur un projet mono-dépôt seulement : ailleurs la
              branche d'un ticket ne dit pas dans quel dépôt elle vit. */}


          {/* Icône PR uniquement */}
          {task.prUrl && (
            <a
              href={task.prUrl}
              target="_blank"
              rel="noreferrer"
              className="p-1 rounded text-purple-300 bg-purple-500/10 hover:bg-purple-500/25 border border-purple-500/30 transition-all hover:scale-105"
              title={task.prUrl.includes('gitlab') ? `GitLab MR: ${task.prUrl}` : `GitHub PR: ${task.prUrl}`}
            >
              <GitPullRequest size={12} className="text-purple-400" />
            </a>
          )}

          {/* Labels compacts (max 2 visibles pour ne pas surcharger) */}
          {task.labels && task.labels.slice(0, 2).map(lbl => (
            <span
              key={lbl}
              className="text-[9.5px] px-1.5 py-0.2 rounded bg-[var(--bg-tertiary)] text-[var(--text-muted)] border border-[var(--border-color)]/70 font-mono"
            >
              #{lbl.replace(/^#+/, '')}
            </span>
          ))}
          {task.labels && task.labels.length > 2 && (
            <span className="text-[9px] text-[var(--text-muted)] opacity-70">
              +{task.labels.length - 2}
            </span>
          )}
        </div>

        {/* Assignee / Date */}
        <div className="flex items-center gap-1.5 shrink-0 ml-auto text-[10px] text-[var(--text-muted)]">
          {task.statusChangedAt && shortElapsed(task.statusChangedAt) && (
            <span
              className="flex items-center gap-0.5 font-medium"
              style={{ color: isElapsedStale(task.statusChangedAt) ? 'var(--status-warn)' : 'var(--text-muted)' }}
              title={`Dans cette catégorie de statut depuis le ${new Date(task.statusChangedAt).toLocaleDateString()}`}
            >
              <Clock size={10} />
              <span>{shortElapsed(task.statusChangedAt)}</span>
            </span>
          )}
          {task.assignee && (
            <Avatar name={task.assignee} url={task.assigneeAvatar} size={18} />
          )}
          {task.dueDate && (
            <span className="flex items-center gap-0.5 text-amber-400 font-medium" title={`Échéance : ${task.dueDate}`}>
              <Calendar size={10} />
              <span>{task.dueDate.split('T')[0]?.slice(5)}</span>
            </span>
          )}
        </div>
      </div>

      {/* Ligne 4 : Activité live / queued + menu d'actions (...) */}
      <div className="pt-2 border-t border-[var(--border-color)]/50 flex items-center gap-1.5" onClick={e => e.stopPropagation()}>
        <RemoteRunBadge taskId={task.id} />
        {/* Live / Queued Activity indicator */}


        {/* Épingle : le ticket rejoint la barre de bascule à chaud, en haut. */}
        <button
          type="button"
          onClick={e => {
            e.stopPropagation()
            togglePin(task.id)
          }}
          className={`p-1 rounded-md border transition-colors cursor-pointer ${
            isPinned(task.id)
              ? 'accent-text bg-[var(--accent-light)] border-[var(--accent-color)]/40'
              : 'text-[var(--text-muted)] hover:text-[var(--accent-color)] hover:bg-[var(--accent-light)] border-transparent hover:border-[var(--accent-color)]/30'
          }`}
          title={isPinned(task.id) ? 'Retirer de la barre des épinglés' : 'Épingler pour basculer vite dessus'}
        >
          <Pin size={14} />
        </button>

        {/* Le terminal de la tâche est l'action la plus fréquente : elle mérite
            son icône, le reste vit dans le menu (...) */}
        <button
          type="button"
          disabled={advancing !== null || isFinishedTask}
          onClick={e => {
            e.stopPropagation()
            handleAdvance(false)
          }}
          className="ml-auto p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--accent-color)] hover:bg-[var(--accent-light)] border border-transparent hover:border-[var(--accent-color)]/30 transition-colors cursor-pointer disabled:opacity-40"
          title={nextStepInfo.stepTooltip}
        >
          {advancing === 'step' ? <Loader2 size={14} className="animate-spin" /> : <ChevronRight size={14} />}
        </button>

        <button
          type="button"
          disabled={advancing !== null || isFinishedTask}
          onClick={e => {
            e.stopPropagation()
            handleAdvance(true)
          }}
          className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--accent-color)] hover:bg-[var(--accent-light)] border border-transparent hover:border-[var(--accent-color)]/30 transition-colors cursor-pointer disabled:opacity-40"
          title={nextStepInfo.autoTooltip}
        >
          {advancing === 'auto' ? <Loader2 size={14} className="animate-spin" /> : <ChevronsRight size={14} />}
        </button>


        {actionsMenu}
      </div>
        </>
      )}
    </div>
  )
}
