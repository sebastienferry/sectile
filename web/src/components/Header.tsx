import React, { useRef } from 'react'
import {
  Search,
  Plus,
  X,
  Settings,
  Target,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { Status, Priority } from '../types'

export const Header: React.FC = () => {
  const {
    currentProject,
    projects,
    setIsProjectModalOpen,
    setEditingProject,
    searchQuery,
    setSearchQuery,
    statusFilter,
    setStatusFilter,
    priorityFilter,
    setPriorityFilter,
    labelFilter,
    setLabelFilter,
    pinnedOnly,
    setPinnedOnly,
    activeOnly,
    setActiveOnly,
    sprintFilter,
    setSprintFilter,
    teamFilter,
    setTeamFilter,
    parentFilter,
    setParentFilter,
    assigneeFilter,
    setAssigneeFilter,
    setIsQuickAddOpen,
    t,
  } = useApp()

  const searchInputRef = useRef<HTMLInputElement>(null)

  const hasActiveFilters = Boolean(statusFilter || priorityFilter || labelFilter || sprintFilter || teamFilter || parentFilter || assigneeFilter || searchQuery || pinnedOnly || activeOnly)

  return (
    <header
      className="h-14 border-b border-[var(--header-border)] px-4 flex items-center justify-between gap-3 shrink-0 z-10 select-none text-[var(--header-text)] shadow-xs"
      style={{
        background: 'linear-gradient(90deg, var(--header-accent-tint) 0%, var(--header-bg) 100%)',
      }}
    >
      {/* Left: Project Configuration Button (Settings icon only) */}
      <div className="flex items-center gap-2 shrink-0">
        <button
          type="button"
          onClick={() => {
            setEditingProject(currentProject || projects[0] || null)
            setIsProjectModalOpen(true)
          }}
          className="p-2 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] border border-transparent hover:border-[var(--border-color)] transition-all cursor-pointer shadow-2xs group"
          title={`Configuration du projet : ${currentProject ? currentProject.name : 'Gérer les projets'}`}
        >
          <Settings size={16} className="group-hover:rotate-45 transition-transform duration-300" />
        </button>
      </div>

      {/* Center: Global Search Bar */}
      <div className="flex-1 max-w-lg mx-2 relative flex items-center">
        <div className="absolute left-3 text-[var(--text-muted)] pointer-events-none flex items-center">
          <Search size={15} />
        </div>
        <input
          id="global-search-input"
          ref={searchInputRef}
          type="text"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder={t.header.searchPlaceholder}
          className="w-full pl-9 pr-14 py-1.5 text-xs rounded-lg bg-[var(--header-input-bg)] border border-[var(--header-input-border)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)] focus:ring-1 focus:ring-[var(--accent-color)] transition-all"
        />
        {searchQuery ? (
          <button
            onClick={() => setSearchQuery('')}
            className="absolute right-3 p-0.5 text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors cursor-pointer"
            title="Effacer la recherche (Esc)"
          >
            <X size={14} />
          </button>
        ) : (
          <kbd className="absolute right-2.5 px-1.5 py-0.5 text-[10px] font-mono text-[var(--text-muted)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] rounded pointer-events-none shadow-xs">
            /
          </kbd>
        )}
      </div>

      {/* Middle: Active Filter Chips */}
      {hasActiveFilters && (
        <div className="hidden 2xl:flex items-center gap-1.5 overflow-x-auto max-w-xs">
          {statusFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-[var(--accent-light)] accent-text border border-[var(--accent-color)]/30 shrink-0">
              {t.status[statusFilter as Status]}
              <button onClick={() => setStatusFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {priorityFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-rose-500/20 text-rose-300 border border-rose-500/40 shrink-0">
              {t.priority[priorityFilter as Priority]}
              <button onClick={() => setPriorityFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {labelFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shrink-0">
              #{labelFilter.replace(/^#+/, '')}
              <button onClick={() => setLabelFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {pinnedOnly && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold shrink-0 accent-text bg-[var(--accent-light)] border border-[var(--accent-color)]/40">
              Épinglés
              <button onClick={() => setPinnedOnly(false)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {activeOnly && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold shrink-0 accent-text bg-[var(--accent-light)] border border-[var(--accent-color)]/40">
              En cours
              <button onClick={() => setActiveOnly(false)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {parentFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/40 shrink-0">
              <Target size={10} />
              {parentFilter === '__no_macro__' || parentFilter === 'none' ? 'Sans macro' : parentFilter}
              <button onClick={() => setParentFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {sprintFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shrink-0">
              {sprintFilter}
              <button onClick={() => setSprintFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {teamFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-violet-500/20 text-violet-300 border border-violet-500/40 shrink-0">
              {teamFilter}
              <button onClick={() => setTeamFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
          {assigneeFilter && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 shrink-0">
              {assigneeFilter}
              <button onClick={() => setAssigneeFilter(null)} className="hover:opacity-75 cursor-pointer">
                <X size={11} />
              </button>
            </span>
          )}
        </div>
      )}

      {/* Right: Quick Add */}
      <div className="flex items-center gap-2 shrink-0">
        {/* Quick Add Button (+) */}
        <button
          onClick={() => setIsQuickAddOpen(true)}
          className="flex items-center justify-center w-8 h-8 rounded-lg text-sm font-bold text-white shadow-md accent-bg hover:opacity-90 active:scale-95 transition-all cursor-pointer"
          title={`${t.header.quickAdd} (N)`}
        >
          <Plus size={16} strokeWidth={2.5} />
        </button>
      </div>
    </header>
  )
}
