import React, { useState, useEffect } from 'react'
import {
  X,
  Palette,
  Sun,
  Moon,
  Globe,
  Check,
  Sliders,
  PanelRight,
  Square,
  Bot,
  Terminal,
  FileCode,
  HelpCircle,
  Flame,
  ShieldCheck,
  CheckCircle2,
  Info,
  KeyRound,
  UserRound,
  Monitor,
  Workflow,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { LocalAgentSetup } from './LocalAgentSetup'
import { ApiKeysPanel } from './ApiKeys'
import { TrackerCredentialsTab } from './TrackerCredentialsTab'
import { SignInStatus } from './SignInStatus'
import { MCPEngineConfig } from './MCPEngineConfig'
import type { Theme, Language, Density, ViewMode, DetailMode, AIProvider, SpecFramework } from '../types'
import { AIModelField } from './AIModelField'
import { ProviderModelsField } from './ProviderModelsField'
import { CommandModePreview } from './CommandModePreview'
import { commandPreview } from '../lib/commandTemplate'
import { isValidModel } from '../lib/aiModels'

type SettingsTab = 'account' | 'appearance' | 'trackers' | 'aiEngine' | 'sdd' | 'workstations'

// A provider with no template runs the command lines the agent attests for each
// execution mode, so selecting one clears the field rather than pinning a single
// mode. Only a custom CLI has nothing to fall back to and needs a template.
const AI_PROVIDERS: { id: AIProvider; label: string; sub: string; defaultCmd: string; icon: string }[] = [
  { id: 'agy', label: 'Antigravity (agy)', sub: 'Google Deepmind AGY CLI', defaultCmd: '', icon: '🚀' },
  { id: 'claude', label: 'Claude Code (claude)', sub: 'Anthropic Claude Code CLI', defaultCmd: '', icon: '🟣' },
  { id: 'codex', label: 'Codex CLI', sub: 'OpenAI Codex CLI', defaultCmd: '', icon: '🤖' },
  { id: 'custom', label: 'CLI Personnalisé', sub: 'Binaire ou script custom', defaultCmd: `/path/to/custom-cli {mode:-p|-i} '{prompt}'`, icon: '⚙️' },
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

export const ProfileModal: React.FC = () => {
  const {
    isProfileOpen,
    setIsProfileOpen,
    settings,
    updateSettings,
    t,
  } = useApp()

  const [activeTab, setActiveTab] = useState<SettingsTab>('account')

  // Appearance
  const [theme, setTheme] = useState<Theme>(settings.theme)
  const [language, setLanguage] = useState<Language>(settings.language)
  const [density, setDensity] = useState<Density>(settings.density)
  const [defaultView, setDefaultView] = useState<ViewMode>(settings.defaultView)
  const [detailMode, setDetailMode] = useState<DetailMode>(settings.detailMode || 'panel')

  // Agentic AI & CLI Configuration
  const [aiProvider, setAiProvider] = useState<AIProvider>(settings.aiProvider || 'agy')
  const [aiCommandTemplate, setAiCommandTemplate] = useState(settings.aiCommandTemplate || '')
  const [aiCommandAutonomous, setAiCommandAutonomous] = useState(settings.aiCommandTemplateAutonomous || '')
  const [aiModel, setAiModel] = useState(settings.aiModel || '')
  const [aiProviderModels, setAiProviderModels] = useState<Record<string, string[]>>(settings.aiProviderModels || {})
  const [specFramework, setSpecFramework] = useState<SpecFramework>(settings.specFramework || 'speckit')

  // Skill Prompts
  const [promptClarify, setPromptClarify] = useState(settings.promptClarify || '')
  const [promptSpecify, setPromptSpecify] = useState(settings.promptSpecify || '')
  const [promptImplement, setPromptImplement] = useState(settings.promptImplement || '')
  const [promptAdjust, setPromptAdjust] = useState(settings.promptAdjust || '')
  const [promptHandoff, setPromptHandoff] = useState(settings.promptHandoff || '')

  useEffect(() => {
    if (isProfileOpen) {
      setTheme(settings.theme)
      setLanguage(settings.language)
      setDensity(settings.density)
      setDefaultView(settings.defaultView)
      setDetailMode(settings.detailMode || 'panel')
      setAiProvider(settings.aiProvider || 'agy')
      setAiCommandTemplate(settings.aiCommandTemplate || '')
      setAiCommandAutonomous(settings.aiCommandTemplateAutonomous || '')
      setAiModel(settings.aiModel || '')
      setAiProviderModels(settings.aiProviderModels || {})
      setSpecFramework(settings.specFramework || 'speckit')
      setPromptClarify(settings.promptClarify || '')
      setPromptSpecify(settings.promptSpecify || '')
      setPromptImplement(settings.promptImplement || '')
      setPromptAdjust(settings.promptAdjust || '')
      setPromptHandoff(settings.promptHandoff || '')
    }
  }, [isProfileOpen, settings])

  useEffect(() => {
    if (!isProfileOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsProfileOpen(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isProfileOpen, setIsProfileOpen])

  if (!isProfileOpen) return null


  const densities: { id: Density; label: string; desc: string }[] = [
    { id: 'compact', label: 'Compact', desc: '13px font, padding réduit' },
    { id: 'standard', label: 'Standard', desc: '14px font, équilibre optimal' },
    { id: 'comfortable', label: 'Confortable', desc: '15px font, grands espacements' },
  ]

  const handleProviderSelect = (provider: typeof AI_PROVIDERS[0]) => {
    setAiProvider(provider.id)
    // A template written for another CLI cannot serve this one, and the empty
    // value is the right default: it hands both modes back to the provider.
    if (aiCommandTemplate.trim() === '' || AI_PROVIDERS.some(p => p.defaultCmd !== '' && p.defaultCmd === aiCommandTemplate)) {
      setAiCommandTemplate(provider.defaultCmd)
      setAiCommandAutonomous('')
    }
  }

  // Un modèle mal formé désactive l'enregistrement : le bouton est en pied de
  // modale, loin du champ, et un clic sans effet n'indique rien.
  const modelIsValid =
    isValidModel(aiModel) && Object.values(aiProviderModels).every(list => list.every(model => isValidModel(model)))

  const handleSave = async () => {
    if (!modelIsValid) return
    await updateSettings({
      userName: settings.userName,
      userEmail: settings.userEmail,
      theme,
      language,
      density,
      defaultView,
      detailMode,
      aiProvider,
      aiCommandTemplate: aiCommandTemplate.trim(),
      aiCommandTemplateAutonomous: aiCommandAutonomous.trim(),
      aiModel: aiModel.trim(),
      aiProviderModels,
      specFramework,
      promptClarify: promptClarify.trim(),
      promptSpecify: promptSpecify.trim(),
      promptImplement: promptImplement.trim(),
      promptAdjust: promptAdjust.trim(),
      promptHandoff: promptHandoff.trim(),
      promptCreatePr: '',
    })
    setIsProfileOpen(false)
  }

  const tabs: { id: SettingsTab; label: string; icon: React.ReactNode; iconColor: string }[] = [
    { id: 'account', label: 'Account', icon: <UserRound size={15} />, iconColor: 'text-[var(--text-secondary)]' },
    { id: 'appearance', label: 'Apparence', icon: <Palette size={15} />, iconColor: 'text-purple-400' },
    { id: 'trackers', label: 'Trackers', icon: <KeyRound size={15} />, iconColor: 'text-emerald-400' },
    { id: 'aiEngine', label: 'Moteur Agentic IA', icon: <Bot size={15} />, iconColor: 'text-indigo-400' },
    { id: 'sdd', label: 'Compétences & SDD', icon: <Workflow size={15} />, iconColor: 'text-blue-400' },
    { id: 'workstations', label: 'Workstations & Agent', icon: <Monitor size={15} />, iconColor: 'text-cyan-400' },
  ]

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200"
      onClick={e => {
        if (e.target === e.currentTarget) setIsProfileOpen(false)
      }}
    >
      <div
        className="relative w-[960px] h-[650px] max-w-[calc(var(--app-w)-32px)] max-h-[calc(var(--app-h)-32px)] rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-150"
        role="dialog"
        aria-modal="true"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl accent-bg text-white flex items-center justify-center shadow">
              <Sliders size={16} />
            </div>
            <div>
              <h3 className="text-sm font-bold text-[var(--text-primary)]">
                {t.profileModal.title}
              </h3>
              <p className="text-[11px] text-[var(--text-muted)]">
                Compte, apparence et configuration des outils
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setIsProfileOpen(false)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={17} />
          </button>
        </div>

        {/* Modal 2-Column Body: Left Sidebar Tabs + Right Content Area */}
        <div className="flex flex-1 min-h-0 overflow-hidden">
          {/* Left Sidebar Navigation */}
          <div className="w-56 shrink-0 border-r border-[var(--border-color)] bg-[var(--bg-tertiary)]/25 p-3 flex flex-col justify-between overflow-y-auto">
            <nav className="space-y-1">
              {tabs.map(tab => {
                const isActive = activeTab === tab.id
                return (
                  <button
                    key={tab.id}
                    type="button"
                    onClick={() => setActiveTab(tab.id)}
                    className={`w-full flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-xs font-semibold transition-all cursor-pointer text-left ${
                      isActive
                        ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                        : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]/60'
                    }`}
                  >
                    <span className={isActive ? 'accent-text' : tab.iconColor}>
                      {tab.icon}
                    </span>
                    <span className="truncate">{tab.label}</span>
                  </button>
                )
              })}
            </nav>

            <div className="pt-3 border-t border-[var(--border-color)]/60 px-2 text-[10px] text-[var(--text-muted)]">
              <span>Sectile Preferences</span>
            </div>
          </div>

          {/* Right Scrollable Content Pane */}
          <div className="flex-1 min-w-0 p-6 overflow-y-auto space-y-6 text-xs">
            {/* TAB 1: ACCOUNT */}
            {activeTab === 'account' && (
              <div className="space-y-6 animate-in fade-in duration-150">
                <SignInStatus />
              </div>
            )}

          {/* TAB 2: APPEARANCE */}
          {activeTab === 'appearance' && (
            <div className="space-y-6 animate-in fade-in duration-150">

              {/* Theme & Language */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    {t.profileModal.theme}
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setTheme('dark')}
                      className={`flex items-center justify-center gap-2 py-2 rounded-xl border font-medium transition-all cursor-pointer ${
                        theme === 'dark'
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                          : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      <Moon size={14} />
                      <span>{t.profileModal.themes.dark}</span>
                    </button>

                    <button
                      type="button"
                      onClick={() => setTheme('light')}
                      className={`flex items-center justify-center gap-2 py-2 rounded-xl border font-medium transition-all cursor-pointer ${
                        theme === 'light'
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                          : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      <Sun size={14} />
                      <span>{t.profileModal.themes.light}</span>
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    {t.profileModal.language}
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setLanguage('fr')}
                      className={`flex items-center justify-center gap-1.5 py-2 rounded-xl border font-medium transition-all cursor-pointer ${
                        language === 'fr'
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                          : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      <Globe size={14} />
                      <span>FR</span>
                    </button>

                    <button
                      type="button"
                      onClick={() => setLanguage('en')}
                      className={`flex items-center justify-center gap-1.5 py-2 rounded-xl border font-medium transition-all cursor-pointer ${
                        language === 'en'
                          ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                          : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                      }`}
                    >
                      <Globe size={14} />
                      <span>EN</span>
                    </button>
                  </div>
                </div>
              </div>

              {/* Story Detail Mode (Right Panel vs Modal) */}
              <div className="space-y-2">
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                  {t.profileModal.detailMode}
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setDetailMode('panel')}
                    className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${
                      detailMode === 'panel'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-indigo-400 border border-[var(--border-color)] shrink-0">
                      <PanelRight size={18} />
                    </div>
                    <div className="truncate">
                      <div className="font-bold text-xs">{t.profileModal.detailModes.panel}</div>
                      <div className="text-[10px] text-[var(--text-muted)] opacity-80">Glissement latéral à droite</div>
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setDetailMode('modal')}
                    className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${
                      detailMode === 'modal'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-purple-400 border border-[var(--border-color)] shrink-0">
                      <Square size={18} />
                    </div>
                    <div className="truncate">
                      <div className="font-bold text-xs">{t.profileModal.detailModes.modal}</div>
                      <div className="text-[10px] text-[var(--text-muted)] opacity-80">Boîte de dialogue au centre</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Density */}
              <div className="space-y-2">
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                  {t.profileModal.density}
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                  {densities.map(d => {
                    const isSelected = density === d.id
                    return (
                      <button
                        key={d.id}
                        type="button"
                        onClick={() => setDensity(d.id)}
                        className={`p-2.5 rounded-xl border text-left transition-all cursor-pointer ${
                          isSelected
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                      >
                        <div className="font-bold text-xs">{d.label}</div>
                        <div className="text-[10px] opacity-75 mt-0.5">{d.desc}</div>
                      </button>
                    )
                  })}
                </div>
              </div>

              {/* Default Code Editor */}


              {/* Default External Terminal */}


            </div>
          )}

          {/* TAB 2: TRACKER CREDENTIALS, one activatable zone per tracker */}
          {activeTab === 'trackers' && <TrackerCredentialsTab />}

          {/* TAB 4: MOTEUR AGENTIC IA */}
          {activeTab === 'aiEngine' && (
            <div className="space-y-6 animate-in fade-in duration-150">
              <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                Configurez le moteur d'intelligence artificielle par défaut, les modèles et les commandes CLI d'exécution des skills.
              </p>

              {/* Agentic CLI Provider Selection */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <Bot size={14} className="text-indigo-400" />
                    <span>Moteur Agentic IA par défaut</span>
                  </label>
                  <span className="text-[10px] text-indigo-400 font-mono font-bold">
                    {aiProvider.toUpperCase()}
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  {AI_PROVIDERS.map(p => {
                    const isSelected = aiProvider === p.id
                    return (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => handleProviderSelect(p)}
                        className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-2.5 ${
                          isSelected
                            ? 'bg-indigo-500/15 border-indigo-500 text-white ring-2 ring-indigo-500/30 shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                      >
                        <span className="text-base">{p.icon}</span>
                        <div className="truncate flex-1">
                          <div className="font-bold text-xs flex items-center justify-between">
                            <span>{p.label}</span>
                            {isSelected && <Check size={14} className="text-indigo-400" />}
                          </div>
                          <div className="text-[10px] text-[var(--text-muted)] mt-0.5">{p.sub}</div>
                          <div className="text-[9px] font-mono text-indigo-300/80 truncate mt-1">
                            {p.defaultCmd || commandPreview(p.id, '', aiModel, false).command}
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </div>

              <AIModelField
                provider={aiProvider}
                commandTemplate={aiCommandTemplate}
                value={aiModel}
                onChange={setAiModel}
                placeholder="Défaut du CLI (ex : claude-opus-5)"
                label="Modèle par défaut"
              />

              <ProviderModelsField
                provider={aiProvider}
                providers={AI_PROVIDERS.map(p => p.id)}
                value={aiProviderModels}
                onChange={setAiProviderModels}
              />

              {/* Command Line Template Configuration */}
              <div className="space-y-2.5 p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)]">
                <div className="flex items-center justify-between">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-primary)] flex items-center gap-1.5">
                    <Terminal size={13} className="text-indigo-400" />
                    <span>Modèle de ligne de commande d'exécution (CLI)</span>
                  </label>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono">Template bash / zsh</span>
                </div>

                <div className="space-y-1.5">
                  <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    Commande interactive
                  </label>
                  <input
                    type="text"
                    value={aiCommandTemplate}
                    onChange={e => setAiCommandTemplate(e.target.value)}
                    placeholder={`Ex : claude --model {model} '{prompt}'`}
                    className="w-full px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] transition-all"
                  />
                  <label className="block pt-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    Commande autonome (headless)
                  </label>
                  <input
                    type="text"
                    value={aiCommandAutonomous}
                    onChange={e => setAiCommandAutonomous(e.target.value)}
                    placeholder={`Ex : claude -p --permission-mode bypassPermissions --model {model} '{prompt}'`}
                    className="w-full px-3 py-2 text-xs font-mono rounded-xl bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] transition-all"
                  />
                  <div className="flex flex-wrap items-center gap-1 text-[10.5px] text-[var(--text-muted)] leading-relaxed pt-1">
                    <Info size={12} className="text-indigo-400 shrink-0" />
                    <span className="font-semibold text-[var(--text-secondary)]">Variables disponibles :</span>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{prompt}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{issueKey}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{issueTitle}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{branchName}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{repoPath}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{model}'}</code>
                    <code className="bg-[var(--bg-primary)] text-indigo-400 border border-[var(--border-color)] px-1 py-0.5 rounded text-[9.5px] font-mono">{'{mode:AUTONOMOUS|INTERACTIVE}'}</code>
                  </div>
                  <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">
                    Deux champs vides donnent les commandes attestées du fournisseur pour les
                    deux modes. La commande autonome sert les lancements headless ; laissée
                    vide, c'est la commande interactive qui les sert, et elle doit alors
                    porter le marqueur <code className="font-mono">{'{mode:…|…}'}</code> pour
                    dire quels mots appartiennent à quel mode.
                  </p>
                  <CommandModePreview
                    provider={aiProvider}
                    template={aiCommandTemplate}
                    model={aiModel}
                    autonomousTemplate={aiCommandAutonomous}
                  />
                </div>

                {/* Fast Preset buttons */}
                <div className="pt-2 border-t border-[var(--border-color)]">
                  <div className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider font-bold mb-1.5">
                    Modèles de commande rapides :
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {COMMAND_PRESETS.map(preset => (
                      <button
                        key={preset.label}
                        type="button"
                        onClick={() => { setAiCommandTemplate(preset.cmd); setAiCommandAutonomous(preset.autonomous) }}
                        className="px-2 py-1 bg-[var(--bg-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent-color)] text-[10.5px] rounded-lg font-mono transition-colors cursor-pointer"
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Direct MCP Configuration without local agent */}
              <MCPEngineConfig
                selectedProvider={aiProvider}
                onNavigateToWorkstations={() => setActiveTab('workstations')}
              />
            </div>
          )}

          {/* TAB: SPEC-DRIVEN DESIGN (SDD) */}
          {activeTab === 'sdd' && (
            <div className="space-y-6 animate-in fade-in duration-150">
              {/* Header Description */}
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <Workflow size={18} className="text-blue-400" />
                  <h4 className="text-sm font-bold text-[var(--text-primary)]">
                    Framework Spec-Driven Design (SDD)
                  </h4>
                  <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[9.5px] font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30">
                    Contract-First
                  </span>
                </div>
                <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                  Le Spec-Driven Design garantit qu'une spécification claire, structurée et vérifiable est rédigée et validée avant toute génération de code par les agents d'IA.
                </p>
              </div>

              {/* Framework Choice */}
              <div className="space-y-2">
                <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center justify-between">
                  <span>Framework par défaut du projet</span>
                  <span className="text-[10px] text-blue-400 font-mono font-bold">
                    {specFramework === 'openspec' ? 'OpenSpec' : 'GitHub Spec Kit'}
                  </span>
                </label>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {/* GitHub Spec Kit Card */}
                  <button
                    type="button"
                    onClick={() => setSpecFramework('speckit')}
                    className={`p-4 rounded-xl border text-left transition-all cursor-pointer flex flex-col justify-between gap-3 ${
                      specFramework === 'speckit'
                        ? 'bg-blue-500/15 border-blue-500 text-white ring-2 ring-blue-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className="text-xl">📑</span>
                        <div>
                          <div className="font-bold text-xs text-[var(--text-primary)]">
                            GitHub Spec Kit
                          </div>
                          <span className="text-[10px] font-mono text-blue-400">
                            CLI specify
                          </span>
                        </div>
                      </div>
                      {specFramework === 'speckit' && (
                        <div className="w-5 h-5 rounded-full bg-blue-500/20 text-blue-400 flex items-center justify-center">
                          <Check size={13} />
                        </div>
                      )}
                    </div>

                    <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                      Convention standard GitHub : structure modulaire dans <code className="px-1 py-0.5 rounded bg-[var(--bg-secondary)] font-mono text-[10px]">.specify/</code> et <code className="px-1 py-0.5 rounded bg-[var(--bg-secondary)] font-mono text-[10px]">specs/</code> (<code className="font-mono text-[10px]">spec.md</code>, <code className="font-mono text-[10px]">plan.md</code>, <code className="font-mono text-[10px]">tasks.md</code>).
                    </p>

                    <div className="pt-2 border-t border-[var(--border-color)]/60 flex items-center justify-between text-[10px] text-[var(--text-muted)]">
                      <span>Commandes : /specify-issue, /code-issue</span>
                      <span className="px-1.5 py-0.5 rounded bg-[var(--bg-secondary)] font-mono">.specify/</span>
                    </div>
                  </button>

                  {/* OpenSpec Card */}
                  <button
                    type="button"
                    onClick={() => setSpecFramework('openspec')}
                    className={`p-4 rounded-xl border text-left transition-all cursor-pointer flex flex-col justify-between gap-3 ${
                      specFramework === 'openspec'
                        ? 'bg-emerald-500/15 border-emerald-500 text-white ring-2 ring-emerald-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className="text-xl">🚩</span>
                        <div>
                          <div className="font-bold text-xs text-[var(--text-primary)]">
                            OpenSpec
                          </div>
                          <span className="text-[10px] font-mono text-emerald-400">
                            CLI openspec
                          </span>
                        </div>
                      </div>
                      {specFramework === 'openspec' && (
                        <div className="w-5 h-5 rounded-full bg-emerald-500/20 text-emerald-400 flex items-center justify-center">
                          <Check size={13} />
                        </div>
                      )}
                    </div>

                    <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                      Spécification formelle par deltas et exigences vérifiables. Les propositions de changements sont validées et revues avant l'écriture de code.
                    </p>

                    <div className="pt-2 border-t border-[var(--border-color)]/60 flex items-center justify-between text-[10px] text-[var(--text-muted)]">
                      <span>Commandes : openspec propose, validate</span>
                      <span className="px-1.5 py-0.5 rounded bg-[var(--bg-secondary)] font-mono">openspec/</span>
                    </div>
                  </button>
                </div>
              </div>

              {/* SDD Pipeline Lifecycle in Sectile */}
              <div className="p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Workflow size={15} className="text-blue-400" />
                    <span className="text-xs font-bold text-[var(--text-primary)]">
                      Cycle de vie SDD dans Sectile
                    </span>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      document.getElementById('skill-prompts-section')?.scrollIntoView({ behavior: 'smooth' })
                    }}
                    className="text-[10.5px] text-indigo-400 hover:text-indigo-300 font-semibold cursor-pointer flex items-center gap-1"
                  >
                    <span>Personnaliser les prompts</span>
                    <span>↓</span>
                  </button>
                </div>

                <div className="grid grid-cols-2 sm:grid-cols-5 gap-2 text-center text-[10.5px]">
                  <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                    <div className="font-bold text-sky-400">1. Clarifier</div>
                    <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/clarify-issue</div>
                  </div>
                  <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                    <div className="font-bold text-blue-400">2. Spécifier</div>
                    <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/specify-issue</div>
                  </div>
                  <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                    <div className="font-bold text-indigo-400">3. Coder</div>
                    <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/code-issue</div>
                  </div>
                  <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                    <div className="font-bold text-purple-400">4. Ajuster</div>
                    <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/adjust-issue</div>
                  </div>
                  <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                    <div className="font-bold text-emerald-400">5. Clôturer</div>
                    <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/handoff-issue</div>
                  </div>
                </div>
              </div>

              {/* SECTION: PERSONNALISATION DES PROMPTS DES SKILLS */}
              <div id="skill-prompts-section" className="pt-4 border-t border-[var(--border-color)]/60 space-y-4">
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <FileCode size={16} className="text-amber-400" />
                    <h4 className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
                      Personnalisation des Prompts par Compétence
                    </h4>
                  </div>
                  <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">
                    Personnalisez les invites (prompts) envoyées au CLI Agentic pour chaque étape du workflow SDD. Si laissé vide, les invites par défaut sont utilisées.
                  </p>
                </div>

                {/* Clarify Prompt */}
                <div className="space-y-1.5">
                  <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-amber-400">
                      <HelpCircle size={13} />
                      <span>Prompt de Cadrage (/clarify-issue)</span>
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] font-mono">Défaut : /clarify-issue {'{issueKey}'} tracked on {'{tracker}'} in {'{repo}'}</span>
                  </label>
                  <textarea
                    value={promptClarify}
                    onChange={e => setPromptClarify(e.target.value)}
                    rows={3}
                    placeholder="/clarify-issue {issueKey} tracked on {tracker} in {repo}"
                    className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-amber-500 transition-all resize-y"
                  />
                </div>

                {/* Specify Prompt */}
                <div className="space-y-1.5">
                  <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-blue-400">
                      <FileCode size={13} />
                      <span>Prompt de Spécification (/specify-issue)</span>
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] font-mono">Spécification Spec Kit / OpenSpec</span>
                  </label>
                  <textarea
                    value={promptSpecify}
                    onChange={e => setPromptSpecify(e.target.value)}
                    rows={3}
                    placeholder='Tu es le Product Owner pour {issueKey}. Rédige la spécification selon le framework SDD configuré...'
                    className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-blue-500 transition-all resize-y"
                  />
                </div>

                {/* Implement Prompt */}
                <div className="space-y-1.5">
                  <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-indigo-400">
                      <Flame size={13} />
                      <span>Prompt d'Implémentation (/code-issue)</span>
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] font-mono">Développement & Tests</span>
                  </label>
                  <textarea
                    value={promptImplement}
                    onChange={e => setPromptImplement(e.target.value)}
                    rows={3}
                    placeholder='Tu es le développeur senior pour {issueKey}. Implémente le code dans {repoPath}...'
                    className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-indigo-500 transition-all resize-y"
                  />
                </div>

                {/* Adjust Prompt */}
                <div className="space-y-1.5">
                  <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-purple-400">
                      <ShieldCheck size={13} />
                      <span>Prompt d'Ajustement & Revue (/adjust-issue)</span>
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] font-mono">Revue de code & màj PR</span>
                  </label>
                  <textarea
                    value={promptAdjust}
                    onChange={e => setPromptAdjust(e.target.value)}
                    rows={3}
                    placeholder='Tu es le reviewer senior pour {issueKey}. Revois les changements, applique les correctifs et mets à jour la PR...'
                    className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-all resize-y"
                  />
                </div>

                {/* Handoff Prompt */}
                <div className="space-y-1.5">
                  <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-emerald-400">
                      <CheckCircle2 size={13} />
                      <span>Prompt de Clôture & Handoff (/handoff-issue)</span>
                    </span>
                    <span className="text-[10px] text-[var(--text-muted)] font-mono">Documentation & Nettoyage local</span>
                  </label>
                  <textarea
                    value={promptHandoff}
                    onChange={e => setPromptHandoff(e.target.value)}
                    rows={3}
                    placeholder='Tu es responsable de la clôture pour {issueKey}. Vérifie la fusion, rédige le rapport de handoff et nettoie le worktree...'
                    className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-emerald-500 transition-all resize-y"
                  />
                </div>
              </div>
            </div>
          )}

          {/* TAB: WORKSTATIONS & AGENT */}
          {activeTab === 'workstations' && (
            <div className="space-y-6 animate-in fade-in duration-150">
              <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                Gérez vos machines de développement appairées, les clés de signature des agents et la configuration de l'agent local.
              </p>
              <ApiKeysPanel />
              <LocalAgentSetup />
            </div>
          )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2.5 px-6 py-4 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 shrink-0">
          <button
            type="button"
            onClick={() => setIsProfileOpen(false)}
            className="px-4 py-2 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            {t.taskModal.cancel}
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={!modelIsValid}
            className="px-5 py-2 rounded-xl text-xs font-semibold text-white accent-bg shadow hover:opacity-90 active:scale-95 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {t.profileModal.save}
          </button>
        </div>
      </div>
    </div>
  )
}
