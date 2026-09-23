import React, { useState, useRef, useCallback } from 'react'
import {
  Inbox,
  HelpCircle,
  FileCode,
  Flame,
  CheckCircle2,
  Columns,
  ListFilter,
  Activity,
  Map,
  Clock,
  Inbox as TriageIcon,
  Settings,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  ShieldCheck,
  User,
  Users,
  Layers,
  RefreshCw,
  Folder,
  Terminal,
  Zap,
  Box,
  Code2,
  Cpu,
  Sparkles,
  Workflow,
  Plus,
  Settings2,
  FileCode2,
  Shield,
  Search,
  Star,
  Bookmark,
  Pencil,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useClickOutside } from '../hooks/useClickOutside'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { accentBadgeStyle } from '../lib/accents'
import { enabledOptionalViews } from '../lib/optionalViews'
import type { Status, TaskSource } from '../types'
import { SectileLogo } from './SectileLogo'

const renderProjectIcon = (iconName: string, size = 15, className = '') => {
  switch (iconName) {
    case 'Terminal': return <Terminal size={size} className={className} />
    case 'Zap': return <Zap size={size} className={className} />
    case 'Flame': return <Flame size={size} className={className} />
    case 'Layers': return <Layers size={size} className={className} />
    case 'Box': return <Box size={size} className={className} />
    case 'Code2': return <Code2 size={size} className={className} />
    case 'Cpu': return <Cpu size={size} className={className} />
    case 'Sparkles': return <Sparkles size={size} className={className} />
    case 'Workflow': return <Workflow size={size} className={className} />
    default: return <Folder size={size} className={className} />
  }
}

/**
 * Section repliable de la barre latérale. Le repli est mémorisé par section : la
 * barre porte cinq listes, et personne ne les regarde toutes en même temps.
 */
const SidebarSection: React.FC<{
  id: string
  title: string
  collapsedBar: boolean
  children: React.ReactNode
  action?: React.ReactNode
}> = ({ id, title, collapsedBar, children, action }) => {
  const [isOpen, setIsOpen] = useState<boolean>(() => {
    try {
      const val = localStorage.getItem(`sectile_sidebar_section_${id}`) ?? localStorage.getItem(`taskacao_sidebar_section_${id}`)
      return val !== 'closed'
    } catch {
      return true
    }
  })

  const toggle = () => {
    setIsOpen(prev => {
      const next = !prev
      try {
        localStorage.setItem(`sectile_sidebar_section_${id}`, next ? 'open' : 'closed')
      } catch {
        // stockage indisponible : le repli vaut pour cette session
      }
      return next
    })
  }

  // Barre repliée : les titres disparaissent déjà, replier n'aurait plus de sens.
  if (collapsedBar) {
    return <div>{children}</div>
  }

  return (
    <div>
      <div className="flex items-center gap-1 px-2 pb-1">
        <button
          type="button"
          onClick={toggle}
          className="flex items-center gap-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          title={isOpen ? `Replier ${title}` : `Déplier ${title}`}
        >
          <ChevronDown
            size={11}
            className={`transition-transform ${isOpen ? '' : '-rotate-90'}`}
          />
          {title}
        </button>
        {action && <span className="ml-auto">{action}</span>}
      </div>
      {isOpen && children}
    </div>
  )
}

