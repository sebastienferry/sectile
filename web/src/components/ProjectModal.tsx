import React, { useState, useEffect, useCallback } from 'react'
import {
  X,
  Folder,
  Terminal,
  Zap,
  Flame,
  Layers,
  Box,
  Code2,
  Cpu,
  Sparkles,
  Workflow,
  Save,
  Trash2,
  Check,
  CheckCircle2,
  FileCode,
  ShieldCheck,
  HelpCircle,
  GitBranch,
  Sliders,
  Globe,
  Key,
  Info,
  Inbox,
  Map,
  Clock,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { format } from '../lib/i18n'
import type { ProjectSettingsStrings } from '../locales/projectSettings'
import { OPTIONAL_VIEWS, enabledOptionalViews } from '../lib/optionalViews'
import { BoardColumnsEditor } from './BoardColumnsEditor'
import type {
  AccentColor,
  IssueTracker,
  ProjectSkillsStatus,
  DetectedStatus,
  TrackerColumn,
  SpecFramework,
  SpecFrameworkStatus,
  SpecFrameworkInstallResult,
  SkillMode,
  OptionalViewMode,
} from '../types'
import { ACCENT_COLORS, accentBadgeStyle, normalizeAccentColor, DEFAULT_PROJECT_ACCENT } from '../lib/accents'
import { PROJECT_TRACKERS, needsCredentialsFor } from '../lib/trackers'
import { formatProjectKeyList, parseProjectKeyList } from '../lib/roadmapProjects'
import { declaredRepositories, droppedRepositoryPaths, duplicateRepository, repositoryIdentity } from '../lib/repositories'

type ProjectTab = 'general' | 'tracker' | 'workflow' | 'skills'

/**
 * Optional views offered in the project settings. The view names are product
 * names and stay the same in every language; the hint, which says what the
 * view shows, comes from the catalog (`projectSettings.views.hints`).
 */
const OPTIONAL_VIEW_CARDS: {
  id: OptionalViewMode
  label: string
  icon: React.FC<{ size?: number; className?: string }>
  iconColor: string
}[] = [
  { id: 'triage', label: 'Triage', icon: Inbox, iconColor: 'text-rose-400' },
  { id: 'roadmap', label: 'Roadmap', icon: Map, iconColor: 'text-amber-400' },
  { id: 'timeline', label: 'Timeline', icon: Clock, iconColor: 'text-blue-400' },
]

/** The tabs of the modal; their labels come from `projectSettings.tabs`. */
const TABS: {
  id: ProjectTab
  icon: React.FC<{ size?: number; className?: string }>
  iconColor: string
}[] = [
  { id: 'general', icon: Folder, iconColor: 'text-amber-400' },
  { id: 'tracker', icon: Sliders, iconColor: 'text-emerald-400' },
  { id: 'workflow', icon: Workflow, iconColor: 'text-blue-400' },
  { id: 'skills', icon: Sparkles, iconColor: 'text-cyan-400' },
]

// The tooltip of an icon is its lucide name, the same in every language.
const AVAILABLE_ICONS = [
  { name: 'Folder', Icon: Folder },
  { name: 'Terminal', Icon: Terminal },
  { name: 'Zap', Icon: Zap },
  { name: 'Flame', Icon: Flame },
  { name: 'Layers', Icon: Layers },
  { name: 'Box', Icon: Box },
  { name: 'Code2', Icon: Code2 },
  { name: 'Cpu', Icon: Cpu },
  { name: 'Sparkles', Icon: Sparkles },
  { name: 'Workflow', Icon: Workflow },
]

// Skill names and command identifiers are never translated; the description
// comes from `projectSettings.skills.descriptions`.
const WORKFLOW_SKILLS: { id: WorkflowSkillId; defaultName: string; code: string; icon: React.ComponentType<{ size?: number; className?: string }>; color: string }[] = [
  { id: 'clarify', defaultName: 'Clarify', code: 'clarify-issue', icon: HelpCircle, color: 'amber' },
  { id: 'specify', defaultName: 'Specify', code: 'specify-issue', icon: FileCode, color: 'blue' },
  { id: 'implement', defaultName: 'Implement', code: 'code-issue', icon: Flame, color: 'indigo' },
  { id: 'adjust', defaultName: 'Adjust', code: 'adjust-issue', icon: ShieldCheck, color: 'purple' },
  { id: 'handoff', defaultName: 'Handoff', code: 'handoff-issue', icon: Sparkles, color: 'emerald' },
]

type WorkflowSkillId = keyof ProjectSettingsStrings['skills']['descriptions']

/**
 * Renders a catalog template whose `{name}` placeholders are elements (a
 * `<code>`, an `<em>`) rather than plain text, keeping the surrounding words in
 * the order the language puts them.
 */
const richText = (template: string, parts: Record<string, React.ReactNode>): React.ReactNode[] =>
  template.split(/(\{\w+\})/).map((chunk, index) => {
    const match = /^\{(\w+)\}$/.exec(chunk)
    return match && match[1] in parts
      ? <React.Fragment key={index}>{parts[match[1]]}</React.Fragment>
      : chunk
  })

const extractGithubRepoFromGitUrl = (url: string): string => {
  const clean = url.trim().replace(/\.git$/, '')
  if (clean.startsWith('git@github.com:')) return clean.replace('git@github.com:', '')
  if (clean.startsWith('https://github.com/')) return clean.replace('https://github.com/', '')
  if (clean.startsWith('http://github.com/')) return clean.replace('http://github.com/', '')
  if (clean.startsWith('ssh://git@github.com/')) return clean.replace('ssh://git@github.com/', '')
  return clean
}

export const ProjectModal: React.FC = () => {
  const {
    isProjectModalOpen,
    setIsProjectModalOpen,
    editingProject,
    setEditingProject,
    createProject,
    updateProject,
    deleteProject,
    fetchProjectIssueTypes,
    setIsTrackerSetupOpen,
    userCredentials,
    refreshUserCredentials,
    settings,
    t,
  } = useApp()
  const ps = t.projectSettings

  const [activeTab, setActiveTab] = useState<ProjectTab>('general')

  // Section 1: Général (Titre, description, icône, couleur, projet par défaut)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [icon, setIcon] = useState('Folder')
  const [color, setColor] = useState<AccentColor>(DEFAULT_PROJECT_ACCENT)
  const [isDefault, setIsDefault] = useState(false)

  // Section 2: Git (remote URL, declared repositories)
  const [prCreationStage, setPRCreationStage] = useState<'specified' | 'implemented'>('implemented')
  const [defaultSkillMode, setDefaultSkillMode] = useState<SkillMode>('')
  const [fullChainStopStage, setFullChainStopStage] = useState<'implemented' | 'reviewed'>('reviewed')
  const [trackerColumns, setTrackerColumns] = useState<TrackerColumn[]>([])
  const [stageColumns, setStageColumns] = useState<Record<string, string[]>>({})

  const [gitRemoteUrl, setGitRemoteUrl] = useState('')
  // Remotes declared besides the code remote, which the server always lists
  // first and derives from gitRemoteUrl.
  const [repositories, setRepositories] = useState<string[]>([])
  const [newRepository, setNewRepository] = useState('')
  const [repositoryError, setRepositoryError] = useState('')

  // Section 3: background synchronisation
  // Whether the tasks commit their specification artefacts (#487), a method
  // setting the server keeps.
  const [dropSpecArtifacts, setDropSpecArtifacts] = useState(false)
  const [autoSyncEnabled, setAutoSyncEnabled] = useState(false)
  const [autoSyncIntervalMin, setAutoSyncIntervalMin] = useState(5)

  // Section 4: Compétences IA & Framework SDD
  const [specFramework, setSpecFramework] = useState<SpecFramework>('speckit')
  const [skillsStatus, setSkillsStatus] = useState<ProjectSkillsStatus | null>(null)




  // Spec-Driven Design toolchain (GitHub Spec Kit / OpenSpec) install state
  const [, setSddStatuses] = useState<SpecFrameworkStatus[]>([])


  const [, setSddResult] = useState<SpecFrameworkInstallResult | null>(null)

  // Section 5: Tracker (Type, URL, Clef, Mapping)
  const [issueTracker, setIssueTracker] = useState<IssueTracker>('local')
  const [trackerUrl, setTrackerUrl] = useState('')

  // L'instance d'un projet Jira part de celle de la personne : il faut donc
  // connaître ses accès avant qu'elle ne choisisse le tracker.
  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])
  const [githubRepo, setGithubRepo] = useState('')
  // The project's own instance URL. Empty, the user configuration applies. A
  // project carries no token: the server credential of its provider serves
  // every project (#464).
  const [githubApiUrl, setGithubApiUrl] = useState('')
  // A GitLab project is named by its path (group/sub/project) on one instance.
  // Both empty mean "those of the user configuration", like githubApiUrl.
  const [gitlabUrl, setGitlabUrl] = useState('')
  const [gitlabProject, setGitlabProject] = useState('')
  const [jiraProject, setJiraProject] = useState('')
  const [roadmapProjects, setRoadmapProjects] = useState('')
  // Types de tickets importés. Vide vaut « les types par défaut » : c'est ce que
  // porte un projet qui n'a jamais eu besoin d'y toucher.
  const [issueTypes, setIssueTypes] = useState<string[]>([])
  // Vues optionnelles affichées par le projet. Vide vaut « aucune », ce que
  // porte un projet qui n'a jamais demandé Triage, Roadmap ou Timeline.
  const [enabledViews, setEnabledViews] = useState<OptionalViewMode[]>([])
  // Couleur par épic sur les cartes. Désactivée tant que le projet ne la demande pas.
  const [epicColors, setEpicColors] = useState(false)
  const [availableIssueTypes, setAvailableIssueTypes] = useState<string[]>([])
  const [isLoadingIssueTypes, setIsLoadingIssueTypes] = useState(false)
  const [detectedStatuses, setDetectedStatuses] = useState<DetectedStatus[]>([])

  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)


  const fetchDetectedStatuses = async (tracker?: IssueTracker, ghRepo?: string) => {
    try {
      const targetTracker = tracker !== undefined ? tracker : issueTracker
      const targetRepo = ghRepo !== undefined ? ghRepo : (githubRepo || extractGithubRepoFromGitUrl(gitRemoteUrl))
      const params = new URLSearchParams()
      if (targetTracker) params.append('tracker', targetTracker)
      if (targetRepo) params.append('repo', targetRepo)
      if (editingProject) params.append('projectId', editingProject.id)

      const res = await fetch(`/api/projects/detected-statuses?${params.toString()}`)
      if (res.ok) {
        const data: { statuses: DetectedStatus[] } = await res.json()
        setDetectedStatuses(data.statuses || [])
      }
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    if (editingProject) {
      setName(editingProject.name)
      setSlug(editingProject.slug)
      setDescription(editingProject.description || '')
      setIcon(editingProject.icon || 'Folder')
      setColor(normalizeAccentColor(editingProject.color) ?? DEFAULT_PROJECT_ACCENT)
      setIsDefault(editingProject.isDefault || false)

      setPRCreationStage(editingProject.prCreationStage || 'implemented')
      setDefaultSkillMode(editingProject.defaultSkillMode || '')
      setFullChainStopStage(editingProject.fullChainStopStage || 'reviewed')
      setTrackerColumns(editingProject.trackerColumns || [])
      setStageColumns(editingProject.stageColumns || {})
      setGitRemoteUrl(editingProject.gitRemoteUrl || '')
      setRepositories(declaredRepositories(editingProject))
      setNewRepository('')
      setRepositoryError('')

      setSpecFramework(editingProject.specFramework || settings.specFramework || 'speckit')
      setDropSpecArtifacts(editingProject.specArtifacts === 'drop')
      setAutoSyncEnabled(Boolean(editingProject.autoSyncEnabled))
      setAutoSyncIntervalMin(editingProject.autoSyncIntervalMin || 5)

      setIssueTracker(editingProject.issueTracker || 'local')
      setTrackerUrl(editingProject.trackerUrl || '')
      setGithubRepo(editingProject.githubRepo || '')
      setGithubApiUrl(editingProject.githubApiUrl || '')
      setGitlabUrl(editingProject.gitlabUrl || '')
      setGitlabProject(editingProject.gitlabProject || '')
      // Le jeton n'est jamais renvoyé : le champ reste vide et le laisser vide
      // conserve celui qui est enregistré.
      setJiraProject(editingProject.jiraProject || '')
      setRoadmapProjects(formatProjectKeyList(editingProject.roadmapProjects))
      setIssueTypes(editingProject.issueTypes || [])
      setEnabledViews(enabledOptionalViews(editingProject))
      setEpicColors(editingProject.epicColors === true)


      fetchDetectedStatuses(editingProject.issueTracker, editingProject.githubRepo)
      // Types réellement exposés par le projet Jira : sans eux, le réglage se
      // ferait à l'aveugle, et c'est justement là que se cache un projet qui ne
      // ramène rien.
      if (editingProject.issueTracker === 'jira') {
        setIsLoadingIssueTypes(true)
        fetchProjectIssueTypes(editingProject.id).then((list: string[]) => {
          setAvailableIssueTypes(list)
          setIsLoadingIssueTypes(false)
        })
      }
    } else {
      setPRCreationStage('implemented')
      setName('')
      setSlug('')
      setDescription('')
      setIcon('Folder')
      setColor('indigo')
      setIsDefault(false)

      setGitRemoteUrl('')
      setRepositories([])
      setNewRepository('')
      setRepositoryError('')

      setSpecFramework(settings.specFramework || 'speckit')
      setDropSpecArtifacts(false)
      setEpicColors(false)
      setAutoSyncEnabled(false)
      setAutoSyncIntervalMin(5)

      setIssueTracker('local')
      setTrackerUrl('')
      setGithubRepo('')
      setGitlabUrl('')
      setGitlabProject('')
      setJiraProject('')
      setRoadmapProjects('')
      setSkillsStatus(null)
      setSddStatuses([])
      setSddResult(null)
      fetchDetectedStatuses('local', '')
    }
    setActiveTab('general')
  }, [editingProject, isProjectModalOpen, settings.specFramework])


  const handleClose = useCallback(() => {
    setIsProjectModalOpen(false)
    setEditingProject(null)
  }, [setIsProjectModalOpen, setEditingProject])
  const backdrop = useBackdropDismiss(handleClose)

  useEffect(() => {
    if (!isProjectModalOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isProjectModalOpen, handleClose])

  if (!isProjectModalOpen) return null

  const handleNameChange = (val: string) => {
    setName(val)
    if (!editingProject) {
      const generatedSlug = val
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/(^-|-$)/g, '')
      setSlug(generatedSlug)
    }
  }


  // The same identity rule as the server: a second spelling of one remote
  // (SSH or HTTPS, case, ".git") is refused before it reaches the save.
  const addRepository = () => {
    const url = newRepository.trim()
    if (!url) return
    if (duplicateRepository(gitRemoteUrl, [...repositories, url])) {
      setRepositoryError(format(ps.repositories.duplicate, { identity: repositoryIdentity(url) }))
      return
    }
    setRepositories(prev => [...prev, url])
    setNewRepository('')
    setRepositoryError('')
  }

  const droppedPaths = droppedRepositoryPaths(editingProject?.repositoriesMigration)

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (!name.trim() || isSubmitting) return

    setIsSubmitting(true)
    try {
      // A remote typed but not added yet is part of what is being saved.
      const pending = newRepository.trim()
      if (pending && duplicateRepository(gitRemoteUrl, [...repositories, pending])) {
        setRepositoryError(format(ps.repositories.duplicate, { identity: repositoryIdentity(pending) }))
        return
      }
      const savedRepositories = pending ? [...repositories, pending] : repositories
      const computedGithubRepo = githubRepo.trim() || extractGithubRepoFromGitUrl(gitRemoteUrl)
      const payload = {
        name: name.trim(),
        slug: slug.trim() || name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
        description: description.trim(),
        icon,
        color,
        isDefault,
        prCreationStage,
        defaultSkillMode,
        fullChainStopStage,
        trackerColumns,
        stageColumns,
        gitRemoteUrl: gitRemoteUrl.trim(),
        repositories: savedRepositories,
        specFramework,
        specArtifacts: dropSpecArtifacts ? 'drop' as const : 'keep' as const,
        autoSyncEnabled,
        autoSyncIntervalMin,
        issueTracker,
        trackerUrl: trackerUrl.trim(),
        githubRepo: computedGithubRepo,
        githubApiUrl: githubApiUrl.trim(),
        gitlabUrl: gitlabUrl.trim(),
        gitlabProject: gitlabProject.trim().replace(/^\/+|\/+$/g, ''),
        jiraProject: jiraProject.trim().toUpperCase(),
        roadmapProjects: issueTracker === 'jira' ? parseProjectKeyList(roadmapProjects, jiraProject) : [],
        issueTypes,
        enabledViews,
        epicColors,
      }

      const saved = editingProject
        ? await updateProject(editingProject.id, payload)
        : await createProject(payload)
      // The error toast carries the server's reason, such as a repository
      // declared twice; the form stays open so the entry can be corrected.
      if (!saved) return
      setIsProjectModalOpen(false)
      setEditingProject(null)

      // Un projet posé sur un tracker distant sans accès configurés ne ramènera
      // rien : autant le dire maintenant, plutôt qu'après une synchronisation
      // vide. L'écran se ferme sans rien remplir, la configuration pouvant venir
      // plus tard depuis les réglages.
      if (needsCredentialsFor(issueTracker, settings, userCredentials)) {
        setIsTrackerSetupOpen(true)
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!editingProject || isDeleting) return
    if (confirm(format(ps.feedback.deleteConfirm, { name: editingProject.name }))) {
      setIsDeleting(true)
      try {
        await deleteProject(editingProject.id)
        setIsProjectModalOpen(false)
        setEditingProject(null)
      } finally {
        setIsDeleting(false)
      }
    }
  }

  // Tracker product names stay as they are; what Sectile says about them
  // follows the UI language.
  const providerKey: keyof ProjectSettingsStrings['providers']['capabilities'] =
    issueTracker === 'github' || issueTracker === 'jira' || issueTracker === 'gitlab' ? issueTracker : 'local'
  const providerDescription = ps.providers.descriptions[providerKey]
  const providerCapabilities = ps.providers.capabilities[providerKey]

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200 select-none"
      {...backdrop}
    >
      <div
        className="relative w-[980px] h-[680px] max-w-[calc(var(--app-w)-32px)] max-h-[calc(var(--app-h)-32px)] rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-150"
        role="dialog"
        aria-modal="true"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <div className="flex items-center gap-2.5">
            <div
              className="w-8 h-8 rounded-xl flex items-center justify-center border shadow-xs"
              style={accentBadgeStyle(color)}
            >
              <Layers size={16} />
            </div>
            <div>
              <h3 className="text-sm font-bold text-[var(--text-primary)]">
                {editingProject ? format(ps.shell.settingsTitle, { name: editingProject.name }) : ps.shell.newProjectTitle}
              </h3>
              <p className="text-[11px] text-[var(--text-muted)]">
                {editingProject ? ps.shell.editSubtitle : ps.shell.newSubtitle}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={() => {
              setIsProjectModalOpen(false)
              setEditingProject(null)
            }}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            title={ps.shell.close}
            aria-label={ps.shell.close}
          >
            <X size={17} />
          </button>
        </div>

        {/* Modal 2-Column Body: Left Sidebar Tabs + Right Content Area */}
        <div className="flex flex-1 min-h-0 overflow-hidden">
          {/* Left Sidebar Navigation */}
          <div className="w-56 shrink-0 border-r border-[var(--border-color)] bg-[var(--bg-tertiary)]/25 p-3 flex flex-col justify-between overflow-y-auto">
            <nav className="space-y-1">
              {TABS.map(tab => {
                const Icon = tab.icon
                const isActive = activeTab === tab.id
                return (
                  <button
                    key={tab.id}
                    type="button"
                    onClick={() => setActiveTab(tab.id)}
                    className={`w-full flex items-center justify-between gap-2.5 px-3 py-2.5 rounded-xl text-xs font-semibold transition-all cursor-pointer text-left ${
                      isActive
                        ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                        : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]/60'
                    }`}
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className={isActive ? 'accent-text' : tab.iconColor}>
                        <Icon size={15} />
                      </span>
                      <span className="truncate">{ps.tabs[tab.id]}</span>
                    </div>

                    {/* Status Indicators / Badges */}
                    {tab.id === 'general' && skillsStatus && (
                      <span
                        className={`w-2 h-2 rounded-full shrink-0 ${skillsStatus.isGitRepo ? 'bg-emerald-400' : 'bg-amber-400'}`}
                        title={skillsStatus.isGitRepo ? ps.shell.gitRepoValid : ps.shell.notGitRepo}
                      />
                    )}
                    {tab.id === 'skills' && skillsStatus && (
                      <span className={`text-[9.5px] font-mono px-1.5 py-0.5 rounded-full font-bold shrink-0 ${
                        skillsStatus.installedAll ? 'bg-emerald-500/20 text-emerald-300' : 'bg-amber-500/20 text-amber-300'
                      }`}>
                        {(skillsStatus.skills || []).filter(s => s.installed).length}/5
                      </span>
                    )}
                    {tab.id === 'tracker' && detectedStatuses.length > 0 && (
                      <span className="text-[9.5px] font-mono px-1.5 py-0.5 rounded-full bg-[var(--accent-light)] accent-text font-bold shrink-0">
                        {detectedStatuses.length}
                      </span>
                    )}
                  </button>
                )
              })}
            </nav>

            <div className="pt-3 border-t border-[var(--border-color)]/60 px-2 flex items-center justify-between text-[10px] text-[var(--text-muted)]">
              <span className="truncate font-medium">{editingProject ? editingProject.name : ps.shell.newProject}</span>
              {editingProject && (
                <span className="font-mono text-[9px] px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                  {editingProject.issueTracker || 'local'}
                </span>
              )}
            </div>
          </div>

          {/* Right Scrollable Content Pane */}
          <form id="project-modal-form" onSubmit={handleSubmit} className="flex-1 min-w-0 p-6 overflow-y-auto space-y-6 text-xs">
          {/* ========================================================= */}
          {/* SECTION 1: GÉNÉRAL (Identité, apparence, dépôt Git)        */}
          {/* ========================================================= */}
          {activeTab === 'general' && (
            <div className="space-y-4 animate-in fade-in duration-150">
              {/* Identité à gauche, apparence à droite */}
              <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_260px] gap-4 items-start">
                {/* Colonne gauche : titre, description, slug, projet par défaut */}
                <div className="space-y-3.5">
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.general.titleLabel}
                    </label>
                    <input
                      type="text"
                      required
                      value={name}
                      onChange={e => handleNameChange(e.target.value)}
                      placeholder={ps.general.titlePlaceholder}
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.general.descriptionLabel}
                    </label>
                    <input
                      type="text"
                      value={description}
                      onChange={e => setDescription(e.target.value)}
                      placeholder={ps.general.descriptionPlaceholder}
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.general.slugLabel}
                    </label>
                    <input
                      type="text"
                      value={slug}
                      onChange={e => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9]+/g, '-'))}
                      placeholder={ps.general.slugPlaceholder}
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                    />
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="isDefault"
                      checked={isDefault}
                      onChange={e => setIsDefault(e.target.checked)}
                      className="rounded border-[var(--border-color)] text-[var(--accent-color)] focus:ring-[var(--accent-color)] cursor-pointer"
                    />
                    <label htmlFor="isDefault" className="text-xs text-[var(--text-primary)] cursor-pointer font-medium">
                      {ps.general.isDefault}
                    </label>
                  </div>
                </div>

                {/* Colonne droite : icône et couleur d'accent */}
                <div className="space-y-3.5">
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      {ps.general.iconLabel}
                    </label>
                    <div className="flex flex-wrap gap-1.5 p-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                      {AVAILABLE_ICONS.map(({ name: iconName, Icon }) => {
                        const isSel = icon === iconName
                        return (
                          <button
                            key={iconName}
                            type="button"
                            onClick={() => setIcon(iconName)}
                            className={`p-1.5 rounded-lg flex items-center justify-center transition-all cursor-pointer ${
                              isSel
                                ? 'bg-[var(--accent-color)] text-white shadow-sm scale-105'
                                : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-primary)]'
                            }`}
                            title={iconName}
                          >
                            <Icon size={15} />
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      {t.profileModal.accentColor}
                    </label>
                    <div className="grid grid-cols-6 gap-1.5 p-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)]">
                      {ACCENT_COLORS.map(c => {
                        const isSel = color === c.name
                        return (
                          <button
                            key={c.name}
                            type="button"
                            onClick={() => setColor(c.name)}
                            className={`h-6 rounded-lg flex items-center justify-center transition-all cursor-pointer ${
                              isSel ? 'ring-2 ring-white ring-offset-2 ring-offset-[var(--bg-secondary)] scale-105 shadow-sm' : 'opacity-80 hover:opacity-100'
                            }`}
                            style={{ backgroundColor: c.hex }}
                            title={t.profileModal.accents[c.name] || c.label}
                          >
                            {isSel && <Check size={11} className="text-white drop-shadow" />}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                </div>
              </div>

              {/* Dépôt Git : l'adresse distante suffit, le clone local est géré par l'agent */}
              <div className="pt-3 border-t border-[var(--border-color)] space-y-2">
                <div className="flex items-center gap-1.5">
                  <GitBranch size={13} className="text-[var(--accent-color)]" />
                  <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-primary)]">
                    {ps.repositories.sectionTitle}
                  </span>
                </div>
                <div>
                  <label htmlFor="gitRemoteUrl" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                    {ps.repositories.remoteUrlLabel}
                  </label>
                  <input
                    id="gitRemoteUrl"
                    type="text"
                    value={gitRemoteUrl}
                    onChange={e => setGitRemoteUrl(e.target.value)}
                    placeholder="git@github.com:owner/repository.git"
                    className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                  />
                </div>
                <div>
                  <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                    {ps.repositories.listLabel}
                  </span>
                  <ul className="space-y-1">
                    {gitRemoteUrl.trim() && (
                      <li
                        className="flex items-center gap-2 px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-muted)] font-mono"
                        title={ps.repositories.codeRepositoryTitle}
                      >
                        <span className="truncate flex-1">{gitRemoteUrl.trim()}</span>
                        <span className="font-sans text-[10px] shrink-0">{ps.repositories.codeBadge}</span>
                      </li>
                    )}
                    {repositories.map(url => (
                      <li
                        key={url}
                        className="flex items-center gap-2 px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono"
                      >
                        <span className="truncate flex-1">{url}</span>
                        <button
                          type="button"
                          onClick={() => setRepositories(prev => prev.filter(entry => entry !== url))}
                          className="shrink-0 text-[var(--text-muted)] hover:text-red-500 cursor-pointer"
                          title={ps.repositories.removeTitle}
                          aria-label={format(ps.repositories.removeAria, { url })}
                        >
                          <X size={12} />
                        </button>
                      </li>
                    ))}
                  </ul>
                  <div className="flex gap-2 mt-1.5">
                    <input
                      type="text"
                      value={newRepository}
                      onChange={e => {
                        setNewRepository(e.target.value)
                        setRepositoryError('')
                      }}
                      onKeyDown={e => {
                        if (e.key === 'Enter') {
                          e.preventDefault()
                          addRepository()
                        }
                      }}
                      placeholder="git@github.com:owner/other-repository.git"
                      aria-label={ps.repositories.otherRepositoryAria}
                      className="flex-1 min-w-0 px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <button
                      type="button"
                      onClick={addRepository}
                      disabled={!newRepository.trim()}
                      className="px-3 py-1.5 text-xs rounded-xl border border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--accent-color)] disabled:opacity-50 cursor-pointer"
                    >
                      {ps.repositories.add}
                    </button>
                  </div>
                  {repositoryError && (
                    <p role="alert" className="mt-1 text-[10px] text-red-500">{repositoryError}</p>
                  )}
                </div>
                {droppedPaths.length > 0 && (
                  <p className="text-[10px] text-amber-500 leading-relaxed">
                    {format(ps.repositories.droppedPaths, { paths: droppedPaths.map(entry => `${entry.path} (${entry.reason})`).join(', ') })}
                  </p>
                )}
                <p className="text-[10px] text-[var(--text-muted)] leading-relaxed">
                  {ps.repositories.desktopNote}
                </p>
              </div>

              {/* Vues optionnelles : masquées tant que le projet ne les demande pas */}
              <div className="pt-3 border-t border-[var(--border-color)] space-y-2">
                <div className="flex items-center gap-1.5">
                  <Layers size={13} className="text-[var(--accent-color)]" />
                  <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-primary)]">
                    {ps.views.sectionTitle}
                  </span>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                  {OPTIONAL_VIEW_CARDS.map(view => {
                    const Icon = view.icon
                    const isActive = enabledViews.includes(view.id)
                    return (
                      <button
                        key={view.id}
                        type="button"
                        onClick={() =>
                          // Forme fonctionnelle : deux clics dans le même cycle
                          // de rendu liraient sinon le même état, et le second
                          // annulerait le premier.
                          setEnabledViews(current =>
                            OPTIONAL_VIEWS.filter(id =>
                              id === view.id ? !current.includes(id) : current.includes(id)
                            )
                          )
                        }
                        aria-pressed={isActive}
                        title={ps.views.hints[view.id]}
                        className={`flex items-start gap-2 p-2.5 rounded-xl border text-left transition-all cursor-pointer ${
                          isActive
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)]/40 accent-text'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        <Icon size={14} className={`shrink-0 mt-0.5 ${isActive ? '' : view.iconColor}`} />
                        <span className="min-w-0">
                          <span className="block text-xs font-bold truncate">{view.label}</span>
                          <span className="block text-[10px] text-[var(--text-muted)] leading-snug">
                            {ps.views.hints[view.id]}
                          </span>
                        </span>
                        {isActive && <Check size={12} className="shrink-0 mt-0.5" />}
                      </button>
                    )
                  })}
                </div>
                <p className="text-[10px] text-[var(--text-muted)] leading-relaxed">
                  {ps.views.hiddenByDefault}
                </p>
                <label className="flex items-start gap-2 pt-1 text-xs text-[var(--text-secondary)] cursor-pointer">
                  <input
                    type="checkbox"
                    checked={epicColors}
                    onChange={e => setEpicColors(e.target.checked)}
                    className="mt-0.5 rounded border-[var(--border-color)] accent-[var(--accent-color)]"
                  />
                  <span>
                    <span className="block font-bold text-[var(--text-primary)]">{ps.views.epicColors}</span>
                    <span className="block text-[10px] text-[var(--text-muted)] leading-snug">
                      {ps.views.epicColorsHelp}
                    </span>
                  </span>
                </label>
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* SECTION 3B: AGENTIC WORKFLOW (PR/MR, mapping des statuts) */}
          {/* ========================================================= */}
          {activeTab === 'workflow' && (
            <div className="space-y-4 animate-in fade-in duration-150">
              {/* Étape à laquelle l'agent ouvre la Pull/Merge Request */}
              <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-2">
                <div className="flex items-center gap-2.5">
                  <div className="w-8 h-8 rounded-xl bg-[var(--accent-light)] accent-text flex items-center justify-center shrink-0 border border-[var(--accent-color)]/30">
                    <GitBranch size={16} />
                  </div>
                  <div>
                    <span className="text-xs font-bold text-[var(--text-primary)] block">
                      {ps.workflow.prTitle}
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] block">
                      {ps.workflow.prHelp}
                    </span>
                  </div>
                </div>
                <select
                  id="prCreationStage"
                  value={prCreationStage}
                  onChange={e => setPRCreationStage(e.target.value as 'specified' | 'implemented')}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="implemented">{ps.workflow.draftAfterImplementation}</option>
                  <option value="specified">{ps.workflow.draftAfterSpecification}</option>
                </select>
              </div>

              <label className="flex items-start gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
                <input
                  type="checkbox"
                  checked={dropSpecArtifacts}
                  onChange={e => setDropSpecArtifacts(e.target.checked)}
                  className="mt-0.5 rounded border-[var(--border-color)] accent-[var(--accent-color)]"
                />
                <span>
                  {ps.workflow.dropSpecArtifacts}
                  <span className="block text-[11px] text-[var(--text-muted)]">
                    {ps.workflow.dropSpecArtifactsHelp}
                  </span>
                </span>
              </label>

              <div>
                <label htmlFor="defaultSkillMode" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {ps.execution.defaultModeLabel}
                </label>
                <select
                  id="defaultSkillMode"
                  value={defaultSkillMode}
                  onChange={e => setDefaultSkillMode(e.target.value as SkillMode)}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="interactive">{ps.execution.interactive}</option>
                  <option value="autonomous">{ps.execution.autonomous}</option>
                  <option value="">{ps.execution.perSkill}</option>
                </select>
                <p className="mt-1 text-[10px] text-[var(--text-muted)] leading-relaxed">
                  {ps.execution.defaultModeHelp}
                </p>
              </div>

              <div>
                <label htmlFor="fullChainStopStage" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {ps.execution.fullChainStopLabel}
                </label>
                <select
                  id="fullChainStopStage"
                  value={fullChainStopStage}
                  onChange={e => setFullChainStopStage(e.target.value as 'implemented' | 'reviewed')}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="reviewed">{ps.execution.afterPr}</option>
                  <option value="implemented">{ps.execution.beforePr}</option>
                </select>
                <p className="mt-1 text-[10px] text-[var(--text-muted)] leading-relaxed">
                  {ps.execution.mergeStaysManual}
                </p>
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* SECTION 4: TRACKER (Sectile local, GitHub, Jira)          */}
          {/* ========================================================= */}
          {activeTab === 'tracker' && (
            <div className="space-y-3.5 animate-in fade-in duration-150">
              {/* Tracker Type Radio Pills */}
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {ps.tracker.typeLabel}
                </label>
                <select
                  value={issueTracker}
                  onChange={e => {
                    const newTrk = e.target.value as IssueTracker
                    setIssueTracker(newTrk)
                    fetchDetectedStatuses(newTrk)
                    // Le projet part de l'instance de la personne, celle que
                    // porte son accès personnel, et retombe sur celle du
                    // serveur. Une valeur déjà saisie ici n'est pas écrasée.
                    if (newTrk === 'jira' && !trackerUrl.trim()) {
                      const mine = userCredentials.find(c => c.tracker === 'jira')?.siteUrl?.trim()
                      const inherited = mine || settings.jiraUrl?.trim()
                      if (inherited) setTrackerUrl(inherited)
                    }
                    // The project path starts from the default of the
                    // settings; the instance stays empty, which follows them.
                    if (newTrk === 'gitlab' && !gitlabProject.trim() && settings.gitlabProject?.trim()) {
                      setGitlabProject(settings.gitlabProject.trim())
                    }
                  }}
                  className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium cursor-pointer"
                >
                  {PROJECT_TRACKERS.map(tracker => (
                    <option key={tracker.id} value={tracker.id}>
                      {tracker.label}
                    </option>
                  ))}
                </select>
              </div>

              {/* Panneau d'information & fonctionnalités supportées par le tracker */}
              <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)] space-y-2.5 text-xs">
                <div className="flex items-center gap-1.5 font-semibold text-[var(--text-primary)]">
                  <Info size={13} className="text-[var(--accent-color)]" />
                  <span>
                    {issueTracker === 'local' && ps.providers.localName}
                    {issueTracker === 'github' && 'GitHub Issues'}
                    {issueTracker === 'jira' && 'Jira'}
                    {issueTracker === 'gitlab' && 'GitLab'}
                  </span>
                </div>

                <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">
                  {providerDescription}
                </p>

                {/* Capabilities the selected tracker supports */}
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-1.5 pt-2 border-t border-[var(--border-color)]/50">
                  {providerCapabilities.map(capability => (
                    <div key={capability} className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                      <CheckCircle2 size={11} className="shrink-0" />
                      <span>{capability}</span>
                    </div>
                  ))}
                  {issueTracker === 'github' && (
                    <div className="flex items-center gap-1 text-[10.5px] text-amber-400 font-medium" title={ps.providers.githubStatusNoteTitle}>
                      <Info size={11} className="shrink-0" />
                      <span>{ps.providers.githubStatusNote}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Team Key & Github Repo inputs */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {issueTracker === 'github' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.githubRepoLabel}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={githubRepo}
                        onChange={e => {
                          setGithubRepo(e.target.value)
                          fetchDetectedStatuses('github', e.target.value)
                        }}
                        placeholder={ps.tracker.githubRepoPlaceholder}
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'github' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.githubInstanceLabel}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={githubApiUrl}
                        onChange={e => setGithubApiUrl(e.target.value)}
                        placeholder={ps.tracker.instancePlaceholder}
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'gitlab' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.gitlabProjectLabel}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={gitlabProject}
                        onChange={e => setGitlabProject(e.target.value)}
                        placeholder={settings.gitlabProject?.trim() || ps.tracker.gitlabProjectPlaceholder}
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'gitlab' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.gitlabInstanceLabel}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={gitlabUrl}
                        onChange={e => setGitlabUrl(e.target.value)}
                        placeholder={settings.gitlabUrl?.trim() || 'https://gitlab.com/api/v4'}
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'jira' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.jiraProjectLabel}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={jiraProject}
                        onChange={e => {
                          const val = e.target.value.toUpperCase()
                          setJiraProject(val)
                          fetchDetectedStatuses('jira')
                        }}
                        placeholder={ps.tracker.jiraProjectPlaceholder}
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono uppercase focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Key size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      {richText(ps.tracker.jiraProjectHelp, { link: <em>{t.trackerCredentials.setup.title}</em> })}
                    </span>
                  </div>
                )}

                {issueTracker === 'jira' && (
                  <div>
                    <label htmlFor="project-roadmap-projects" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.roadmapProjectsLabel}
                    </label>
                    <input
                      id="project-roadmap-projects"
                      type="text"
                      value={roadmapProjects}
                      onChange={e => setRoadmapProjects(e.target.value.toUpperCase())}
                      placeholder={ps.tracker.roadmapProjectsPlaceholder}
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono uppercase focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      {ps.tracker.roadmapProjectsHelp}
                    </span>
                  </div>
                )}

                {issueTracker === 'jira' && (
                  <div className="col-span-2">
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      {ps.tracker.issueTypesLabel}
                    </label>
                    {isLoadingIssueTypes ? (
                      <span className="text-[10px] text-[var(--text-muted)]">{ps.tracker.issueTypesLoading}</span>
                    ) : availableIssueTypes.length === 0 ? (
                      <span className="text-[10px] text-[var(--text-muted)]">
                        {ps.tracker.issueTypesUnavailable}
                      </span>
                    ) : (
                      <div className="flex flex-wrap gap-1.5">
                        {availableIssueTypes.map(type => {
                          const isDefault = type === 'Task' || type === 'Story'
                          const isActive = issueTypes.length === 0 ? isDefault : issueTypes.includes(type)
                          return (
                            <button
                              key={type}
                              type="button"
                              onClick={() => {
                                // Le premier clic fige la sélection courante : sans
                                // cela, décocher « Task » sur un projet resté aux
                                // valeurs par défaut ne changerait rien.
                                const current = issueTypes.length === 0
                                  ? availableIssueTypes.filter(t2 => t2 === 'Task' || t2 === 'Story')
                                  : issueTypes
                                setIssueTypes(
                                  current.includes(type)
                                    ? current.filter(t2 => t2 !== type)
                                    : [...current, type]
                                )
                              }}
                              className="px-2 py-1 rounded-lg text-[10.5px] font-semibold border cursor-pointer transition-colors"
                              style={{
                                color: isActive ? 'var(--accent-color)' : 'var(--text-secondary)',
                                background: isActive ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                                borderColor: isActive ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
                              }}
                            >
                              {type}
                            </button>
                          )
                        })}
                      </div>
                    )}
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      {ps.tracker.issueTypesHelp}
                    </span>
                  </div>
                )}

                <div className={issueTracker === 'local' ? 'col-span-2' : ''}>
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                    {issueTracker === 'jira' ? ps.tracker.jiraUrlLabel : ps.tracker.trackerUrlLabel}
                    {issueTracker === 'jira' && trackerUrl.trim() && trackerUrl.trim() === userCredentials.find(c => c.tracker === 'jira')?.siteUrl?.trim() ? (
                      <span className="ml-1 font-normal normal-case text-[9px] text-[var(--text-muted)]">
                        {ps.tracker.inheritedFromAccess}
                      </span>
                    ) : null}
                  </label>
                  <div className="relative">
                    <input
                      type="text"
                      value={trackerUrl}
                      onChange={e => setTrackerUrl(e.target.value)}
                      placeholder={issueTracker === 'jira' ? ps.tracker.jiraUrlPlaceholder : issueTracker === 'gitlab' ? ps.tracker.gitlabUrlPlaceholder : 'https://github.com/owner/repository'}
                      className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                  </div>
                  {issueTracker === 'jira' && (
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      {richText(ps.tracker.browseHelp, { path: <code className="text-cyan-400">/browse/&lt;KEY&gt;</code> })}
                    </span>
                  )}
                </div>
              </div>

              {/* Section Synchronisation en arrière-plan */}
              <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-xs font-bold text-[var(--text-primary)] block">
                      {ps.tracker.syncTitle}
                    </label>
                    <span className="text-[10px] text-[var(--text-secondary)] block mt-0.5">
                      {ps.tracker.syncHelp}
                    </span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={autoSyncEnabled}
                    onClick={() => setAutoSyncEnabled(v => !v)}
                    className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out ${
                      autoSyncEnabled ? 'bg-[var(--accent-color)]' : 'bg-[var(--border-color)]'
                    }`}
                  >
                    <span
                      className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-lg ring-0 transition duration-200 ease-in-out ${
                        autoSyncEnabled ? 'translate-x-4' : 'translate-x-0'
                      }`}
                    />
                  </button>
                </div>

                {autoSyncEnabled && (
                  <div className="pt-2 border-t border-[var(--border-color)] space-y-1.5">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-[var(--text-secondary)] font-medium">{ps.tracker.syncPeriod}</span>
                      <span className="font-mono font-bold text-[var(--accent-color)]">{format(ps.tracker.minutes, { count: autoSyncIntervalMin })}</span>
                    </div>
                    <input
                      type="range"
                      min={1}
                      max={30}
                      value={autoSyncIntervalMin}
                      onChange={e => setAutoSyncIntervalMin(parseInt(e.target.value, 10) || 5)}
                      className="w-full h-1.5 bg-[var(--bg-primary)] rounded-lg appearance-none cursor-pointer accent-[var(--accent-color)]"
                    />
                    <div className="flex justify-between text-[9px] text-[var(--text-muted)] font-mono">
                      <span>{format(ps.tracker.minutes, { count: 1 })}</span>
                      <span>{format(ps.tracker.minutes, { count: 15 })}</span>
                      <span>{format(ps.tracker.minutes, { count: 30 })}</span>
                    </div>
                  </div>
                )}
              </div>

              {/* Colonnes du board */}
              <div className="pt-3 border-t border-[var(--border-color)]">
                <BoardColumnsEditor
                  project={editingProject}
                  columns={trackerColumns}
                  onColumnsChange={newCols => {
                    setTrackerColumns(newCols)
                  }}
                  stageColumns={stageColumns}
                  onStageColumnsChange={setStageColumns}
                  issueTracker={issueTracker}
                  githubRepo={githubRepo}
                />
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* SECTION 5: COMPÉTENCES IA & SDD (Spec Kit vs OpenSpec)     */}
          {/* ========================================================= */}
          {activeTab === 'skills' && (
            <div className="space-y-4 animate-in fade-in duration-150">
              {/* SDD Framework Selection Cards */}
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  {ps.skills.frameworkLabel}
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  <button
                    type="button"
                    onClick={() => setSpecFramework('speckit')}
                    className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-3 ${
                      specFramework === 'speckit'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                    }`}
                  >
                    <div className={`p-2 rounded-xl shrink-0 ${specFramework === 'speckit' ? 'bg-[var(--accent-color)] text-white' : 'bg-[var(--bg-primary)] text-[var(--text-muted)]'}`}>
                      <FileCode size={18} />
                    </div>
                  <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5 mb-0.5">
                        <span className={`font-bold text-xs ${specFramework === 'speckit' ? 'accent-text' : 'text-[var(--text-primary)]'}`}>
                          GitHub Spec Kit
                        </span>
                        <span className="text-[9px] font-mono px-1.5 py-0.2 rounded-full font-bold bg-blue-500/20 text-blue-300 border border-blue-500/30">
                          {ps.skills.recommended}
                        </span>
                      </div>
                      <span className="text-[10px] text-[var(--text-muted)] leading-relaxed block">
                        {richText(ps.skills.speckitDescription, {
                          cli: <code className="text-cyan-400">specify</code>,
                          specifyDir: <code className="text-cyan-400">.specify/</code>,
                          specsDir: <code className="text-cyan-400">specs/</code>,
                          commands: <code className="text-cyan-400">/speckit.*</code>,
                        })}
                      </span>
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setSpecFramework('openspec')}
                    className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-3 ${
                      specFramework === 'openspec'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                    }`}
                  >
                    <div className={`p-2 rounded-xl shrink-0 ${specFramework === 'openspec' ? 'bg-[var(--accent-color)] text-white' : 'bg-[var(--bg-primary)] text-[var(--text-muted)]'}`}>
                      <Sparkles size={18} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5 mb-0.5">
                        <span className={`font-bold text-xs ${specFramework === 'openspec' ? 'accent-text' : 'text-[var(--text-primary)]'}`}>
                          OpenSpec
                        </span>
                        <span className="text-[9px] font-mono px-1.5 py-0.2 rounded-full font-bold bg-purple-500/20 text-purple-300 border border-purple-500/30">
                          {ps.skills.specBeforeCode}
                        </span>
                      </div>
                      <span className="text-[10px] text-[var(--text-muted)] leading-relaxed block">
                        {richText(ps.skills.openspecDescription, {
                          cli: <code className="text-cyan-400">openspec</code>,
                          dir: <code className="text-cyan-400">openspec/</code>,
                        })}
                      </span>
                    </div>
                  </button>
                </div>
              </div>

              {/* SDD toolchain installer: installs the real CLI and initializes it */}


              {/* Skills Overrides Table List */}
              <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-2.5">
                <div className="flex items-center justify-between pb-1 border-b border-[var(--border-color)]">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-primary)]">
                    {ps.skills.workflowSkillsTitle}
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)]">
                    {ps.skills.workflowSkillsNote}
                  </span>
                </div>

                <div className="space-y-2">
                  {WORKFLOW_SKILLS.map(s => {
                    const isInst = (skillsStatus?.skills || []).find(sk => sk.id === s.id)?.installed || false
                    const SkillIcon = s.icon
                    const displayDefaultName = s.id === 'specify'
                      ? (specFramework === 'openspec' ? 'Specify (OpenSpec)' : 'Specify (Spec Kit)')
                      : s.defaultName

                    return (
                      <div
                        key={s.id}
                        className="p-2.5 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] flex flex-col sm:flex-row sm:items-center justify-between gap-2.5"
                      >
                        {/* Skill info Left */}
                        <div className="flex items-center gap-2.5 min-w-[200px]">
                          <div className={`w-6 h-6 rounded-lg flex items-center justify-center text-${s.color}-400 bg-${s.color}-500/15 shrink-0 border border-${s.color}-500/30`}>
                            <SkillIcon size={13} />
                          </div>
                          <div className="flex flex-col min-w-0">
                            <div className="flex items-center gap-1.5">
                              <span className="font-bold text-xs text-[var(--text-primary)]">
                                {displayDefaultName}
                              </span>
                              <span className="text-[9px] font-mono text-[var(--text-muted)] bg-[var(--bg-tertiary)] px-1 py-0.2 rounded-md">
                                /{s.code}
                              </span>
                            </div>
                            <span className="text-[10px] text-[var(--text-muted)] truncate max-w-[220px]">
                              {s.id === 'specify'
                                ? (specFramework === 'openspec' ? ps.skills.specifyOpenSpec : ps.skills.specifySpecKit)
                                : ps.skills.descriptions[s.id]}
                            </span>
                          </div>
                        </div>

                        {/* Installation state */}
                        <div className="flex items-center gap-2 flex-1 justify-end">
                          <div className="shrink-0 w-20 text-right">
                            {isInst ? (
                              <span className="inline-flex items-center gap-1 text-[10px] font-bold text-emerald-400">
                                <Check size={11} />
                                <span>{ps.skills.installed}</span>
                              </span>
                            ) : (
                              <span className="text-[10px] font-medium text-[var(--text-muted)]">
                                {ps.skills.notInstalled}
                              </span>
                            )}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          )}
          </form>
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-between px-6 py-4 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
          <div>
            {editingProject && !editingProject.isDefault && (
              <button
                type="button"
                onClick={handleDelete}
                disabled={isDeleting}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold text-rose-400 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 transition-colors cursor-pointer"
                title={ps.feedback.deleteTitle}
              >
                <Trash2 size={13} />
                <span>{isDeleting ? ps.feedback.deleting : ps.feedback.delete}</span>
              </button>
            )}
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => {
                setIsProjectModalOpen(false)
                setEditingProject(null)
              }}
              className="px-4 py-2 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            >
              {t.taskModal.cancel}
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={isSubmitting || !name.trim()}
              className="px-5 py-2 rounded-xl text-xs font-bold text-white accent-bg shadow-md hover:opacity-90 active:scale-95 flex items-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer"
            >
              <Save size={14} />
              <span>{isSubmitting ? ps.feedback.saving : editingProject ? ps.feedback.update : ps.feedback.create}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
