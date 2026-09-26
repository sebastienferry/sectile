import React, { useState } from 'react'
import {
  RefreshCw,
  FolderGit2,
  CheckCircle2,
  Clock,
  ExternalLink,
  Save,
  Check,
  Zap,
  Terminal,
  Activity as ActivityIcon,
  ChevronRight,
  Sliders,
  AlertCircle,
  Folder,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { IssueTracker, TaskActivity } from '../types'
import { PROJECT_TRACKERS } from '../lib/trackers'

export const SyncView: React.FC = () => {
  const {
    currentProject,
    updateProject,
    setIsProjectModalOpen,
    setEditingProject,
    tasks,
    activities,
    setSelectedActivity,
    setActiveView,
    settings,
    updateSettings,
    syncCurrentProject,
    isSyncing,
    activeJobCount,
    t,
  } = useApp()

  // Active project issue tracker
  const activeTracker: IssueTracker = currentProject?.issueTracker || 'local'

  // Local form state initialized from active project or fallback to global settings
  const [githubRepo, setGithubRepo] = useState(currentProject?.githubRepo || settings.githubRepo || '')
  const [jiraKey, setJiraKey] = useState(currentProject?.jiraProject || settings.jiraProject || '')
  const [issueTracker, setIssueTracker] = useState<IssueTracker>(activeTracker)
  const [isSaved, setIsSaved] = useState(false)

  // Custom parameters for manual triggers on the active project
  const [customGithubRepo, setCustomGithubRepo] = useState(currentProject?.githubRepo || settings.githubRepo || '')
  const [customJiraKey, setCustomJiraKey] = useState(currentProject?.jiraProject || settings.jiraProject || '')

  // Keep form updated when currentProject changes
  React.useEffect(() => {
    if (currentProject) {
      setGithubRepo(currentProject.githubRepo || '')
      setJiraKey(currentProject.jiraProject || '')
      setIssueTracker(currentProject.issueTracker || 'local')
      setCustomGithubRepo(currentProject.githubRepo || '')
      setCustomJiraKey(currentProject.jiraProject || '')
    }
  }, [currentProject])

  const handleSaveOptions = async (e: React.FormEvent) => {
    e.preventDefault()
    if (currentProject) {
      await updateProject(currentProject.id, {
        githubRepo: githubRepo.trim(),
        jiraProject: jiraKey.trim().toUpperCase(),
        issueTracker,
      })
    }
    await updateSettings({
      githubRepo: githubRepo.trim(),
      jiraProject: jiraKey.trim().toUpperCase(),
      issueTracker,
    })
    setIsSaved(true)
    setTimeout(() => setIsSaved(false), 3000)
  }

  // Filter activities that are sync related for this project
  const syncActivities = activities.filter(
    a => (a.skillId === 'sync_github' || a.skillId === 'sync_jira' || a.skillId === 'sync_all' || a.skillId.startsWith('sync')) &&
         (!currentProject || !a.projectId || a.projectId === currentProject.id)
  )

  const githubCount = tasks.filter(t => t.source === 'github').length
  const jiraCount = tasks.filter(t => t.source === 'jira').length
  const gitlabCount = tasks.filter(t => t.source === 'gitlab').length
  const localCount = tasks.filter(t => !t.source || t.source === 'local').length

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
            <RefreshCw size={11} className="animate-spin" /> En cours
          </span>
        )
      case 'queued':
      case 'pending':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30">
            <Clock size={11} /> En file d'attente
          </span>
        )
      case 'completed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <CheckCircle2 size={11} /> Terminé
          </span>
        )
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
            <AlertCircle size={11} /> Échoué
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
            {status}
          </span>
        )
    }
  }

  const handleInspectActivity = (act: TaskActivity) => {
    setSelectedActivity(act)
    setActiveView('activities')
  }

  return (
    <div className="flex-1 flex flex-col h-full bg-[var(--bg-primary)] overflow-y-auto">
      {/* Header */}
      <div className="border-b border-[var(--border-color)] bg-[var(--bg-secondary)] px-6 py-5 shrink-0">
        <div className="max-w-5xl mx-auto flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-[var(--accent-light)] text-[var(--accent-color)] border border-[var(--accent-color)]/30">
                <RefreshCw size={20} className={isSyncing ? 'animate-spin' : ''} />
              </div>
              <div>
                <h1 className="text-xl font-bold text-[var(--text-primary)] flex items-center gap-2">
                  {t.syncView.title}
                  <span className="text-xs px-2 py-0.5 rounded-full font-mono font-medium bg-[var(--accent-light)] accent-text border border-[var(--accent-color)]/30">
                    {currentProject?.name || 'Projet Actif'}
                  </span>
                </h1>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">
                  Synchronisation ciblée pour le gestionnaire de tickets du projet en cours ({activeTracker.toUpperCase()})
                </p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {activeJobCount > 0 && (
              <button
                onClick={() => setActiveView('activities')}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold bg-blue-500/15 text-blue-400 border border-blue-500/30 hover:bg-blue-500/25 transition-all animate-pulse"
              >
                <ActivityIcon size={14} className="animate-spin" />
                <span>{activeJobCount} job(s) actif(s)</span>
                <ChevronRight size={13} />
              </button>
            )}

            <button
              onClick={syncCurrentProject}
              disabled={isSyncing}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-bold text-white shadow-md accent-bg hover:opacity-90 transition-all disabled:opacity-50 cursor-pointer"
              title={`Synchroniser le projet ${currentProject?.name || ''}`}
            >
              <Zap size={14} className={isSyncing ? 'animate-spin' : ''} />
              <span>
                {activeTracker === 'github'
                  ? 'Synchroniser GitHub'
                  : activeTracker === 'jira'
                  ? 'Synchroniser Jira'
                  : activeTracker === 'gitlab'
                  ? 'Synchroniser GitLab'
                  : 'Recharger les tâches'}
              </span>
            </button>
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 max-w-5xl w-full mx-auto p-6 space-y-6">
        {/* Active Project Scope Banner */}
        {currentProject && (
          <div className="rounded-xl border border-[var(--sidebar-border)] bg-[var(--accent-light)]/20 p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-[var(--accent-light)] border border-[var(--accent-color)]/30 flex items-center justify-center accent-text font-bold text-base shadow-[0_0_12px_var(--accent-glow)] shrink-0">
                ⚡
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold uppercase tracking-wider text-[var(--accent-color)]">Projet Actif</span>
                  <span className="text-[10px] px-2 py-0.2 rounded-full font-mono bg-[var(--bg-tertiary)] text-[var(--text-secondary)]">
                    {tasks.length} tâches
                  </span>
                  <span className={`text-[10px] px-2 py-0.2 rounded-full font-bold uppercase ${
                    activeTracker === 'github'
                      ? 'bg-purple-500/20 text-purple-300 border border-purple-500/30'
                      : activeTracker === 'jira'
                      ? 'bg-blue-500/20 text-blue-300 border border-blue-500/30'
                      : activeTracker === 'gitlab'
                      ? 'bg-orange-500/20 text-orange-300 border border-orange-500/30'
                      : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                  }`}>
                    {activeTracker === 'github' ? 'GitHub Issues' : activeTracker === 'jira' ? 'Jira' : activeTracker === 'gitlab' ? 'GitLab' : 'Local SQLite'}
                  </span>
                </div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  {currentProject.name}
                </h3>
                <p className="text-xs text-[var(--text-muted)] font-mono truncate max-w-lg">
                  {[
                    currentProject.githubRepo ? `GitHub: ${currentProject.githubRepo}` : '',
                    currentProject.jiraProject ? `Jira: ${currentProject.jiraProject}` : '',
                  ].filter(Boolean).join(' · ')}
                </p>
              </div>
            </div>

            <button
              type="button"
              onClick={() => {
                setEditingProject(currentProject)
                setIsProjectModalOpen(true)
              }}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-secondary)] hover:bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border-color)] transition-all cursor-pointer shrink-0"
            >
              ⚙️ Modifier le projet
            </button>
          </div>
        )}

        {/* Current Project Synchronization Card ONLY */}
        {activeTracker === 'github' && (
          <div className="rounded-xl border border-purple-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-purple-500/15 border border-purple-500/30 flex items-center justify-center text-purple-400">
                  <FolderGit2 size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    Synchronisation GitHub Issues
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    Connecté à GitHub · Dépôt {customGithubRepo || currentProject?.githubRepo || 'Non configuré'}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-purple-500/15 text-purple-300 font-bold border border-purple-500/30">
                {githubCount} issues synchronisées
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              Synchronise les issues ouvertes et Pull Requests du dépôt GitHub associé à ce projet via la CLI GitHub officielle.
            </p>
          </div>
        )}

        {activeTracker === 'jira' && (
          <div className="rounded-xl border border-blue-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-blue-500/15 border border-blue-500/30 flex items-center justify-center text-blue-400 font-black font-sans text-lg">
                  J
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    Synchronisation Jira
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    Connecté à Jira · Projet {customJiraKey || currentProject?.jiraProject || 'Non défini'}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-blue-500/15 text-blue-300 font-bold border border-blue-500/30">
                {jiraCount} tickets Jira
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              Synchronise les tickets et anomalies de votre projet Jira via l'API REST Atlassian.
            </p>
          </div>
        )}

        {activeTracker === 'gitlab' && (
          <div className="rounded-xl border border-orange-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-orange-500/15 border border-orange-500/30 flex items-center justify-center text-orange-400">
                  <FolderGit2 size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    Synchronisation GitLab
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
                    Connecté à GitLab · Projet {currentProject?.gitlabProject || settings.gitlabProject || 'Non configuré'}
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-orange-500/15 text-orange-300 font-bold border border-orange-500/30">
                {gitlabCount} issues GitLab
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              Synchronise les issues du projet GitLab via son API REST, sur gitlab.com ou une instance auto-hébergée.
            </p>
          </div>
        )}

        {activeTracker === 'local' && (
          <div className="rounded-xl border border-emerald-500/40 bg-[var(--bg-secondary)] p-6 shadow-sm space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-[var(--border-color)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-emerald-500/15 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
                  <Folder size={20} />
                </div>
                <div>
                  <h2 className="text-base font-bold text-[var(--text-primary)]">
                    Gestionnaire Local (SQLite)
                  </h2>
                  <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                    <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
                    Base de données locale active
                  </span>
                </div>
              </div>
              <span className="text-xs px-2.5 py-1 rounded-md font-mono bg-emerald-500/15 text-emerald-300 font-bold border border-emerald-500/30">
                {localCount} tâches locales
              </span>
            </div>

            <p className="text-xs text-[var(--text-secondary)] leading-relaxed">
              Ce projet est configuré pour fonctionner exclusivement en local. Les tâches, spécifications et activités sont stockées directement dans la base SQLite locale sans dépendance à un service externe.
            </p>
          </div>
        )}

        {/* Sync Options Form Section for the Active Project */}
        <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-6 shadow-xs">
          <div className="flex items-center justify-between pb-4 border-b border-[var(--border-color)] mb-5">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-[var(--bg-tertiary)] text-[var(--text-secondary)]">
                <Sliders size={18} />
              </div>
              <div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  Configuration du gestionnaire de tickets du projet
                </h3>
                <p className="text-xs text-[var(--text-muted)]">
                  Modifiez le suivi des tickets et les paramètres propres à {currentProject?.name || 'ce projet'}
                </p>
              </div>
            </div>

            {isSaved && (
              <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 animate-fade-in">
                <Check size={13} /> Enregistré pour ce projet
              </span>
            )}
          </div>

          <form onSubmit={handleSaveOptions} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {/* Issue Tracker Selector */}
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
                  Gestionnaire de tickets du projet
                </label>
                <select
                  value={issueTracker}
                  onChange={e => setIssueTracker(e.target.value as IssueTracker)}
                  className="w-full px-3 py-2 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                >
                  {PROJECT_TRACKERS.map(t => (
                    <option key={t.id} value={t.id}>
                      {t.label}
                    </option>
                  ))}
                </select>
              </div>

              {/* Conditional Tracker Parameter */}
              {issueTracker === 'github' && (
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
                    Dépôt GitHub (owner/repo)
                  </label>
                  <input
                    type="text"
                    value={githubRepo}
                    onChange={e => {
                      setGithubRepo(e.target.value)
                      setCustomGithubRepo(e.target.value)
                    }}
                    placeholder="Ex: owner/repo"
                    className="w-full px-3 py-2 text-xs font-mono rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                  />
                </div>
              )}

              {issueTracker === 'jira' && (
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
                    Clé du projet Jira
                  </label>
                  <input
                    type="text"
                    value={jiraKey}
                    onChange={e => {
                      setJiraKey(e.target.value)
                      setCustomJiraKey(e.target.value)
                    }}
                    placeholder="Ex: PROJ"
                    className="w-full px-3 py-2 text-xs font-mono rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] uppercase"
                  />
                </div>
              )}

              {issueTracker === 'local' && (
                <div className="flex items-center p-2.5 rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-xs text-[var(--text-muted)]">
                  <span>Stockage SQLite autonome sans clé distante requise.</span>
                </div>
              )}
            </div>

            <div className="pt-3 flex justify-end">
              <button
                type="submit"
                className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-bold text-white shadow-xs accent-bg hover:opacity-90 transition-all cursor-pointer"
              >
                <Save size={14} />
                <span>Enregistrer pour ce projet</span>
              </button>
            </div>
          </form>
        </div>

        {/* Recent Sync Activities History for the active project */}
        <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-6 shadow-xs">
          <div className="flex items-center justify-between pb-4 border-b border-[var(--border-color)] mb-4">
            <div className="flex items-center gap-2.5">
              <div className="p-2 rounded-lg bg-indigo-500/15 text-indigo-400">
                <Terminal size={18} />
              </div>
              <div>
                <h3 className="text-sm font-bold text-[var(--text-primary)]">
                  Historique des synchronisations de ce projet
                </h3>
                <p className="text-xs text-[var(--text-muted)]">
                  Journal des tâches de synchronisation exécutées en arrière-plan
                </p>
              </div>
            </div>

            <button
              onClick={() => setActiveView('activities')}
              className="text-xs text-[var(--accent-color)] hover:underline flex items-center gap-1 font-medium cursor-pointer"
            >
              <span>Voir dans Activités</span>
              <ChevronRight size={13} />
            </button>
          </div>

          {syncActivities.length === 0 ? (
            <div className="text-center py-8 text-[var(--text-muted)]">
              <RefreshCw size={24} className="mx-auto mb-2 opacity-40" />
              <p className="text-xs">Aucune activité de synchronisation récente pour ce projet</p>
            </div>
          ) : (
            <div className="divide-y divide-[var(--border-color)]">
              {syncActivities.slice(0, 8).map(act => (
                <div
                  key={act.id}
                  onClick={() => handleInspectActivity(act)}
                  className="py-3.5 px-2 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:bg-[var(--bg-tertiary)]/50 rounded-lg cursor-pointer transition-colors"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-[var(--bg-tertiary)] border border-[var(--border-color)] flex items-center justify-center shrink-0">
                      {act.skillId === 'sync_github' ? (
                        <FolderGit2 size={14} className="text-purple-400" />
                      ) : act.skillId === 'sync_jira' ? (
                        <span className="text-blue-400 font-bold text-xs">J</span>
                      ) : (
                        <RefreshCw size={14} className="text-cyan-400" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[var(--text-primary)] truncate">
                          {act.skillName}
                        </span>
                        {getStatusBadge(act.status)}
                      </div>
                      <p className="text-xs text-[var(--text-secondary)] truncate mt-0.5">
                        {act.summary || act.action}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 shrink-0 text-xs text-[var(--text-muted)] sm:self-center">
                    {act.duration && (
                      <span className="font-mono text-[11px] bg-[var(--bg-tertiary)] px-2 py-0.5 rounded">
                        {act.duration}
                      </span>
                    )}
                    <span className="text-[11px]">
                      {new Date(act.createdAt).toLocaleString(settings.language === 'fr' ? 'fr-FR' : 'en-US', {
                        hour: '2-digit',
                        minute: '2-digit',
                        day: '2-digit',
                        month: 'short',
                      })}
                    </span>
                    <button
                      onClick={e => {
                        e.stopPropagation()
                        handleInspectActivity(act)
                      }}
                      className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)]"
                      title="Inspecter dans Activités"
                    >
                      <ExternalLink size={13} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
