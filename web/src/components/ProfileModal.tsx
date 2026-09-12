import React, { useState, useEffect } from 'react'
import {
  X,
  User,
  Mail,
  Palette,
  Sun,
  Moon,
  Globe,
  Check,
  Sliders,
  PanelRight,
  Square,
  Code2,
  Bot,
  Terminal,
  FileCode,
  HelpCircle,
  CalendarDays,
  Flame,
  GitPullRequest,
  Info,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { Theme, Language, Density, ViewMode, DetailMode, AIProvider, SpecFramework } from '../types'

type SettingsTab = 'appearance' | 'agentic' | 'prompts'

const AI_PROVIDERS: { id: AIProvider; label: string; sub: string; defaultCmd: string; icon: string }[] = [
  { id: 'agy', label: 'Antigravity (agy)', sub: 'Google Deepmind AGY CLI', defaultCmd: 'agy --dangerously-skip-permissions -p "{prompt}"', icon: '🚀' },
  { id: 'claude', label: 'Claude Code (claude)', sub: 'Anthropic Claude Code CLI', defaultCmd: 'claude --dangerously-skip-permissions -p "{prompt}"', icon: '🟣' },
  { id: 'vibe', label: 'Mistral Vibe (vibe)', sub: 'Mistral AI CLI', defaultCmd: 'vibe -p "{prompt}" --auto-approve', icon: '⚡' },
  { id: 'gemini', label: 'Gemini (gemini)', sub: 'Google Gemini CLI', defaultCmd: 'gemini -p "{prompt}"', icon: '♊' },
  { id: 'cursor', label: 'Cursor Agent (cursor)', sub: 'Cursor Editor CLI Agent', defaultCmd: 'cursor agent -p "{prompt}"', icon: '💻' },
  { id: 'codex', label: 'Codex CLI', sub: 'OpenAI Codex CLI', defaultCmd: 'codex -p "{prompt}"', icon: '🤖' },
  { id: 'custom', label: 'CLI Personnalisé', sub: 'Binaire ou script custom', defaultCmd: '/path/to/custom-cli -p "{prompt}"', icon: '⚙️' },
]

export const ProfileModal: React.FC = () => {
  const {
    isProfileOpen,
    setIsProfileOpen,
    settings,
    updateSettings,
    t,
  } = useApp()

  const [activeTab, setActiveTab] = useState<SettingsTab>('appearance')

  // Appearance & User
  const [userName, setUserName] = useState(settings.userName)
  const [userEmail, setUserEmail] = useState(settings.userEmail)
  const [theme, setTheme] = useState<Theme>(settings.theme)
  const [language, setLanguage] = useState<Language>(settings.language)
  const [density, setDensity] = useState<Density>(settings.density)
  const [defaultView, setDefaultView] = useState<ViewMode>(settings.defaultView)
  const [detailMode, setDetailMode] = useState<DetailMode>(settings.detailMode || 'panel')
  const [editorCommand, setEditorCommand] = useState(settings.editorCommand || 'code')
  const [externalTerminalCommand, setExternalTerminalCommand] = useState(settings.externalTerminalCommand || '')

  // Agentic AI & CLI Configuration
  const [aiProvider, setAiProvider] = useState<AIProvider>(settings.aiProvider || 'agy')
  const [aiCommandTemplate, setAiCommandTemplate] = useState(settings.aiCommandTemplate || 'agy -p "{prompt}"')
  const [specFramework, setSpecFramework] = useState<SpecFramework>(settings.specFramework || 'speckit')

  // Skill Prompts
  const [promptDigestAgenda, setPromptDigestAgenda] = useState(settings.promptDigestAgenda || '')
  const [promptClarify, setPromptClarify] = useState(settings.promptClarify || '')
  const [promptSpecify, setPromptSpecify] = useState(settings.promptSpecify || '')
  const [promptImplement, setPromptImplement] = useState(settings.promptImplement || '')
  const [promptCreatePr, setPromptCreatePr] = useState(settings.promptCreatePr || '')

  useEffect(() => {
    if (isProfileOpen) {
      setUserName(settings.userName)
      setUserEmail(settings.userEmail)
      setTheme(settings.theme)
      setLanguage(settings.language)
      setDensity(settings.density)
      setDefaultView(settings.defaultView)
      setDetailMode(settings.detailMode || 'panel')
      setEditorCommand(settings.editorCommand || 'code')
      setExternalTerminalCommand(settings.externalTerminalCommand || '')
      setAiProvider(settings.aiProvider || 'agy')
      setAiCommandTemplate(settings.aiCommandTemplate || 'agy -p "{prompt}"')
      setSpecFramework(settings.specFramework || 'speckit')
      setPromptDigestAgenda(settings.promptDigestAgenda || '')
      setPromptClarify(settings.promptClarify || '')
      setPromptSpecify(settings.promptSpecify || '')
      setPromptImplement(settings.promptImplement || '')
      setPromptCreatePr(settings.promptCreatePr || '')
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
    if (!aiCommandTemplate || aiCommandTemplate.trim() === '' || AI_PROVIDERS.some(p => p.defaultCmd === aiCommandTemplate)) {
      setAiCommandTemplate(provider.defaultCmd)
    }
  }

  const handleSave = async () => {
    await updateSettings({
      userName: userName.trim(),
      userEmail: userEmail.trim(),
      theme,
      language,
      density,
      defaultView,
      detailMode,
      editorCommand: editorCommand.trim() || 'code',
      externalTerminalCommand: externalTerminalCommand.trim(),
      aiProvider,
      aiCommandTemplate: aiCommandTemplate.trim() || `${aiProvider} -p "{prompt}"`,
      specFramework,
      promptDigestAgenda: promptDigestAgenda.trim(),
      promptClarify: promptClarify.trim(),
      promptSpecify: promptSpecify.trim(),
      promptImplement: promptImplement.trim(),
      promptCreatePr: promptCreatePr.trim(),
    })
    setIsProfileOpen(false)
  }

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200">
      <div className="relative w-full max-w-2xl rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.92)]">
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
                Configuration générale, CLI Agentic et préférences d'affichage
              </p>
            </div>
          </div>
          <button
            onClick={() => setIsProfileOpen(false)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={17} />
          </button>
        </div>

        {/* Tab Navigation Header */}
        <div className="flex items-center gap-4 px-6 pt-3 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] shrink-0 text-xs font-semibold">
          <button
            type="button"
            onClick={() => setActiveTab('appearance')}
            className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
              activeTab === 'appearance'
                ? 'border-[var(--accent-color)] accent-text font-bold'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            <Palette size={14} />
            <span>Apparence & Profil</span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('agentic')}
            className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
              activeTab === 'agentic'
                ? 'border-indigo-400 text-indigo-400 font-bold'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            <Bot size={14} className="text-indigo-400" />
            <span>CLI Agentic & Commande</span>
          </button>

          <button
            type="button"
            onClick={() => setActiveTab('prompts')}
            className={`pb-2.5 flex items-center gap-1.5 border-b-2 transition-all cursor-pointer ${
              activeTab === 'prompts'
                ? 'border-amber-400 text-amber-400 font-bold'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            <FileCode size={14} className="text-amber-400" />
            <span>Prompts des Skills</span>
          </button>
        </div>

        {/* Modal Form Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
          {/* TAB 1: APPEARANCE & PROFILE */}
          {activeTab === 'appearance' && (
            <div className="space-y-6 animate-in fade-in duration-150">
              {/* User info */}
              <div className="space-y-3">
                <div className="text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                  <User size={13} className="text-cyan-400" />
                  <span>{t.profileModal.userSection}</span>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[11px] font-medium text-[var(--text-secondary)] mb-1">
                      {t.profileModal.name}
                    </label>
                    <div className="relative">
                      <input
                        type="text"
                        value={userName}
                        onChange={e => setUserName(e.target.value)}
                        className="w-full pl-8 pr-3 py-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <User size={14} className="absolute left-2.5 top-2.5 text-[var(--text-muted)]" />
                    </div>
                  </div>

                  <div>
                    <label className="block text-[11px] font-medium text-[var(--text-secondary)] mb-1">
                      {t.profileModal.email}
                    </label>
                    <div className="relative">
                      <input
                        type="email"
                        value={userEmail}
                        onChange={e => setUserEmail(e.target.value)}
                        className="w-full pl-8 pr-3 py-2 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                      />
                      <Mail size={14} className="absolute left-2.5 top-2.5 text-[var(--text-muted)]" />
                    </div>
                  </div>
                </div>
              </div>

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
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <Code2 size={13} className="text-[var(--accent-color)]" />
                    <span>Éditeur de code (Ouvrir le dossier / code)</span>
                  </label>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono">Défaut : code (VS Code)</span>
                </div>
                
                {/* Quick Presets */}
                <div className="grid grid-cols-3 sm:grid-cols-6 gap-1.5">
                  {[
                    { cmd: 'code', label: 'VS Code' },
                    { cmd: 'cursor', label: 'Cursor' },
                    { cmd: 'zed', label: 'Zed' },
                    { cmd: 'subl', label: 'Sublime' },
                    { cmd: 'idea', label: 'IntelliJ' },
                    { cmd: 'webstorm', label: 'WebStorm' },
                  ].map(preset => {
                    const isSelected = editorCommand === preset.cmd
                    return (
                      <button
                        key={preset.cmd}
                        type="button"
                        onClick={() => setEditorCommand(preset.cmd)}
                        className={`py-1.5 px-2 rounded-lg text-xs font-semibold border transition-all cursor-pointer text-center ${
                          isSelected
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--text-muted)]'
                        }`}
                      >
                        {preset.label}
                      </button>
                    )
                  })}
                </div>

                {/* Custom Command Input */}
                <div className="flex items-center gap-2 pt-1">
                  <input
                    type="text"
                    value={editorCommand}
                    onChange={e => setEditorCommand(e.target.value)}
                    placeholder="Ex: code, cursor, zed"
                    className="w-full px-3 py-1.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] transition-all"
                  />
                </div>
              </div>

              {/* Default External Terminal */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <Terminal size={13} className="text-[var(--accent-color)]" />
                    <span>Application de Terminal externe</span>
                  </label>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono">Défaut : Terminal OS par défaut</span>
                </div>

                {/* Quick Presets */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-1.5">
                  {[
                    { cmd: '', label: 'Auto (OS)', desc: 'Terminal par défaut' },
                    { cmd: 'Ghostty', label: 'Ghostty', desc: 'macOS & Linux' },
                    { cmd: 'Terminal', label: 'Terminal.app', desc: 'macOS natif' },
                    { cmd: 'iTerm', label: 'iTerm2', desc: 'macOS' },
                    { cmd: 'Alacritty', label: 'Alacritty', desc: 'GPU accéléré' },
                    { cmd: 'kitty', label: 'Kitty', desc: 'GPU accéléré' },
                    { cmd: 'WezTerm', label: 'WezTerm', desc: 'Multiplexeur' },
                    { cmd: 'Warp', label: 'Warp', desc: 'AI Terminal' },
                  ].map(preset => {
                    const isSelected = externalTerminalCommand === preset.cmd
                    return (
                      <button
                        key={preset.cmd}
                        type="button"
                        onClick={() => setExternalTerminalCommand(preset.cmd)}
                        className={`py-2 px-2.5 rounded-xl text-left border transition-all cursor-pointer ${
                          isSelected
                            ? 'bg-[var(--accent-light)] border-[var(--accent-color)] accent-text shadow-xs'
                            : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--text-muted)]'
                        }`}
                      >
                        <div className="font-bold text-xs">{preset.label}</div>
                        <div className="text-[9.5px] opacity-75">{preset.desc}</div>
                      </button>
                    )
                  })}
                </div>

                {/* Custom Command Input */}
                <div className="space-y-1 pt-1">
                  <input
                    type="text"
                    value={externalTerminalCommand}
                    onChange={e => setExternalTerminalCommand(e.target.value)}
                    placeholder="Ex: Ghostty, Terminal, iTerm, alacritty, kitty ou modèle custom"
                    className="w-full px-3 py-1.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] transition-all"
                  />
                  <p className="text-[10px] text-[var(--text-muted)]">
                    Pour une commande sur mesure, vous pouvez utiliser le placeholder <code className="text-amber-400 font-bold">{'{script}'}</code> (ex: <code className="text-[var(--text-secondary)]">ghostty -e {'{script}'}</code> ou <code className="text-[var(--text-secondary)]">open -na Ghostty --args -e {'{script}'}</code>).
                  </p>
                </div>
              </div>

            </div>
          )}

          {/* TAB 2: AGENTIC CLI & COMMAND LINE */}
          {activeTab === 'agentic' && (
            <div className="space-y-6 animate-in fade-in duration-150">
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
                            {p.defaultCmd}
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </div>

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
                  <input
                    type="text"
                    value={aiCommandTemplate}
                    onChange={e => setAiCommandTemplate(e.target.value)}
                    placeholder='Ex: agy -p "{prompt}" ou claude -p "{prompt}"'
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
                  </div>
                </div>

                {/* Fast Preset buttons */}
                <div className="pt-2 border-t border-[var(--border-color)]">
                  <div className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider font-bold mb-1.5">
                    Modèles de commande rapides :
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {[
                      { label: 'AGY sans confirmation', cmd: 'agy --dangerously-skip-permissions -p "{prompt}"' },
                      { label: 'AGY interactif', cmd: 'agy -i "{prompt}"' },
                      { label: 'Claude sans confirmation', cmd: 'claude --dangerously-skip-permissions -p "{prompt}"' },
                      { label: 'Mistral Vibe', cmd: 'vibe -p "{prompt}" --auto-approve' },
                      { label: 'Gemini CLI', cmd: 'gemini -p "{prompt}"' },
                      { label: 'Cursor Agent', cmd: 'cursor agent -p "{prompt}"' },
                    ].map(preset => (
                      <button
                        key={preset.label}
                        type="button"
                        onClick={() => setAiCommandTemplate(preset.cmd)}
                        className="px-2 py-1 bg-[var(--bg-primary)] hover:bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent-color)] text-[10.5px] rounded-lg font-mono transition-colors cursor-pointer"
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              {/* Spec-Driven Design Framework Selection */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
                    <FileCode size={14} className="text-blue-400" />
                    <span>Framework Spec-Driven Design par défaut</span>
                  </label>
                  <span className="text-[10px] text-blue-400 font-mono font-bold">
                    {specFramework === 'openspec' ? 'OpenSpec' : 'Spec Kit'}
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setSpecFramework('speckit')}
                    className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-2.5 ${
                      specFramework === 'speckit'
                        ? 'bg-blue-500/15 border-blue-500 text-white ring-2 ring-blue-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <span className="text-base">📑</span>
                    <div className="truncate flex-1">
                      <div className="font-bold text-xs flex items-center justify-between">
                        <span>GitHub Spec Kit</span>
                        {specFramework === 'speckit' && <Check size={14} className="text-blue-400" />}
                      </div>
                      <div className="text-[10px] text-[var(--text-muted)] mt-0.5">CLI specify : .specify/ + specs/ (spec.md, plan.md, tasks.md)</div>
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setSpecFramework('openspec')}
                    className={`p-3 rounded-xl border text-left transition-all cursor-pointer flex items-start gap-2.5 ${
                      specFramework === 'openspec'
                        ? 'bg-emerald-500/15 border-emerald-500 text-white ring-2 ring-emerald-500/30 shadow-xs'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
                    }`}
                  >
                    <span className="text-base">🚩</span>
                    <div className="truncate flex-1">
                      <div className="font-bold text-xs flex items-center justify-between">
                        <span>OpenSpec</span>
                        {specFramework === 'openspec' && <Check size={14} className="text-emerald-400" />}
                      </div>
                      <div className="text-[10px] text-[var(--text-muted)] mt-0.5">CLI openspec : propositions de changement et deltas de specs validés avant code</div>
                    </div>
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* TAB 3: PROMPTS DES SKILLS */}
          {activeTab === 'prompts' && (
            <div className="space-y-5 animate-in fade-in duration-150">
              <div className="text-[11px] text-[var(--text-muted)] leading-relaxed">
                Personnalisez les invites (prompts) envoyées au CLI Agentic pour chaque étape du workflow. Si laissé vide, les invites par défaut sont utilisées.
              </div>

              {/* Digest Agenda Prompt : la seule partie du digest qui passe par
                  l'agent, les autres sections étant calculées sur les tickets. */}
              <div className="space-y-1.5">
                <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                  <span className="flex items-center gap-1.5 text-emerald-400">
                    <CalendarDays size={13} />
                    <span>Prompt de l'agenda du Daily Digest</span>
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono">
                    Marqueurs : {'{project}'} {'{date}'}
                  </span>
                </label>
                <textarea
                  value={promptDigestAgenda}
                  onChange={e => setPromptDigestAgenda(e.target.value)}
                  rows={3}
                  placeholder="/daily-brief {date}"
                  className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-emerald-500 transition-all resize-y"
                />
                <span className="text-[10px] text-[var(--text-muted)] block">
                  Vide garde le prompt d'origine, qui demande un tableau des réunions et interdit
                  d'inventer un agenda. Une commande de votre agent fait aussi l'affaire, par exemple
                  <code className="text-cyan-400"> /daily-brief {'{date}'}</code> : c'est ce texte,
                  marqueurs substitués, qui lui est envoyé tel quel.
                </span>
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

              {/* Create PR Prompt */}
              <div className="space-y-1.5">
                <label className="block text-[11px] font-bold text-[var(--text-primary)] flex items-center justify-between">
                  <span className="flex items-center gap-1.5 text-purple-400">
                    <GitPullRequest size={13} />
                    <span>Prompt de Pull Request & Release (/create-pr)</span>
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono">Commit, Push & PR / Merge</span>
                </label>
                <textarea
                  value={promptCreatePr}
                  onChange={e => setPromptCreatePr(e.target.value)}
                  rows={3}
                  placeholder='Tu es l ingénieur DevOps pour {issueKey}. Commite sur {branchName} et crée la PR...'
                  className="w-full p-2.5 text-xs font-mono rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-all resize-y"
                />
              </div>
            </div>
          )}
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
            className="px-5 py-2 rounded-xl text-xs font-semibold text-white accent-bg shadow hover:opacity-90 active:scale-95 transition-all cursor-pointer"
          >
            {t.profileModal.save}
          </button>
        </div>
      </div>
    </div>
  )
}
