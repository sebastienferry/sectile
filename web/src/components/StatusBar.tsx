import React, { useState } from 'react'
import {
  GitBranch,
  FolderGit2,
  GitCommit,
  RefreshCw,
  Bot,
  Copy,
  Check,
  Layers,
  ZoomIn,
  Moon,
  Sun,
  RefreshCcwDot,
  Radio,
} from 'lucide-react'
import { useApp, UI_SCALE_OPTIONS } from '../context/AppContext'
import { useAgentStatus } from '../hooks/useAgentStatus'

export const StatusBar: React.FC = () => {
  const {
    gitStatus,
    isFetchingGitStatus,
    fetchGitStatus,
    currentProject,
    settings,
    activeJobCount,
    activities,
    setActiveView,
    setIsProfileOpen,
    setIsProjectModalOpen,
    setIsBranchModalOpen,
    updateSettings,
    autoSync,
    t,
    addToast,
  } = useApp()

  const { isConnected: isAgentConnected, agents: connectedAgents } = useAgentStatus()

  const [copied, setCopied] = useState(false)

  // The selected project overrides the global engine, so the pill must not
  // advertise the global one while the project runs another CLI.
  const statusProvider = currentProject?.aiProvider || settings.aiProvider || 'agy'
  const activeBranch = gitStatus?.branch || (currentProject?.repoPath ? 'main' : null)
  const isGit = gitStatus?.isGitRepo ?? Boolean(gitStatus?.branch)
  const isClean = gitStatus?.isClean ?? true
  const modifiedCount = gitStatus?.modifiedCount ?? 0
  const untrackedCount = gitStatus?.untrackedCount ?? 0
  const totalDirty = modifiedCount + untrackedCount

  // Display name for repository / CWD
  const cwdDisplay = React.useMemo(() => {
    const rawPath = gitStatus?.repoPath || currentProject?.repoPath || settings.repoPath || ''
    if (!rawPath) return 'tasks'
    const parts = rawPath.replace(/\/+$/, '').split('/')
    return parts[parts.length - 1] || rawPath
  }, [gitStatus?.repoPath, currentProject?.repoPath, settings.repoPath])

  const fullCwdPath = gitStatus?.repoPath || currentProject?.repoPath || settings.repoPath || ''

  const handleCopyBranch = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!activeBranch) return
    navigator.clipboard.writeText(activeBranch)
    setCopied(true)
    addToast({
      type: 'info',
      title: t.statusBar.branchCopied,
      description: activeBranch,
    })
    setTimeout(() => setCopied(false), 2000)
  }

  const handleRefresh = (e: React.MouseEvent) => {
    e.stopPropagation()
    fetchGitStatus(undefined, true)
  }

  const latestRunningActivity = activities.find(
    a => a.status === 'running' || a.status === 'pending' || a.status === 'queued'
  )

  return (
    <footer
      className="h-7 w-full bg-[var(--bg-secondary)] border-t border-[var(--border-color)] px-3 flex items-center justify-between text-[11px] select-none z-20 shrink-0 font-mono tracking-tight text-[var(--text-secondary)] shadow-sm"
      role="status"
      aria-label="Status Bar"
    >
      {/* Left Section: Git Branch, Dirty State, CWD */}
      <div className="flex items-center gap-2 overflow-hidden min-w-0">
        {/* CWD / Project Repo Name */}
        <div
          onClick={() => setIsProjectModalOpen(true)}
          className="flex items-center gap-1.5 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors shrink-0"
          title={`Dossier CWD : ${fullCwdPath || 'Non défini'}`}
        >
          <FolderGit2 size={12} className="text-indigo-400 shrink-0" />
          <span className="font-semibold truncate max-w-[120px] text-[var(--text-primary)]">
            {currentProject ? currentProject.name : cwdDisplay}
          </span>
        </div>

        <span className="text-[var(--border-color)]">/</span>

        {/* Git Branch Badge & Status */}
        {isGit && activeBranch ? (
          <div
            onClick={() => setIsBranchModalOpen(true)}
            className="group flex items-center gap-1.5 px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)]/70 hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] cursor-pointer transition-all border border-[var(--border-color)]/60"
            title={`${t.statusBar.branch} : ${activeBranch} (Cliquer pour changer de branche)`}
          >
            <GitBranch size={12} className="text-cyan-400 shrink-0 group-hover:scale-110 transition-transform" />
            <span className="font-bold text-cyan-300 dark:text-cyan-400 truncate max-w-[180px] sm:max-w-[260px]">
              {activeBranch}
            </span>

            {/* Clean / Modified Status Badge */}
            {isClean ? (
              <span
                className="flex items-center gap-1 text-[10px] text-emerald-400 pl-0.5"
                title={t.statusBar.clean}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                <span className="hidden sm:inline text-[9px] uppercase font-bold tracking-wider">clean</span>
              </span>
            ) : (
              <span
                className="flex items-center gap-1 text-[10px] text-amber-400 pl-0.5 font-bold"
                title={`${modifiedCount} ${t.statusBar.modified}, ${untrackedCount} ${t.statusBar.untracked}`}
              >
                <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse"></span>
                <span>*{totalDirty}</span>
              </span>
            )}

            {/* Ahead / Behind Indicator */}
            {gitStatus && (gitStatus.ahead > 0 || gitStatus.behind > 0) && (
              <span
                className="hidden sm:inline-flex items-center gap-0.5 text-[9px] text-cyan-300 font-mono pl-0.5 font-bold"
                title={`${gitStatus.ahead} commit(s) en avance, ${gitStatus.behind} commit(s) en retard`}
              >
                {gitStatus.ahead > 0 && <span>↑{gitStatus.ahead}</span>}
                {gitStatus.behind > 0 && <span>↓{gitStatus.behind}</span>}
              </span>
            )}

            {/* Copy icon on hover */}
            <span
              onClick={handleCopyBranch}
              className="opacity-0 group-hover:opacity-100 hover:text-cyan-300 text-[var(--text-muted)] transition-opacity ml-0.5 p-0.5 rounded cursor-pointer"
              title="Copier le nom de la branche"
            >
              {copied ? <Check size={10} className="text-emerald-400" /> : <Copy size={10} />}
            </span>
          </div>
        ) : (
          <div
            className="flex items-center gap-1 text-[var(--text-muted)] px-1.5 py-0.5"
            title={gitStatus?.error || t.statusBar.notGit}
          >
            <GitBranch size={12} className="opacity-40" />
            <span className="italic text-[10px]">{t.statusBar.noRepo}</span>
          </div>
        )}

        {/* Latest Commit (if available, on wider screens) */}
        {gitStatus?.latestCommit && (
          <div
            className="hidden xl:flex items-center gap-1 text-[10px] text-[var(--text-muted)] truncate max-w-[220px] px-1 hover:text-[var(--text-secondary)] transition-colors"
            title={`Dernier commit : ${gitStatus.latestCommit}`}
          >
            <GitCommit size={11} className="shrink-0 text-slate-500" />
            <span className="truncate">{gitStatus.latestCommit}</span>
          </div>
        )}
      </div>

      {/* Center Section: Live Execution / Background Job Ticker */}
      <div className="hidden md:flex items-center justify-center gap-2 overflow-hidden px-2">
        {activeJobCount > 0 && latestRunningActivity ? (
          <button
            type="button"
            onClick={() => setActiveView('activities')}
            className="flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-blue-500/15 border border-blue-500/30 text-blue-400 hover:bg-blue-500/25 transition-all animate-in fade-in"
            title="Afficher la file d'attente des activités"
          >
            <span className="w-1.5 h-1.5 rounded-full bg-blue-400 animate-ping shrink-0" />
            <span className="font-semibold truncate max-w-[200px]">
              {latestRunningActivity.skillName} ({latestRunningActivity.taskKey || 'Tâche'})
            </span>
            <span className="text-[9px] px-1 rounded bg-blue-500/20 font-bold">
              {activeJobCount} {t.statusBar.activeJobs}
            </span>
          </button>
        ) : (
          <div className="flex items-center gap-1.5 text-[10px] text-[var(--text-muted)]">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500/80"></span>
            <span>{t.statusBar.ready}</span>
          </div>
        )}
      </div>

      {/* Right Section: AI Provider, Issue Tracker, CLI Readiness, Refresh */}
      <div className="flex items-center gap-2 shrink-0">
        {/* Issue Tracker Sync pill */}
        <div
          onClick={() => setActiveView('sync')}
          className="hidden sm:flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors text-[10px]"
          title={`Gestionnaire de tickets : ${settings.issueTracker.toUpperCase()}`}
        >
          <Layers size={11} className="text-indigo-400" />
          <span className="font-semibold">{settings.issueTracker.toUpperCase()}</span>
          {settings.linearTeam && <span className="opacity-75">({settings.linearTeam})</span>}
        </div>

        {/* Synchronisation de fond : discrète tant qu'elle va bien, explicite
            quand le tracker l'a mise en pause ou qu'une passe a échoué. */}
        {autoSync?.enabled && (
          <div
            onClick={() => setActiveView('sync')}
            className="hidden md:flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] cursor-pointer transition-colors text-[10px]"
            title={
              autoSync.backoffUntil
                ? `Pause demandée par Jira jusqu'à ${new Date(autoSync.backoffUntil).toLocaleTimeString()}`
                : autoSync.lastError
                ? `Dernière passe en échec : ${autoSync.lastError}`
                : `Synchronisation toutes les ${Math.round(autoSync.intervalSec / 60) || 1} min. ${autoSync.passes} passe(s), ${autoSync.imported} ticket(s) mis à jour${
                    autoSync.lastRunAt ? `, dernière à ${new Date(autoSync.lastRunAt).toLocaleTimeString()}` : ''
                  }`
            }
          >
            <RefreshCcwDot
              size={11}
              className={
                autoSync.backoffUntil || autoSync.lastError
                  ? 'text-amber-400'
                  : autoSync.running
                  ? 'text-cyan-400 animate-spin'
                  : 'text-emerald-400'
              }
            />
            <span className="font-semibold">
              {autoSync.backoffUntil ? 'pause' : autoSync.lastImported > 0 ? `+${autoSync.lastImported}` : 'auto'}
            </span>
          </div>
        )}

        {/* Thème : le basculement clair / sombre est un geste de tous les jours
            (une salle éclairée, une démo au vidéoprojecteur), il n'a rien à faire
            au fond d'un écran de réglages. La valeur est enregistrée comme dans
            le profil, donc les deux endroits disent la même chose. */}
        <button
          type="button"
          onClick={() => updateSettings({ theme: settings.theme === 'light' ? 'dark' : 'light' }, { silent: true })}
          className="flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors text-[10px]"
          title={settings.theme === 'light' ? 'Passer en thème sombre' : 'Passer en thème clair'}
        >
          {settings.theme === 'light' ? (
            <Sun size={11} className="text-amber-400" />
          ) : (
            <Moon size={11} className="text-indigo-300" />
          )}
          <span className="hidden lg:inline font-semibold">
            {settings.theme === 'light' ? 'Clair' : 'Sombre'}
          </span>
        </button>

        {/* Échelle de l'interface : quatre crans, changés d'un clic et enregistrés
            dans les réglages, parce que c'est un besoin qui arrive en travaillant
            (un écran externe, une démo) et pas au moment de configurer l'outil. */}
        <div
          className="hidden sm:flex items-center gap-0.5 px-1 py-0.5 rounded border border-[var(--border-color)] text-[10px]"
          title="Échelle de l'interface"
        >
          <ZoomIn size={11} className="text-[var(--text-muted)] mr-0.5" />
          {UI_SCALE_OPTIONS.map(option => {
            const isActive = (settings.uiScale || 100) === option
            return (
              <button
                key={option}
                type="button"
                onClick={() => updateSettings({ uiScale: option }, { silent: true })}
                className={`px-1 rounded font-bold cursor-pointer transition-colors ${
                  isActive
                    ? 'text-[var(--accent-color)] bg-[var(--accent-light)]'
                    : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]'
                }`}
                title={`Afficher l'interface à ${option} %`}
              >
                {option}
              </button>
            )
          })}
        </div>

        {/* Local Agent Connection Status */}
        <div
          className="flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] transition-colors text-[10px]"
          title={
            isAgentConnected
              ? `Agent local connecté (${connectedAgents.map(a => a.deviceId || a.userId).join(', ') || 'actif'})`
              : "Aucun agent local connecté. Lancez 'taskflow agent' sur votre machine pour exécuter les workflows LLM et Git."
          }
        >
          <Radio size={11} className={isAgentConnected ? "text-emerald-400 animate-pulse" : "text-[var(--text-muted)]"} />
          <span className={`font-semibold ${isAgentConnected ? "text-emerald-400" : "text-[var(--text-muted)]"}`}>
            {isAgentConnected ? "Agent actif" : "Agent hors-ligne"}
          </span>
        </div>

        {/* AI Provider pill */}
        <div
          onClick={() => setIsProfileOpen(true)}
          className="flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors text-[10px]"
          title={`Moteur IA actif : ${statusProvider.toUpperCase()}${currentProject?.aiProvider ? ` (défini par le projet ${currentProject.name})` : ''} - Cliquer pour configurer`}
        >
          <Bot size={11} className="text-amber-400" />
          <span className="font-bold text-[var(--accent-color)]">{statusProvider.toUpperCase()}</span>
        </div>

        {/* Refresh Git Status Button */}
        <button
          type="button"
          onClick={handleRefresh}
          className="p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-all"
          title={t.statusBar.refresh}
        >
          <RefreshCw size={11} className={isFetchingGitStatus ? 'animate-spin text-cyan-400' : ''} />
        </button>
      </div>
    </footer>
  )
}

export default StatusBar
