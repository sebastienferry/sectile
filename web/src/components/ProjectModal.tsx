import React, { useState, useEffect } from 'react'
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
  RotateCcw,
  Bot,
  Info,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { BoardColumnsEditor } from './BoardColumnsEditor'
import { CommandModePreview } from './CommandModePreview'
import type {
  AccentColor,
  IssueTracker,
  ProjectSkillsStatus,
  DetectedStatus,
  AIProvider,
  TrackerColumn,
  SpecFramework,
  SpecFrameworkStatus,
  SpecFrameworkInstallResult,
  SkillMode,
} from '../types'
import { ACCENT_COLORS, accentBadgeStyle, normalizeAccentColor, DEFAULT_PROJECT_ACCENT } from '../lib/accents'
import { AIModelField } from './AIModelField'
import { isValidModel } from '../lib/aiModels'

type ProjectTab = 'general' | 'agent' | 'workflow' | 'tracker' | 'skills'

const TABS: { id: ProjectTab; label: string; icon: React.FC<{ size?: number; className?: string }> }[] = [
  { id: 'general', label: 'Général', icon: Folder },
  { id: 'tracker', label: 'Tracker', icon: Sliders },
  { id: 'agent', label: 'Agent settings', icon: Bot },
  { id: 'workflow', label: 'Agentic workflow', icon: Workflow },
  { id: 'skills', label: 'Compétences IA & SDD', icon: Sparkles },
]

// An empty template is the right default: the agent then runs the command line
// it attests for each execution mode. Only a custom CLI must spell one out.
const AI_PROVIDERS: { id: AIProvider; label: string; sub: string; defaultCmd: string; icon: string }[] = [
  { id: 'agy', label: 'AGY CLI (Google Antigravity)', sub: 'Agent autonome DeepMind & outils natifs', defaultCmd: '', icon: '🤖' },
  { id: 'claude', label: 'Claude Code CLI (Anthropic)', sub: 'Agent Terminal Claude Code', defaultCmd: '', icon: '🧠' },
  { id: 'codex', label: 'Codex', sub: 'Codex CLI', defaultCmd: '', icon: '💻' },
  { id: 'custom', label: 'Commande Personnalisée', sub: 'Modèle de commande arbitraire', defaultCmd: `/path/to/custom-cli {mode:-p|-i} '{prompt}'`, icon: '⚙️' },
]

// A preset fills both fields at once, since the two commands of one CLI are
// written together. An empty pair hands both modes back to the provider.
const COMMAND_PRESETS: { label: string; cmd: string; autonomous: string }[] = [
  { label: 'Défaut du fournisseur', cmd: '', autonomous: '' },
  {
    label: 'Claude',
    cmd: `claude --model {model} '{prompt}'`,
    autonomous: `claude -p --permission-mode bypassPermissions --model {model} '{prompt}'`,
  },
  { label: 'AGY', cmd: `agy -i '{prompt}'`, autonomous: `agy -p --dangerously-skip-permissions '{prompt}'` },
  { label: 'Codex', cmd: `codex --model {model} '{prompt}'`, autonomous: `codex exec --model {model} '{prompt}'` },
]

// The agents Sectile can install its skills and MCP registration for, beyond the
// one that runs the tasks. Providers without a skill convention are not listed.
const SETUP_PROVIDERS: { id: string; label: string; sub: string; icon: string }[] = [
  { id: 'claude', label: 'Claude Code', sub: '~/.claude/skills et registre MCP', icon: '🧠' },
  { id: 'codex', label: 'Codex', sub: '~/.codex/skills et config.toml', icon: '💻' },
  { id: 'agy', label: 'Antigravity', sub: '~/.agy/skills et registre MCP', icon: '🤖' },
]

const AVAILABLE_ICONS = [
  { name: 'Folder', Icon: Folder, label: 'Dossier' },
  { name: 'Terminal', Icon: Terminal, label: 'Terminal' },
  { name: 'Zap', Icon: Zap, label: 'Éclair' },
  { name: 'Flame', Icon: Flame, label: 'Flamme' },
  { name: 'Layers', Icon: Layers, label: 'Calques' },
  { name: 'Box', Icon: Box, label: 'Module' },
  { name: 'Code2', Icon: Code2, label: 'Code' },
  { name: 'Cpu', Icon: Cpu, label: 'Core / CPU' },
  { name: 'Sparkles', Icon: Sparkles, label: 'IA / Magic' },
  { name: 'Workflow', Icon: Workflow, label: 'Workflow' },
]

