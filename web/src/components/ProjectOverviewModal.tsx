import React, { useEffect, useMemo, useState } from 'react'
import { LayoutGrid, Search, Star, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { accentBadgeStyle } from '../lib/accents'
import { shortElapsed } from '../lib/elapsed'
import { format, plural } from '../lib/i18n'
import { PROJECT_TRACKERS, matchProject, overviewProjects, trackerLabel } from '../lib/projectPicker'
import type { IssueTracker, Project } from '../types'
import { Highlighted, ProjectLocationLine } from './ProjectPicker'
import { renderProjectIcon } from './ProjectIcon'

/**
 * Every project on one screen (#582), for whoever does not remember a name:
 * the picker only lists recents and favorites. Opened from "Browse projects…"
 * or from the picker's "more" links, the latter with the picker's query.
 */
export const ProjectOverviewModal: React.FC = () => {
  const { isProjectOverviewOpen } = useApp()
  // Mounted on open, so each opening starts from its own filter.
  return isProjectOverviewOpen ? <ProjectOverviewDialog /> : null
}

const ProjectOverviewDialog: React.FC = () => {
  const {
    projects,
    projectHistory,
    projectOverviewQuery,
    closeProjectOverview,
    selectedProjectId,
    setSelectedProjectId,
    toggleProjectBookmark,
    settings,
    t,
  } = useApp()
  const strings = t.shell.projectPicker

  const [query, setQuery] = useState(projectOverviewQuery)
  const [tracker, setTracker] = useState<IssueTracker | 'all'>('all')
  const backdrop = useBackdropDismiss(closeProjectOverview)

  const shown = useMemo(
    () => overviewProjects(projects, projectHistory, query, tracker, settings.language),
    [projects, projectHistory, query, tracker, settings.language],
  )
  const openedAt = useMemo(() => new Map(projectHistory.map(entry => [entry.id, entry.openedAt])), [projectHistory])

  const trackerCount = (id: IssueTracker | 'all') =>
    id === 'all' ? projects.length : projects.filter(p => (p.issueTracker || 'local') === id).length

  const openProject = (project: Project) => {
    setSelectedProjectId(project.id)
    closeProjectOverview()
  }

  // Keys stay in the dialog: Escape closes it, and the board's single-letter
  // shortcuts must not fire from a focused card.
  const onKeyDown = (e: React.KeyboardEvent) => {
    e.stopPropagation()
    if (e.key === 'Escape') {
      e.preventDefault()
      closeProjectOverview()
    }
  }

  // A click on the dialog's empty space leaves the focus on the page, where
  // the handler above never hears Escape: the window does.
  useEffect(() => {
    const onWindowKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeProjectOverview()
    }
    window.addEventListener('keydown', onWindowKeyDown)
    return () => window.removeEventListener('keydown', onWindowKeyDown)
  }, [closeProjectOverview])

  const chips: { id: IssueTracker | 'all'; label: string }[] = [
    { id: 'all', label: strings.overviewAllTrackers },
    ...PROJECT_TRACKERS.map(id => ({ id, label: trackerLabel(id) })),
  ]

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150"
      {...backdrop}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="project-overview-title"
        onKeyDown={onKeyDown}
        className="relative w-full max-w-5xl max-h-[calc(var(--app-h)-2rem)] flex flex-col rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 px-5 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <div id="project-overview-title" className="flex items-center gap-2 font-semibold text-xs text-[var(--text-primary)]">
            <div className="w-5 h-5 rounded-md accent-bg text-white flex items-center justify-center">
              <LayoutGrid size={12} />
            </div>
            <span>{strings.overviewTitle}</span>
            <span className="font-mono text-[10px] text-[var(--text-muted)]">
              {plural(settings.language, projects.length, strings.projectCount)}
            </span>
          </div>

          <div className="flex flex-wrap items-center gap-2 min-w-0">
            <div className="relative flex items-center w-60 max-w-full">
              <Search size={12} className="absolute left-2.5 text-[var(--text-muted)] pointer-events-none" />
              <input
                type="text"
                autoFocus
                value={query}
                onChange={e => setQuery(e.target.value)}
                placeholder={strings.overviewFilter}
                aria-label={strings.overviewFilterLabel}
                className="w-full pl-7 pr-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]"
              />
            </div>
            <div role="group" aria-label={strings.overviewTrackers} className="flex flex-wrap gap-1.5">
              {chips.map(chip => (
                <button
                  key={chip.id}
                  type="button"
                  aria-pressed={tracker === chip.id}
                  onClick={() => setTracker(chip.id)}
                  className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full border text-[11px] font-semibold transition-colors cursor-pointer ${
                    tracker === chip.id
                      ? 'border-[var(--accent-color)]/40 bg-[var(--accent-light)] accent-text'
                      : 'border-[var(--border-color)] bg-[var(--bg-primary)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  {chip.label}
                  <span className="font-mono text-[10px] opacity-75">{trackerCount(chip.id)}</span>
                </button>
              ))}
            </div>
            <button
              type="button"
              onClick={closeProjectOverview}
              aria-label={strings.close}
              title={strings.close}
              className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
            >
              <X size={16} />
            </button>
          </div>
        </div>

        <div className="p-5 overflow-y-auto min-h-0">
          {shown.length === 0 ? (
            <div className="py-10 text-center text-xs text-[var(--text-muted)]">{strings.overviewEmpty}</div>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(min(230px,100%),1fr))] gap-2.5">
              {shown.map(p => {
                const match = matchProject(p, query)
                const opened = openedAt.get(p.id)
                const isSel = selectedProjectId === p.id || selectedProjectId === p.slug
                return (
                  <div
                    key={p.id}
                    role="button"
                    tabIndex={0}
                    aria-label={format(strings.openProject, { name: p.name })}
                    onClick={() => openProject(p)}
                    onKeyDown={e => {
                      if (e.target !== e.currentTarget) return
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        openProject(p)
                      }
                    }}
                    className={`group flex flex-col gap-2 p-3 rounded-xl border bg-[var(--bg-primary)] text-left cursor-pointer transition-colors min-w-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-color)] ${
                      isSel ? 'border-[var(--accent-color)]/50' : 'border-[var(--border-color)] hover:border-[var(--accent-color)]/40'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2 min-w-0">
                      <div className="flex items-center gap-2 min-w-0">
                        <div className="w-6 h-6 rounded-lg flex items-center justify-center shrink-0" style={accentBadgeStyle(p.color)}>
                          {renderProjectIcon(p.icon, 13)}
                        </div>
                        <span className="text-[13px] font-bold text-[var(--text-primary)] truncate">
                          <Highlighted text={p.name} query={match?.field === 'name' ? query : ''} />
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={async e => {
                          e.stopPropagation()
                          await toggleProjectBookmark(p.id)
                        }}
                        className={`p-1 rounded-md transition-colors cursor-pointer shrink-0 ${
                          p.bookmarked ? 'text-amber-400 hover:text-amber-500' : 'text-[var(--text-muted)] hover:text-amber-400'
                        }`}
                        title={p.bookmarked ? strings.removeFavorite : strings.addFavorite}
                        aria-label={p.bookmarked ? strings.removeFavorite : strings.addFavorite}
                      >
                        <Star size={13} className={p.bookmarked ? 'fill-current' : ''} />
                      </button>
                    </div>

                    <p className="text-xs text-[var(--text-secondary)] line-clamp-2 min-h-[2.5rem] break-words">
                      <Highlighted text={p.description || ''} query={match?.field === 'description' ? query : ''} />
                    </p>

                    <div className="text-[10.5px] text-[var(--text-muted)] truncate">
                      <ProjectLocationLine project={p} match={match} query={query} />
                    </div>

                    <div className="flex items-center justify-between gap-2 text-[10.5px] text-[var(--text-muted)]">
                      <span>{plural(settings.language, p.taskCount || 0, strings.taskCount)}</span>
                      {opened && (
                        <span className="truncate">
                          {format(strings.openedAgo, { elapsed: shortElapsed(opened, t.shell.elapsed) })}
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
