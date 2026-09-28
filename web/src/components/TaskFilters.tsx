import React from 'react'
import { Flame, Calendar, Layers, Pin, User, SlidersHorizontal, Check, Shapes, Settings2, Target, Loader2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useClickOutside } from '../hooks/useClickOutside'
import { LookupField, type LookupOption } from './LookupField'
import { valueLookup } from '../lib/lookups'
import { matchesSearch } from '../lib/searchFold'
import { PRIORITY_COLORS, PRIORITY_LEVELS } from '../lib/priority'
import { format, plural } from '../lib/i18n'

/**
 * Les filtres de tri transversaux, posés dans la barre d'outils de chaque vue
 * plutôt que dans l'en-tête global, pour rester à côté du contenu qu'ils
 * filtrent. Un seul composant partagé par le board et la liste : deux copies
 * finiraient par diverger.
 *
 * Sprint et Équipe n'apparaissent que si le tracker alimente réellement ces
 * champs, ce que dit l'endpoint des facettes. Un projet local ou GitHub n'affiche
 * donc que la priorité.
 *
 * Le filtre par personne se restreint à l'équipe sélectionnée quand il y en a
 * une, et propose alors ses membres même ceux qui ne portent encore aucun
 * ticket : c'est ce qui rend une charge vide visible.
 */