const WORKFLOW_SKILLS: { id: string; defaultName: string; code: string; desc: string; icon: React.ComponentType<{ size?: number; className?: string }>; color: string }[] = [
  { id: 'clarify', defaultName: 'Clarify', code: 'clarify-issue', desc: 'Questions de cadrage & inputs produit', icon: HelpCircle, color: 'amber' },
  { id: 'specify', defaultName: 'Specify', code: 'specify-issue', desc: 'Spécification technique (Spec Kit / OpenSpec)', icon: FileCode, color: 'blue' },
  { id: 'implement', defaultName: 'Implement', code: 'code-issue', desc: 'Développement & codage de la story', icon: Flame, color: 'indigo' },
  { id: 'adjust', defaultName: 'Adjust', code: 'adjust-issue', desc: 'Revue de code, tests & Pull Request', icon: ShieldCheck, color: 'purple' },
  { id: 'handoff', defaultName: 'Handoff', code: 'handoff-issue', desc: 'Compte-rendu de passation & nettoyage local', icon: Sparkles, color: 'emerald' },
]

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
    settings,
    t,
  } = useApp()

  const [activeTab, setActiveTab] = useState<ProjectTab>('general')

  // Section 1: Général (Titre, description, icône, couleur, projet par défaut)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [icon, setIcon] = useState('Folder')
  const [color, setColor] = useState<AccentColor>(DEFAULT_PROJECT_ACCENT)
  const [isDefault, setIsDefault] = useState(false)

  // Section 2: Git (Local path, URL distante git@..., init git)
  const [repoPath, setRepoPath] = useState('')
  const [prCreationStage, setPRCreationStage] = useState<'specified' | 'implemented'>('implemented')
  const [defaultSkillMode, setDefaultSkillMode] = useState<SkillMode>('')
  const [fullChainStopStage, setFullChainStopStage] = useState<'implemented' | 'reviewed'>('reviewed')
  // Mono-dépôt : conditionne tout ce qui parle de « la » branche courante.
  const [trackerColumns, setTrackerColumns] = useState<TrackerColumn[]>([])
  const [stageColumns, setStageColumns] = useState<Record<string, string[]>>({})

  const [gitRemoteUrl, setGitRemoteUrl] = useState('')

  // Section 3: Agent IA & CLI
  const [aiProvider, setAiProvider] = useState<AIProvider | ''>('')
  const [aiCommandTemplate, setAiCommandTemplate] = useState('')
  const [aiCommandAutonomous, setAiCommandAutonomous] = useState('')
  const [aiModel, setAiModel] = useState('')
  const [aiSkillModels, setAiSkillModels] = useState<Record<string, string>>({})
  const [useCustomAgent, setUseCustomAgent] = useState(false)
  const [setupProviders, setSetupProviders] = useState<string[]>([])
  const [useWorktrees, setUseWorktrees] = useState(true)
  const [autoSyncEnabled, setAutoSyncEnabled] = useState(false)
  const [autoSyncIntervalMin, setAutoSyncIntervalMin] = useState(5)

  // Section 4: Compétences IA & Framework SDD
  const [specFramework, setSpecFramework] = useState<SpecFramework>('speckit')
  const [skillOverrides, setSkillOverrides] = useState<Record<string, string>>({})
  const [skillsStatus, setSkillsStatus] = useState<ProjectSkillsStatus | null>(null)




  // Spec-Driven Design toolchain (GitHub Spec Kit / OpenSpec) install state
  const [, setSddStatuses] = useState<SpecFrameworkStatus[]>([])


  const [, setSddResult] = useState<SpecFrameworkInstallResult | null>(null)

  // Section 5: Tracker (Type, URL, Clef, Mapping)
  const [issueTracker, setIssueTracker] = useState<IssueTracker>('local')
  const [trackerUrl, setTrackerUrl] = useState('')
  const [githubRepo, setGithubRepo] = useState('')
  // Paramètres de connexion propres au projet. Vides, ce sont ceux de la
  // configuration utilisateur qui s'appliquent : un projet n'en a besoin que
  // pour joindre une autre instance, ou une même instance avec un autre compte.
  const [githubApiUrl, setGithubApiUrl] = useState('')
  const [githubToken, setGithubToken] = useState('')
  const [jiraProject, setJiraProject] = useState('')
  // Types de tickets importés. Vide vaut « les types par défaut » : c'est ce que
  // porte un projet qui n'a jamais eu besoin d'y toucher.
  const [issueTypes, setIssueTypes] = useState<string[]>([])
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

      setRepoPath(editingProject.repoPath || '')
      setPRCreationStage(editingProject.prCreationStage || 'implemented')
      setDefaultSkillMode(editingProject.defaultSkillMode || '')
      setFullChainStopStage(editingProject.fullChainStopStage || 'reviewed')
      setTrackerColumns(editingProject.trackerColumns || [])
      setStageColumns(editingProject.stageColumns || {})
      setGitRemoteUrl(editingProject.gitRemoteUrl || '')

      const hasCustomAgent = Boolean(editingProject.aiProvider || editingProject.aiCommandTemplate)
      setUseCustomAgent(hasCustomAgent)
      setAiProvider(editingProject.aiProvider || '')
      setAiCommandTemplate(editingProject.aiCommandTemplate || '')
      setAiCommandAutonomous(editingProject.aiCommandTemplateAutonomous || '')
      setAiModel(editingProject.aiModel || '')
      setAiSkillModels(editingProject.aiSkillModels || {})
      setSetupProviders(editingProject.setupProviders || [])
      setSpecFramework(editingProject.specFramework || settings.specFramework || 'speckit')
      setUseWorktrees(editingProject.useWorktrees !== false)
      setAutoSyncEnabled(Boolean(editingProject.autoSyncEnabled))
      setAutoSyncIntervalMin(editingProject.autoSyncIntervalMin || 5)

      setIssueTracker(editingProject.issueTracker || 'local')
      setTrackerUrl(editingProject.trackerUrl || '')
      setGithubRepo(editingProject.githubRepo || '')
      setGithubApiUrl(editingProject.githubApiUrl || '')
      // Le jeton n'est jamais renvoyé : le champ reste vide et le laisser vide
      // conserve celui qui est enregistré.
      setGithubToken('')
      setJiraProject(editingProject.jiraProject || '')
      setIssueTypes(editingProject.issueTypes || [])
      setSkillOverrides(editingProject.skillOverrides || {})


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

      setRepoPath('')
      setGitRemoteUrl('')

      setUseCustomAgent(false)
      setAiProvider('')
      setAiCommandTemplate('')
      setSetupProviders([])
      setSpecFramework(settings.specFramework || 'speckit')
      setUseWorktrees(true)
      setAutoSyncEnabled(false)
      setAutoSyncIntervalMin(5)

      setIssueTracker('local')
      setTrackerUrl('')
      setGithubRepo('')
      setJiraProject('')
      setSkillOverrides({})
      setSkillsStatus(null)
      setSddStatuses([])
      setSddResult(null)
      fetchDetectedStatuses('local', '')
    }
    setActiveTab('general')
  }, [editingProject, isProjectModalOpen, settings.specFramework])


  useEffect(() => {
    if (!isProjectModalOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsProjectModalOpen(false)
        setEditingProject(null)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isProjectModalOpen, setIsProjectModalOpen, setEditingProject])

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


  const handleSkillOverrideChange = (skillId: string, customName: string) => {
    setSkillOverrides(prev => ({
      ...prev,
      [skillId]: customName,
    }))
  }

  // Un modèle mal formé désactive l'enregistrement plutôt que de le laisser
  // échouer en silence : l'entrée fautive peut être dans l'onglet Compétences,
  // loin du bouton, et un clic sans effet n'indique rien.
  const modelsAreValid =
    isValidModel(aiModel) && Object.values(aiSkillModels).every(model => isValidModel(model))

  // Une entrée vidée disparaît de la carte : elle signifie « hérite », et non
  // « aucun modèle », ce qui est exactement ce que le serveur normalise.
  const handleSkillModelChange = (skillId: string, model: string) => {
    setAiSkillModels(prev => {
      const next = { ...prev }
      if (model.trim() === '') delete next[skillId]
      else next[skillId] = model
      return next
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || isSubmitting || !modelsAreValid) return

    setIsSubmitting(true)
    try {
      const computedGithubRepo = githubRepo.trim() || extractGithubRepoFromGitUrl(gitRemoteUrl)
      const payload = {
        name: name.trim(),
        slug: slug.trim() || name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
        description: description.trim(),
        icon,
        color,
        isDefault,
        repoPath: repoPath.trim(),
        prCreationStage,
        defaultSkillMode,
        fullChainStopStage,
        trackerColumns,
        stageColumns,
        gitRemoteUrl: gitRemoteUrl.trim(),
        aiProvider: useCustomAgent && aiProvider ? (aiProvider as AIProvider) : undefined,
        aiCommandTemplate: useCustomAgent && aiCommandTemplate.trim() ? aiCommandTemplate.trim() : undefined,
        aiCommandTemplateAutonomous: useCustomAgent && aiCommandAutonomous.trim() ? aiCommandAutonomous.trim() : undefined,
        // Toujours transmis, y compris vide : c'est ainsi qu'on efface une valeur
        // au lieu de conserver silencieusement celle qui est enregistrée.
        aiModel: useCustomAgent ? aiModel.trim() : '',
        aiSkillModels,
        setupProviders,
        specFramework,
        useWorktrees,
        autoSyncEnabled,
        autoSyncIntervalMin,
        issueTracker,
        trackerUrl: trackerUrl.trim(),
        githubRepo: computedGithubRepo,
        githubApiUrl: githubApiUrl.trim(),
        githubToken: githubToken.trim(),
        jiraProject: jiraProject.trim().toUpperCase(),
        issueTypes,
        skillOverrides,
      }

      if (editingProject) {
        await updateProject(editingProject.id, payload)
      } else {
        await createProject(payload)
      }
      setIsProjectModalOpen(false)
      setEditingProject(null)

      // Un projet posé sur un tracker distant sans accès configurés ne ramènera
      // rien : autant le dire maintenant, plutôt qu'après une synchronisation
      // vide. L'écran se ferme sans rien remplir, la configuration pouvant venir
      // plus tard depuis les réglages.
      const needsCredentials =
        issueTracker === 'jira' && !settings.jiraUrl?.trim() && !settings.jiraApiTokenSet
      if (needsCredentials) {
        setIsTrackerSetupOpen(true)
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!editingProject || isDeleting) return
    if (confirm(`Êtes-vous sûr de vouloir supprimer le projet "${editingProject.name}" ? Les tâches associées seront conservées.`)) {
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

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150 select-none">
      <div className="relative w-full max-w-5xl xl:max-w-6xl 2xl:max-w-[1400px] h-[calc(var(--app-h)*0.92)] rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
          <div className="flex items-center gap-2.5">
            <div
              className="w-8 h-8 rounded-xl flex items-center justify-center border"
              style={accentBadgeStyle(color)}
            >
              <Layers size={16} />
            </div>
            <div>
              <h3 className="text-sm font-bold text-[var(--text-primary)]">
                {editingProject ? `Paramètres : ${editingProject.name}` : 'Nouveau Projet'}
              </h3>
              <p className="text-[11px] text-[var(--text-muted)]">
                {editingProject ? `Project settings, AI provider, tracker and skills` : 'Créez un espace dédié avec son propre dépôt Git, agent IA et tracker'}
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
          >
            <X size={18} />
          </button>
        </div>

        {/* Navigation Tabs Bar */}
        <div className="flex items-center gap-1 px-6 pt-1.5 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0 overflow-x-auto">
          {TABS.map(tab => {
            const Icon = tab.icon
            const isSel = activeTab === tab.id
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-3.5 py-2 text-xs font-bold border-b-2 transition-all cursor-pointer whitespace-nowrap ${
                  isSel
                    ? 'border-[var(--accent-color)] accent-text'
                    : 'border-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--border-color)]'
                }`}
              >
                <Icon size={14} className={isSel ? 'text-[var(--accent-color)]' : 'text-[var(--text-muted)]'} />
                <span>{tab.label}</span>
                {tab.id === 'general' && skillsStatus && (
                  <span className={`w-2 h-2 rounded-full ${skillsStatus.isGitRepo ? 'bg-emerald-400' : 'bg-amber-400'}`} />
                )}
                {tab.id === 'agent' && useCustomAgent && (
                  <span className="w-2 h-2 rounded-full bg-[var(--accent-color)]" />
                )}
                {tab.id === 'skills' && skillsStatus && (
                  <span className={`text-[9px] font-mono px-1.5 py-0.2 rounded-full font-bold ${
                    skillsStatus.installedAll ? 'bg-emerald-500/20 text-emerald-300' : 'bg-amber-500/20 text-amber-300'
                  }`}>
                    {(skillsStatus.skills || []).filter(s => s.installed).length}/5
                  </span>
                )}
                {tab.id === 'tracker' && detectedStatuses.length > 0 && (
                  <span className="text-[9px] font-mono px-1.5 py-0.2 rounded-full bg-[var(--accent-light)] accent-text font-bold">
                    {detectedStatuses.length}
                  </span>
                )}
              </button>
            )
          })}
        </div>

        {/* Form Body by Tab */}
        <form onSubmit={handleSubmit} className="p-5 overflow-y-auto space-y-4 flex-1 text-xs">
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
                      Titre du Projet *
                    </label>
                    <input
                      type="text"
                      required
                      value={name}
                      onChange={e => handleNameChange(e.target.value)}
                      placeholder="Ex: Mon Projet, Mobile App, Backend API..."
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Description
                    </label>
                    <input
                      type="text"
                      value={description}
                      onChange={e => setDescription(e.target.value)}
                      placeholder="Ex: Application principale Web et backend Go..."
                      className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Identifiant / Slug
                    </label>
                    <input
                      type="text"
                      value={slug}
                      onChange={e => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9]+/g, '-'))}
                      placeholder="mon-projet"
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
                      Définir comme projet par défaut
                    </label>
                  </div>
                </div>

                {/* Colonne droite : icône et couleur d'accent */}
                <div className="space-y-3.5">
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      Icône
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
                    Dépôt Git
                  </span>
                </div>
                <div>
                  <label htmlFor="gitRemoteUrl" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                    Git remote URL
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
                <p className="text-[10px] text-[var(--text-muted)] leading-relaxed">
                  Local repositories and execution consoles are managed in the desktop agent.
                </p>
              </div>
            </div>
          )}

          {/* ========================================================= */}
          {/* SECTION 3: AGENT IA & CLI (Configuration du moteur/CLI)   */}
          {/* ========================================================= */}
          {activeTab === 'agent' && (
            <div className="space-y-4 animate-in fade-in duration-150">
              {/* Header card */}
              <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)]">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-xl bg-[var(--accent-light)] accent-text flex items-center justify-center shrink-0 border border-[var(--accent-color)]/30">
                      <Bot size={16} />
                    </div>
                    <div>
                      <span className="text-xs font-bold text-[var(--text-primary)] block">
                        Moteur IA & Ligne de Commande pour ce Projet
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] block">
                        Personnalisez le CLI utilisé (AGY, Claude Code, Codex...) et les options de prompt pour ce dépôt.
                      </span>
                    </div>
                  </div>

                  <div className="flex items-center gap-1 bg-[var(--bg-secondary)] p-1 rounded-xl border border-[var(--border-color)]">
                    <button
                      type="button"
                      onClick={() => setUseCustomAgent(false)}
                      className={`px-2.5 py-1 text-[11px] font-bold rounded-lg transition-all cursor-pointer ${
                        !useCustomAgent
                          ? 'bg-[var(--accent-color)] text-white shadow-xs'
                          : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      Hériter du Global
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setUseCustomAgent(true)
                        if (!aiProvider) setAiProvider(settings.aiProvider || 'agy')
                        if (!aiCommandTemplate) setAiCommandTemplate(settings.aiCommandTemplate || '')
                      }}
                      className={`px-2.5 py-1 text-[11px] font-bold rounded-lg transition-all cursor-pointer ${
                        useCustomAgent
                          ? 'bg-[var(--accent-color)] text-white shadow-xs'
                          : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      Spécifique au Projet
                    </button>
                  </div>
                </div>
              </div>

              {!useCustomAgent ? (
                <div className="p-4 rounded-xl bg-[var(--bg-primary)] border border-dashed border-[var(--border-color)] flex items-center gap-3 text-[var(--text-secondary)]">
                  <Info size={16} className="text-[var(--accent-color)] shrink-0" />
                  <div className="text-xs">
                    <span className="font-semibold text-[var(--text-primary)]">Configuration Globale Active : </span>
                    <span className="font-mono text-[var(--accent-color)] font-bold">{settings.aiProvider.toUpperCase()}</span>
                    <span className="text-[var(--text-muted)] block mt-0.5 font-mono text-[11px]">
                      Modèle de commande : {settings.aiCommandTemplate || 'défaut du fournisseur'}
                    </span>
                    <CommandModePreview
                      provider={settings.aiProvider}
                      template={settings.aiCommandTemplate || ''}
                      model={settings.aiModel || ''}
                      autonomousTemplate={settings.aiCommandTemplateAutonomous || ''}
                    />
                  </div>
                </div>
              ) : (
                <div className="space-y-3.5 animate-in fade-in duration-150">
                  {/* Select AI Provider */}
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                      Fournisseur de l'Agent IA
                    </label>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {AI_PROVIDERS.map(p => {
                        const isSel = (aiProvider || settings.aiProvider) === p.id
                        return (
                          <button
                            key={p.id}
                            type="button"
                            onClick={() => {
                              setAiProvider(p.id)
                              // A template written for another CLI cannot serve
                              // this one; a hand-written one is left alone.
                              if (aiCommandTemplate.trim() === '' || COMMAND_PRESETS.some(preset => preset.cmd !== '' && preset.cmd === aiCommandTemplate)) {
                                setAiCommandTemplate(p.defaultCmd)
                                setAiCommandAutonomous('')
                              }
                            }}
                            className={`p-2.5 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-2.5 ${
                              isSel
                                ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-xs'
                                : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                            }`}
                          >
                            <span className="text-lg">{p.icon}</span>
                            <div className="min-w-0 flex-1">
                              <span className={`text-xs font-bold block truncate ${isSel ? 'accent-text' : 'text-[var(--text-primary)]'}`}>
                                {p.label}
                              </span>
                              <span className="text-[10px] text-[var(--text-muted)] block truncate">
                                {p.sub}
                              </span>
                            </div>
                            {isSel && <Check size={14} className="text-[var(--accent-color)] shrink-0 mt-0.5" />}
                          </button>
                        )
                      })}
                    </div>
                  </div>

                  <AIModelField
                    provider={aiProvider || settings.aiProvider}
                    commandTemplate={aiCommandTemplate}
                    value={aiModel}
                    onChange={setAiModel}
                    placeholder={settings.aiModel ? `Hérite du global : ${settings.aiModel}` : 'Défaut du CLI (ex : claude-opus-5)'}
                    label="Modèle du projet"
                  />

                  {/* AI Command Line Template */}
                  <div>
                    <div className="flex items-center justify-between mb-1">
                      <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                        Commande interactive
                      </label>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">
                        Token requis : <code className="text-amber-400 font-bold">{'{prompt}'}</code>
                      </span>
                    </div>
                    <div className="relative">
                      <input
                        type="text"
                        value={aiCommandTemplate}
                        onChange={e => setAiCommandTemplate(e.target.value)}
                        placeholder={`claude --model {model} '{prompt}'`}
                        className="w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Terminal size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>

                    <label className="block mt-2 mb-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                      Commande autonome (headless)
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={aiCommandAutonomous}
                        onChange={e => setAiCommandAutonomous(e.target.value)}
                        placeholder={`claude -p --permission-mode bypassPermissions --model {model} '{prompt}'`}
                        className="w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Terminal size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>

                    {/* Presets */}
                    <div className="flex items-center flex-wrap gap-1.5 mt-2">
                      <span className="text-[10px] text-[var(--text-muted)] mr-1 font-semibold">Presets :</span>
                      {COMMAND_PRESETS.map(pr => (
                        <button
                          key={pr.cmd}
                          type="button"
                          onClick={() => { setAiCommandTemplate(pr.cmd); setAiCommandAutonomous(pr.autonomous) }}
                          className="px-2 py-0.5 rounded-lg text-[10px] font-mono bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent-color)] transition-colors cursor-pointer"
                        >
                          {pr.label}
                        </button>
                      ))}
                    </div>

                    {/* Variable tokens guide */}
                    <div className="p-2.5 rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] mt-2 flex items-center flex-wrap gap-2 text-[10px] text-[var(--text-muted)]">
                      <span className="font-bold text-[var(--text-secondary)]">Variables disponibles :</span>
                      {['{prompt}', '{issueKey}', '{issueTitle}', '{repoPath}', '{branchName}', '{model}', '{mode:AUTONOMOUS|INTERACTIVE}'].map(tag => (
                        <span key={tag} className="font-mono bg-[var(--bg-tertiary)] px-1.5 py-0.5 rounded text-[var(--text-primary)] border border-[var(--border-color)]">
                          {tag}
                        </span>
                      ))}
                    </div>
                    <p className="mt-2 text-[10.5px] text-[var(--text-muted)] leading-relaxed">
                      Champs vides : le fournisseur fournit les deux commandes. La commande
                      autonome laissée vide renvoie les lancements headless sur la commande
                      interactive, qui doit alors porter le
                      marqueur <code className="font-mono">{'{mode:…|…}'}</code>.
                    </p>
                    <CommandModePreview
                      provider={aiProvider || settings.aiProvider}
                      template={aiCommandTemplate}
                      model={aiModel || settings.aiModel || ''}
                      autonomousTemplate={aiCommandAutonomous}
                    />
                  </div>
                </div>
              )}

              {/* Agents the local agent installs skills and the MCP registration for. */}
              <div className="p-3.5 rounded-xl border border-[var(--border-color)]">
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  Agents à configurer
                </label>
                <p className="text-xs text-[var(--text-muted)] mb-3">
                  Les compétences et l'enregistrement MCP sont installés dans la configuration utilisateur de chaque agent coché.
                  L'agent qui exécute les tâches est toujours configuré, qu'il soit coché ou non.
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                  {SETUP_PROVIDERS.map(p => {
                    const checked = setupProviders.includes(p.id)
                    return (
                      <label
                        key={p.id}
                        className={`p-2.5 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-2.5 ${
                          checked
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)] shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/50'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={e =>
                            setSetupProviders(current =>
                              e.target.checked ? [...current, p.id] : current.filter(id => id !== p.id)
                            )
                          }
                          className="mt-0.5"
                        />
                        <span className="text-lg">{p.icon}</span>
                        <div className="min-w-0 flex-1">
                          <span className={`text-xs font-bold block truncate ${checked ? 'accent-text' : 'text-[var(--text-primary)]'}`}>
                            {p.label}
                          </span>
                          <span className="text-[10px] text-[var(--text-muted)] block truncate">{p.sub}</span>
                        </div>
                      </label>
                    )
                  })}
                </div>
              </div>

              {/* Server execution defaults; local agents can override these values. */}
              <div className="p-3.5 rounded-xl border border-[var(--border-color)]">
                <h3>Local agent execution defaults</h3>
                <p className="text-xs text-[var(--text-muted)]">Inherited by local agents unless overridden in the companion app.</p>
                <label className="flex items-center gap-2 mt-3"><input type="checkbox" checked={useWorktrees} onChange={e=>setUseWorktrees(e.target.checked)} />Use a worktree for each task</label>
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
                      Create PR/MR
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] block">
                      Étape du workflow à laquelle l'agent ouvre la Pull/Merge Request en brouillon.
                    </span>
                  </div>
                </div>
                <select
                  id="prCreationStage"
                  value={prCreationStage}
                  onChange={e => setPRCreationStage(e.target.value as 'specified' | 'implemented')}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="implemented">Draft after implementation</option>
                  <option value="specified">Draft after specification</option>
                </select>
              </div>

              <div>
                <label htmlFor="defaultSkillMode" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  Mode d'exécution par défaut
                </label>
                <select
                  id="defaultSkillMode"
                  value={defaultSkillMode}
                  onChange={e => setDefaultSkillMode(e.target.value as SkillMode)}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="interactive">Interactif</option>
                  <option value="autonomous">Autonome (headless)</option>
                  <option value="">Choisir par skill</option>
                </select>
                <p className="mt-1 text-[10px] text-[var(--text-muted)] leading-relaxed">
                  S'applique aux skills qui ne fixent pas leur propre mode : un skill qui
                  fixe le sien l'emporte toujours. « Choisir par skill » n'impose rien au
                  niveau du projet et laisse chaque skill décider ; ceux qui ne décident pas
                  restent interactifs. Une surcharge au lancement l'emporte sur tout, pour
                  ce lancement seulement.
                </p>
              </div>

              <div>
                <label htmlFor="fullChainStopStage" className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1.5">
                  Arrêt de la chaîne complète
                </label>
                <select
                  id="fullChainStopStage"
                  value={fullChainStopStage}
                  onChange={e => setFullChainStopStage(e.target.value as 'implemented' | 'reviewed')}
                  className="w-full px-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  <option value="reviewed">Après la PR (reviewed)</option>
                  <option value="implemented">Avant la PR (implemented)</option>
                </select>
                <p className="mt-1 text-[10px] text-[var(--text-muted)] leading-relaxed">
                  La fusion reste manuelle dans les deux cas.
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
                  Type de Tracker d'Issues
                </label>
                <select
                  value={issueTracker}
                  onChange={e => {
                    const newTrk = e.target.value as IssueTracker
                    setIssueTracker(newTrk)
                    fetchDetectedStatuses(newTrk)
                  }}
                  className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium cursor-pointer"
                >
                  <option value="local">Sectile (Local)</option>
                  <option value="github">GitHub Issues</option>
                </select>
              </div>

              {/* Panneau d'information & fonctionnalités supportées par le tracker */}
              <div className="p-3 rounded-xl bg-[var(--bg-tertiary)]/50 border border-[var(--border-color)] space-y-2.5 text-xs">
                <div className="flex items-center gap-1.5 font-semibold text-[var(--text-primary)]">
                  <Info size={13} className="text-[var(--accent-color)]" />
                  <span>
                    {issueTracker === 'local' && 'Sectile (Stockage Local)'}
                    {issueTracker === 'github' && 'GitHub Issues'}
                    {issueTracker === 'jira' && 'Jira'}
                  </span>
                </div>

                <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">
                  {issueTracker === 'local' &&
                    'Stockage direct en base SQLite locale. Idéal pour travailler hors-ligne avec toutes les capacités des agents et de gestion de branches.'}
                  {issueTracker === 'github' &&
                    'Synchronisation bidirectionnelle via la CLI GitHub. Les statuts du workflow sont reflétés par des labels (#new, #clarified, #specified, etc.) et l’état Open/Closed.'}
                  {issueTracker === 'jira' &&
                    'Intégration avec les projets Jira Software via l’API Atlassian.'}
                </p>

                {/* Grille des fonctionnalités supportées */}
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-1.5 pt-2 border-t border-[var(--border-color)]/50">
                  {issueTracker === 'local' && (
                    <>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Workflow 6 étapes</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Kanban personnalisé</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Branches Git & Diff</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Labels & Sous-tâches</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Priorités & Échéances</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Zéro latence / Offline</span>
                      </div>
                    </>
                  )}

                  {issueTracker === 'github' && (
                    <>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Sync Issues (Titre/Corps)</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Labels d'étape (#new, etc.)</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Ouverture / Clôture</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Assignation de membres</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Pull Requests liées</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-amber-400 font-medium" title="GitHub Issues n'a pas de colonnes de statut natives (uniquement Open/Closed). Sectile utilise les labels d'étapes.">
                        <Info size={11} className="shrink-0" />
                        <span>Statuts via Labels</span>
                      </div>
                    </>
                  )}

                  {issueTracker === 'jira' && (
                    <>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Sync Workitems</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Transitions de Workflow</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Labels & Composants</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Assignations</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Sprints & Macros</span>
                      </div>
                      <div className="flex items-center gap-1 text-[10.5px] text-emerald-400 font-medium">
                        <CheckCircle2 size={11} className="shrink-0" />
                        <span>Commentaires Jira</span>
                      </div>
                    </>
                  )}
                </div>
              </div>

              {/* Team Key & Github Repo inputs */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {issueTracker === 'github' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Dépôt GitHub (owner/repo)
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={githubRepo}
                        onChange={e => {
                          setGithubRepo(e.target.value)
                          fetchDetectedStatuses('github', e.target.value)
                        }}
                        placeholder="owner/nom-du-repo"
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'github' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Instance GitHub (optionnel)
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={githubApiUrl}
                        onChange={e => setGithubApiUrl(e.target.value)}
                        placeholder="Celle de la configuration utilisateur"
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'github' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Jeton GitHub (optionnel)
                    </label>
                    <div className="relative">
                      <input
                        type="password"
                        value={githubToken}
                        onChange={e => setGithubToken(e.target.value)}
                        placeholder={
                          editingProject?.githubTokenSet
                            ? 'Déjà configuré, laissez vide pour le garder'
                            : 'Celui de la configuration utilisateur'
                        }
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                  </div>
                )}

                {issueTracker === 'jira' && (
                  <div>
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Projet Jira (clé)
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
                        placeholder="Ex: PE, ENG, OPS..."
                        className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] font-mono uppercase focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Key size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                    </div>
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      Le projet interrogé par la synchronisation REST. Le site, l'e-mail et le jeton se configurent dans <em>Connecter votre tracker</em>.
                    </span>
                  </div>
                )}

                {issueTracker === 'jira' && (
                  <div className="col-span-2">
                    <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                      Types de tickets importés
                    </label>
                    {isLoadingIssueTypes ? (
                      <span className="text-[10px] text-[var(--text-muted)]">Lecture des types du projet…</span>
                    ) : availableIssueTypes.length === 0 ? (
                      <span className="text-[10px] text-[var(--text-muted)]">
                        Types indisponibles : enregistrez le projet avec sa clé Jira, puis rouvrez cette fiche.
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
                      Rien de sélectionné vaut Task et Story. Un projet dont le tracker n'expose que
                      son propre type (un type maison, par exemple) n'importe aucun ticket tant
                      qu'il n'est pas coché ici.
                    </span>
                  </div>
                )}

                <div className={issueTracker === 'local' ? 'col-span-2' : ''}>
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                    {issueTracker === 'jira' ? 'URL Jira (base)' : 'URL du Tracker'}
                  </label>
                  <div className="relative">
                    <input
                      type="text"
                      value={trackerUrl}
                      onChange={e => setTrackerUrl(e.target.value)}
                      placeholder={issueTracker === 'jira' ? 'https://mon-org.atlassian.net' : 'https://github.com/owner/repository'}
                      className="w-full pl-8 pr-3 py-1.5 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <Globe size={14} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                  </div>
                  {issueTracker === 'jira' && (
                    <span className="text-[9px] text-[var(--text-muted)] mt-1 block">
                      Sert à construire les liens <code className="text-cyan-400">/browse/&lt;KEY&gt;</code> des tickets.
                    </span>
                  )}
                </div>
              </div>

              {/* Section Synchronisation en arrière-plan */}
              <div className="p-3.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-xs font-bold text-[var(--text-primary)] block">
                      Synchronisation en arrière-plan
                    </label>
                    <span className="text-[10px] text-[var(--text-secondary)] block mt-0.5">
                      Génère automatiquement des tâches de synchronisation en file d'attente pour les tickets non terminés de ce projet.
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
                      <span className="text-[var(--text-secondary)] font-medium">Période de synchronisation :</span>
                      <span className="font-mono font-bold text-[var(--accent-color)]">{autoSyncIntervalMin} min</span>
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
                      <span>1 min</span>
                      <span>15 min</span>
                      <span>30 min</span>
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
                  repoPath={repoPath}
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
                  Framework Spec-Driven Design (SDD) pour ce Projet
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
                          Recommandé
                        </span>
                      </div>
                      <span className="text-[10px] text-[var(--text-muted)] leading-relaxed block">
                        CLI <code className="text-cyan-400">specify</code>. Scaffolde <code className="text-cyan-400">.specify/</code> et <code className="text-cyan-400">specs/</code> : spec.md, plan.md, tasks.md et les commandes <code className="text-cyan-400">/speckit.*</code>.
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
                          Spec avant code
                        </span>
                      </div>
                      <span className="text-[10px] text-[var(--text-muted)] leading-relaxed block">
                        CLI <code className="text-cyan-400">openspec</code>. Scaffolde <code className="text-cyan-400">openspec/</code> : propositions de changement, deltas de specs et checklists validés avant implémentation.
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
                    Compétences du Workflow & Surcharge des Noms
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)]">
                    Personnalisez le libellé de chaque compétence
                  </span>
                </div>

                <div className="space-y-2">
                  {WORKFLOW_SKILLS.map(s => {
                    const isInst = (skillsStatus?.skills || []).find(sk => sk.id === s.id)?.installed || false
                    const SkillIcon = s.icon
                    const customValue = skillOverrides[s.id] || ''
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
                                ? (specFramework === 'openspec' ? 'Proposition de changement OpenSpec (deltas de specs, checklist)' : 'Spécification Spec Kit (spec.md, plan.md, tasks.md, BDD Given/When/Then)')
                                : s.desc}
                            </span>
                          </div>
                        </div>

                        {/* Custom Name Override Input Right */}
                        <div className="flex items-center gap-2 flex-1 justify-end">
                          <div className="relative flex-1 max-w-[260px]">
                            <input
                              type="text"
                              value={customValue}
                              onChange={e => handleSkillOverrideChange(s.id, e.target.value)}
                              placeholder={`Surcharge : ${displayDefaultName}`}
                              className="w-full px-2.5 py-1 text-xs rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] font-medium"
                            />
                            {customValue && (
                              <button
                                type="button"
                                onClick={() => handleSkillOverrideChange(s.id, '')}
                                className="absolute right-1.5 top-1.5 text-[var(--text-muted)] hover:text-rose-400 p-0.5"
                                title="Réinitialiser au nom par défaut"
                              >
                                <RotateCcw size={11} />
                              </button>
                            )}
                          </div>

                          <div className="relative w-[150px] shrink-0">
                            <input
                              type="text"
                              value={aiSkillModels[s.id] || ''}
                              onChange={e => handleSkillModelChange(s.id, e.target.value)}
                              placeholder="Modèle hérité"
                              aria-label={`Modèle pour ${displayDefaultName}`}
                              aria-invalid={!isValidModel(aiSkillModels[s.id] || '')}
                              className={`w-full px-2.5 py-1 text-xs font-mono rounded-lg bg-[var(--bg-secondary)] border text-[var(--text-primary)] focus:outline-none ${
                                isValidModel(aiSkillModels[s.id] || '')
                                  ? 'border-[var(--border-color)] focus:border-[var(--accent-color)]'
                                  : 'border-red-500'
                              }`}
                            />
                          </div>

                          <div className="shrink-0 w-20 text-right">
                            {isInst ? (
                              <span className="inline-flex items-center gap-1 text-[10px] font-bold text-emerald-400">
                                <Check size={11} />
                                <span>Installé</span>
                              </span>
                            ) : (
                              <span className="text-[10px] font-medium text-[var(--text-muted)]">
                                Non installé
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

        {/* Modal Footer */}
        <div className="flex items-center justify-between px-6 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
          <div>
            {editingProject && !editingProject.isDefault && (
              <button
                type="button"
                onClick={handleDelete}
                disabled={isDeleting}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold text-rose-400 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 transition-colors cursor-pointer"
                title="Supprimer ce projet"
              >
                <Trash2 size={13} />
                <span>{isDeleting ? 'Suppression...' : 'Supprimer'}</span>
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
              disabled={isSubmitting || !name.trim() || !modelsAreValid}
              className="px-5 py-2 rounded-xl text-xs font-bold text-white accent-bg shadow-md hover:opacity-90 active:scale-95 flex items-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer"
            >
              <Save size={14} />
              <span>{isSubmitting ? 'Enregistrement...' : editingProject ? 'Mettre à jour' : 'Créer le projet'}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
