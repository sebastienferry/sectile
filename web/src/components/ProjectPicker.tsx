import React, { useMemo, useState } from 'react'
import type { RefObject } from 'react'
import { ArrowRight, Clock, LayoutGrid, Layers, Plus, Search, Settings2, Star } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { accentBadgeStyle } from '../lib/accents'
import { format, plural } from '../lib/i18n'
import {
  descriptionExcerpt,
  highlightParts,
  pickerModel,
  projectLocation,
  type PickerRow,
  type ProjectMatch,
} from '../lib/projectPicker'
import type { Project } from '../types'
import { renderProjectIcon } from './ProjectIcon'

/** The query's first occurrence in the text, in a <mark>. */
export const Highlighted: React.FC<{ text: string; query: string }> = ({ text, query }) => (
  <>
    {highlightParts(text, query).map((part, i) =>
      part.match
        ? <mark key={i} className="rounded-sm bg-[var(--accent-color)]/20 text-inherit">{part.text}</mark>
        : <React.Fragment key={i}>{part.text}</React.Fragment>
    )}
  </>
)

/** "Tracker · repository", with the part the query matched highlighted. */
export const ProjectLocationLine: React.FC<{ project: Project; match: ProjectMatch | null; query: string }> = ({ project, match, query }) => {
  const { tracker, key, location, matched } = projectLocation(project, match)
  const mark = (text: string, part: typeof matched) => (matched === part ? <Highlighted text={text} query={query} /> : text)
  return (
    <>
      {mark(tracker, 'tracker')}
      {key && <> <span className="font-mono">{mark(key, 'key')}</span></>}
      {' · '}
      <span className="font-mono">{mark(location, 'location')}</span>
    </>
  )
}

/**
 * The line under a project's name: where the query matched when it matched in
 * the description, the tracker and the repository otherwise.
 */
export const ProjectSubline: React.FC<{ row: PickerRow; query: string }> = ({ row, query }) =>
  row.match.field === 'description'
    ? <Highlighted text={descriptionExcerpt(row.project.description, query)} query={query} />
    : <ProjectLocationLine project={row.project} match={row.match} query={query} />

type PickerOption = { kind: 'project'; row: PickerRow } | { kind: 'more' }

/** What the highlight follows: a project id, or the "more" link. */
const MORE_KEY = '#more'
const optionKey = (option: PickerOption) => (option.kind === 'project' ? option.row.project.id : MORE_KEY)
const optionId = (index: number) => `project-picker-option-${index}`

/**
 * The project switcher's menu (#582): recent projects and favorites when the
 * search is empty, every matching project otherwise, never more rows than fit
 * without a scrollbar. The rest of the projects is one step away, in the
 * overview. Once open, it is driven from its search field with the arrows,
 * Enter and Escape.
 */