export const Sidebar: React.FC = () => {
  const {
    projects,
    selectedProjectId,
    setSelectedProjectId,
    currentProject,
    setIsProjectModalOpen,
    setEditingProject,
    toggleProjectBookmark,
    activities,
    activeJobCount,
    activeView,
    setActiveView,
    statusFilter,
    setStatusFilter,
    priorityFilter,
    setPriorityFilter,
    labelFilter,
    setLabelFilter,
    assigneeFilter,
    setAssigneeFilter,
    sourceFilter,
    setSourceFilter,
    sidebarCollapsed,
    setSidebarCollapsed,
    setIsProfileOpen,
    setIsAdminOpen,
    isSyncing,
    settings,
    teams,
    tasks,
    taskFacets,
    boardGrouping,
    trackerStatusFilters,
    setTrackerStatusFilters,
    boardViews,
    selectedViewId,
    openBoardView,
    openBoardViewModal,
    t,
  } = useApp()

  const { user: currentUser } = useCurrentUser()

  const [isProjectDropdownOpen, setIsProjectDropdownOpen] = useState(false)
  const [projectSearch, setProjectSearch] = useState('')
  const projectDropdownRef = useRef<HTMLDivElement>(null)

  const closeProjectDropdown = useCallback(() => {
    setIsProjectDropdownOpen(false)
    setProjectSearch('')
  }, [])
  useClickOutside(projectDropdownRef, closeProjectDropdown)

  const { searchBookmarked, searchOthers } = React.useMemo(() => {
    const q = projectSearch.trim().toLowerCase()
    if (!q) {
      return {
        searchBookmarked: projects.filter(p => p.bookmarked),
        searchOthers: [],
      }
    }
    const matches = projects.filter(p =>
      p.name.toLowerCase().includes(q) ||
      p.slug.toLowerCase().includes(q) ||
      (p.description && p.description.toLowerCase().includes(q)) ||
      (p.repoPath && p.repoPath.toLowerCase().includes(q))
    )
    return {
      searchBookmarked: matches.filter(p => p.bookmarked),
      searchOthers: matches.filter(p => !p.bookmarked),
    }
  }, [projects, projectSearch])

  /**
   * Les compteurs viennent des facettes du serveur, qui ignorent les filtres et
   * ne connaissent que le projet. Les calculer sur la liste affichée les faisait
   * fondre à mesure qu'on filtrait, jusqu'à zéro en croisant deux filtres, et un
   * compteur de filtre qui compte le résultat du filtre ne sert à rien.
   */
  const facetCount = (values: { value: string; count: number }[], ...keys: string[]): number =>
    values.filter(v => keys.includes(v.value)).reduce((sum, v) => sum + v.count, 0)

  /**
   * Les facettes peuvent manquer : serveur d'une version antérieure, appel en
   * échec, ou premier rendu avant leur arrivée. Dans ce cas les compteurs se
   * calculent sur la liste affichée, comme avant. C'est moins juste dès qu'un
   * filtre est posé, mais un compteur approximatif vaut mieux qu'un zéro
   * partout.
   */
  const hasStatusFacets = taskFacets.statuses.length > 0
  const hasSourceFacets = taskFacets.sources.length > 0
  const totalCount = taskFacets.total || tasks.length

  const counts: Record<string, number> = hasStatusFacets
    ? {
        all: totalCount,
        to_clarify: facetCount(taskFacets.statuses, 'to_clarify', 'backlog'),
        clarified: facetCount(taskFacets.statuses, 'clarified'),
        to_implement: facetCount(taskFacets.statuses, 'to_implement', 'in_progress', 'specified'),
        to_test: facetCount(taskFacets.statuses, 'to_test', 'to_validate'),
        to_close: facetCount(taskFacets.statuses, 'to_close'),
        finished: facetCount(taskFacets.statuses, 'finished', 'done'),
      }
    : {
        all: tasks.length,
        to_clarify: tasks.filter(t => t.status === 'to_clarify' || t.status === 'backlog').length,
        clarified: tasks.filter(t => t.status === 'clarified').length,
        to_implement: tasks.filter(t => t.status === 'to_implement' || t.status === 'in_progress' || t.status === 'specified').length,
        to_test: tasks.filter(t => t.status === 'to_test' || t.status === 'to_validate').length,
        to_close: tasks.filter(t => t.status === 'to_close').length,
        finished: tasks.filter(t => t.status === 'finished' || t.status === 'done').length,
      }

  // Tracker origin counts, used by the source filter below the quick filters.
  const sourceCounts: Record<'all' | TaskSource, number> = hasSourceFacets
    ? {
        all: totalCount,
        github: facetCount(taskFacets.sources, 'github'),
        jira: facetCount(taskFacets.sources, 'jira'),
        local: facetCount(taskFacets.sources, 'local'),
      }
    : {
        all: tasks.length,
        github: tasks.filter(t => t.source === 'github').length,
        jira: tasks.filter(t => t.source === 'jira').length,
        local: tasks.filter(t => !t.source || t.source === 'local').length,
      }

  const sourceItems: { id: 'all' | TaskSource; label: string; icon: string; color: string }[] = [
    { id: 'all', label: t.nav.allSources, icon: '◎', color: 'text-slate-400' },
    { id: 'github', label: 'GitHub', icon: '⑄', color: 'text-slate-300' },
    { id: 'jira', label: 'Jira', icon: '◆', color: 'text-blue-400' },
    { id: 'local', label: t.nav.localSource, icon: '▤', color: 'text-emerald-400' },
  ]

  const workflowItems: { status: Status | null; label: string; stageLabel: string; stageColor: string; icon: React.ReactNode; count: number; color: string }[] = [
    { status: null, label: t.nav.allTasks, stageLabel: '', stageColor: '', icon: <Inbox size={16} />, count: counts.all, color: 'text-slate-400' },
    { status: 'to_clarify', label: t.status.to_clarify, stageLabel: '#new', stageColor: 'bg-cyan-500/15 text-cyan-400 border-cyan-500/30', icon: <Sparkles size={16} />, count: counts.to_clarify, color: 'text-cyan-400' },
    { status: 'clarified', label: t.status.clarified, stageLabel: '#clarified', stageColor: 'bg-amber-500/15 text-amber-400 border-amber-500/30', icon: <HelpCircle size={16} />, count: counts.clarified, color: 'text-amber-400' },
    { status: 'to_implement', label: t.status.to_implement, stageLabel: '#specified', stageColor: 'bg-blue-500/15 text-blue-400 border-blue-500/30', icon: <FileCode size={16} />, count: counts.to_implement, color: 'text-blue-400' },
    { status: 'to_test', label: t.status.to_test, stageLabel: '#implemented', stageColor: 'bg-indigo-500/15 text-indigo-400 border-indigo-500/30', icon: <Flame size={16} />, count: counts.to_test, color: 'text-indigo-400' },
    { status: 'to_close', label: t.status.to_close, stageLabel: '#reviewed', stageColor: 'bg-purple-500/15 text-purple-400 border-purple-500/30', icon: <ShieldCheck size={16} />, count: counts.to_close, color: 'text-purple-400' },
    { status: 'finished', label: t.status.finished, stageLabel: '#finished', stageColor: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30', icon: <CheckCircle2 size={16} />, count: counts.finished, color: 'text-emerald-400' },
  ]

  // Vues de planification que ce projet affiche. Aucune par défaut : elles
  // s'activent depuis les réglages du projet.
  const optionalViews = enabledOptionalViews(currentProject)

  const isMyTasksActive = assigneeFilter === settings.userName

  /**
   * La barre suit le mode du board : en mode « statuts », les étapes du workflow
   * agentique n'ont pas cours à l'écran, ce sont les colonnes du tracker qui
   * découpent le travail. Le filtre posé est celui des statuts, le même que la
   * barre d'outils des vues.
   */
  const showTrackerStatuses = boardGrouping === 'status'

  const trackerColumnItems = React.useMemo(() => {
    const columns = (currentProject?.trackerColumns || []).filter(col => !col.hidden)
    // Aucun board importé : chaque statut vaut sa propre entrée. C'est moins
    // structuré que les colonnes du tracker, et bien plus utile qu'une liste
    // d'étapes que l'écran de droite n'utilise pas.
    if (columns.length === 0) {
      return taskFacets.trackerStatuses.map(status => ({
        name: status.value,
        statuses: [status.value],
        count: status.count,
      }))
    }
    return columns.map(col => {
      const statuses = col.statuses || []
      const lowered = statuses.map(st => st.toLowerCase())
      const fromFacets = taskFacets.trackerStatuses
        .filter(st => lowered.includes(st.value.toLowerCase()))
        .reduce((sum, st) => sum + st.count, 0)
      return {
        name: col.name,
        statuses,
        count:
          taskFacets.trackerStatuses.length > 0
            ? fromFacets
            : tasks.filter(t => lowered.includes((t.trackerStatus || '').toLowerCase())).length,
      }
    })
  }, [currentProject?.trackerColumns, taskFacets.trackerStatuses, tasks])

  return (
    <aside
      className={`relative flex flex-col transition-all duration-300 ease-in-out select-none ${
        sidebarCollapsed ? 'w-16' : 'w-64'
      } h-full z-20 shrink-0 shadow-xs`}
      style={{
        background: 'var(--bg-secondary)',
      }}
    >
      {/* Brand header */}
      <div className="flex items-center justify-between h-14 px-3 border-b border-[var(--sidebar-border)]/50 bg-[var(--bg-secondary)]">
        {!sidebarCollapsed ? (
          <>
            <div className="flex items-center gap-2.5 min-w-0 overflow-hidden">
              <div className="p-0.5 rounded-xl bg-[var(--accent-light)] border border-[var(--accent-color)]/30">
                <SectileLogo size={28} className="shrink-0" />
              </div>
              <span className="font-bold tracking-tight text-base text-[var(--text-primary)] truncate">
                {t.app.title}
              </span>
            </div>

            <button
              type="button"
              onClick={() => setSidebarCollapsed(true)}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors shrink-0 cursor-pointer"
              title={t.nav.toggleSidebar || 'Replier'}
            >
              <ChevronLeft size={16} />
            </button>
          </>
        ) : (
          <div className="w-full flex items-center justify-center">
            <button
              type="button"
              onClick={() => setSidebarCollapsed(false)}
              className="w-10 h-10 rounded-xl flex items-center justify-center hover:bg-[var(--bg-tertiary)] transition-all relative group cursor-pointer"
              title={`${t.app.title} - ${t.nav.toggleSidebar || 'Déplier'}`}
            >
              <div className="p-0.5 rounded-lg bg-[var(--accent-light)] border border-[var(--accent-color)]/30 shadow-[0_0_8px_var(--accent-glow)]">
                <SectileLogo size={24} className="shrink-0" />
              </div>
              <span className="absolute -bottom-0.5 -right-0.5 bg-[var(--bg-secondary)] border border-[var(--sidebar-border)] rounded-full p-0.5 text-[var(--text-muted)] group-hover:text-[var(--text-primary)] opacity-0 group-hover:opacity-100 transition-opacity shadow-xs">
                <ChevronRight size={10} />
              </span>
            </button>
          </div>
        )}
      </div>

      {/* Project Switcher Bar */}
      <div className="px-2.5 pt-2.5 pb-1 relative" ref={projectDropdownRef}>
        {!sidebarCollapsed ? (
          <div className="relative">
            <button
              type="button"
              onClick={() => setIsProjectDropdownOpen(prev => !prev)}
              className="w-full flex items-center justify-between p-2 rounded-xl bg-[var(--bg-tertiary)]/70 hover:bg-[var(--bg-tertiary)] border border-[var(--sidebar-border)] hover:border-[var(--accent-color)]/50 transition-all text-left group shadow-xs cursor-pointer"
            >
              <div className="flex items-center gap-2 min-w-0">
                <div
                  className="w-6 h-6 rounded-lg flex items-center justify-center shrink-0 border"
                  style={currentProject
                    ? accentBadgeStyle(currentProject.color)
                    : { color: 'var(--accent-color)', backgroundColor: 'var(--accent-light)', borderColor: 'var(--accent-glow)' }}
                >
                  {currentProject ? renderProjectIcon(currentProject.icon, 13) : <Layers size={13} />}
                </div>
                <div className="flex flex-col min-w-0">
                  <span className="text-xs font-bold text-[var(--text-primary)] truncate">
                    {currentProject ? currentProject.name : 'Tous les projets'}
                  </span>
                  <span className="text-[10px] text-[var(--text-muted)] font-mono truncate">
                    {currentProject ? `${currentProject.taskCount || 0} tâches` : `${projects.length} projets`}
                  </span>
                </div>
              </div>

              <ChevronDown size={14} className={`text-[var(--text-muted)] group-hover:text-[var(--text-primary)] transition-transform duration-200 ${isProjectDropdownOpen ? 'rotate-180' : ''}`} />
            </button>

            {/* Dropdown Menu */}
            {isProjectDropdownOpen && (
              <div className="absolute left-0 top-full mt-1.5 w-72 rounded-2xl bg-[var(--bg-secondary)] border border-[var(--sidebar-border)] shadow-2xl p-1.5 z-50 animate-in fade-in zoom-in-95 duration-150">
                <div className="px-2.5 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center justify-between">
                  <span>Espaces Projets</span>
                  <span className="font-mono text-[9px]">{projects.length} projets</span>
                </div>

                {/* Search Input */}
                <div className="px-1.5 py-1">
                  <div className="relative flex items-center">
                    <Search size={12} className="absolute left-2.5 text-[var(--text-muted)] pointer-events-none" />
                    <input
                      type="text"
                      value={projectSearch}
                      onChange={e => setProjectSearch(e.target.value)}
                      placeholder="Rechercher un projet..."
                      className="w-full pl-7 pr-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]"
                      onClick={e => e.stopPropagation()}
                    />
                  </div>
                </div>

                {/* Project List: Bookmarked projects at the top */}
                <div className="max-h-48 overflow-y-auto space-y-0.5 mt-1">
                  {searchBookmarked.length === 0 && searchOthers.length === 0 ? (
                    <div className="px-2 py-3 text-center text-xs text-[var(--text-muted)]">
                      {projectSearch.trim() ? 'Aucun projet trouvé' : 'Aucun projet favori'}
                    </div>
                  ) : (
                    <>
                      {searchBookmarked.length > 0 && (
                        <div className="space-y-0.5">
                          {projectSearch.trim() && searchOthers.length > 0 && (
                            <div className="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1">
                              <Star size={10} className="text-amber-400 fill-current" />
                              <span>Favoris</span>
                            </div>
                          )}
                          {searchBookmarked.map(p => {
                            const isSel = selectedProjectId === p.id || selectedProjectId === p.slug
                            return (
                              <div
                                key={p.id}
                                onClick={() => {
                                  setSelectedProjectId(p.id)
                                  setIsProjectDropdownOpen(false)
                                  setProjectSearch('')
                                }}
                                className={`group/item w-full flex items-center justify-between p-2 rounded-xl text-xs cursor-pointer transition-all ${
                                  isSel
                                    ? 'bg-[var(--accent-light)] accent-text font-bold border border-[var(--accent-color)]/30'
                                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                                }`}
                              >
                                <div className="flex items-center gap-2 min-w-0">
                                  <div
                                    className="w-5 h-5 rounded-md flex items-center justify-center shrink-0"
                                    style={accentBadgeStyle(p.color)}
                                  >
                                    {renderProjectIcon(p.icon, 12)}
                                  </div>
                                  <div className="flex flex-col min-w-0">
                                    <span className="truncate">{p.name}</span>
                                    <span className="text-[9px] text-[var(--text-muted)] font-mono truncate max-w-[120px]">
                                      {p.repoPath ? p.repoPath.split('/').pop() : ''}
                                    </span>
                                  </div>
                                </div>

                                <div className="flex items-center gap-1">
                                  <button
                                    type="button"
                                    onClick={async (e) => {
                                      e.stopPropagation()
                                      await toggleProjectBookmark(p.id)
                                    }}
                                    className={`p-1 rounded-md transition-colors cursor-pointer ${
                                      p.bookmarked
                                        ? 'text-amber-400 hover:text-amber-500'
                                        : 'text-[var(--text-muted)] hover:text-amber-400 opacity-0 group-hover/item:opacity-100'
                                    }`}
                                    title={p.bookmarked ? 'Retirer des favoris' : 'Ajouter aux favoris'}
                                  >
                                    <Star size={12} className={p.bookmarked ? 'fill-current' : ''} />
                                  </button>
                                  <span className="text-[10px] font-mono px-1.5 py-0.2 rounded-full bg-[var(--bg-primary)] text-[var(--text-muted)]">
                                    {p.taskCount || 0}
                                  </span>
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation()
                                      setEditingProject(p)
                                      setIsProjectModalOpen(true)
                                      setIsProjectDropdownOpen(false)
                                      setProjectSearch('')
                                    }}
                                    className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-primary)] opacity-0 group-hover/item:opacity-100 transition-opacity cursor-pointer"
                                    title="Configurer ce projet"
                                  >
                                    <Settings2 size={12} />
                                  </button>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}

                      {searchOthers.length > 0 && (
                        <div className="space-y-0.5 mt-1.5">
                          <div className="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
                            Autres projets
                          </div>
                          {searchOthers.map(p => {
                            const isSel = selectedProjectId === p.id || selectedProjectId === p.slug
                            return (
                              <div
                                key={p.id}
                                onClick={() => {
                                  setSelectedProjectId(p.id)
                                  setIsProjectDropdownOpen(false)
                                  setProjectSearch('')
                                }}
                                className={`group/item w-full flex items-center justify-between p-2 rounded-xl text-xs cursor-pointer transition-all ${
                                  isSel
                                    ? 'bg-[var(--accent-light)] accent-text font-bold border border-[var(--accent-color)]/30'
                                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                                }`}
                              >
                                <div className="flex items-center gap-2 min-w-0">
                                  <div
                                    className="w-5 h-5 rounded-md flex items-center justify-center shrink-0"
                                    style={accentBadgeStyle(p.color)}
                                  >
                                    {renderProjectIcon(p.icon, 12)}
                                  </div>
                                  <div className="flex flex-col min-w-0">
                                    <span className="truncate">{p.name}</span>
                                    <span className="text-[9px] text-[var(--text-muted)] font-mono truncate max-w-[120px]">
                                      {p.repoPath ? p.repoPath.split('/').pop() : ''}
                                    </span>
                                  </div>
                                </div>

                                <div className="flex items-center gap-1">
                                  <button
                                    type="button"
                                    onClick={async (e) => {
                                      e.stopPropagation()
                                      await toggleProjectBookmark(p.id)
                                    }}
                                    className={`p-1 rounded-md transition-colors cursor-pointer ${
                                      p.bookmarked
                                        ? 'text-amber-400 hover:text-amber-500'
                                        : 'text-[var(--text-muted)] hover:text-amber-400 opacity-0 group-hover/item:opacity-100'
                                    }`}
                                    title={p.bookmarked ? 'Retirer des favoris' : 'Ajouter aux favoris'}
                                  >
                                    <Star size={12} className={p.bookmarked ? 'fill-current' : ''} />
                                  </button>
                                  <span className="text-[10px] font-mono px-1.5 py-0.2 rounded-full bg-[var(--bg-primary)] text-[var(--text-muted)]">
                                    {p.taskCount || 0}
                                  </span>
                                  <button
                                    type="button"
                                    onClick={(e) => {
                                      e.stopPropagation()
                                      setEditingProject(p)
                                      setIsProjectModalOpen(true)
                                      setIsProjectDropdownOpen(false)
                                      setProjectSearch('')
                                    }}
                                    className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-primary)] opacity-0 group-hover/item:opacity-100 transition-opacity cursor-pointer"
                                    title="Configurer ce projet"
                                  >
                                    <Settings2 size={12} />
                                  </button>
                                </div>
                              </div>
                            )
                          })}
                        </div>
                      )}
                    </>
                  )}
                </div>

                <div className="my-1 border-t border-[var(--border-color)]"></div>

                {/* All Projects Option (Below bookmarks) */}
                <button
                  type="button"
                  onClick={() => {
                    setSelectedProjectId('all')
                    setIsProjectDropdownOpen(false)
                    setProjectSearch('')
                  }}
                  className={`w-full flex items-center justify-between p-2 rounded-xl text-xs transition-all cursor-pointer ${
                    selectedProjectId === 'all' && !selectedViewId
                      ? 'bg-[var(--accent-light)] accent-text font-bold border border-[var(--accent-color)]/30'
                      : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  <div className="flex items-center gap-2">
                    <div className="w-5 h-5 rounded-md bg-[var(--accent-color)]/20 text-[var(--accent-color)] flex items-center justify-center">
                      <Layers size={12} />
                    </div>
                    <span>Tous les projets</span>
                  </div>
                  <span className="text-[10px] font-mono opacity-75">{counts.all}</span>
                </button>

                <div className="my-1 border-t border-[var(--border-color)]"></div>

                {/* Add New Project Button */}
                <button
                  type="button"
                  onClick={() => {
                    setEditingProject(null)
                    setIsProjectModalOpen(true)
                    setIsProjectDropdownOpen(false)
                  }}
                  className="w-full flex items-center gap-2 p-2 rounded-xl text-xs font-semibold text-[var(--accent-color)] hover:bg-[var(--accent-light)] transition-colors cursor-pointer"
                >
                  <Plus size={14} />
                  <span>Nouveau projet...</span>
                </button>
              </div>
            )}
          </div>
        ) : (
          <button
            type="button"
            onClick={() => setIsProjectModalOpen(true)}
            className="w-full flex items-center justify-center p-2 rounded-xl bg-[var(--bg-tertiary)]/70 hover:bg-[var(--bg-tertiary)] text-[var(--accent-color)] border border-[var(--sidebar-border)] transition-colors cursor-pointer"
            title={currentProject ? currentProject.name : 'Changer de projet'}
          >
            {currentProject ? renderProjectIcon(currentProject.icon, 16) : <Layers size={16} />}
          </button>
        )}
      </div>

      {/* Navigation & Filters Container */}
      <div className="flex-1 overflow-y-auto px-2.5 py-3 space-y-4">
        {/* Quick Views */}
        <SidebarSection id="views" title="Vues" collapsedBar={sidebarCollapsed}>
          <div className="space-y-0.5">
            {/* 1. Mes tâches */}
            <button
              onClick={() => setAssigneeFilter(isMyTasksActive ? null : settings.userName)}
              className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                isMyTasksActive
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title={`${t.nav.myTasks} (${settings.userName || 'nom non renseigné dans le profil'})`}
            >
              <div className="flex items-center gap-2.5 truncate">
                <User size={15} className="text-cyan-400 shrink-0" />
                {!sidebarCollapsed && <span className="truncate">{t.nav.myTasks}</span>}
              </div>
            </button>

            {/* 3. Backlog */}
            <button
              onClick={() => setActiveView('list')}
              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'list'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title="Backlog"
            >
              <ListFilter size={15} className="shrink-0 text-indigo-400" />
              {!sidebarCollapsed && <span className="truncate">Backlog</span>}
            </button>

            {/* 4. Board */}
            <button
              onClick={() => setActiveView('board')}
              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'board'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title="Board"
            >
              <Columns size={15} className="shrink-0 text-emerald-400" />
              {!sidebarCollapsed && <span className="truncate">Board</span>}
            </button>

            {/* 5. Triage (optionnel, activé par projet) */}
            {optionalViews.includes('triage') && (
            <button
              onClick={() => setActiveView('triage')}
              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'triage'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title={t.nav.triageTooltip}
            >
              <TriageIcon size={15} className="shrink-0 text-rose-400" />
              {!sidebarCollapsed && <span className="truncate">{t.nav.triage}</span>}
            </button>
            )}

            {/* 5a. Roadmap (optionnel, activé par projet) */}
            {optionalViews.includes('roadmap') && (
            <button
              onClick={() => setActiveView('roadmap')}
              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'roadmap'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title={t.nav.roadmapTooltip}
            >
              <Map size={15} className="shrink-0 text-amber-400" />
              {!sidebarCollapsed && <span className="truncate">{t.nav.roadmap}</span>}
            </button>
            )}

            {/* 5b. Timeline (optionnel, activé par projet) */}
            {optionalViews.includes('timeline') && (
            <button
              onClick={() => setActiveView('timeline')}
              className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'timeline'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title={t.nav.timelineTooltip}
            >
              <Clock size={15} className="shrink-0 text-blue-400" />
              {!sidebarCollapsed && <span className="truncate">{t.nav.timeline}</span>}
            </button>
            )}

            {/* 6. Activités */}
            <button
              onClick={() => setActiveView('activities')}
              className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'activities'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title={t.nav.activities}
            >
              <div className="flex items-center gap-2.5 min-w-0 truncate">
                <Activity size={15} className="shrink-0 text-cyan-400" />
                {!sidebarCollapsed && <span className="truncate">{t.nav.activities}</span>}
              </div>
              {!sidebarCollapsed && (
                activeJobCount > 0 ? (
                  <span className="text-[10px] px-1.5 py-0.2 rounded-full font-mono bg-blue-500 text-white font-bold animate-pulse">
                    {activeJobCount}
                  </span>
                ) : activities.length > 0 ? (
                  <span className="text-[10px] px-1.5 py-0.2 rounded-full font-mono bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
                    {activities.length}
                  </span>
                ) : null
              )}
            </button>

            {/* 7. Skills */}
            <button
              onClick={() => setActiveView('skills')}
              className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'skills'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title="Skills du workflow agentique : une par étape, éditables ici"
            >
              <div className="flex items-center gap-2.5 min-w-0 truncate">
                <FileCode2 size={15} className="shrink-0 text-amber-400" />
                {!sidebarCollapsed && <span className="truncate">Skills</span>}
              </div>
            </button>

            {/* 8. Synchro */}
            <button
              onClick={() => setActiveView('sync')}
              className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                activeView === 'sync'
                  ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                  : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
              }`}
              title="Synchro (Synchronisation)"
            >
              <div className="flex items-center gap-2.5 min-w-0 truncate">
                <RefreshCw size={15} className={`shrink-0 text-indigo-400 ${isSyncing ? 'animate-spin' : ''}`} />
                {!sidebarCollapsed && <span className="truncate">Synchro</span>}
              </div>
            </button>
            {/* Équipes : la charge par personne, quand les tickets portent une équipe */}
            {teams.length > 0 && (
              <button
                onClick={() => setActiveView('team')}
                className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeView === 'team'
                    ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                }`}
                title="Charge de l'équipe, personne par personne"
              >
                <span className="flex items-center gap-2.5 min-w-0">
                  <Users size={15} className="shrink-0 text-violet-400" />
                  {!sidebarCollapsed && <span className="truncate">Équipes</span>}
                </span>
                {!sidebarCollapsed && (
                  <span className="text-[9px] font-bold px-1.5 rounded text-violet-300 bg-violet-400/10 border border-violet-400/30">
                    {teams.length}
                  </span>
                )}
              </button>
            )}

            {/* Saved views (#387): personal selections of projects and labels
                over the all-projects board. */}
            {boardViews.length > 0 && <div className="my-1 border-t border-[var(--border-color)]" />}
            {boardViews.map(view => {
              const isActive = selectedViewId === view.id
              return (
                <div key={view.id} className="group relative" data-board-view={view.id}>
                  <button
                    type="button"
                    onClick={() => openBoardView(view.id)}
                    aria-current={isActive ? 'page' : undefined}
                    className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                      isActive
                        ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                    }`}
                    title={view.name}
                  >
                    <Bookmark size={15} className="shrink-0 text-sky-400" />
                    {!sidebarCollapsed && <span className="truncate pr-5">{view.name}</span>}
                  </button>
                  {!sidebarCollapsed && (
                    <button
                      type="button"
                      onClick={() => openBoardViewModal(view)}
                      className={`absolute right-1.5 top-1/2 -translate-y-1/2 p-1 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] ${isActive ? "opacity-100" : "opacity-0"} group-hover:opacity-100 focus:opacity-100 transition-opacity cursor-pointer`}
                      title={t.boardViews.editView}
                      aria-label={`${t.boardViews.editView} ${view.name}`}
                    >
                      <Pencil size={12} />
                    </button>
                  )}
                </div>
              )
            })}
            <button
              type="button"
              onClick={() => openBoardViewModal(null)}
              className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-xs font-medium text-[var(--text-muted)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)] transition-all cursor-pointer"
              title={t.boardViews.newView}
            >
              <Plus size={15} className="shrink-0" />
              {!sidebarCollapsed && <span className="truncate">{t.boardViews.newView}</span>}
            </button>
          </div>
        </SidebarSection>

        {/* Étapes ou statuts : la barre suit le mode du board, sinon elle propose
            un découpage que l'écran de droite n'utilise pas. */}
        <SidebarSection
          id="stages"
          title={showTrackerStatuses ? t.list.columns.status : 'Agentic Workflow'}
          collapsedBar={sidebarCollapsed}
        >
          {showTrackerStatuses ? (
            <div className="space-y-0.5">
              {trackerColumnItems.map(item => {
                const isActive =
                  item.statuses.length > 0 &&
                  item.statuses.every(st => trackerStatusFilters.includes(st)) &&
                  trackerStatusFilters.length === item.statuses.length
                return (
                  <button
                    key={item.name}
                    onClick={() => setTrackerStatusFilters(isActive ? [] : item.statuses)}
                    className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                      isActive
                        ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                    }`}
                    title={`${item.name} : ${item.statuses.join(', ')}`}
                  >
                    <div className="flex items-center gap-2 truncate">
                      <Columns size={14} className="shrink-0 text-cyan-400" />
                      {!sidebarCollapsed && <span className="truncate">{item.name}</span>}
                    </div>
                    {!sidebarCollapsed && (
                      <span
                        className={`text-[10px] px-1.5 py-0.2 rounded-full font-mono font-bold ${
                          isActive
                            ? 'bg-[var(--accent-color)] text-white shadow-xs'
                            : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)]'
                        }`}
                      >
                        {item.count}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          ) : (
          <div className="space-y-0.5">
            {workflowItems.map(item => {
              const isActive = statusFilter === item.status && !assigneeFilter && !priorityFilter && !labelFilter
              return (
                <button
                  key={item.status || 'all'}
                  onClick={() => {
                    setStatusFilter(item.status)
                    setAssigneeFilter(null)
                    setPriorityFilter(null)
                    setLabelFilter(null)
                  }}
                  className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                    isActive
                      ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                      : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                  }`}
                  title={item.label}
                >
                  <div className="flex items-center gap-2 truncate">
                    <span className={`${item.color} shrink-0`}>{item.icon}</span>
                    {!sidebarCollapsed && (
                      <div className="flex items-center gap-1.5 truncate">
                        <span className="truncate">{item.label}</span>
                        {item.stageLabel && (
                          <span className={`text-[9px] font-bold px-1 py-0 rounded border ${item.stageColor}`}>
                            {item.stageLabel}
                          </span>
                        )}
                      </div>
                    )}
                  </div>
                  {!sidebarCollapsed && (
                    <span
                      className={`text-[10px] px-1.5 py-0.2 rounded-full font-mono font-bold ${
                        isActive
                          ? 'bg-[var(--accent-color)] text-white shadow-xs'
                          : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)]'
                      }`}
                    >
                      {item.count}
                    </span>
                  )}
                </button>
              )
            })}
          </div>
          )}
        </SidebarSection>

        {/* Tracker Origin Filter (GitHub / Jira / Local) */}
        <div>
          {!sidebarCollapsed && (
            <div className="px-2 pb-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
              {t.nav.sources}
            </div>
          )}
          <div className="space-y-0.5">
            {sourceItems.map(item => {
              const isActive = sourceFilter === item.id
              if (item.id !== 'all' && sourceCounts[item.id] === 0 && !isActive) return null
              return (
                <button
                  key={item.id}
                  onClick={() => setSourceFilter(item.id)}
                  className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                    isActive
                      ? 'bg-[var(--accent-light)] accent-text font-bold shadow-xs'
                      : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
                  }`}
                  title={item.label}
                >
                  <div className="flex items-center gap-2.5 truncate">
                    <span className={`${item.color} shrink-0 font-black font-mono w-[15px] text-center`}>{item.icon}</span>
                    {!sidebarCollapsed && <span className="truncate">{item.label}</span>}
                  </div>
                  {!sidebarCollapsed && (
                    <span
                      className={`text-[10px] px-1.5 py-0.2 rounded-full font-mono font-bold ${
                        isActive
                          ? 'bg-[var(--accent-color)] text-white shadow-xs'
                          : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)]'
                      }`}
                    >
                      {sourceCounts[item.id]}
                    </span>
                  )}
                </button>
              )
            })}
          </div>
        </div>

      </div>

      {/* Footer Profile & Settings */}
      <div className="p-2 border-t border-[var(--sidebar-border)]/50 bg-[var(--accent-light)]/10 backdrop-blur-xs space-y-1">
        {currentUser?.role === 'admin' && (
          <button
            type="button"
            onClick={() => setIsAdminOpen(true)}
            className="w-full flex items-center gap-2.5 p-2 rounded-xl hover:bg-[var(--bg-tertiary)] text-[var(--text-primary)] transition-colors group text-left cursor-pointer"
            title="Administration"
          >
            <div className="w-7 h-7 rounded-full flex items-center justify-center font-bold text-xs bg-amber-500/20 text-amber-500 shadow-xs shrink-0">
              <Shield size={14} />
            </div>
            {!sidebarCollapsed && (
              <div className="flex-1 min-w-0">
                <div className="text-xs font-semibold truncate text-[var(--text-primary)] flex items-center gap-1.5">
                  <span>Administration</span>
                  <span className="text-[9px] px-1 py-0.2 rounded bg-amber-500/20 text-amber-500 font-bold uppercase tracking-wider">Admin</span>
                </div>
                <div className="text-[10px] text-[var(--text-muted)] truncate">
                  Utilisateurs & rôles
                </div>
              </div>
            )}
          </button>
        )}
        <button
          onClick={() => setIsProfileOpen(true)}
          className="w-full flex items-center gap-2.5 p-2 rounded-xl hover:bg-[var(--bg-tertiary)] text-[var(--text-primary)] transition-colors group text-left cursor-pointer"
          title={t.nav.settings}
        >
          <div className="w-7 h-7 rounded-full flex items-center justify-center font-bold text-xs accent-bg text-white shadow-md shrink-0">
            {settings.userName ? settings.userName.substring(0, 2).toUpperCase() : null}
          </div>
          {!sidebarCollapsed && (
            <div className="flex-1 min-w-0">
              <div className="text-xs font-semibold truncate text-[var(--text-primary)]">
                {settings.userName}
              </div>
              <div className="text-[10px] text-[var(--text-muted)] truncate">
                {settings.userEmail}
              </div>
            </div>
          )}
          {!sidebarCollapsed && (
            <Settings size={15} className="text-[var(--text-muted)] group-hover:text-[var(--text-primary)] transition-transform group-hover:rotate-45" />
          )}
        </button>

      </div>
    </aside>
  )
}
