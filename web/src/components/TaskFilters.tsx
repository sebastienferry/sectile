import React from 'react'
import { Flame, Calendar, Layers, Pin, User, Settings2, Target, Loader2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { LookupOption } from './LookupField'
import { valueLookup } from '../lib/lookups'
import { matchesSearch } from '../lib/searchFold'
import { PRIORITY_COLORS, PRIORITY_LEVELS } from '../lib/priority'
import { format, plural } from '../lib/i18n'
import { countSetPickers, multiValueFilters, type ActiveFilter } from '../lib/activeFilters'
import { FilterToggle } from './FilterToggle'
import { FilterPanel } from './FilterPanel'
import { SinglePicker } from './FilterPicker'
import { MultiPicker } from './MultiPicker'
import { ActiveFilterChips } from './ActiveFilterChips'
import { useMacroFilterSearch } from './useMacroFilterSearch'

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
 *
 * Built on the shared filter controls of `filters/` (#866), which every other
 * view uses too.
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

  // Les valeurs proposées sont celles que le board porte réellement, filtrées en
  // mémoire : elles arrivent déjà avec les facettes, aucun appel n'est utile.
  const searchSprintValue = React.useMemo(() => valueLookup(taskFacets.sprints), [taskFacets.sprints])
  const searchTeamValue = React.useMemo(() => valueLookup(taskFacets.teams), [taskFacets.teams])
  const searchAssigneeValue = React.useMemo(() => {
    const base = valueLookup(availableAssignees)
    return async (query: string): Promise<LookupOption[]> => {
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
  const macro = useMacroFilterSearch()

  const assigneeLabel = assigneeFilter === unassignedFilterValue ? F.unassigned : assigneeFilter || ''

  const hasMacros = taskFacets.macros.length > 0 || availableParents.length > 0
  const hasPeople = availableAssignees.length > 0 || taskFacets.unassignedCount > 0
  const hasPanel =
    taskFacets.trackerStatuses.length > 0 ||
    taskFacets.issueTypes.length > 1 ||
    hasMacros ||
    taskFacets.sprints.length > 0 ||
    taskFacets.teams.length > 0 ||
    hasPeople

  const activePanelFilters = countSetPickers([
    trackerStatusFilters,
    issueTypeFilters,
    parentFilter,
    sprintFilter,
    teamFilter,
    assigneeFilter,
  ])

  const resetPanelFilters = () => {
    setTrackerStatusFilters([])
    setIssueTypeFilters([])
    setParentFilter(null)
    setSprintFilter(null)
    setTeamFilter(null)
    setAssigneeFilter(null)
  }

  // What the panel narrows, as chips: the toggles and the priority dots already
  // show their own state in the toolbar.
  const chips = React.useMemo<ActiveFilter[]>(
    () => [
      ...(parentFilter ? [{ id: 'macro', label: macro.label, clear: () => setParentFilter(null) }] : []),
      ...(sprintFilter ? [{ id: 'sprint', label: sprintFilter, clear: () => setSprintFilter(null) }] : []),
      ...(teamFilter ? [{ id: 'team', label: teamFilter, clear: () => setTeamFilter(null) }] : []),
      ...(assigneeFilter ? [{ id: 'assignee', label: assigneeLabel, clear: () => setAssigneeFilter(null) }] : []),
      ...multiValueFilters('status', trackerStatusFilters, v => v, v =>
        setTrackerStatusFilters(trackerStatusFilters.filter(s => s !== v))
      ),
      ...multiValueFilters('type', issueTypeFilters, v => v, v =>
        setIssueTypeFilters(issueTypeFilters.filter(s => s !== v))
      ),
    ],
    [
      parentFilter,
      macro.label,
      sprintFilter,
      teamFilter,
      assigneeFilter,
      assigneeLabel,
      trackerStatusFilters,
      issueTypeFilters,
      setParentFilter,
      setSprintFilter,
      setTeamFilter,
      setAssigneeFilter,
      setTrackerStatusFilters,
      setIssueTypeFilters,
    ]
  )

  const toggleIn = (list: string[], value: string) => (list.includes(value) ? list.filter(v => v !== value) : [...list, value])

  return (
    <div className="flex items-center gap-2 min-w-0 flex-wrap">
      {/* Épinglés : le retour immédiat aux chantiers en cours quand le board en
          porte trois cents. Le filtre est tenu par le serveur, sur la colonne
          indexée, donc il vaut aussi pour la recherche et les autres filtres. */}
      <FilterToggle
        pressed={pinnedOnly}
        onClick={() => setPinnedOnly(!pinnedOnly)}
        disabled={!pinnedOnly && pinnedTasks.length === 0}
        icon={<Pin size={12} />}
        label={t.shell.pinned.title}
        count={pinnedTasks.length}
        title={
          pinnedTasks.length === 0
            ? F.noPinned
            : pinnedOnly
              ? t.shell.board.showAll
              : plural(lang, pinnedTasks.length, F.onlyPinned)
        }
      />

      {/* In progress: the answer to the question the cards' badge asks (what is
          running right now?) without scanning three hundred tickets for a
          coloured mark. The client holds the filter, over the activities already
          loaded for that badge: a run's state is not a ticket column, so a run
          that ends drops its ticket when the activities refresh, not the tickets. */}
      <FilterToggle
        pressed={activeOnly}
        onClick={() => setActiveOnly(!activeOnly)}
        disabled={!activeOnly && activeTasks.size === 0}
        icon={<Loader2 size={12} className={activeOnly ? 'animate-spin' : undefined} />}
        label={t.shell.header.activeOnly}
        count={activeTasks.size}
        title={
          activeTasks.size === 0
            ? t.shell.list.noActive
            : activeOnly
              ? t.shell.board.showAll
              : plural(lang, activeTasks.size, F.onlyActive)
        }
      />

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

      {hasPanel && (
        <FilterPanel activeCount={activePanelFilters} onReset={resetPanelFilters} title={F.panelTitle}>
          {close => (
            <>
              {hasMacros && (
                <SinglePicker
                  icon={<Target size={12} className={parentFilter ? 'text-amber-400' : 'text-[var(--text-muted)]'} />}
                  label={F.macro}
                  value={macro.label}
                  placeholder={F.allMacrosPlaceholder}
                  clearLabel={F.allMacros}
                  onSearch={macro.search}
                  onPick={setParentFilter}
                />
              )}

              {taskFacets.sprints.length > 0 && (
                <SinglePicker
                  icon={<Calendar size={12} className={sprintFilter ? 'text-cyan-400' : 'text-[var(--text-muted)]'} />}
                  label={F.sprint}
                  value={sprintFilter || ''}
                  placeholder={F.allSprints}
                  onSearch={searchSprintValue}
                  onPick={setSprintFilter}
                />
              )}

              {taskFacets.teams.length > 0 && (
                <SinglePicker
                  icon={<Layers size={12} className={teamFilter ? 'text-violet-400' : 'text-[var(--text-muted)]'} />}
                  label={F.team}
                  value={teamFilter || ''}
                  placeholder={F.allTeams}
                  onSearch={searchTeamValue}
                  onPick={setTeamFilter}
                />
              )}

              {hasPeople && (
                <SinglePicker
                  icon={<User size={12} className={assigneeFilter ? 'text-emerald-400' : 'text-[var(--text-muted)]'} />}
                  label={F.person}
                  value={assigneeLabel}
                  placeholder={teamFilter ? F.wholeTeam : F.allPeople}
                  onSearch={searchAssigneeValue}
                  onPick={setAssigneeFilter}
                />
              )}

              {/* Statuts affichés : la même sélection vaut pour le board, la liste et le
                  triage, puisque les trois lisent la même liste de tickets. Vide veut
                  dire « tous », ce qui est l'état par défaut. */}
              {taskFacets.trackerStatuses.length > 0 && (
                <MultiPicker
                  title={F.status}
                  options={taskFacets.trackerStatuses}
                  isSelected={value => trackerStatusFilters.includes(value)}
                  onToggle={value => setTrackerStatusFilters(toggleIn(trackerStatusFilters, value))}
                  clearLabel={F.all}
                  canClear={trackerStatusFilters.length > 0}
                  onClear={() => setTrackerStatusFilters([])}
                />
              )}

              {taskFacets.issueTypes.length > 1 && (
                <MultiPicker
                  title={F.issueTypes}
                  // Un conteneur n'est pas montré tant qu'il n'est pas demandé :
                  // sans cette mention, son compteur face à une liste qui n'en
                  // affiche aucun serait incompréhensible.
                  options={taskFacets.issueTypes.map(option => ({
                    ...option,
                    hint:
                      !issueTypeFilters.includes(option.value) &&
                      ['macro', 'epic', 'initiative'].includes(option.value.toLowerCase())
                        ? F.hidden
                        : undefined,
                  }))}
                  isSelected={value => issueTypeFilters.includes(value)}
                  onToggle={value => setIssueTypeFilters(toggleIn(issueTypeFilters, value))}
                  clearLabel={F.all}
                  canClear={issueTypeFilters.length > 0}
                  onClear={() => setIssueTypeFilters([])}
                  footer={
                    <button
                      type="button"
                      onClick={() => {
                        close()
                        setEditingProject(currentProject || null)
                        setIsProjectModalOpen(true)
                      }}
                      disabled={!currentProject}
                      title={
                        currentProject
                          ? format(F.importedTypes, { project: currentProject.name })
                          : F.selectProjectForTypes
                      }
                      className="w-full flex items-center gap-1.5 px-1.5 py-1 mt-1 rounded-lg text-[10px] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-40"
                    >
                      <Settings2 size={10} />
                      <span className="text-left leading-snug">{F.importedTypesHint}</span>
                    </button>
                  }
                />
              )}
            </>
          )}
        </FilterPanel>
      )}

      <ActiveFilterChips filters={chips} />
    </div>
  )
}