export const ProjectPicker: React.FC<{
  onClose: () => void
  /** The switcher button, which gets the focus back when Escape closes the menu. */
  triggerRef: RefObject<HTMLButtonElement | null>
  /** The task count "All projects" shows. */
  allTasksCount: number
}> = ({ onClose, triggerRef, allTasksCount }) => {
  const {
    projects,
    projectHistory,
    selectedProjectId,
    selectedViewId,
    setSelectedProjectId,
    toggleProjectBookmark,
    setEditingProject,
    setIsProjectModalOpen,
    openProjectOverview,
    settings,
    t,
  } = useApp()
  const strings = t.shell.projectPicker

  const [query, setQuery] = useState('')
  // The highlighted option, by key rather than position: a star toggled from
  // the menu reorders the rows, and Enter must still open the row it shows.
  const [activeKey, setActiveKey] = useState<string | null>(null)

  const model = useMemo(
    () => pickerModel(projects, projectHistory, query, settings.language),
    [projects, projectHistory, query, settings.language],
  )
  const searching = query.trim() !== ''
  const hidden = searching ? model.hiddenMatches : model.hiddenFavorites

  // Keyboard order: the rows as drawn, then the "more" link.
  const options: PickerOption[] = [
    ...[...model.recent, ...model.favorites, ...model.others].map(row => ({ kind: 'project' as const, row })),
    ...(hidden > 0 ? [{ kind: 'more' as const }] : []),
  ]
  const active = activeKey === null ? -1 : options.findIndex(option => optionKey(option) === activeKey)
  const setActive = (index: number) => setActiveKey(index >= 0 && options[index] ? optionKey(options[index]) : null)
  const indexOf = (project: Project) =>
    options.findIndex(option => option.kind === 'project' && option.row.project.id === project.id)

  const openProject = (project: Project) => {
    setSelectedProjectId(project.id)
    onClose()
  }

  const openOverview = (filter: string) => {
    openProjectOverview(filter)
    onClose()
  }

  const choose = (option: PickerOption) => {
    if (option.kind === 'project') openProject(option.row.project)
    else openOverview(query.trim())
  }

  const changeQuery = (value: string) => {
    setQuery(value)
    setActive(-1)
  }

  const onSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      e.stopPropagation()
      if (options.length === 0) return
      const step = e.key === 'ArrowDown' ? 1 : -1
      // From no highlight, ↓ goes to the first option and ↑ to the last.
      setActive(active < 0 ? (step > 0 ? 0 : options.length - 1) : (active + step + options.length) % options.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      e.stopPropagation()
      const option = options[active] ?? (searching ? options.find(o => o.kind === 'project') : undefined)
      if (option) choose(option)
    }
  }

  // Escape from anywhere in the menu: the query first, then the menu itself.
  const onMenuKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'Escape') return
    e.preventDefault()
    e.stopPropagation()
    if (query) {
      changeQuery('')
      return
    }
    onClose()
    triggerRef.current?.focus()
  }

  const renderRow = (row: PickerRow) => {
    const p = row.project
    const index = indexOf(p)
    const isSel = selectedProjectId === p.id || selectedProjectId === p.slug
    const isActive = index === active
    return (
      <div
        key={p.id}
        id={optionId(index)}
        role="option"
        aria-selected={isActive}
        onClick={() => openProject(p)}
        onMouseEnter={() => setActive(index)}
        className={`group/item w-full flex items-center justify-between gap-2 p-2 rounded-xl text-xs cursor-pointer transition-all min-w-0 ${
          isSel
            ? 'bg-[var(--accent-light)] accent-text font-bold border border-[var(--accent-color)]/30'
            : isActive
            ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)]'
            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]'
        }`}
      >
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <div className="w-5 h-5 rounded-md flex items-center justify-center shrink-0" style={accentBadgeStyle(p.color)}>
            {renderProjectIcon(p.icon, 12)}
          </div>
          <div className="flex flex-col min-w-0">
            <span className="truncate"><Highlighted text={p.name} query={row.match.field === 'name' ? query : ''} /></span>
            <span className="text-[9px] text-[var(--text-muted)] font-normal truncate">
              <ProjectSubline row={row} query={query} />
            </span>
          </div>
        </div>

        <div className="flex items-center gap-1 shrink-0">
          <button
            type="button"
            onClick={async (e) => {
              e.stopPropagation()
              await toggleProjectBookmark(p.id)
            }}
            className={`p-1 rounded-md transition-colors cursor-pointer ${
              p.bookmarked
                ? 'text-amber-400 hover:text-amber-500'
                : `text-[var(--text-muted)] hover:text-amber-400 focus-visible:opacity-100 ${isActive ? 'opacity-100' : 'opacity-0 group-hover/item:opacity-100'}`
            }`}
            title={p.bookmarked ? strings.removeFavorite : strings.addFavorite}
            aria-label={p.bookmarked ? strings.removeFavorite : strings.addFavorite}
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
              onClose()
            }}
            className={`p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-primary)] focus-visible:opacity-100 transition-opacity cursor-pointer ${
              isActive ? 'opacity-100' : 'opacity-0 group-hover/item:opacity-100'
            }`}
            title={strings.configure}
            aria-label={strings.configure}
          >
            <Settings2 size={12} />
          </button>
        </div>
      </div>
    )
  }

  const sectionLabel = (label: string, icon?: React.ReactNode) => (
    <div aria-hidden="true" className="px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1">
      {icon}
      <span>{label}</span>
    </div>
  )

  const moreIndex = options.findIndex(option => option.kind === 'more')

  return (
    <div
      className="absolute left-0 top-full mt-1.5 w-72 rounded-2xl bg-[var(--bg-secondary)] border border-[var(--sidebar-border)] shadow-2xl p-1.5 z-50 overflow-hidden animate-in fade-in zoom-in-95 duration-150"
      onKeyDown={onMenuKeyDown}
    >
      <div className="px-2.5 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center justify-between">
        <span>{strings.title}</span>
        <span className="font-mono text-[9px]">{plural(settings.language, projects.length, strings.projectCount)}</span>
      </div>

      <div className="px-1.5 py-1">
        <div className="relative flex items-center">
          <Search size={12} className="absolute left-2.5 text-[var(--text-muted)] pointer-events-none" />
          <input
            type="text"
            autoFocus
            role="combobox"
            aria-expanded="true"
            aria-controls="project-picker-listbox"
            aria-autocomplete="list"
            aria-activedescendant={active >= 0 ? optionId(active) : undefined}
            aria-label={strings.searchLabel}
            value={query}
            onChange={e => changeQuery(e.target.value)}
            onKeyDown={onSearchKeyDown}
            placeholder={strings.searchPlaceholder}
            className="w-full pl-7 pr-2 py-1 text-xs rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]"
            onClick={e => e.stopPropagation()}
          />
        </div>
      </div>

      {/* Bounded lists, never a scroll area: the overview holds the rest. */}
      <div id="project-picker-listbox" role="listbox" aria-label={strings.title} className="space-y-0.5 mt-1 min-w-0">
        {searching && options.length === 0 && (
          <div className="px-2 py-3 text-center text-xs text-[var(--text-muted)] space-y-1">
            <p className="break-words">{format(strings.noMatchFor, { query: query.trim() })}</p>
            <p>{strings.searchCovers}</p>
          </div>
        )}

        {model.recent.length > 0 && (
          <div role="group" aria-label={strings.recent} className="space-y-0.5">
            {sectionLabel(strings.recent, <Clock size={10} />)}
            {model.recent.map(renderRow)}
          </div>
        )}

        {model.favorites.length > 0 && (
          <div role="group" aria-label={strings.favorites} className={`space-y-0.5 ${model.recent.length > 0 ? 'mt-1.5' : ''}`}>
            {sectionLabel(strings.favorites, <Star size={10} className="text-amber-400 fill-current" />)}
            {model.favorites.map(renderRow)}
          </div>
        )}

        {model.others.length > 0 && (
          <div role="group" aria-label={strings.otherProjects} className="space-y-0.5 mt-1.5">
            {sectionLabel(strings.otherProjects)}
            {model.others.map(renderRow)}
          </div>
        )}

        {hidden > 0 && (
          <div
            id={optionId(moreIndex)}
            role="option"
            aria-selected={active === moreIndex}
            onClick={() => openOverview(query.trim())}
            onMouseEnter={() => setActive(moreIndex)}
            className={`w-full flex items-center gap-1.5 px-2 py-1.5 rounded-xl text-[11px] font-semibold text-[var(--accent-color)] cursor-pointer transition-colors ${
              active === moreIndex ? 'bg-[var(--accent-light)]' : 'hover:bg-[var(--accent-light)]'
            }`}
          >
            <span className="truncate">
              {plural(settings.language, hidden, searching ? strings.moreMatches : strings.moreFavorites)}
            </span>
            <ArrowRight size={11} className="shrink-0" />
          </div>
        )}
      </div>

      {options.length > 0 && (
        <div className="px-2.5 pt-1.5 text-[9px] text-[var(--text-muted)] truncate" aria-hidden="true">
          {strings.keyboardHint}
        </div>
      )}

      <div className="my-1 border-t border-[var(--border-color)]"></div>

      {/* All Projects Option (Below bookmarks) */}
      <button
        type="button"
        onClick={() => {
          setSelectedProjectId('all')
          onClose()
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
          <span>{t.shell.statusBar.allProjects}</span>
        </div>
        <span className="text-[10px] font-mono opacity-75">{allTasksCount}</span>
      </button>

      <button
        type="button"
        onClick={() => openOverview('')}
        className="w-full flex items-center justify-between p-2 rounded-xl text-xs transition-all cursor-pointer text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] hover:text-[var(--text-primary)]"
      >
        <div className="flex items-center gap-2">
          <div className="w-5 h-5 rounded-md bg-[var(--accent-color)]/20 text-[var(--accent-color)] flex items-center justify-center">
            <LayoutGrid size={12} />
          </div>
          <span>{strings.browse}</span>
        </div>
        <span className="text-[10px] font-mono opacity-75">{projects.length}</span>
      </button>

      <div className="my-1 border-t border-[var(--border-color)]"></div>

      {/* Add New Project Button */}
      <button
        type="button"
        onClick={() => {
          setEditingProject(null)
          setIsProjectModalOpen(true)
          onClose()
        }}
        className="w-full flex items-center gap-2 p-2 rounded-xl text-xs font-semibold text-[var(--accent-color)] hover:bg-[var(--accent-light)] transition-colors cursor-pointer"
      >
        <Plus size={14} />
        <span>{strings.newProject}</span>
      </button>
    </div>
  )
}
