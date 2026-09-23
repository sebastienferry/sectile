import React, { useState, useEffect, useCallback } from 'react'
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
  FileCode,
  HelpCircle,
  Flame,
  ShieldCheck,
  CheckCircle2,
  KeyRound,
  UserRound,
  Monitor,
  Workflow,
  Kanban,
  List,
  ZoomIn,
} from 'lucide-react'
import { useApp, UI_SCALE_OPTIONS } from '../context/AppContext'
import { SectileDesktopPanel, HeadlessCliAgentPanel } from './LocalAgentSetup'
import { WorkstationsPanel } from './ApiKeys'
import { TrackerCredentialsTab } from './TrackerCredentialsTab'
import { SignInStatus } from './SignInStatus'
import { MCPEngineConfig } from './MCPEngineConfig'
import { Antigravity, Claude, OpenAI } from './icons'
import type { Theme, Language, Density, ViewMode, DetailMode, AIProvider, SpecFramework } from '../types'
import { AIModelField } from './AIModelField'
import { ProviderModelsField } from './ProviderModelsField'
import { isValidModel, providerModels } from '../lib/aiModels'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'

type SettingsTab = 'account' | 'appearance' | 'trackers' | 'aiEngine' | 'sdd' | 'workstations'

const AI_PROVIDERS: { id: AIProvider; label: string; sub: string; icon: React.ReactNode }[] = [
  { id: 'agy', label: 'Antigravity', sub: 'Google Deepmind AGY CLI', icon: <Antigravity size={16} /> },
  { id: 'claude', label: 'Claude', sub: 'Anthropic Claude Code CLI', icon: <Claude size={16} /> },
  { id: 'codex', label: 'Codex', sub: 'OpenAI Codex CLI', icon: <OpenAI size={16} /> },
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
  const [uiScale, setUiScale] = useState<number>(settings.uiScale || 100)

  // Agentic AI & Model Configuration
  const [aiProvider, setAiProvider] = useState<AIProvider>(settings.aiProvider || 'agy')
  const [aiModel, setAiModel] = useState(settings.aiModel || '')
  const [aiProviderModels, setAiProviderModels] = useState<Record<string, string[]>>(settings.aiProviderModels || {})
  const [specFramework, setSpecFramework] = useState<SpecFramework>(settings.specFramework || 'speckit')

  // Skill Prompts
  const [promptClarify, setPromptClarify] = useState(settings.promptClarify || '')
  const [promptSpecify, setPromptSpecify] = useState(settings.promptSpecify || '')
  const [promptImplement, setPromptImplement] = useState(settings.promptImplement || '')
  const [promptAdjust, setPromptAdjust] = useState(settings.promptAdjust || '')
  const [promptHandoff, setPromptHandoff] = useState(settings.promptHandoff || '')
  const [devicesVersion, setDevicesVersion] = useState(0)

  useEffect(() => {
    if (isProfileOpen) {
      setTheme(settings.theme)
      setLanguage(settings.language)
      setDensity(settings.density)
      setDefaultView(settings.defaultView)
      setDetailMode(settings.detailMode || 'panel')
      setUiScale(settings.uiScale || 100)
      setAiProvider(settings.aiProvider || 'agy')
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

  const handleClose = useCallback(() => setIsProfileOpen(false), [setIsProfileOpen])
  const backdrop = useBackdropDismiss(handleClose)

  useEffect(() => {
    if (!isProfileOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isProfileOpen, handleClose])

  if (!isProfileOpen) return null

  const densities: { id: Density; label: string; desc: string }[] = [
    { id: 'compact', label: language === 'fr' ? 'Compact' : 'Compact', desc: t.profileModal.densityDesc?.compact || '13px font, padding réduit' },
    { id: 'standard', label: language === 'fr' ? 'Standard' : 'Standard', desc: t.profileModal.densityDesc?.standard || '14px font, équilibre optimal' },
    { id: 'comfortable', label: language === 'fr' ? 'Confortable' : 'Comfortable', desc: t.profileModal.densityDesc?.comfortable || '15px font, grands espacements' },
  ]

  const handleProviderSelect = (provider: typeof AI_PROVIDERS[0]) => {
    setAiProvider(provider.id)
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
      uiScale,
      aiProvider,
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
    { id: 'account', label: t.profileModal.tabs.account, icon: <UserRound size={15} />, iconColor: 'text-[var(--text-secondary)]' },
    { id: 'appearance', label: t.profileModal.tabs.appearance, icon: <Palette size={15} />, iconColor: 'text-purple-400' },
    { id: 'trackers', label: t.profileModal.tabs.trackers, icon: <KeyRound size={15} />, iconColor: 'text-emerald-400' },
    { id: 'aiEngine', label: t.profileModal.tabs.aiEngine, icon: <Bot size={15} />, iconColor: 'text-indigo-400' },
    { id: 'sdd', label: t.profileModal.tabs.sdd, icon: <Workflow size={15} />, iconColor: 'text-blue-400' },
    { id: 'workstations', label: t.profileModal.tabs.workstations, icon: <Monitor size={15} />, iconColor: 'text-cyan-400' },
  ]

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200"
      {...backdrop}
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
                {t.profileModal.subtitle}
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
                    className={`w-full flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-xs font-semibold transition-all cursor-pointer text-left ${isActive
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
                        className={`flex items-center justify-center gap-2 py-2 rounded-xl border font-medium transition-all cursor-pointer ${theme === 'dark'
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
                        className={`flex items-center justify-center gap-2 py-2 rounded-xl border font-medium transition-all cursor-pointer ${theme === 'light'
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
                        className={`flex items-center justify-center gap-1.5 py-2 rounded-xl border font-medium transition-all cursor-pointer ${language === 'fr'
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
                        className={`flex items-center justify-center gap-1.5 py-2 rounded-xl border font-medium transition-all cursor-pointer ${language === 'en'
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

                {/* Default View (Board vs List) */}
                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    {t.profileModal.defaultView}
                  </label>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setDefaultView('board')}
                      className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${defaultView === 'board'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-sky-400 border border-[var(--border-color)] shrink-0">
                        <Kanban size={18} />
                      </div>
                      <div className="truncate">
                        <div className="font-bold text-xs">{t.profileModal.defaultViews?.board || 'Tableau (Kanban)'}</div>
                        <div className="text-[10px] text-[var(--text-muted)] opacity-80">{t.nav.board}</div>
                      </div>
                    </button>

                    <button
                      type="button"
                      onClick={() => setDefaultView('list')}
                      className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${defaultView === 'list'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-emerald-400 border border-[var(--border-color)] shrink-0">
                        <List size={18} />
                      </div>
                      <div className="truncate">
                        <div className="font-bold text-xs">{t.profileModal.defaultViews?.list || 'Liste détaillée'}</div>
                        <div className="text-[10px] text-[var(--text-muted)] opacity-80">{t.nav.list}</div>
                      </div>
                    </button>
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
                      className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${detailMode === 'panel'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-indigo-400 border border-[var(--border-color)] shrink-0">
                        <PanelRight size={18} />
                      </div>
                      <div className="truncate">
                        <div className="font-bold text-xs">{t.profileModal.detailModes.panel}</div>
                        <div className="text-[10px] text-[var(--text-muted)] opacity-80">
                          {t.profileModal.detailModeDesc?.panel || 'Glissement latéral à droite'}
                        </div>
                      </div>
                    </button>

                    <button
                      type="button"
                      onClick={() => setDetailMode('modal')}
                      className={`flex items-center gap-3 p-3 rounded-xl border text-left transition-all cursor-pointer ${detailMode === 'modal'
                        ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text ring-2 ring-[var(--accent-glow)]'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="w-8 h-8 rounded-lg bg-[var(--bg-secondary)] flex items-center justify-center text-purple-400 border border-[var(--border-color)] shrink-0">
                        <Square size={18} />
                      </div>
                      <div className="truncate">
                        <div className="font-bold text-xs">{t.profileModal.detailModes.modal}</div>
                        <div className="text-[10px] text-[var(--text-muted)] opacity-80">
                          {t.profileModal.detailModeDesc?.modal || 'Boîte de dialogue au centre'}
                        </div>
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
                          className={`p-2.5 rounded-xl border text-left transition-all cursor-pointer ${isSelected
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

                {/* Interface Scaling (UI Scale / Zoom) */}
                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <ZoomIn size={13} className="text-indigo-400" />
                    <span>{t.profileModal.uiScale || 'Échelle de l\'interface'}</span>
                  </label>
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                    {UI_SCALE_OPTIONS.map(opt => {
                      const isSelected = (uiScale || 100) === opt
                      const scaleKey = `s${opt}` as keyof NonNullable<typeof t.profileModal.uiScales>
                      const label = t.profileModal.uiScales?.[scaleKey] || `${opt}%`
                      return (
                        <button
                          key={opt}
                          type="button"
                          onClick={() => setUiScale(opt)}
                          className={`p-2.5 rounded-xl border text-center transition-all cursor-pointer font-medium text-xs ${isSelected
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs font-bold'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                            }`}
                        >
                          <div>{label}</div>
                        </button>
                      )
                    })}
                  </div>
                </div>

              </div>
            )}

            {/* TAB 2: TRACKER CREDENTIALS, one activatable zone per tracker */}
            {activeTab === 'trackers' && <TrackerCredentialsTab />}

            {/* TAB 4: MOTEUR AGENTIC IA */}
            {activeTab === 'aiEngine' && (
              <div className="space-y-6 animate-in fade-in duration-150">
                <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                  {t.profileModal.ai.engineDesc}
                </p>

                {/* Agentic CLI Provider Selection */}
                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <Bot size={14} className="text-indigo-400" />
                    <span>{t.profileModal.ai.defaultEngine}</span>
                  </label>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {AI_PROVIDERS.map(p => {
                      const isSelected = aiProvider === p.id
                      const label = p.id === 'custom' ? (t.profileModal.ai.customProviderLabel || p.label) : p.label
                      const sub = p.id === 'custom' ? (t.profileModal.ai.customProviderSub || p.sub) : p.sub
                      return (
                        <button
                          key={p.id}
                          type="button"
                          onClick={() => handleProviderSelect(p)}
                          className={`px-3 py-2 rounded-xl border text-left transition-all cursor-pointer text-xs font-semibold flex items-center gap-2.5 truncate ${isSelected
                            ? 'bg-indigo-500/15 border-indigo-500 text-white ring-2 ring-indigo-500/30 shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)] hover:text-[var(--text-primary)]'
                            }`}
                          title={sub}
                        >
                          <span className="shrink-0 flex items-center justify-center">{p.icon}</span>
                          <span className="truncate">{label}</span>
                        </button>
                      )
                    })}
                  </div>
                </div>

                <ProviderModelsField
                  provider={aiProvider}
                  providers={AI_PROVIDERS.map(p => p.id)}
                  value={aiProviderModels}
                  onChange={setAiProviderModels}
                  label={t.profileModal.ai.proposedModelsFor}
                />

                <AIModelField
                  provider={aiProvider}
                  value={aiModel}
                  onChange={setAiModel}
                  availableModels={providerModels({ aiProviderModels }, aiProvider)}
                  placeholder={t.profileModal.ai.defaultModelPlaceholder || 'Défaut du CLI'}
                  label={t.profileModal.ai.defaultModel}
                />
                <MCPEngineConfig key={aiProvider} selectedProvider={aiProvider} onNavigateToWorkstations={() => setActiveTab('workstations')} onKeyCreated={() => setDevicesVersion(v => v + 1)} />
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
                      {t.profileModal.sdd?.title || 'Framework Spec-Driven Design (SDD)'}
                    </h4>
                  </div>
                  <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
                    {t.profileModal.sdd?.subtitle || "Le Spec-Driven Design garantit qu'une spécification claire, structurée et vérifiable est rédigée et validée avant toute génération de code par les agents d'IA."}
                  </p>
                </div>

                {/* Framework Choice */}
                <div className="space-y-2">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                    <span>{t.profileModal.sdd?.defaultFramework || 'Framework par défaut du projet'}</span>
                  </label>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    {/* GitHub Spec Kit Card */}
                    <button
                      type="button"
                      onClick={() => setSpecFramework('speckit')}
                      className={`p-4 rounded-xl border text-left transition-all cursor-pointer flex flex-col justify-between gap-3 ${specFramework === 'speckit'
                        ? 'bg-blue-500/15 border-blue-500 text-white ring-2 ring-blue-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex items-center gap-2">
                          <span className="text-xl">📑</span>
                          <div>
                            <div className="font-bold text-xs text-[var(--text-primary)]">
                              {t.profileModal.sdd?.speckitTitle || 'GitHub Spec Kit'}
                            </div>
                            <span className="text-[10px] font-mono text-blue-400">
                              {t.profileModal.sdd?.speckitSubtitle || 'CLI specify'}
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
                        {t.profileModal.sdd?.speckitDesc || "Convention standard GitHub : structure modulaire dans .specify/ et specs/ (spec.md, plan.md, tasks.md)."}
                      </p>

                      <div className="pt-2 flex items-center justify-between text-[10px] text-[var(--text-muted)]">
                        <span>{t.profileModal.sdd?.speckitCommands || 'Commandes : /specify-issue, /code-issue'}</span>
                        <span className="px-1.5 py-0.5 rounded bg-[var(--bg-secondary)] font-mono">.specify/</span>
                      </div>
                    </button>

                    {/* OpenSpec Card */}
                    <button
                      type="button"
                      onClick={() => setSpecFramework('openspec')}
                      className={`p-4 rounded-xl border text-left transition-all cursor-pointer flex flex-col justify-between gap-3 ${specFramework === 'openspec'
                        ? 'bg-emerald-500/15 border-emerald-500 text-white ring-2 ring-emerald-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                        }`}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex items-center gap-2">
                          <span className="text-xl">🚩</span>
                          <div>
                            <div className="font-bold text-xs text-[var(--text-primary)]">
                              {t.profileModal.sdd?.openspecTitle || 'OpenSpec'}
                            </div>
                            <span className="text-[10px] font-mono text-emerald-400">
                              {t.profileModal.sdd?.openspecSubtitle || 'CLI openspec'}
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
                        {t.profileModal.sdd?.openspecDesc || "Spécification formelle par deltas et exigences vérifiables. Les propositions de changements sont validées et revues avant l'écriture de code."}
                      </p>

                      <div className="pt-2 flex items-center justify-between text-[10px] text-[var(--text-muted)]">
                        <span>{t.profileModal.sdd?.openspecCommands || 'Commandes : openspec propose, validate'}</span>
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
                        {t.profileModal.sdd?.lifecycleTitle || 'Cycle de vie SDD dans Sectile'}
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        document.getElementById('skill-prompts-section')?.scrollIntoView({ behavior: 'smooth' })
                      }}
                      className="text-[10.5px] text-indigo-400 hover:text-indigo-300 font-semibold cursor-pointer flex items-center gap-1"
                    >
                      <span>{t.profileModal.sdd?.customizePrompts || 'Personnaliser les prompts'}</span>
                      <span>↓</span>
                    </button>
                  </div>

                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-2 text-center text-[10.5px]">
                    <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                      <div className="font-bold text-sky-400">{t.profileModal.sdd?.steps.clarify || '1. Clarifier'}</div>
                      <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/clarify-issue</div>
                    </div>
                    <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                      <div className="font-bold text-blue-400">{t.profileModal.sdd?.steps.specify || '2. Spécifier'}</div>
                      <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/specify-issue</div>
                    </div>
                    <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                      <div className="font-bold text-indigo-400">{t.profileModal.sdd?.steps.code || '3. Coder'}</div>
                      <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/code-issue</div>
                    </div>
                    <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                      <div className="font-bold text-purple-400">{t.profileModal.sdd?.steps.adjust || '4. Ajuster'}</div>
                      <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/adjust-issue</div>
                    </div>
                    <div className="p-2 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)]">
                      <div className="font-bold text-emerald-400">{t.profileModal.sdd?.steps.handoff || '5. Clôturer'}</div>
                      <div className="text-[9.5px] font-mono text-[var(--text-muted)] mt-0.5">/handoff-issue</div>
                    </div>
                  </div>
                </div>

                {/* SECTION: PERSONNALISATION DES PROMPTS DES SKILLS */}
                <div id="skill-prompts-section" className="space-y-4 pt-1">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <FileCode size={16} className="text-amber-400" />
                      <h4 className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
                        {t.profileModal.sdd?.promptsSectionTitle || 'Personnalisation des Prompts par Compétence'}
                      </h4>
                    </div>
                    <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">
                      {t.profileModal.sdd?.promptsSectionDesc || 'Personnalisez les invites (prompts) envoyées au CLI Agentic pour chaque étape du workflow SDD. Si laissé vide, les invites par défaut sont utilisées.'}
                    </p>
                  </div>

                  {/* Clarify Prompt */}
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-amber-400">
                        <HelpCircle size={13} />
                        <span>{t.profileModal.sdd?.prompts.clarify.label || 'Prompt de Cadrage (/clarify-issue)'}</span>
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">{t.profileModal.sdd?.prompts.clarify.hint || 'Défaut : /clarify-issue {issueKey} tracked on {tracker} in {repo}'}</span>
                    </label>
                    <textarea
                      value={promptClarify}
                      onChange={e => setPromptClarify(e.target.value)}
                      rows={3}
                      placeholder={t.profileModal.sdd?.prompts.clarify.placeholder || '/clarify-issue {issueKey} tracked on {tracker} in {repo}'}
                      className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-amber-500 transition-all resize-y"
                    />
                  </div>

                  {/* Specify Prompt */}
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-blue-400">
                        <FileCode size={13} />
                        <span>{t.profileModal.sdd?.prompts.specify.label || 'Prompt de Spécification (/specify-issue)'}</span>
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">{t.profileModal.sdd?.prompts.specify.hint || 'Spécification Spec Kit / OpenSpec'}</span>
                    </label>
                    <textarea
                      value={promptSpecify}
                      onChange={e => setPromptSpecify(e.target.value)}
                      rows={3}
                      placeholder={t.profileModal.sdd?.prompts.specify.placeholder || 'Tu es le Product Owner pour {issueKey}. Rédige la spécification selon le framework SDD configuré...'}
                      className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-blue-500 transition-all resize-y"
                    />
                  </div>

                  {/* Implement Prompt */}
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-indigo-400">
                        <Flame size={13} />
                        <span>{t.profileModal.sdd?.prompts.implement.label || "Prompt d'Implémentation (/code-issue)"}</span>
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">{t.profileModal.sdd?.prompts.implement.hint || 'Développement & Tests'}</span>
                    </label>
                    <textarea
                      value={promptImplement}
                      onChange={e => setPromptImplement(e.target.value)}
                      rows={3}
                      placeholder={t.profileModal.sdd?.prompts.implement.placeholder || 'Tu es le développeur senior pour {issueKey}. Implémente le code dans {repoPath}...'}
                      className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-indigo-500 transition-all resize-y"
                    />
                  </div>

                  {/* Adjust Prompt */}
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-purple-400">
                        <ShieldCheck size={13} />
                        <span>{t.profileModal.sdd?.prompts.adjust.label || "Prompt d'Ajustement & Revue (/adjust-issue)"}</span>
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">{t.profileModal.sdd?.prompts.adjust.hint || 'Revue de code & màj PR'}</span>
                    </label>
                    <textarea
                      value={promptAdjust}
                      onChange={e => setPromptAdjust(e.target.value)}
                      rows={3}
                      placeholder={t.profileModal.sdd?.prompts.adjust.placeholder || 'Tu es le reviewer senior pour {issueKey}. Revois les changements, applique les correctifs et mets à jour la PR...'}
                      className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-all resize-y"
                    />
                  </div>

                  {/* Handoff Prompt */}
                  <div className="space-y-1.5">
                    <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-emerald-400">
                        <CheckCircle2 size={13} />
                        <span>{t.profileModal.sdd?.prompts.handoff.label || 'Prompt de Clôture & Handoff (/handoff-issue)'}</span>
                      </span>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono">{t.profileModal.sdd?.prompts.handoff.hint || 'Documentation & Nettoyage local'}</span>
                    </label>
                    <textarea
                      value={promptHandoff}
                      onChange={e => setPromptHandoff(e.target.value)}
                      rows={3}
                      placeholder={t.profileModal.sdd?.prompts.handoff.placeholder || 'Tu es responsable de la clôture pour {issueKey}. Vérifie la fusion, rédige le rapport de handoff et nettoie le worktree...'}
                      className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-emerald-500 transition-all resize-y"
                    />
                  </div>
                </div>
              </div>
            )}

            {/* TAB: WORKSTATIONS & AGENT */}
            {activeTab === 'workstations' && (
              <div className="space-y-6 animate-in fade-in duration-150">
                {/* 1. Workstations (Pairing & Connected Machines) */}
                <WorkstationsPanel
                  refreshTrigger={devicesVersion}
                  onDeviceChange={() => setDevicesVersion(v => v + 1)}
                />

                {/* 2. Sectile Desktop App */}
                <SectileDesktopPanel />

                {/* 5. Headless Local Agent (CLI) */}
                <HeadlessCliAgentPanel />
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
