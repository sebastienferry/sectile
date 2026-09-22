import { RemoteRunBadge } from './RemoteRunBadge'
import { CopyTaskSkillMenu } from './CopyTaskSkillMenu'
import React, { useState, useEffect, useMemo, useCallback } from 'react'
import {
  X,
  Trash2,
  Pin,
  PinOff,
  Users,
  CalendarRange,
  Calendar,
  User,
  Sparkles,
  HelpCircle,
  FileCode,
  Flame,
  ShieldCheck,
  ExternalLink,
  Loader2,
  CheckCircle2,
  Clock,
  AlertCircle,
  History,
  Terminal,
  PanelRight,
  Square,
  Bot,
  MessageCircle,
  Save,
  Check,
  Copy,
  CopyPlus,
  MessageSquare,
  ArrowRight,
  Folder,
  FolderGit2,
  Maximize2,
  Minimize2,
  RefreshCw,
  Target,
  GitPullRequest,
  Plus,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import type { TeamMember, Status, Priority, DetailMode, SpecFramework, WorkflowStage, MacroMeta, SkillMode, PullRequestLink } from '../types'
import { WORKFLOW_ORDER, prRecoverySkill, resolveTaskStage } from '../lib/workflow'
import { addPullRequestLink, taskPullRequestLinks } from '../lib/pullRequests'
import { TaskComments } from './TaskComments'
import { LookupField, type LookupOption } from './LookupField'
import { MarkdownEditor } from './Markdown'
import { sprintLookup, macroLookup, isProjectCompatible } from '../lib/lookups'
import { issueTypeStyle } from '../lib/issueTypes'
import { providerModels, providerTakesModel, resolveConfiguredModel, templateGovernsCommand } from '../lib/aiModels'
import { runEngineLabel } from '../lib/runEngine'

export const TaskDetailModal: React.FC = () => {
  const {
    selectedTask,
    setSelectedTask,
    updateTask,
    deleteTask,
    openCloneModal,
    migrateTasks,
    runSkill,
    isSkillRunning,
    runningSkillId,
    skills,
    projects,
    tasks,
    activities: globalActivities,
    settings,
    updateSettings,
    addToast,
    membersForTeam,
    searchAssignableUsers,
    searchTrackerTeams,
    setTaskTeam,
    setTaskSprint,
    setTaskMacro,
    createMacro,
    fetchProjectMacros,
    togglePin,
    isPinned,
    syncSingleTask,
    t,
  } = useApp()

  const [projectMacros, setProjectMacros] = useState<MacroMeta[]>([])

  useEffect(() => {
    const projId = selectedTask?.projectId || projects[0]?.id
    if (projId) {
      fetchProjectMacros(projId).then(macros => {
        setProjectMacros(macros || [])
      }).catch(() => {})
    }
  }, [selectedTask?.projectId, projects, fetchProjectMacros])

  const availableMacros = useMemo(() => {
    const combined: MacroMeta[] = [...projectMacros]
    const currentProjId = selectedTask?.projectId || projects[0]?.id
    const distinctTaskMacros = tasks
      .filter(t => t.projectId === currentProjId && (t.parentKey || t.parentTitle))
      .map(t => ({ key: t.parentKey || t.parentTitle || '', title: t.parentTitle || t.parentKey || '' }))
    for (const dm of distinctTaskMacros) {
      if (!combined.some(e => e.key.toLowerCase() === dm.key.toLowerCase() || (e.title && e.title.toLowerCase() === dm.title.toLowerCase()))) {
        combined.push({
          projectId: currentProjId || 'default',
          key: dm.key,
          title: dm.title,
          horizon: 'now',
          description: '',
          todos: [],
          status: 'open',
          closed: false,
          updatedAt: new Date().toISOString(),
        })
      }
    }
    return combined
  }, [projectMacros, selectedTask?.projectId, projects, tasks])

  const searchMacro = useMemo(() => macroLookup(availableMacros), [availableMacros])

  // The task's own project drives the AI provider, the command template and the
  // skill overrides. It is NOT necessarily the project selected in the sidebar:
  // on an "all projects" view currentProject is null, which used to silently
  // fall back to the global settings (hence "AGY" on a Claude-configured project).
  // Deliberately no fallback to the sidebar's selected project: falling back to
  // it is what produced the wrong provider. With no task project we let the
  // global settings apply, which is the honest default.
  const taskProject = React.useMemo(
    () => projects.find(p => p.id === selectedTask?.projectId) || null,
    [projects, selectedTask?.projectId]
  )

  // Resolved once for the whole modal: every label and every command must name
  // the same CLI, otherwise the badge says AGY while the command runs Claude.
  const activeProvider = taskProject?.aiProvider || settings.aiProvider || 'agy'

  // Les modèles proposés au lancement sont ceux configurés pour le moteur de ce
  // projet : rien n'est saisi à la main ici, contrairement aux réglages. Le
  // premier choix est le modèle que la précédence résout, et il n'envoie aucune
  // surcharge, donc un lancement non touché reproduit la commande d'avant.
  const launchModels = providerModels(settings, activeProvider)
  // Le sélecteur vaut pour toutes les compétences de la vue, alors que la
  // résolution dépend de la compétence lancée : on nomme donc le modèle de base,
  // celui qu'appliquent les compétences qu'aucun niveau ne singularise. Il est
  // retiré des autres choix, sinon le reprendre enverrait une surcharge là où le
  // premier choix n'en envoie aucune.
  const configuredLaunchModel = resolveConfiguredModel(taskProject || undefined, settings)
  const activeTemplate = taskProject?.aiCommandTemplate || settings.aiCommandTemplate || ''
  const launchModelNotice = templateGovernsCommand(activeProvider, activeTemplate)
    ? (t?.profileModal?.ai?.modelTemplatePlaceholderNotice || "Le modèle est appliqué via le marqueur {model} dans la commande.")
    : !providerTakesModel(activeProvider)
      ? (t?.profileModal?.ai?.providerIgnoresModel
          ? t.profileModal.ai.providerIgnoresModel.replace('{provider}', activeProvider.toUpperCase())
          : `${activeProvider.toUpperCase()} n'accepte pas de sélection de modèle : la valeur est ignorée.`)
      : ''



  const [isSyncingTask, setIsSyncingTask] = useState(false)

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [status, setStatus] = useState<Status>('backlog')
  const [priority, setPriority] = useState<Priority>('medium')
  const [taskProjectId, setTaskProjectId] = useState<string>(projects[0]?.id || '')
  const [branchName, setBranchName] = useState('')
  // L'ensemble ordonné des pull requests du ticket. `prUrl` en est le dernier
  // lien : le serveur le recalcule, la fiche n'édite que l'ensemble.
  const [prLinks, setPrLinks] = useState<PullRequestLink[]>([])
  const [newPrUrl, setNewPrUrl] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [trackerStatus, setTrackerStatus] = useState('')
  const [sprint, setSprint] = useState('')
  const [labels, setLabels] = useState<string[]>([])
  const [newLabelInput, setNewLabelInput] = useState('')
  const [assignee, setAssignee] = useState('')
  // Personnes de l'équipe du ticket. L'équipe est facultative : sans elle, le
  // champ reste une saisie libre, comme avant.
  const [teamMembers, setTeamMembers] = useState<TeamMember[]>([])
  // Identifiant de compte du choix courant : Jira n'assigne que par accountId.
  const [assigneeAccountId, setAssigneeAccountId] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [activeTab, setActiveTab] = useState<'details' | 'comments' | 'skills' | 'git' | 'cadrage' | 'history'>('details')
  const [customPrompt, setCustomPrompt] = useState('')
  // Surcharge ponctuelle du mode d'exécution. Vide veut dire « mode configuré » :
  // aucune surcharge n'est envoyée et la précédence s'applique normalement.
  const [launchMode, setLaunchMode] = useState<SkillMode>('')
  // Le modèle choisi pour les lancements de cette vue. Vide veut dire « le
  // modèle configuré », donc aucune surcharge envoyée.
  const [launchModel, setLaunchModel] = useState('')
  // Le choix ne vaut que pour la liste devant lui : ouvrir une tâche d'un projet
  // dont le moteur diffère ne doit pas lancer le modèle retenu pour le projet
  // précédent. Dériver la valeur plutôt que la remettre à zéro dans un effet
  // évite aussi qu'un rendu intermédiaire l'envoie encore.
  const effectiveLaunchModel = launchModels.includes(launchModel) ? launchModel : ''

  const [specFramework, setSpecFramework] = useState<SpecFramework>(settings.specFramework || 'speckit')
  const [isExpandedSpec, setIsExpandedSpec] = useState(false)
  const [copiedSpec, setCopiedSpec] = useState(false)
  const [withComments, setWithComments] = useState(false)
  const [dismissedRewriteId, setDismissedRewriteId] = useState<string | null>(null)
  const [isMaximized, setIsMaximized] = useState(false)


  const [isTtyExpanded, setIsTtyExpandedState] = useState<boolean>(() => {
    try {
      return localStorage.getItem('sectile_modal_tty_expanded') === 'true'
    } catch {
      return false
    }
  })
  const setIsTtyExpanded = (expanded: boolean | ((prev: boolean) => boolean)) => {
    setIsTtyExpandedState(prev => {
      const next = typeof expanded === 'function' ? expanded(prev) : expanded
      try {
        localStorage.setItem('sectile_modal_tty_expanded', String(next))
      } catch {}
      return next
    })
  }


  const detailMode: DetailMode = settings.detailMode || 'panel'

  // CWD hérité (projet, puis réglage global) et CWD réellement utilisé, le
  // ticket pouvant épingler son propre dépôt.
  const inheritedRepoPath = taskProject?.repoPath || settings.repoPath || ''
  const effectiveRepoPath = repoPath.trim() || inheritedRepoPath
  // Répertoires proposés : celui du projet, puis ceux enregistrés sur le projet
  // (alimentés automatiquement dès qu'un ticket en épingle un nouveau).
  // Statuts du projet, groupés par colonne, et étape du workflow associée. Les
  // deux sélecteurs de la fiche sont deux vues du même mapping : changer l'un
  // met l'autre à jour, et le serveur refait la même dérivation de son côté.
  const projectColumns = taskProject?.trackerColumns || []
  const projectStageColumns = taskProject?.stageColumns || {}
  const hasProjectStatuses = projectColumns.some(c => c.statuses.length > 0)

  const stageOfStatus = (value: string): WorkflowStage | null => {
    const clean = value.toLowerCase().trim()
    if (clean === 'done' || clean === 'closed' || clean === 'finished') return 'finished'
    const column = projectColumns.find(c => c.statuses.some(st => st.toLowerCase() === clean) || c.name.toLowerCase() === clean)
    if (!column) return null
    if (column.name.toLowerCase() === 'done' || column.name.toLowerCase() === 'closed') return 'finished'
    for (const stage of WORKFLOW_ORDER) {
      if ((projectStageColumns[stage] || []).includes(column.name)) return stage
    }
    return null
  }

  const statusOfStage = (stage: WorkflowStage): string => {
    for (const columnName of projectStageColumns[stage] || []) {
      const column = projectColumns.find(c => c.name === columnName)
      if (column?.statuses.length) return column.statuses[0]
    }
    if (stage === 'finished') return 'Done'
    return ''
  }

  const currentStage: WorkflowStage =
    (labels.map(l => l.toLowerCase().replace(/^#+/, '')).find(l =>
      (WORKFLOW_ORDER as string[]).includes(l)
    ) as WorkflowStage | undefined) ||
    (trackerStatus && stageOfStatus(trackerStatus)) ||
    'new'

  const applyStage = (stage: WorkflowStage) => {
    // L'étape remplace le label de workflow existant et emmène le statut avec
    // elle quand le projet dit vers quelle colonne aller.
    const others = labels.filter(l => !(WORKFLOW_ORDER as string[]).includes(l.toLowerCase().replace(/^#+/, '')))
    setLabels([...others, stage])
    const target = statusOfStage(stage)
    if (target) setTrackerStatus(target)
  }

  const applyTrackerStatus = (value: string) => {
    setTrackerStatus(value)
    const stage = stageOfStatus(value)
    if (stage) {
      const others = labels.filter(l => !(WORKFLOW_ORDER as string[]).includes(l.toLowerCase().replace(/^#+/, '')))
      setLabels([...others, stage])
    }
  }


  useEffect(() => {
    if (selectedTask) {
      setTitle(selectedTask.title)
      setDescription(selectedTask.description || '')
      setStatus(selectedTask.status)
      setPriority(selectedTask.priority)
      setTaskProjectId(selectedTask.projectId || projects[0]?.id || '')
      setBranchName(selectedTask.branchName || '')
      setPrLinks(taskPullRequestLinks(selectedTask))
      setNewPrUrl('')
      setRepoPath(selectedTask.repoPath || '')
      setTrackerStatus(selectedTask.trackerStatus || '')
      setSprint(selectedTask.sprint || '')
      setLabels(selectedTask.labels || [])
      setAssignee(selectedTask.assignee || '')
      setAssigneeAccountId('')
      setDueDate(selectedTask.dueDate || '')
      setSpecFramework(settings.specFramework || 'speckit')
    }
  }, [selectedTask, projects, settings.specFramework])

  // Les membres de l'équipe du ticket alimentent le choix de l'assigné. Le
  // rapprochement avec l'assigné courant se fait sur le nom affiché : c'est ce
  // que le tracker écrit sur le ticket.
  useEffect(() => {
    const team = (selectedTask?.team || '').trim()
    if (!team) {
      setTeamMembers([])
      return
    }
    let alive = true
    membersForTeam(team).then(list => {
      if (alive) setTeamMembers(list)
    })
    return () => {
      alive = false
    }
  }, [selectedTask?.id, selectedTask?.team, membersForTeam])

  useEffect(() => {
    if (!assignee) {
      setAssigneeAccountId('')
      return
    }
    const match = teamMembers.find(m => m.displayName === assignee)
    setAssigneeAccountId(match?.accountId || '')
  }, [assignee, teamMembers])

  // Recherche des personnes assignables : sans frappe, le serveur répond par
  // l'équipe du ticket, ce qui couvre l'essentiel des cas sans appel au tracker.
  const searchAssignee = React.useCallback(
    async (query: string): Promise<LookupOption[]> => {
      if (!selectedTask) return []
      const people = await searchAssignableUsers(selectedTask.id, query)
      const options: LookupOption[] = people.map(m => ({
        id: m.accountId,
        label: m.displayName,
        sublabel: m.email || (m.teamName ? `Équipe ${m.teamName}` : undefined),
        avatarUrl: m.avatarUrl,
        muted: !m.active,
      }))
      // L'assigné courant reste proposé même s'il ne ressort pas de la
      // recherche : sinon le champ paraîtrait vide de toute valeur valable.
      if (!query && assignee && !options.some(o => o.label === assignee)) {
        options.unshift({ id: assigneeAccountId, label: assignee, sublabel: 'assigné actuel' })
      }
      return options
    },
    [selectedTask, searchAssignableUsers, assignee, assigneeAccountId]
  )

  // Sprints du projet ou extraits des tickets : cherchés au clavier avec auto-complétion
  const taskSprints = React.useMemo(() => {
    const proj = projects.find(p => p.id === (selectedTask?.projectId || taskProjectId))
    const projSprints = (proj?.sprints || []).filter(sp => sp.name && sp.state !== 'closed')
    const distinctTaskSprints = Array.from(
      new Set(
        tasks
          .filter(t => !selectedTask?.projectId || t.projectId === selectedTask.projectId)
          .map(t => (t.sprint || '').trim())
          .filter(Boolean)
      )
    )
    const combined = [...projSprints]
    for (const name of distinctTaskSprints) {
      if (!combined.some(s => s.name.toLowerCase() === name.toLowerCase() || (s.id && s.id.toLowerCase() === name.toLowerCase()))) {
        combined.push({ id: name, name, state: 'future' })
      }
    }
    return combined
  }, [projects, selectedTask?.projectId, taskProjectId, tasks])
  const searchSprint = React.useMemo(() => sprintLookup(taskSprints), [taskSprints])

  const searchTeam = React.useCallback(
    async (query: string): Promise<LookupOption[]> => {
      const found = await searchTrackerTeams(query)
      return found
        .filter(team => team.id)
        .map(team => ({
          id: team.id,
          label: team.name,
          sublabel: team.taskCount ? `${team.taskCount} ticket(s) sur ce board` : undefined,
        }))
    },
    [searchTrackerTeams]
  )

  const currentTaskProject = projects.find(p => p.id === (selectedTask?.projectId || taskProjectId))
  const targetGithubRepo = (currentTaskProject?.githubRepo || settings.githubRepo || '').replace(/^https?:\/\/github\.com\//, '').replace(/\.git$/, '')
  const externalUrl = selectedTask?.externalUrl || (
    selectedTask?.source === 'github' && targetGithubRepo && selectedTask?.key?.startsWith('#')
      ? `https://github.com/${targetGithubRepo}/issues/${selectedTask.key.replace('#', '')}`
      : undefined
  )

  // Un lien ajouté prend la branche du ticket : c'est elle que les validateurs
  // comparent pour distinguer une PR de suite d'une PR sans rapport.
  const addPrLink = () => {
    const next = addPullRequestLink(prLinks, newPrUrl, branchName || selectedTask?.branchName)
    if (next === prLinks) return
    setPrLinks(next)
    setNewPrUrl('')
  }

  const handleClose = async () => {
    if (selectedTask && title.trim()) {
      const isModified =
        title.trim() !== selectedTask.title ||
        description.trim() !== (selectedTask.description || '').trim() ||
        status !== selectedTask.status ||
        priority !== selectedTask.priority ||
        taskProjectId !== (selectedTask.projectId || '') ||
        branchName.trim() !== (selectedTask.branchName || '').trim() ||
        JSON.stringify(prLinks) !== JSON.stringify(taskPullRequestLinks(selectedTask)) ||
        repoPath.trim() !== (selectedTask.repoPath || '').trim() ||
        trackerStatus.trim() !== (selectedTask.trackerStatus || '').trim() ||
        sprint.trim() !== (selectedTask.sprint || '').trim() ||
        assignee.trim() !== (selectedTask.assignee || '').trim() ||
        (dueDate || '') !== (selectedTask.dueDate || '') ||
        JSON.stringify(labels) !== JSON.stringify(selectedTask.labels || [])

      if (isModified) {
        await updateTask(selectedTask.id, {
          title: title.trim(),
          description: description.trim(),
          status,
          priority,
          projectId: taskProjectId,
          labels,
          assignee: assignee.trim(),
          assigneeAccountId,
          sprint: sprint.trim(),
          dueDate: dueDate || null,
          branchName: branchName.trim() || undefined,
          prLinks,
          repoPath: repoPath.trim(),
          trackerStatus: trackerStatus.trim(),
        })
      }
    }
    setSelectedTask(null)
  }

  useEffect(() => {
    if (!selectedTask) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (isTtyExpanded) {
          setIsTtyExpanded(false)
        } else if (isExpandedSpec) {
          setIsExpandedSpec(false)
        } else {
          handleClose()
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedTask, isTtyExpanded, isExpandedSpec, title, description, status, priority, taskProjectId, branchName, prLinks, repoPath, assignee, dueDate, labels])

  // Le clic à côté ferme comme la croix : handleClose pour la fiche, qui
  // enregistre en sortant, et le seul lecteur de spécification pour sa propre
  // couche, qui laisse la fiche ouverte dessous.
  const backdrop = useBackdropDismiss(handleClose)
  const closeExpandedSpec = useCallback(() => setIsExpandedSpec(false), [setIsExpandedSpec])
  const expandedSpecBackdrop = useBackdropDismiss(closeExpandedSpec)

  if (!selectedTask) return null

  // A tracker URL for any key of the same tracker as this task: used for the
  // task itself and for its parent, which the tracker payload carries as a key
  // without a URL of its own.
  const trackerUrlForKey = (key?: string): string | undefined => {
    if (!key) return undefined
    const source = selectedTask.source
    if (source === 'jira') {
      const base = (currentTaskProject?.trackerUrl || settings.jiraUrl || '').replace(/\/+$/, '')
      if (base) return `${base}/browse/${key}`
      // No base configured: reuse the host of the task's own tracker URL.
      const m = externalUrl?.match(/^(https?:\/\/[^/]+)\/browse\//)
      return m ? `${m[1]}/browse/${key}` : undefined
    }
    if (source === 'github') {
      const num = key.replace(/^#/, '')
      return targetGithubRepo && /^\d+$/.test(num)
        ? `https://github.com/${targetGithubRepo}/issues/${num}`
        : undefined
    }
    return undefined
  }

  const taskUrl = externalUrl || trackerUrlForKey(selectedTask.key)
  const parentUrl = trackerUrlForKey(selectedTask.parentKey)
  const trackerName =
    selectedTask.source === 'github' ? 'GitHub'
    : selectedTask.source === 'jira' ? 'Jira'
    : 'le tracker'

  /**
   * The reference badge: ParentKey / TaskKey, each opening its own tracker
   * item. Replaces the former key badge + parent chip + separate tracker link.
   */
  const renderTaskRef = () => (
    <span className="font-mono text-sm font-bold text-[var(--accent-color)] bg-[var(--accent-light)] px-2.5 py-1 rounded-lg flex items-center gap-1.5 shrink-0">
      {selectedTask.source === 'github' && <FolderGit2 size={13} className="text-purple-400" />}
      {selectedTask.source === 'jira' && <span className="text-blue-400 font-sans font-black text-xs">J</span>}
      {(!selectedTask.source || selectedTask.source === 'local') && <Folder size={13} className="text-emerald-400" />}

      <span className="inline-flex items-baseline min-w-0">
        {selectedTask.parentKey && (
          <>
            {parentUrl ? (
              <a
                href={parentUrl}
                target="_blank"
                rel="noreferrer"
                className="text-[var(--text-muted)] hover:text-violet-300 hover:underline"
                title={`Ouvrir ${selectedTask.parentType || 'le parent'} ${selectedTask.parentKey}${selectedTask.parentTitle ? ` — ${selectedTask.parentTitle}` : ''} sur ${trackerName}`}
              >
                {selectedTask.parentKey}
              </a>
            ) : (
              <span
                className="text-[var(--text-muted)]"
                title={`${selectedTask.parentType || 'Parent'} ${selectedTask.parentKey}${selectedTask.parentTitle ? ` — ${selectedTask.parentTitle}` : ''}`}
              >
                {selectedTask.parentKey}
              </span>
            )}
            <span className="mx-1 text-[var(--text-muted)] opacity-50">/</span>
          </>
        )}
        {taskUrl ? (
          <a
            href={taskUrl}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 hover:underline"
            title={`Ouvrir ${selectedTask.key} sur ${trackerName}`}
          >
            <span>{selectedTask.key}</span>
            <ExternalLink size={11} className="opacity-70" />
          </a>
        ) : (
          <span>{selectedTask.key}</span>
        )}
      </span>
    </span>
  )

  const renderIssueTypeSelector = () => {
    if (!selectedTask) return null
    const current = selectedTask.issueType || ''
    return (
      <select
        value={current}
        onChange={e => updateTask(selectedTask.id, { issueType: e.target.value })}
        className="px-2 py-0.5 rounded-lg text-[10px] font-bold uppercase tracking-wider border cursor-pointer focus:outline-none transition-colors bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--accent-color)]/50 shrink-0"
        style={
          current
            ? {
                color: issueTypeStyle(current).color,
                background: issueTypeStyle(current).background,
                borderColor: issueTypeStyle(current).border,
              }
            : {}
        }
        title="Changer le type de ticket (Story, Bug, Tâche...)"
      >
        <option value="">Type: Défaut</option>
        <option value="Story">📘 Story</option>
        <option value="Bug">🐛 Bug</option>
        <option value="Task">📝 Tâche</option>
        <option value="Improvement">⚡ Amélioration</option>
        <option value="Technical debt">🔧 Dette technique</option>
      </select>
    )
  }

  const handleSave = async () => {
    if (!title.trim() || isSaving) return

    setIsSaving(true)
    await updateTask(selectedTask.id, {
      title: title.trim(),
      description: description.trim(),
      status,
      priority,
      projectId: taskProjectId,
      labels,
      assignee: assignee.trim(),
      assigneeAccountId,
      sprint: sprint.trim(),
      dueDate: dueDate || null,
      branchName: branchName.trim() || undefined,
      prLinks,
      repoPath: repoPath.trim(),
      trackerStatus: trackerStatus.trim(),
    })
    setIsSaving(false)
    setSelectedTask(null)
  }

  const handleStatusChange = async (newStatus: Status) => {
    setStatus(newStatus)
    if (selectedTask) {
      await updateTask(selectedTask.id, { status: newStatus })
    }
  }

  const handlePriorityChange = async (newPriority: Priority) => {
    setPriority(newPriority)
    if (selectedTask) {
      await updateTask(selectedTask.id, { priority: newPriority })
    }
  }

  const handleDelete = async () => {
    if (window.confirm(t.taskModal.deleteConfirm)) {
      await deleteTask(selectedTask.id)
      setSelectedTask(null)
    }
  }

  const handleAddLabel = async () => {
    const clean = newLabelInput.replace(/^#+/, '').trim()
    if (clean && !labels.some(l => l.replace(/^#+/, '').toLowerCase() === clean.toLowerCase())) {
      const nextLabels = [...labels, clean]
      setLabels(nextLabels)
      setNewLabelInput('')
      if (selectedTask) {
        await updateTask(selectedTask.id, { labels: nextLabels })
      }
    }
  }

  const handleRemoveLabel = async (tagToRemove: string) => {
    const cleanTarget = tagToRemove.replace(/^#+/, '').toLowerCase()
    const nextLabels = labels.filter(l => l.replace(/^#+/, '').toLowerCase() !== cleanTarget)
    setLabels(nextLabels)
    if (selectedTask) {
      await updateTask(selectedTask.id, { labels: nextLabels })
    }
  }

  const handleToggleDetailMode = async () => {
    const nextMode: DetailMode = detailMode === 'panel' ? 'modal' : 'panel'
    await updateSettings({ detailMode: nextMode })
  }

  const getNextRecommendedSkill = () => {
    switch (status) {
      case 'to_clarify':
      case 'backlog':
        return skills.find(s => s.id === 'clarify') || skills[0]
      case 'clarified':
        return skills.find(s => s.id === 'specify') || skills[1]
      case 'to_implement':
      case 'in_progress':
      case 'specified':
        return skills.find(s => s.id === 'implement') || skills[2]
      case 'to_test':
      case 'to_validate':
        return skills.find(s => s.id === 'adjust' || s.id === 'review') || skills[3]
      case 'finished':
        return undefined
      case 'to_close':
        return skills.find(s => s.id === 'handoff') || skills[4]
      default:
        return skills[0]
    }
  }

  // modeOverride est le choix fait sur la carte de la skill, valable pour ce
  // lancement seulement. Sans lui on retombe sur le sélecteur du panneau, dont
  // la valeur vide veut dire « pas de surcharge » : c'est alors la précédence
  // (skill, puis défaut du projet, puis interactif) qui décide.
  const handleTriggerSkill = async (skillId: string, overridePrompt?: string, modeOverride?: SkillMode) => {
    if (!selectedTask || isSkillRunning) return
    const promptToUse = overridePrompt || customPrompt
    const activity = await runSkill(selectedTask.id, skillId, promptToUse, { mode: modeOverride ?? launchMode, model: effectiveLaunchModel })
    if (activity && !overridePrompt) {
      setCustomPrompt('')
    }
  }

  const nextSkill = getNextRecommendedSkill()

  const getSkillIcon = (iconName: string) => {
    switch (iconName) {
      case 'HelpCircle':
        return <HelpCircle size={15} className="text-amber-400" />
      case 'FileCode':
        return <FileCode size={15} className="text-blue-400" />
      case 'Flame':
        return <Flame size={15} className="text-indigo-400" />
      case 'ShieldCheck':
        return <ShieldCheck size={15} className="text-purple-400" />
      default:
        return <Sparkles size={15} className="text-emerald-400" />
    }
  }


  const activities = selectedTask
    ? globalActivities.filter(a => a.taskId === selectedTask.id || a.taskKey === selectedTask.key)
    : []
  const latestActivity = activities.length > 0 ? activities[0] : null
  const clarifyActivity = activities.find(a => a.skillId === 'clarify')
  const specifyActivity = activities.find(a => a.skillId === 'specify')
  const rewriteActivity = activities.find(a => a.skillId === 'rewrite_story' || a.skillId === 'rewrite-story')

  const handleCopySpec = () => {
    if (!specifyActivity?.output) return
    navigator.clipboard?.writeText(specifyActivity.output)
    setCopiedSpec(true)
    setTimeout(() => setCopiedSpec(false), 2000)
    addToast({
      type: 'success',
      title: 'Spécification copiée',
      description: 'La spécification technique a été copiée dans votre presse-papiers.',
    })
  }

  // Technical Specification (Spec Kit / OpenSpec) Section Box
  const renderSpecificationSection = () => {
    if (!specifyActivity?.output) return null

    const isOpenSpec = specFramework === 'openspec'
    const frameworkLabel = isOpenSpec ? 'OpenSpec' : 'Spec Kit'

    return (
      <div className="p-4 rounded-2xl bg-linear-to-b from-[var(--bg-tertiary)] to-[var(--bg-secondary)] border border-blue-500/40 shadow-md space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <FileCode size={16} className={isOpenSpec ? 'text-emerald-400' : 'text-blue-400'} />
            <h4 className="text-xs font-bold text-[var(--text-primary)]">
              Spécification Technique ({frameworkLabel})
            </h4>
          </div>
          <div className="flex items-center gap-2">
            <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase border ${
              isOpenSpec
                ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30'
                : 'bg-blue-500/20 text-blue-300 border-blue-500/30'
            }`}>
              {frameworkLabel} SDD
            </span>
            <button
              type="button"
              onClick={handleCopySpec}
              className="inline-flex items-center gap-1 px-2 py-1 rounded-lg text-[10px] font-mono font-bold bg-[var(--bg-primary)] hover:bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] transition-colors cursor-pointer"
              title="Copier la spec complète"
            >
              {copiedSpec ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
              <span>{copiedSpec ? 'Copié !' : 'Copier'}</span>
            </button>
            <button
              type="button"
              onClick={() => setIsExpandedSpec(true)}
              className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-lg text-[10px] font-bold transition-all shadow-2xs hover:scale-105 cursor-pointer ${
                isOpenSpec
                  ? 'text-emerald-300 bg-emerald-500/15 hover:bg-emerald-500/25 border border-emerald-500/30'
                  : 'text-blue-300 bg-blue-500/15 hover:bg-blue-500/25 border border-blue-500/30'
              }`}
              title="Agrandir la spécification en mode grand format / plein écran"
            >
              <Maximize2 size={11} />
              <span>Agrandir</span>
            </button>
          </div>
        </div>

        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 font-mono text-[11px] max-h-80 overflow-y-auto leading-relaxed shadow-inner">
          <div className="whitespace-pre-wrap">
            {specifyActivity.output}
          </div>
        </div>

        <div className="flex items-center justify-between pt-1">
          <div className="text-[11px] text-[var(--text-muted)] font-mono">
            {specifyActivity.completedAt ? `Généré le ${new Date(specifyActivity.completedAt).toLocaleString()}` : 'Spécification prête'}
          </div>
          <button
            type="button"
            onClick={() => handleTriggerSkill('implement')}
            disabled={isSkillRunning}
            className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-bold text-white shadow-md bg-linear-to-r from-blue-600 to-indigo-600 hover:opacity-90 active:scale-95 transition-all disabled:opacity-50 cursor-pointer"
            title="Lancer l'implémentation du code conformément à cette spécification"
          >
            {isSkillRunning && runningSkillId === 'implement' ? (
              <Loader2 size={13} className="animate-spin" />
            ) : (
              <>
                <Flame size={13} className="text-amber-300" />
                <span>Lancer Implement Code</span>
                <ArrowRight size={13} />
              </>
            )}
          </button>
        </div>
      </div>
    )
  }


  // -------------------------------------------------------------
  // DEDICATED TAB: Git, branche et revue
  // -------------------------------------------------------------
  // Séparé de la Story : ces champs ne servent qu'au moment de coder, et ils
  // occupaient un tiers de l'onglet pour tous les autres moments.
  // -------------------------------------------------------------
  // DEDICATED TAB: Cadrage & Spécifications
  // -------------------------------------------------------------
  const renderCadrageSection = () => (
    <div className="space-y-5">
      <RemoteRunBadge taskId={selectedTask.id} />
      <CopyTaskSkillMenu key={selectedTask.id} task={selectedTask} />
      <div className="rounded-xl border border-[var(--border-color)] p-4 space-y-3">
        <h3 className="font-semibold">Clarification and specification</h3>
        <p className="text-xs text-[var(--text-muted)]">Launch a skill on your local agent. Follow its execution in Sectile Desktop.</p>
        <div className="flex gap-2">
          <button type="button" disabled={isSkillRunning} onClick={()=>handleTriggerSkill('clarify')} className="rounded-lg bg-amber-500/15 text-amber-400 px-3 py-2">Clarify</button>
          <button type="button" disabled={isSkillRunning} onClick={()=>handleTriggerSkill('specify')} className="rounded-lg bg-blue-500/15 text-blue-400 px-3 py-2">Specify</button>
        </div>
      </div>
      {renderSpecificationSection()}
    </div>
  )

  const renderStoryInfoSection = () => (
    <div className="space-y-5">
      {/* Title Input */}
      <div>
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
          Titre de la Story
        </label>
        <input
          type="text"
          value={title}
          onChange={e => setTitle(e.target.value)}
          placeholder={t.taskModal.titlePlaceholder}
          className="w-full px-3.5 py-2 text-sm font-semibold rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
        />
      </div>

      {/* Deux lignes de quatre champs. Première ligne : ce qui pilote le workflow
          (statut, étape, priorité, projet). Seconde ligne, ci-dessous : qui porte
          le ticket, pour quand et pour quelle équipe. Les six colonnes d'avant
          écrasaient chaque champ sur un écran ordinaire. */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {/* Status */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {t.taskModal.status}
          </label>
          {hasProjectStatuses ? (
            <select
              value={trackerStatus}
              onChange={e => applyTrackerStatus(e.target.value)}
              className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
              title="Statuts du projet, tels que le tracker les nomme"
            >
              <option value="">— non défini —</option>
              {projectColumns.map(col => (
                <optgroup key={col.name} label={col.name}>
                  {col.statuses.map(st => (
                    <option key={st} value={st}>{st}</option>
                  ))}
                </optgroup>
              ))}
            </select>
          ) : (
            <select
              value={status}
              onChange={e => handleStatusChange(e.target.value as Status)}
              className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
            >
              <option value="to_clarify">{t.status.to_clarify} (#new)</option>
              <option value="clarified">{t.status.clarified} (#clarified)</option>
              <option value="to_implement">{t.status.to_implement} (#specified)</option>
              <option value="to_test">{t.status.to_test} (#implemented)</option>
              <option value="to_close">{t.status.to_close} (#reviewed)</option>
              <option value="finished">{t.status.finished} (#finished)</option>
            </select>
          )}
        </div>

        {/* Étape du workflow agentique, couplée au statut par le mapping */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Étape agentique
          </label>
          <select
            value={currentStage}
            onChange={e => applyStage(e.target.value as WorkflowStage)}
            className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
            title="Label du workflow agentique. Le statut suit selon le mapping du projet."
          >
            {WORKFLOW_ORDER.map(stage => (
              <option key={stage} value={stage}>#{stage}</option>
            ))}
          </select>
        </div>

        {/* Priority */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {t.taskModal.priority}
          </label>
          <select
            value={priority}
            onChange={e => handlePriorityChange(e.target.value as Priority)}
            className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
          >
            <option value="urgent">{t.priority.urgent}</option>
            <option value="high">{t.priority.high}</option>
            <option value="medium">{t.priority.medium}</option>
            <option value="low">{t.priority.low}</option>
          </select>
        </div>

        {/* Project */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Projet
          </label>
          <select
            value={taskProjectId}
            onChange={async (e) => {
              const val = e.target.value
              if (!val || val === taskProjectId) return
              const sourceProj = projects.find(p => p.id === (selectedTask?.projectId || taskProjectId))
              const targetProj = projects.find(p => p.id === val)
              if (selectedTask) {
                if (sourceProj && targetProj && !isProjectCompatible(sourceProj, targetProj)) {
                  if (!confirm(`Attention: Le projet "${targetProj.name}" a un tracker différent de "${sourceProj.name}". Déplacer ce ticket vers ce projet quand même ?`)) {
                    return
                  }
                }
                setTaskProjectId(val)
                const res = await migrateTasks([selectedTask.id], val)
                if (res.success) {
                  const updated = tasks.find(t => t.id === selectedTask.id)
                  if (updated) setSelectedTask(updated)
                }
              } else {
                setTaskProjectId(val)
              }
            }}
            className="w-full px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
          >
            {(() => {
              const bookmarked = projects.filter(p => p.bookmarked)
              const others = projects.filter(p => !p.bookmarked)
              if (bookmarked.length > 0 && others.length > 0) {
                return (
                  <>
                    <optgroup label="Favoris">
                      {bookmarked.map(p => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({p.issueTracker || 'local'})
                        </option>
                      ))}
                    </optgroup>
                    <optgroup label="Autres projets">
                      {others.map(p => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({p.issueTracker || 'local'})
                        </option>
                      ))}
                    </optgroup>
                  </>
                )
              }
              return projects.map(p => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.issueTracker || 'local'})
                </option>
              ))
            })()}
          </select>
        </div>

      </div>

      {/* Ligne dédiée : assigné, sprint, équipe, échéance. Ces quatre champs sont
          ceux qu'on change en planifiant, et trois d'entre eux sont des écritures
          tracker à part entière. */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {/* Assignee : les membres de l'équipe du ticket sans frappe, puis toute
            l'instance dès qu'on tape. Un assigné hors équipe reste proposé pour
            ne pas effacer silencieusement ce que porte le ticket. */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {t.taskModal.assignee}
            {selectedTask.team && (
              <span className="ml-1 font-normal normal-case tracking-normal text-[var(--text-muted)]">
                · {selectedTask.team}
              </span>
            )}
          </label>
          {selectedTask.source === 'jira' ? (
            <LookupField
              value={assignee}
              icon={<User size={12} />}
              placeholder="Chercher une personne…"
              clearLabel="Non assigné"
              emptyHint="Personne trouvée. Tapez un nom ou un e-mail."
              onSearch={searchAssignee}
              onPick={option => {
                setAssignee(option?.label || '')
                setAssigneeAccountId(option?.id || '')
              }}
            />
          ) : (
            <div className="relative">
              <input
                type="text"
                value={assignee}
                onChange={e => setAssignee(e.target.value)}
                placeholder="Assigné à..."
                className="w-full pl-7 pr-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
              />
              <User size={12} className="absolute left-2.5 top-2.5 text-[var(--text-muted)]" />
            </div>
          )}

        </div>

        {/* Sprint : sélection ou recherche de sprint pour le ticket */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Sprint
          </label>
          <LookupField
            value={sprint}
            icon={<CalendarRange size={12} />}
            placeholder="Chercher ou nommer un sprint…"
            clearLabel="Backlog (aucun sprint)"
            emptyHint="Aucun sprint trouvé. Tapez un nom pour créer."
            onSearch={async (query: string) => {
              const res = await searchSprint(query)
              if (query.trim() && !res.some(o => o.label.toLowerCase() === query.trim().toLowerCase())) {
                res.unshift({ id: query.trim(), label: query.trim(), sublabel: 'Nouveau sprint' })
              }
              return res
            }}
            onPick={option => {
              const val = option?.label || ''
              setSprint(val)
              if (selectedTask && selectedTask.source === 'jira') {
                setTaskSprint(selectedTask.id, option?.id || '', val)
              }
            }}
          />
        </div>

        {/* Équipe : le champ Team du tracker, modifiable. L'écriture part tout de
            suite dans la file, contrairement aux champs texte qui attendent
            l'enregistrement de la fiche : c'est une opération à part côté Jira. */}
        {selectedTask.source === 'jira' && (
          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Équipe
            </label>
              <LookupField
                value={selectedTask.team || ''}
                icon={<Users size={12} />}
                placeholder="Chercher une équipe…"
                clearLabel="Aucune équipe"
                emptyHint="Aucune équipe trouvée pour cette recherche."
                onSearch={searchTeam}
                onPick={option => {
                  setTaskTeam(selectedTask.id, option?.id || '', option?.label)
                }}
              />
          </div>
        )}

        {/* Macro (Milestone) : sélection ou création de macro / milestone GitHub */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Macro (Milestone)
          </label>
          <LookupField
            value={selectedTask.parentKey || selectedTask.parentTitle || ''}
            icon={<Target size={12} />}
            placeholder="Assigner ou nommer une macro…"
            clearLabel="Détacher de la macro"
            emptyHint="Aucune macro trouvée. Tapez un nom pour créer."
            onSearch={async (query: string) => {
              const res = await searchMacro(query)
              if (query.trim() && !res.some(o => o.label.toLowerCase() === query.trim().toLowerCase() || o.id.toLowerCase() === query.trim().toLowerCase())) {
                res.unshift({ id: `__create__:${query.trim()}`, label: query.trim(), sublabel: 'Créer ce milestone GitHub' })
              }
              return res
            }}
            onPick={async (option) => {
              if (!selectedTask) return
              if (!option?.id) {
                await setTaskMacro(selectedTask.id, '')
                return
              }
              if (option.id.startsWith('__create__:')) {
                const title = option.id.replace('__create__:', '')
                const created = await createMacro(selectedTask.projectId || projects[0]?.id || 'default', title)
                if (created) {
                  await setTaskMacro(selectedTask.id, created.key)
                }
              } else {
                await setTaskMacro(selectedTask.id, option.id)
              }
            }}
          />
        </div>

        {/* Due Date */}
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {t.taskModal.dueDate}
          </label>
          <div className="relative">
            <input
              type="date"
              value={dueDate}
              onChange={e => setDueDate(e.target.value)}
              className="w-full pl-7 pr-2 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
            />
            <Calendar size={12} className="absolute left-2.5 top-2.5 text-[var(--text-muted)] pointer-events-none" />
          </div>
        </div>
      </div>

      {/* Pull Requests : l'ensemble ordonné des PR du ticket. Un ticket en produit
          couramment plusieurs — une première fusionnée, puis une suite poussée sur
          la même branche. La dernière est la PR courante. Corriger ou détacher un
          lien ici est la seule issue quand une PR a été enregistrée à tort. */}
      <div className="space-y-1.5">
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
          Pull Requests
        </label>
        {prLinks.length === 0 && (
          <p className="text-xs text-[var(--text-muted)]">Aucune pull request liée à ce ticket.</p>
        )}
        {prLinks.map((link, index) => (
          <div key={index} className="flex items-center gap-1.5">
            <a
              href={link.url}
              target="_blank"
              rel="noreferrer"
              className="shrink-0 p-1.5 rounded-lg text-purple-400 hover:bg-purple-500/10 transition-colors"
              title={t.skills.viewPr}
            >
              <GitPullRequest size={13} />
            </a>
            <input
              type="url"
              value={link.url}
              onChange={e => setPrLinks(prLinks.map((l, i) => (i === index ? { ...l, url: e.target.value } : l)))}
              className="flex-1 min-w-0 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
            />
            <span className="shrink-0 text-[10px] text-[var(--text-muted)] max-w-[10rem] truncate" title={link.branch || ''}>
              {link.branch || '—'}
            </span>
            {index === prLinks.length - 1 && (
              <span className="shrink-0 text-[10px] font-semibold text-purple-400">courante</span>
            )}
            <button
              type="button"
              onClick={() => setPrLinks(prLinks.filter((_, i) => i !== index))}
              className="shrink-0 p-1.5 rounded-lg text-[var(--text-muted)] hover:text-red-400 hover:bg-red-500/10 transition-colors cursor-pointer"
              title="Détacher cette pull request du ticket"
            >
              <Trash2 size={13} />
            </button>
          </div>
        ))}
        <div className="flex items-center gap-1.5">
          <input
            type="url"
            value={newPrUrl}
            onChange={e => setNewPrUrl(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                addPrLink()
              }
            }}
            placeholder="https://github.com/owner/repo/pull/42"
            className="flex-1 min-w-0 px-2.5 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
          />
          <button
            type="button"
            onClick={addPrLink}
            disabled={!newPrUrl.trim() || prLinks.some(l => l.url === newPrUrl.trim())}
            className="shrink-0 flex items-center gap-1 px-2.5 py-1.5 rounded-xl text-xs font-semibold text-purple-400 bg-purple-500/10 hover:bg-purple-500/20 border border-purple-500/30 transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            title="Lier cette pull request au ticket"
          >
            <Plus size={13} />
            <span>Lier</span>
          </button>
        </div>
      </div>

      {/* Description / Acceptance criteria */}
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
            Description & Contexte Technique
          </label>
          <div className="flex items-center gap-3">
            <label className="inline-flex items-center gap-1.5 text-xs text-[var(--text-secondary)] cursor-pointer select-none">
              <input
                type="checkbox"
                checked={withComments}
                onChange={e => setWithComments(e.target.checked)}
                className="rounded border-[var(--border-color)] text-[var(--accent-color)] focus:ring-0 cursor-pointer"
              />
              <span>Inclure les commentaires</span>
            </label>
            <button
              type="button"
              onClick={() => runSkill(selectedTask.id, 'rewrite_story', '', { withComments })}
              disabled={isSkillRunning}
              className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 hover:bg-cyan-500/20 disabled:opacity-50 transition-colors"
              title="Reformuler la description en User Story structurée GFM"
            >
              {isSkillRunning && runningSkillId === 'rewrite_story' ? (
                <Loader2 size={13} className="animate-spin" />
              ) : (
                <Sparkles size={13} />
              )}
              <span>Reformuler la story</span>
            </button>
          </div>
        </div>

        {rewriteActivity?.output && rewriteActivity.id !== dismissedRewriteId && (
          <div className="p-3.5 rounded-xl bg-cyan-950/20 border border-cyan-500/30 space-y-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-semibold text-cyan-300 flex items-center gap-1.5">
                <Sparkles size={14} />
                Aperçu de la story reformulée
              </span>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={async () => {
                    if (rewriteActivity.output) {
                      setDescription(rewriteActivity.output)
                      await updateTask(selectedTask.id, { description: rewriteActivity.output })
                      setDismissedRewriteId(rewriteActivity.id)
                      addToast({
                        type: 'success',
                        title: 'Description mise à jour',
                        description: 'La description de la tâche a été remplacée par la version reformulée.',
                      })
                    }
                  }}
                  className="px-2.5 py-1 rounded-lg text-xs font-semibold bg-cyan-500 text-slate-950 hover:bg-cyan-400 transition-colors flex items-center gap-1"
                >
                  <Check size={13} />
                  Appliquer à la description
                </button>
                <button
                  type="button"
                  onClick={() => setDismissedRewriteId(rewriteActivity.id)}
                  className="px-2 py-1 rounded-lg text-xs font-medium text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                >
                  Masquer
                </button>
              </div>
            </div>
            <div className="p-3 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] max-h-60 overflow-y-auto">
              <MarkdownEditor value={rewriteActivity.output} onChange={() => {}} minHeight={120} />
            </div>
          </div>
        )}

        <MarkdownEditor
          value={description}
          onChange={setDescription}
          placeholder={t.taskModal.descPlaceholder}
          minHeight={320}
          maxHeight={window.innerHeight * 0.6}
        />
      </div>

      {/* Labels */}
      <div>
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
          {t.taskModal.labels}
        </label>
        <div className="flex flex-wrap items-center gap-1.5 p-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
          {labels.map(lbl => {
            const lower = lbl.toLowerCase()
            let badgeStyle = 'bg-[var(--accent-light)] accent-text'
            if (lower === 'new' || lower === 'untouched') badgeStyle = 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 font-bold'
            else if (lower === 'clarified') badgeStyle = 'bg-amber-500/20 text-amber-300 border border-amber-500/40 font-bold'
            else if (lower === 'specified') badgeStyle = 'bg-blue-500/20 text-blue-300 border border-blue-500/40 font-bold'
            else if (lower === 'implemented') badgeStyle = 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/40 font-bold'
            else if (lower === 'reviewed') badgeStyle = 'bg-purple-500/20 text-purple-300 border border-purple-500/40 font-bold'
            else if (lower === 'finished' || lower === 'closed') badgeStyle = 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 font-bold'

            return (
              <span
                key={lbl}
                className={`inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-md ${badgeStyle}`}
              >
                #{lbl.replace(/^#+/, '')}
                <button
                  type="button"
                  onClick={() => handleRemoveLabel(lbl)}
                  className="hover:opacity-75 ml-0.5"
                >
                  <X size={11} />
                </button>
              </span>
            )
          })}
          <div className="flex items-center gap-1 min-w-[120px] flex-1">
            <input
              type="text"
              value={newLabelInput}
              onChange={e => setNewLabelInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' || e.key === ',') {
                  e.preventDefault()
                  handleAddLabel()
                }
              }}
              placeholder={t.taskModal.addLabel}
              className="w-full text-xs bg-transparent border-none text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none px-1"
            />
          </div>
        </div>
      </div>
    </div>
  )

  // SHARED: Skills & Agent Copilot Section Content
  const renderSkillsCopilotSection = () => (
    <div className="space-y-4 pt-4 border-t border-[var(--border-color)]">
      {/* Copilot Header / Engine Banner */}
      <div className="flex items-center justify-between bg-[var(--bg-tertiary)]/50 p-3 rounded-xl border border-[var(--border-color)]">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg accent-bg text-white flex items-center justify-center font-bold">
            <Bot size={15} />
          </div>
          <div>
            <div className="text-xs font-bold text-[var(--text-primary)] flex items-center gap-1.5">
              <span>Agent Copilot ({activeProvider.toUpperCase()})</span>
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
            </div>
            <div className="text-[10px] text-[var(--text-muted)] font-mono truncate max-w-[280px]">
              {effectiveRepoPath || 'Workspace standard'}
            </div>
          </div>
        </div>

      </div>

      {/* Les cinq pas du workflow, dans l'ordre : clarify, specify, implement, PR, handoff */}
      <div className="space-y-2">
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
          Pipeline d'Avancement des Skills
        </label>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2">
          {skills.map((s, index) => {
              const isRecommended = nextSkill?.id === s.id
              const isCurrentRunning = isSkillRunning && runningSkillId === s.id

              // La carte n'est plus un bouton : elle en contient trois. Un bouton
              // imbriqué dans un bouton n'est pas du HTML valide, et le choix du
              // mode doit être atteignable au clavier comme le lancement.
              return (
                <div
                  key={s.id}
                  className={`p-2.5 rounded-xl border text-left flex flex-col justify-between transition-all group relative overflow-hidden ${
                    isRecommended
                      ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                      : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--accent-color)]/60'
                  }`}
                >
                  <button
                    type="button"
                    onClick={() => handleTriggerSkill(s.id)}
                    disabled={isSkillRunning}
                    aria-label={`Lancer ${s.name} dans le mode configuré`}
                    className="text-left w-full disabled:opacity-60 cursor-pointer"
                  >
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-[10px] font-mono font-bold opacity-60">
                        0{index + 1}
                      </span>
                      {isCurrentRunning ? (
                        <Loader2 size={13} className="animate-spin text-[var(--accent-color)]" />
                      ) : (
                        getSkillIcon(s.icon)
                      )}
                    </div>
                    <div>
                      <div className="font-bold text-xs leading-tight text-[var(--text-primary)] group-hover:text-[var(--accent-color)]">
                        {s.name}
                      </div>
                      <div className="text-[9px] text-[var(--text-muted)] font-mono mt-0.5">
                        {s.command}
                      </div>
                    </div>
                  </button>
                  <div className="mt-2 pt-1.5 flex items-center gap-1 border-t border-[var(--border-color)]/60">
                    <button
                      type="button"
                      onClick={() => handleTriggerSkill(s.id, undefined, 'interactive')}
                      disabled={isSkillRunning}
                      aria-label={`Lancer ${s.name} en interactif`}
                      title="Ouvre un terminal que tu réponds, et tu confirmes la transition"
                      className="flex-1 flex items-center justify-center gap-1 px-1 py-1 rounded-lg text-[9px] font-bold text-[var(--text-secondary)] bg-[var(--bg-secondary)] border border-[var(--border-color)] hover:text-[var(--accent-color)] hover:border-[var(--accent-color)]/60 disabled:opacity-40 cursor-pointer"
                    >
                      <Terminal size={9} />
                      <span>Interactif</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleTriggerSkill(s.id, undefined, 'autonomous')}
                      disabled={isSkillRunning}
                      aria-label={`Lancer ${s.name} en autonome`}
                      title="Lance la CLI en headless, sans terminal ; le worker pose la transition"
                      className="flex-1 flex items-center justify-center gap-1 px-1 py-1 rounded-lg text-[9px] font-bold text-[var(--text-secondary)] bg-[var(--bg-secondary)] border border-[var(--border-color)] hover:text-[var(--accent-color)] hover:border-[var(--accent-color)]/60 disabled:opacity-40 cursor-pointer"
                    >
                      <Bot size={9} />
                      <span>Autonome</span>
                    </button>
                  </div>
                </div>
              )
            })}
        </div>
      </div>

      {/* Discuter : session interactive avec l'agent, hors pipeline de skills. */}
      {selectedTask && resolveTaskStage(selectedTask, taskProject) !== 'finished' && (
        <button
          type="button"
          onClick={() => runSkill(selectedTask.id, 'discuss')}
          disabled={isSkillRunning}
          title="Ouvrir une session avec l'agent sur cette tâche, sans lancer de skill"
          className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-bold border border-[var(--border-color)] bg-[var(--bg-tertiary)] text-[var(--text-primary)] hover:border-[var(--accent-color)]/60 transition-all disabled:opacity-50"
        >
          {isSkillRunning && runningSkillId === 'discuss' ? <Loader2 size={13} className="animate-spin" /> : <MessageCircle size={13} className="text-cyan-400" />}
          <span>Discuter de la tâche</span>
        </button>
      )}

      {/* Main Recommended Action Callout */}
      {selectedTask && !selectedTask.prUrl && resolveTaskStage(selectedTask, taskProject) === 'implemented' && <button type="button" onClick={() => handleTriggerSkill(prRecoverySkill(taskProject), 'PR recovery: preserve accepted work and attained stage; complete owner checks and create/reuse/link the PR. Do not advance to reviewed.')} className="px-4 py-2 text-purple-400 text-sm">Complete PR setup through {prRecoverySkill(taskProject)}</button>}
      {nextSkill && (
        <div className="flex flex-wrap items-center justify-between gap-3 p-3 rounded-xl bg-linear-to-r from-[var(--accent-light)] to-[var(--bg-tertiary)] border border-[var(--accent-color)]/40 shadow-xs">
          <div className="flex items-center gap-2">
            <span className="text-amber-400 animate-bounce">⚡</span>
            <div>
              <div className="text-xs font-bold text-[var(--text-primary)]">
                Étape recommandée : {nextSkill.name}
              </div>
              <div className="text-[10px] text-[var(--text-muted)]">
                {nextSkill.description}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {(status === 'to_clarify' || status === 'backlog' || status === 'clarified' || status === 'specified') && (
              <button
                type="button"
                onClick={() => handleTriggerSkill('implement')}
                disabled={isSkillRunning}
                className="flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-bold text-white bg-linear-to-r from-blue-600 to-indigo-600 shadow hover:opacity-90 active:scale-95 transition-all disabled:opacity-50"
                title="Sauter directement le cadrage et lancer l'implémentation du code"
              >
                <Flame size={13} className="text-amber-300" />
                <span>🚀 Passer direct au Code</span>
              </button>
            )}

            <button
              onClick={() => handleTriggerSkill(nextSkill.id)}
              disabled={isSkillRunning}
              className="flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-bold text-white accent-bg shadow-md hover:opacity-90 active:scale-95 transition-all disabled:opacity-50"
            >
              {isSkillRunning && runningSkillId === nextSkill.id ? (
                <>
                  <Loader2 size={13} className="animate-spin" />
                  <span>Exécution {activeProvider}...</span>
                </>
              ) : (
                <>
                  <Sparkles size={13} className="text-amber-300" />
                  <span>Lancer {nextSkill.name}</span>
                </>
              )}
            </button>
          </div>
        </div>
      )}

      {/* Optional Prompt Refinement */}
      <div>
        <div className="flex items-center justify-between mb-1 gap-2 flex-wrap">
          <label className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
            Instruction / Contexte additionnel pour l'IA (Optionnel)
          </label>
          <label className="flex items-center gap-1 text-[10px] text-[var(--text-secondary)]">
            <span>Mode</span>
            <select
              value={launchMode}
              onChange={e => setLaunchMode(e.target.value as SkillMode)}
              className="px-1.5 py-1 rounded-lg text-[10px] bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
              title="Mode d'exécution pour ce lancement seulement. Aucun réglage enregistré n'est modifié."
            >
              <option value="">Mode configuré</option>
              <option value="interactive">Interactif</option>
              <option value="autonomous">Autonome</option>
            </select>
          </label>
          {launchModels.length > 0 && (
            <label className="flex items-center gap-1 text-[10px] text-[var(--text-secondary)]">
              <span>Modèle</span>
              <select
                value={effectiveLaunchModel}
                onChange={e => setLaunchModel(e.target.value)}
                className="px-1.5 py-1 rounded-lg text-[10px] bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] focus:outline-none focus:border-[var(--accent-color)] cursor-pointer"
                title="Modèle pour ce lancement seulement. Aucun réglage enregistré n'est modifié ; une surcharge poste de travail peut encore s'appliquer."
              >
                <option value="">
                  {configuredLaunchModel ? `Modèle configuré (${configuredLaunchModel})` : 'Modèle configuré'}
                </option>
                {launchModels
                  .filter(model => model !== configuredLaunchModel)
                  .map(model => (
                    <option key={model} value={model}>
                      {model}
                    </option>
                  ))}
              </select>
            </label>
          )}
        </div>
        {launchModels.length > 0 && launchModelNotice && (
          <p className="mb-1 text-[10px] text-[var(--text-muted)] leading-relaxed">{launchModelNotice}</p>
        )}
        <input
          type="text"
          value={customPrompt}
          onChange={e => setCustomPrompt(e.target.value)}
          placeholder="Ex: Utilise Tailwind v4, ajoute des tests Go avec testify..."
          className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-mono text-[11px]"
        />
      </div>

      {/* Latest Output / Console Display */}
      {latestActivity && (
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-[11px]">
            <span className="font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1 text-[10px]">
              <Terminal size={12} className="text-[var(--accent-color)]" />
              Dernière sortie ({latestActivity.skillName})
            </span>
            <span className="text-[10px] text-[var(--text-muted)]">
              {new Date(latestActivity.createdAt).toLocaleTimeString()}
            </span>
          </div>
          <div className="p-3.5 rounded-xl bg-slate-950 text-slate-200 border border-slate-800 font-mono text-xs space-y-2 max-h-56 overflow-y-auto leading-relaxed shadow-inner">
            <div className={`flex items-center gap-2 font-bold text-[11px] pb-1 border-b border-slate-800 ${
              latestActivity.status === 'running'
                ? 'text-indigo-400'
                : latestActivity.status === 'queued' || latestActivity.status === 'pending'
                ? 'text-amber-400'
                : latestActivity.status === 'failed'
                ? 'text-rose-400'
                : 'text-emerald-400'
            }`}>
              {latestActivity.status === 'running' ? (
                <Loader2 size={13} className="animate-spin text-indigo-400" />
              ) : latestActivity.status === 'queued' || latestActivity.status === 'pending' ? (
                <Clock size={13} className="text-amber-400" />
              ) : latestActivity.status === 'failed' ? (
                <AlertCircle size={13} className="text-rose-400" />
              ) : (
                <CheckCircle2 size={13} className="text-emerald-400" />
              )}
              <span>
                {latestActivity.status === 'running'
                  ? `[En cours] ${latestActivity.summary || 'Exécution de la skill...'}`
                  : latestActivity.status === 'queued' || latestActivity.status === 'pending'
                  ? `[En file d'attente] ${latestActivity.summary || 'En attente d\'un worker...'}`
                  : latestActivity.summary}
              </span>
            </div>
            {latestActivity.output && (
              <pre className="whitespace-pre-wrap text-[11px] text-slate-300 font-mono">
                {latestActivity.output}
              </pre>
            )}
          </div>
        </div>
      )}

      {/* Past Activity Accordion if multiple runs */}
      {activities.length > 1 && (
        <div className="space-y-1.5 pt-2">
          <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1">
            <History size={12} />
            Historique des exécutions ({activities.length})
          </span>
          <div className="space-y-1.5">
            {activities.slice(1, 4).map(act => (
              <div
                key={act.id}
                className="p-2 rounded-lg bg-[var(--bg-tertiary)]/60 border border-[var(--border-color)] text-xs flex items-center justify-between"
              >
                <div className="flex items-center gap-2 truncate">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                  <span className="font-bold text-[11px] text-[var(--text-primary)]">{act.skillName}</span>
                  {runEngineLabel(act) && (
                    <span className="text-[10px] font-mono text-[var(--text-muted)]">{runEngineLabel(act)}</span>
                  )}
                  <span className="text-[10px] text-[var(--text-muted)] truncate max-w-[200px]">{act.summary}</span>
                </div>
                <span className="text-[10px] text-[var(--text-muted)] font-mono">
                  {new Date(act.createdAt).toLocaleDateString(undefined, { hour: '2-digit', minute: '2-digit' })}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )

  // -------------------------------------------------------------
  // MODE 1: RIGHT SLIDING PANEL (DRAWER)
  // Layout: Story infos on top, Skills & Copilot underneath!
  // -------------------------------------------------------------
  if (detailMode === 'panel') {
    return (
      <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 overflow-hidden select-none">
        {/* Backdrop overlay */}
        <div
          {...backdrop}
          className="absolute inset-0 bg-black/40 backdrop-blur-2xs animate-in fade-in duration-200"
        />

        {/* Sliding Panel */}
        <div className="absolute inset-y-0 right-0 max-w-full flex pl-2 sm:pl-6">
          <div className="w-[var(--app-w)] max-w-5xl 2xl:max-w-[1500px] bg-[var(--bg-secondary)] border-l border-[var(--border-color)] shadow-2xl flex flex-col h-full animate-in slide-in-from-right duration-200">
            {/* Panel Header */}
            <div className="flex items-center justify-between px-6 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
              {/* Left: Référence ParentKey / TaskKey (chacune ouvre le tracker) */}
              <div className="flex items-center gap-2.5 min-w-0">
                {renderTaskRef()}
                {renderIssueTypeSelector()}
              </div>

              {/* Right: Quick switcher to Modal, PR Link, Delete, Close */}
              <div className="flex items-center gap-1.5 shrink-0">
                {/* Integrated TTY Terminal Button */}


                {/* External TTY Terminal Button */}


                {/* Open in Editor Button */}


                {/* Git Diff Button */}


                {/* Switch to Modal Button */}
                <button
                  type="button"
                  onClick={handleToggleDetailMode}
                  className="flex items-center gap-1 px-2 py-1 rounded-lg text-[11px] font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] transition-colors"
                  title="Afficher en modale centrée"
                >
                  <Square size={13} className="text-purple-400" />
                  <span className="hidden sm:inline">Modale</span>
                </button>

                {/* Two-way Unit Sync Button */}
                <button
                  type="button"
                  disabled={isSyncingTask}
                  onClick={async () => {
                    if (!selectedTask) return
                    setIsSyncingTask(true)
                    try {
                      await syncSingleTask(selectedTask.id)
                    } finally {
                      setIsSyncingTask(false)
                    }
                  }}
                  className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold text-indigo-300 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 transition-all cursor-pointer shadow-xs active:scale-95 disabled:opacity-50"
                  title="Synchroniser ce ticket dans les deux sens avec le tracker distant (GitHub / Jira)"
                >
                  <RefreshCw size={12} className={`text-indigo-400 ${isSyncingTask ? 'animate-spin' : ''}`} />
                  <span className="hidden sm:inline">{isSyncingTask ? 'Sync...' : 'Sync'}</span>
                </button>

                {selectedTask.prUrl && (
                  <a
                    href={selectedTask.prUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold text-purple-400 bg-purple-500/10 hover:bg-purple-500/20 border border-purple-500/30 transition-colors"
                    title={t.skills.viewPr}
                  >
                    <ExternalLink size={12} />
                    <span>PR</span>
                  </a>
                )}

                <button
                  onClick={() => togglePin(selectedTask.id)}
                  className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                    isPinned(selectedTask.id)
                      ? 'text-amber-300 bg-amber-400/10 hover:bg-amber-400/20'
                      : 'text-[var(--text-muted)] hover:text-amber-300 hover:bg-[var(--bg-tertiary)]'
                  }`}
                  title={isPinned(selectedTask.id) ? 'Désépingler ce ticket' : 'Épingler ce ticket'}
                >
                  {isPinned(selectedTask.id) ? <PinOff size={15} /> : <Pin size={15} />}
                </button>

                <button
                  type="button"
                  onClick={() => openCloneModal(selectedTask)}
                  className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-cyan-400 hover:bg-cyan-500/10 transition-colors cursor-pointer"
                  title="Cloner cette story"
                >
                  <CopyPlus size={15} />
                </button>

                <button
                  onClick={handleDelete}
                  className="p-1.5 rounded-lg text-rose-400 hover:bg-rose-500/10 transition-colors"
                  title={t.taskModal.delete}
                >
                  <Trash2 size={15} />
                </button>

                <button
                  onClick={handleClose}
                  className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors"
                >
                  <X size={17} />
                </button>
              </div>
            </div>

            {/* Panel Tab Navigation Header */}
            <div className="flex items-center gap-2 px-6 pt-2.5 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0 text-xs font-semibold">
              <button
                type="button"
                onClick={() => setActiveTab('details')}
                className={`pb-2 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                  activeTab === 'details'
                    ? 'border-[var(--accent-color)] accent-text font-bold'
                    : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                <FileCode size={13} />
                <span>Story</span>
              </button>

              <button
                type="button"
                onClick={() => setActiveTab('comments')}
                className={`pb-2 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                  activeTab === 'comments'
                    ? 'border-cyan-400 text-cyan-400 font-bold'
                    : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                <MessageSquare size={13} className="text-cyan-400" />
                <span>Commentaires</span>
              </button>

              <button
                type="button"
                onClick={() => setActiveTab('skills')}
                className={`pb-2 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                  activeTab === 'skills'
                    ? 'border-[var(--accent-color)] accent-text font-bold'
                    : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                <Sparkles size={13} className="text-purple-400" />
                <span>Skills & Copilot</span>
              </button>


              <button
                type="button"
                onClick={() => setActiveTab('cadrage')}
                className={`pb-2 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                  activeTab === 'cadrage'
                    ? 'border-amber-400 text-amber-400 font-bold'
                    : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                <HelpCircle size={13} className="text-amber-400" />
                <span>Cadrage & Specs</span>
                {Boolean(clarifyActivity || specifyActivity) && (
                  <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />
                )}
              </button>
            </div>

            {/* Panel Scrollable Content Body */}
            <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
              {activeTab === 'details' && renderStoryInfoSection()}
              {activeTab === 'comments' && <TaskComments task={selectedTask} />}
              {activeTab === 'skills' && renderSkillsCopilotSection()}

              {activeTab === 'cadrage' && renderCadrageSection()}
            </div>

            {/* Panel Sticky Footer */}
            <div className="flex items-center justify-between px-6 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
              <span className="text-[11px] text-[var(--text-muted)]">
                {t.taskModal.created} {new Date(selectedTask.createdAt).toLocaleDateString()}
              </span>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleClose}
                  className="px-3.5 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors"
                >
                  {t.taskModal.cancel}
                </button>
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={isSaving}
                  className="px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow hover:opacity-90 active:scale-95 flex items-center gap-1.5 transition-all"
                >
                  <Save size={13} />
                  <span>{isSaving ? 'Enregistrement...' : t.taskModal.save}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  // -------------------------------------------------------------
  // MODE 2: CENTERED MODAL DIALOG
  // With tab navigation & switcher back to right panel
  // -------------------------------------------------------------
  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-2 sm:p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200 select-none" {...backdrop}>
      <div
        className={`relative w-full transition-all duration-200 bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col ${
          isMaximized
            ? 'w-full h-full max-w-none max-h-none rounded-2xl'
            : 'max-w-[calc(var(--app-w)*0.95)] xl:max-w-[1400px] 2xl:max-w-[1700px] h-[calc(var(--app-h)*0.94)] max-h-[calc(var(--app-h)*0.94)] rounded-2xl'
        }`}
      >
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <div className="flex items-center gap-3">
            {renderTaskRef()}
            {renderIssueTypeSelector()}
          </div>

          <div className="flex items-center gap-2">
            {/* Integrated TTY Terminal Button */}


            {/* External TTY Terminal Button */}


            {/* Open in Editor Button */}


            {/* Git Diff Button */}


            {/* Switch to Right Panel Button */}
            <button
              type="button"
              onClick={handleToggleDetailMode}
              className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] transition-colors"
              title="Afficher en panneau latéral droit"
            >
              <PanelRight size={14} className="text-indigo-400" />
              <span className="hidden sm:inline">Panneau droit</span>
            </button>

            {/* Two-way Unit Sync Button */}
            <button
              type="button"
              disabled={isSyncingTask}
              onClick={async () => {
                if (!selectedTask) return
                setIsSyncingTask(true)
                try {
                  await syncSingleTask(selectedTask.id)
                } finally {
                  setIsSyncingTask(false)
                }
              }}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-xs font-semibold text-indigo-300 bg-indigo-500/10 hover:bg-indigo-500/20 border border-indigo-500/30 transition-all cursor-pointer shadow-xs active:scale-95 disabled:opacity-50"
              title="Synchroniser ce ticket dans les deux sens avec le tracker distant (GitHub / Jira)"
            >
              <RefreshCw size={13} className={`text-indigo-400 ${isSyncingTask ? 'animate-spin' : ''}`} />
              <span className="hidden sm:inline">{isSyncingTask ? 'Sync...' : 'Sync'}</span>
            </button>

            {selectedTask.prUrl && (
              <a
                href={selectedTask.prUrl}
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-xs font-semibold text-purple-400 bg-purple-500/10 hover:bg-purple-500/20 border border-purple-500/30 transition-colors"
                title={t.skills.viewPr}
              >
                <ExternalLink size={13} />
                <span>PR</span>
              </a>
            )}

            <button
              onClick={() => togglePin(selectedTask.id)}
              className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                isPinned(selectedTask.id)
                  ? 'text-amber-300 bg-amber-400/10 hover:bg-amber-400/20'
                  : 'text-[var(--text-muted)] hover:text-amber-300 hover:bg-[var(--bg-tertiary)]'
              }`}
              title={isPinned(selectedTask.id) ? 'Désépingler ce ticket' : 'Épingler ce ticket'}
            >
              {isPinned(selectedTask.id) ? <PinOff size={16} /> : <Pin size={16} />}
            </button>

            <button
              type="button"
              onClick={() => openCloneModal(selectedTask)}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-cyan-400 hover:bg-cyan-500/10 transition-colors cursor-pointer"
              title="Cloner cette story"
            >
              <CopyPlus size={16} />
            </button>

            <button
              onClick={handleDelete}
              className="p-1.5 rounded-lg text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
              title={t.taskModal.delete}
            >
              <Trash2 size={16} />
            </button>

            {/* Maximize / Minimize toggle */}
            <button
              type="button"
              onClick={() => setIsMaximized(prev => !prev)}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              title={isMaximized ? "Réduire" : "Plein écran"}
            >
              {isMaximized ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
            </button>

            <button
              onClick={handleClose}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            >
              <X size={18} />
            </button>
          </div>
        </div>

        {/* Tab Navigation Header */}
        <div className="flex items-center justify-between px-6 pt-3 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0">
          <div className="flex items-center gap-5 text-xs font-semibold">
            <button
              type="button"
              onClick={() => setActiveTab('details')}
              className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                activeTab === 'details'
                  ? 'border-[var(--accent-color)] accent-text font-bold'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              <FileCode size={14} />
              <span>Infos de la Story</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab('comments')}
              className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                activeTab === 'comments'
                  ? 'border-cyan-400 text-cyan-400 font-bold'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              <MessageSquare size={14} className="text-cyan-400" />
              <span>Commentaires</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab('skills')}
              className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                activeTab === 'skills'
                  ? 'border-[var(--accent-color)] accent-text font-bold'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              <Sparkles size={14} className="text-purple-400" />
              <span>Skills & Agent Copilot</span>
            </button>


            <button
              type="button"
              onClick={() => setActiveTab('cadrage')}
              className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
                activeTab === 'cadrage'
                  ? 'border-amber-400 text-amber-400 font-bold'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              <HelpCircle size={14} className="text-amber-400" />
              <span>Cadrage & Spécifications</span>
              {Boolean(clarifyActivity || specifyActivity) && (
                <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse ml-0.5" />
              )}
            </button>
          </div>
        </div>

        {/* Modal Scrollable Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
          {activeTab === 'details' && renderStoryInfoSection()}
          {activeTab === 'comments' && <TaskComments task={selectedTask} />}
          {activeTab === 'skills' && renderSkillsCopilotSection()}

          {activeTab === 'cadrage' && renderCadrageSection()}
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <span className="text-[11px] text-[var(--text-muted)]">
            {t.taskModal.created} {new Date(selectedTask.createdAt).toLocaleDateString()}
          </span>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleClose}
              className="px-4 py-2 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors"
            >
              {t.taskModal.cancel}
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={isSaving}
              className="px-5 py-2 rounded-xl text-xs font-bold text-white accent-bg shadow-md hover:opacity-90 active:scale-95 flex items-center gap-1.5 transition-all"
            >
              <Save size={14} />
              <span>{isSaving ? 'Enregistrement...' : t.taskModal.save}</span>
            </button>
          </div>
        </div>
      </div>


      {/* Fullscreen Expanded Specification Reader Modal */}
      {isExpandedSpec && specifyActivity?.output && (
        <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-60 flex items-center justify-center p-3 sm:p-6 bg-black/80 backdrop-blur-xs animate-in fade-in duration-150" {...expandedSpecBackdrop}>
          <div className="relative w-full max-w-5xl h-[calc(var(--app-h)*0.88)] rounded-2xl bg-[var(--bg-secondary)] border border-blue-500/40 shadow-2xl flex flex-col overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/70 shrink-0">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-xl bg-blue-500/20 text-blue-400 border border-blue-500/30 flex items-center justify-center font-bold">
                  <FileCode size={16} />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs font-bold text-blue-400 bg-blue-500/10 px-2 py-0.5 rounded border border-blue-500/20">
                      {selectedTask.key}
                    </span>
                    <h3 className="text-sm font-bold text-[var(--text-primary)]">
                      Spécification Technique (Vue Détaillée)
                    </h3>
                  </div>
                  <p className="text-[11px] text-[var(--text-muted)] mt-0.5">
                    {selectedTask.title}
                  </p>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleCopySpec}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-primary)] hover:bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] transition-colors"
                >
                  {copiedSpec ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                  <span>{copiedSpec ? 'Copié !' : 'Copier spec'}</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsExpandedSpec(false)}
                  className="p-2 rounded-xl hover:bg-[var(--bg-tertiary)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                  title="Réduire (ESC)"
                >
                  <Minimize2 size={16} />
                </button>
                <button
                  type="button"
                  onClick={() => setIsExpandedSpec(false)}
                  className="p-2 rounded-xl hover:bg-[var(--bg-tertiary)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                  title="Fermer"
                >
                  <X size={18} />
                </button>
              </div>
            </div>

            {/* Spec Body */}
            <div className="flex-1 p-6 overflow-y-auto bg-slate-950 text-slate-200 font-mono text-xs leading-relaxed whitespace-pre-wrap select-text">
              {specifyActivity.output}
            </div>

            {/* Footer */}
            <div className="flex items-center justify-between px-6 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/50 shrink-0">
              <span className="text-xs text-[var(--text-muted)] font-mono">
                {specifyActivity.completedAt ? `Généré le ${new Date(specifyActivity.completedAt).toLocaleString()}` : ''}
              </span>

              <button
                type="button"
                onClick={() => {
                  setIsExpandedSpec(false)
                  handleTriggerSkill('implement')
                }}
                disabled={isSkillRunning}
                className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold text-white shadow-md bg-linear-to-r from-blue-600 to-indigo-600 hover:opacity-90 active:scale-95 transition-all disabled:opacity-50"
              >
                {isSkillRunning && runningSkillId === 'implement' ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <>
                    <Flame size={14} className="text-amber-300" />
                    <span>Lancer Implement Code</span>
                    <ArrowRight size={13} />
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