export const TaskFilters: React.FC = () => {
  const {
    priorityFilter,
    setPriorityFilter,
    taskFacets,
    sprintFilter,
    setSprintFilter,
    teamFilter,
    setTeamFilter,
    assigneeFilter,
    setAssigneeFilter,
    parentFilter,
    setParentFilter,
    availableParents,
    trackerStatusFilters,
    setTrackerStatusFilters,
    issueTypeFilters,
    setIssueTypeFilters,
    currentProject,
    setEditingProject,
    setIsProjectModalOpen,
    availableAssignees,
    unassignedFilterValue,
    pinnedOnly,
    setPinnedOnly,
    pinnedTasks,
    activeOnly,
    setActiveOnly,
    activeTasks,
    t,
    settings,
  } = useApp()
  const F = t.shell.filters
  const lang = settings.language

  const [isStatusMenuOpen, setIsStatusMenuOpen] = React.useState(false)
  const statusMenuRef = React.useRef<HTMLDivElement>(null)
  const [isTypeMenuOpen, setIsTypeMenuOpen] = React.useState(false)
  const typeMenuRef = React.useRef<HTMLDivElement>(null)

  const closeTypeMenu = React.useCallback(() => setIsTypeMenuOpen(false), [])
  useClickOutside(typeMenuRef, closeTypeMenu, isTypeMenuOpen)

  // Fermeture au clic extérieur : ce menu vit dans une barre d'outils dense.
  const closeStatusMenu = React.useCallback(() => setIsStatusMenuOpen(false), [])
  useClickOutside(statusMenuRef, closeStatusMenu, isStatusMenuOpen)

  // Les valeurs proposées sont celles que le board porte réellement, filtrées en
  // mémoire : elles arrivent déjà avec les facettes, aucun appel n'est utile.
  const searchSprintValue = React.useMemo(() => valueLookup(taskFacets.sprints), [taskFacets.sprints])
  const searchTeamValue = React.useMemo(() => valueLookup(taskFacets.teams), [taskFacets.teams])
  const searchAssigneeValue = React.useMemo(() => {
    const base = valueLookup(availableAssignees)
    return async (query: string) => {
      const people = await base(query)
      // « Non assigné » est une valeur de filtre à part entière, et c'est souvent
      // la plus utile : elle est proposée en tête tant qu'il y a de quoi la
      // remplir.
      if (taskFacets.unassignedCount > 0 && matchesSearch(query, F.unassigned)) {
        return [
          { id: unassignedFilterValue, label: F.unassigned, sublabel: plural(lang, taskFacets.unassignedCount, F.ticketCount) },
          ...people,
        ]
      }
      return people
    }
  }, [availableAssignees, taskFacets.unassignedCount, unassignedFilterValue, F, lang])

  const searchMacroValue = React.useMemo(() => {
    const macroList: Array<{ id: string; label: string; sublabel?: string }> = []
    const seen = new Set<string>()

    if (taskFacets.macros && taskFacets.macros.length > 0) {
      for (const m of taskFacets.macros) {
        if (!m.key || seen.has(m.key)) continue
        seen.add(m.key)
        macroList.push({
          id: m.key,
          label: m.title ? `${m.key} · ${m.title}` : m.key,
          sublabel: m.count ? plural(lang, m.count, F.ticketCount) : undefined,
        })
      }
    }
    for (const p of availableParents) {
      if (!p.key || seen.has(p.key)) continue
      seen.add(p.key)
      macroList.push({
        id: p.key,
        label: p.title ? `${p.key} · ${p.title}` : p.key,
        sublabel: p.count ? plural(lang, p.count, F.ticketCount) : undefined,
      })
    }

    return async (query: string): Promise<LookupOption[]> => {
      const filtered = macroList.filter(m => matchesSearch(query, m.label, m.id))
      if (taskFacets.noMacroCount > 0 && matchesSearch(query, F.noMacro, F.noMilestone)) {
        return [
          { id: '__no_macro__', label: F.noMacro, sublabel: plural(lang, taskFacets.noMacroCount, F.ticketCount) },
          ...filtered,
        ]
      }
      return filtered
    }
  }, [taskFacets.macros, taskFacets.noMacroCount, availableParents, F, lang])

  const selectedMacroLabel = React.useMemo(() => {
    if (!parentFilter) return ''
    if (parentFilter === '__no_macro__' || parentFilter === 'none') return F.noMacro
    const found = taskFacets.macros?.find(m => m.key === parentFilter)
    if (found) return found.title ? `${found.key} · ${found.title}` : found.key
    const foundParent = availableParents.find(p => p.key === parentFilter)
    if (foundParent) return foundParent.title ? `${foundParent.key} · ${foundParent.title}` : foundParent.key
    return parentFilter
  }, [parentFilter, taskFacets.macros, availableParents, F])

  return (
    <div className="flex items-center gap-2 shrink-0">
      {/* Épinglés : le retour immédiat aux chantiers en cours quand le board en
          porte trois cents. Le filtre est tenu par le serveur, sur la colonne
          indexée, donc il vaut aussi pour la recherche et les autres filtres. */}
      <button
        type="button"
        onClick={() => setPinnedOnly(!pinnedOnly)}
        disabled={!pinnedOnly && pinnedTasks.length === 0}
        className={`flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed ${
          pinnedOnly
            ? 'accent-text bg-[var(--accent-light)] border-[var(--accent-color)]/50 font-bold'
            : 'bg-[var(--bg-secondary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
        }`}
        title={
          pinnedTasks.length === 0
            ? F.noPinned
            : pinnedOnly
              ? t.shell.board.showAll
              : plural(lang, pinnedTasks.length, F.onlyPinned)
        }
      >
        <Pin size={12} />
        <span>{t.shell.pinned.title}</span>
        {pinnedTasks.length > 0 && (
          <span className="font-mono text-[10px] opacity-70">{pinnedTasks.length}</span>
        )}
      </button>

      {/* In progress: the answer to the question the cards' badge asks (what is
          running right now?) without scanning three hundred tickets for a
          coloured mark. The client holds the filter, over the activities already
          loaded for that badge: a run's state is not a ticket column, so a run
          that ends drops its ticket when the activities refresh, not the tickets. */}
      <button
        type="button"
        onClick={() => setActiveOnly(!activeOnly)}
        disabled={!activeOnly && activeTasks.size === 0}
        className={`flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-medium border transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed ${
          activeOnly
            ? 'accent-text bg-[var(--accent-light)] border-[var(--accent-color)]/50 font-bold'
            : 'bg-[var(--bg-secondary)] text-[var(--text-secondary)] border-[var(--border-color)] hover:text-[var(--text-primary)]'
        }`}
        title={
          activeTasks.size === 0
            ? t.shell.list.noActive
            : activeOnly
              ? t.shell.board.showAll
              : plural(lang, activeTasks.size, F.onlyActive)
        }
      >
        <Loader2 size={12} className={activeOnly ? 'animate-spin' : undefined} />
        <span>{t.shell.header.activeOnly}</span>
        {activeTasks.size > 0 && (
          <span className="font-mono text-[10px] opacity-70">{activeTasks.size}</span>
        )}
      </button>

      {/* Priorité en pastilles plutôt qu'en liste déroulante : un select natif ne
          sait afficher que du texte, et la couleur est justement l'information.
          Un clic filtre, un second clic sur la pastille active l'enlève. */}
      <div className="flex items-center gap-1">
        <Flame size={12} className={priorityFilter ? 'text-rose-400' : 'text-[var(--text-muted)]'} />
        <div className="flex items-center gap-1 px-1 py-0.5 rounded-md bg-[var(--bg-secondary)] border border-[var(--border-color)]">
          {PRIORITY_LEVELS.map(level => {
            const isActive = priorityFilter === level
            return (
              <button
                key={level}
                type="button"
                onClick={() => setPriorityFilter(isActive ? null : level)}
                title={format(isActive ? F.removePriority : F.filterPriority, { priority: t.priority[level] })}
                aria-pressed={isActive}
                className={`w-4 h-4 rounded-full flex items-center justify-center transition-all cursor-pointer ${
                  isActive
                    ? 'ring-2 ring-[var(--accent-color)] scale-110'
                    : priorityFilter
                      ? 'opacity-40 hover:opacity-100'
                      : 'hover:scale-110'
                }`}
              >
                <span
                  className="w-2.5 h-2.5 rounded-full ring-1 ring-black/10"
                  style={{ backgroundColor: PRIORITY_COLORS[level] }}
                />
              </button>
            )
          })}
        </div>
      </div>

      {/* Statuts affichés : la même sélection vaut pour le board, la liste et le
          triage, puisque les trois lisent la même liste de tickets. Vide veut
          dire « tous », ce qui est l'état par défaut. */}
      {taskFacets.trackerStatuses.length > 0 && (
        <div className="relative" ref={statusMenuRef}>
          <button
            type="button"
            onClick={() => setIsStatusMenuOpen(open => !open)}
            title={F.chooseStatuses}
            className="flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-semibold border cursor-pointer transition-colors"
            style={{
              color: trackerStatusFilters.length > 0 ? 'var(--accent-color)' : 'var(--text-secondary)',
              background: trackerStatusFilters.length > 0 ? 'var(--accent-light)' : 'var(--bg-secondary)',
              borderColor: trackerStatusFilters.length > 0 ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
            }}
          >
            <SlidersHorizontal size={11} />
            {trackerStatusFilters.length > 0
              ? `${trackerStatusFilters.length} · ${t.list.columns.status}`
              : t.list.columns.status}
          </button>

          {isStatusMenuOpen && (
            <div className="absolute right-0 z-50 mt-1 w-[240px] max-h-[300px] overflow-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-lg p-1">
              <div className="flex items-center justify-between px-1.5 py-1">
                <span className="text-[9.5px] uppercase tracking-wider font-bold text-[var(--text-muted)]">
                  {t.list.columns.status}
                </span>
                {trackerStatusFilters.length > 0 && (
                  <button
                    type="button"
                    onClick={() => setTrackerStatusFilters([])}
                    className="text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                  >
                    {F.all}
                  </button>
                )}
              </div>
              {taskFacets.trackerStatuses.map(status => {
                const isActive = trackerStatusFilters.includes(status.value)
                return (
                  <button
                    key={status.value}
                    type="button"
                    onClick={() =>
                      setTrackerStatusFilters(
                        isActive
                          ? trackerStatusFilters.filter(s => s !== status.value)
                          : [...trackerStatusFilters, status.value]
                      )
                    }
                    className="w-full flex items-center gap-2 px-1.5 py-1 rounded-lg hover:bg-[var(--bg-tertiary)] cursor-pointer"
                  >
                    <span
                      className="w-3 h-3 rounded flex items-center justify-center shrink-0"
                      style={{
                        background: isActive ? 'var(--accent-color)' : 'transparent',
                        border: `1px solid ${isActive ? 'var(--accent-color)' : 'var(--border-color)'}`,
                      }}
                    >
                      {isActive && <Check size={8} className="text-white" />}
                    </span>
                    <span className="text-[11px] text-[var(--text-primary)] truncate flex-1 text-left">
                      {status.value}
                    </span>
                    <span className="text-[9.5px] font-mono text-[var(--text-muted)]">{status.count}</span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
      )}

      {taskFacets.issueTypes.length > 1 && (
        <div className="relative" ref={typeMenuRef}>
          <button
            type="button"
            onClick={() => setIsTypeMenuOpen(open => !open)}
            title={F.chooseTypes}
            className="flex items-center gap-1.5 px-2 py-1 rounded-md text-[11px] font-semibold border cursor-pointer transition-colors"
            style={{
              color: issueTypeFilters.length > 0 ? 'var(--accent-color)' : 'var(--text-secondary)',
              background: issueTypeFilters.length > 0 ? 'var(--accent-light)' : 'var(--bg-secondary)',
              borderColor: issueTypeFilters.length > 0 ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
            }}
          >
            <Shapes size={11} />
            {issueTypeFilters.length > 0 ? plural(lang, issueTypeFilters.length, F.typeCount) : F.types}
          </button>

          {isTypeMenuOpen && (
            <div className="absolute right-0 z-50 mt-1 w-[240px] max-h-[300px] overflow-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-lg p-1">
              <div className="flex items-center justify-between px-1.5 py-1">
                <span className="text-[9.5px] uppercase tracking-wider font-bold text-[var(--text-muted)]">
                  {F.issueTypes}
                </span>
                {issueTypeFilters.length > 0 && (
                  <button
                    type="button"
                    onClick={() => setIssueTypeFilters([])}
                    className="text-[10px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                  >
                    {F.all}
                  </button>
                )}
              </div>
              <button
                type="button"
                onClick={() => {
                  setIsTypeMenuOpen(false)
                  setEditingProject(currentProject || null)
                  setIsProjectModalOpen(true)
                }}
                disabled={!currentProject}
                title={
                  currentProject
                    ? format(F.importedTypes, { project: currentProject.name })
                    : F.selectProjectForTypes
                }
                className="w-full flex items-center gap-1.5 px-1.5 py-1 mb-1 rounded-lg text-[10px] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-40 border-b border-[var(--border-color)] rounded-b-none"
              >
                <Settings2 size={10} />
                <span className="text-left leading-snug">
                  {F.importedTypesHint}
                </span>
              </button>
              {taskFacets.issueTypes.map(type => {
                const isActive = issueTypeFilters.includes(type.value)
                // Un conteneur n'est pas montré tant qu'il n'est pas demandé :
                // sans cette mention, son compteur face à une liste qui n'en
                // affiche aucun serait incompréhensible.
                const isContainer = ['macro', 'epic', 'initiative'].includes(type.value.toLowerCase())
                return (
                  <button
                    key={type.value}
                    type="button"
                    onClick={() =>
                      setIssueTypeFilters(
                        isActive
                          ? issueTypeFilters.filter(t2 => t2 !== type.value)
                          : [...issueTypeFilters, type.value]
                      )
                    }
                    className="w-full flex items-center gap-2 px-1.5 py-1 rounded-lg hover:bg-[var(--bg-tertiary)] cursor-pointer"
                  >
                    <span
                      className="w-3 h-3 rounded flex items-center justify-center shrink-0"
                      style={{
                        background: isActive ? 'var(--accent-color)' : 'transparent',
                        border: `1px solid ${isActive ? 'var(--accent-color)' : 'var(--border-color)'}`,
                      }}
                    >
                      {isActive && <Check size={8} className="text-white" />}
                    </span>
                    <span className="text-[11px] text-[var(--text-primary)] truncate flex-1 text-left">
                      {type.value}
                      {isContainer && !isActive && (
                        <span className="ml-1 text-[9px] text-[var(--text-muted)]">{F.hidden}</span>
                      )}
                    </span>
                    <span className="text-[9.5px] font-mono text-[var(--text-muted)]">{type.count}</span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
      )}

      {(taskFacets.macros.length > 0 || availableParents.length > 0) && (
        <div className="flex items-center gap-1 w-[200px]">
          <Target size={12} className={parentFilter ? 'text-amber-400' : 'text-[var(--text-muted)]'} />
          <div className="flex-1">
            <LookupField
              value={selectedMacroLabel}
              placeholder={F.allMacrosPlaceholder}
              clearLabel={F.allMacros}
              onSearch={searchMacroValue}
              onPick={option => setParentFilter(option?.id || null)}
            />
          </div>
        </div>
      )}

      {taskFacets.sprints.length > 0 && (
        <div className="flex items-center gap-1 w-[190px]">
          <Calendar size={12} className={sprintFilter ? 'text-cyan-400' : 'text-[var(--text-muted)]'} />
          <div className="flex-1">
            <LookupField
              value={sprintFilter || ''}
              placeholder={F.allSprints}
              clearLabel={F.allSprints}
              onSearch={searchSprintValue}
              onPick={option => setSprintFilter(option?.id || null)}
            />
          </div>
        </div>
      )}

      {taskFacets.teams.length > 0 && (
        <div className="flex items-center gap-1 w-[200px]">
          <Layers size={12} className={teamFilter ? 'text-violet-400' : 'text-[var(--text-muted)]'} />
          <div className="flex-1">
            <LookupField
              value={teamFilter || ''}
              placeholder={F.allTeams}
              clearLabel={F.allTeams}
              onSearch={searchTeamValue}
              onPick={option => setTeamFilter(option?.id || null)}
            />
          </div>
        </div>
      )}

      {(availableAssignees.length > 0 || taskFacets.unassignedCount > 0) && (
        <div className="flex items-center gap-1 w-[200px]">
          <User size={12} className={assigneeFilter ? 'text-emerald-400' : 'text-[var(--text-muted)]'} />
          <div className="flex-1">
            <LookupField
              value={assigneeFilter === unassignedFilterValue ? F.unassigned : assigneeFilter || ''}
              placeholder={teamFilter ? F.wholeTeam : F.allPeople}
              clearLabel={teamFilter ? F.wholeTeam : F.allPeople}
              onSearch={searchAssigneeValue}
              onPick={option => setAssigneeFilter(option?.id || null)}
            />
          </div>
        </div>
      )}
    </div>
  )
}
