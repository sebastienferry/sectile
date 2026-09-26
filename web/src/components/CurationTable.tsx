import React, { useEffect, useMemo, useState } from 'react'
import { Check, ExternalLink, Layers, Target, User, CalendarRange, Inbox, Eye, EyeOff, Sparkles } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { LookupField, type LookupOption } from './LookupField'
import { macroLookup, sprintLookup } from '../lib/lookups'
import { trackerHas } from '../lib/trackers'
import { resolveTaskStage } from '../lib/workflow'
import type { MacroMeta, Task } from '../types'

/**
 * Triage du backlog : voir d'un coup ce qui n'est rattaché à rien (pas de sprint,
 * pas d'équipe, pas de macro, personne dessus) et le corriger sans ouvrir une seule
 * fiche. Chaque cellule est le champ lui-même, pas un aperçu de champ.
 *
 * Les quatre dimensions sont traitées ensemble parce que c'est ainsi qu'un
 * backlog se trie : un ticket sans sprint est souvent aussi sans équipe, et
 * passer par la fiche pour chacun coûte quatre clics et deux secondes d'attente.
 *
 * Les écritures partent dans la file d'activités, une par lot : cocher trente
 * tickets pour leur donner un sprint produit une activité, pas trente.
 */

type Dimension = 'sprint' | 'team' | 'macro' | 'assignee'

const DIMENSION_LABELS: Record<Dimension, string> = {
  sprint: 'sans sprint',
  team: 'sans équipe',
  macro: 'sans macro',
  assignee: 'sans assigné',
}

export const CurationTable: React.FC = () => {
  const {
    tasks,
    currentProject,
    searchTrackerTeams,
    searchAssignableUsers,
    setTaskSprint,
    setTasksSprint,
    setTaskTeam,
    setTasksTeam,
    setTaskMacro,
    moveTasksToMacro,
    updateTask,
    fetchProjectMacros,
    setSelectedTask,
    activeJobCount,
    hideDone,
    toggleHideDone,
    startBatchPickup,
    t,
  } = useApp()

  const [macros, setMacros] = useState<MacroMeta[]>([])
  const [dimensions, setDimensions] = useState<Dimension[]>(['sprint', 'team', 'macro'])
  const [checked, setChecked] = useState<Record<string, boolean>>({})
  const [batchBusy, setBatchBusy] = useState<string | null>(null)
  // Un choix de lot porte l'identifiant (ce que l'API prend) et le libellé (ce
  // que le champ affiche). Une chaîne d'identifiant vide reste une instruction
  // valable : retirer du sprint, retirer l'équipe.
  const [batchSprint, setBatchSprint] = useState<{ id: string; name: string }>({ id: '', name: '' })
  const [batchTeam, setBatchTeam] = useState<{ id: string; name: string }>({ id: '', name: '' })
  const [batchMacro, setBatchMacro] = useState('')

  useEffect(() => {
    if (!currentProject?.id) {
      setMacros([])
      return
    }
    fetchProjectMacros(currentProject.id).then(setMacros)
    // activeJobCount : un rattachement à une macro passe par la file, la liste des
    // macros n'est à jour qu'une fois la file vidée.
  }, [currentProject?.id, fetchProjectMacros, activeJobCount])

  const sprintOptions = useMemo(
    () =>
      (currentProject?.sprints || [])
        .filter(sp => sp.id && sp.state !== 'closed')
        .map(sp => ({ id: sp.id as string, name: sp.name, state: sp.state })),
    [currentProject?.sprints]
  )

  const macroOptions = useMemo(
    () => macros.filter(e => !e.closed).map(e => ({ key: e.key, title: e.title || e.key })),
    [macros]
  )

  const isMissing = (task: Task, dimension: Dimension): boolean => {
    if (dimension === 'sprint') return !(task.sprint || '').trim()
    if (dimension === 'team') return !(task.team || '').trim()
    if (dimension === 'macro') return !(task.parentKey || '').trim()
    return !(task.assignee || '').trim()
  }

  const isDoneTask = (t: Task): boolean => {
    if (t.status === 'finished' || t.status === 'done' || (t.status as any) === 'closed') return true
    if (t.trackerStatus && ['done', 'closed', 'finished', 'completed', 'terminé', 'termine'].includes(t.trackerStatus.toLowerCase())) return true
    const labels = (t.labels || []).map(l => l.toLowerCase().replace(/^#+/, ''))
    if (labels.includes('finished') || labels.includes('closed') || labels.includes('done') || labels.includes('termine')) return true
    return resolveTaskStage(t, currentProject) === 'finished'
  }

  // Le décompte porte sur tout ce que la liste reçoit, y compris les dimensions
  // non sélectionnées : c'est ce qui donne envie d'en cocher une autre.
  // Les tâches terminées / fermées sont exclues lorsque hideDone est actif.
  const pool = useMemo(
    () => tasks.filter(t => (hideDone ? !isDoneTask(t) : true)),
    [tasks, hideDone, currentProject]
  )

  const counts = useMemo(() => {
    const out: Record<Dimension, number> = { sprint: 0, team: 0, macro: 0, assignee: 0 }
    pool.forEach(task => {
      (Object.keys(out) as Dimension[]).forEach(dimension => {
        if (isMissing(task, dimension)) out[dimension] += 1
      })
    })
    return out
  }, [pool])

  const rows = useMemo(() => {
    if (dimensions.length === 0) return pool
    return pool.filter(task => dimensions.some(dimension => isMissing(task, dimension)))
  }, [pool, dimensions])

  const selectedIds = useMemo(() => rows.filter(t => checked[t.id]).map(t => t.id), [rows, checked])

  const toggleDimension = (dimension: Dimension) => {
    setDimensions(prev =>
      prev.includes(dimension) ? prev.filter(d => d !== dimension) : [...prev, dimension]
    )
  }

  // Les macros et les sprints sont cherchés au clavier
  const searchMacro = useMemo(() => macroLookup(macros), [macros])
  const searchSprint = useMemo(() => sprintLookup(currentProject?.sprints || []), [currentProject?.sprints])

  const searchTeamOptions = async (query: string): Promise<LookupOption[]> => {
    const found = await searchTrackerTeams(query)
    return found.filter(team => team.id).map(team => ({ id: team.id, label: team.name }))
  }

  const runBatch = async (kind: string, action: () => Promise<unknown>) => {
    setBatchBusy(kind)
    await action()
    setChecked({})
    setBatchSprint({ id: '', name: '' })
    setBatchTeam({ id: '', name: '' })
    setBatchMacro('')
    setBatchBusy(null)
  }

  if (!currentProject?.id) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-2 text-center px-6">
        <Inbox size={26} className="text-[var(--text-muted)]" />
        <p className="text-sm font-bold">Choisissez un projet</p>
        <p className="text-xs text-[var(--text-secondary)] max-w-md">
          Le triage travaille sur les sprints, les équipes et les épics d'un projet : il a besoin de
          savoir lequel.
        </p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      {/* Ce qui manque, et combien. Chaque compteur est un filtre. */}
      <div className="flex flex-wrap items-center gap-2">
        {(Object.keys(DIMENSION_LABELS) as Dimension[]).map(dimension => {
          const isActive = dimensions.includes(dimension)
          return (
            <button
              key={dimension}
              type="button"
              onClick={() => toggleDimension(dimension)}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-bold border cursor-pointer transition-colors"
              style={{
                color: isActive ? 'var(--accent-color)' : 'var(--text-secondary)',
                background: isActive ? 'var(--accent-light)' : 'var(--bg-tertiary)',
                borderColor: isActive ? 'rgb(var(--accent-rgb) / 0.4)' : 'var(--border-color)',
              }}
              title={`${counts[dimension]} ticket(s) ${DIMENSION_LABELS[dimension]}`}
            >
              {DIMENSION_LABELS[dimension]}
              <span className="font-mono">{counts[dimension]}</span>
            </button>
          )
        })}
        <button
          type="button"
          onClick={() => toggleHideDone()}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-bold border cursor-pointer transition-colors"
          style={{
            color: hideDone ? 'var(--status-ok)' : 'var(--text-secondary)',
            background: hideDone ? 'rgb(var(--status-ok-rgb) / 0.12)' : 'var(--bg-tertiary)',
            borderColor: hideDone ? 'rgb(var(--status-ok-rgb) / 0.32)' : 'var(--border-color)',
          }}
          title="Un ticket terminé n'a plus rien à trier : ce raccourci le sort de la liste"
        >
          {hideDone ? <EyeOff size={11} /> : <Eye size={11} />}
          {hideDone ? 'terminés exclus' : 'terminés inclus'}
        </button>

        <span className="text-[10px] text-[var(--text-muted)]">
          {rows.length} ticket(s) à trier
          {dimensions.length === 0 && ' (aucun critère : tout est affiché)'}
        </span>
      </div>

      {/* Actions de lot : une activité par lot, pas une par ticket. */}
      {selectedIds.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 p-2.5 rounded-xl border" style={{ background: 'var(--accent-light)', borderColor: 'rgb(var(--accent-rgb) / 0.4)' }}>
          <span className="text-[11px] font-bold" style={{ color: 'var(--accent-color)' }}>
            {selectedIds.length} sélectionné(s)
          </span>

          {sprintOptions.length > 0 && (
            <div className="flex items-center gap-1 w-[230px]">
              <div className="flex-1">
                <LookupField
                  value={batchSprint.name}
                  icon={<CalendarRange size={11} />}
                  placeholder="sprint..."
                  clearLabel="Backlog (aucun sprint)"
                  onSearch={searchSprint}
                  onPick={option => setBatchSprint({ id: option?.id || '', name: option?.label || 'Backlog' })}
                />
              </div>
              <button
                type="button"
                disabled={!batchSprint.name || batchBusy === 'sprint'}
                onClick={() =>
                  runBatch('sprint', () =>
                    setTasksSprint(currentProject.id, selectedIds, batchSprint.id, batchSprint.name)
                  )
                }
                className="px-2 py-1 rounded-lg text-[11px] font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] shrink-0"
              >
                {batchBusy === 'sprint' ? '…' : 'OK'}
              </button>
            </div>
          )}

          <div className="flex items-center gap-1 w-[250px]">
            <div className="flex-1">
              <LookupField
                value={batchTeam.name}
                icon={<Layers size={11} />}
                placeholder="équipe..."
                clearLabel="Aucune équipe"
                onSearch={searchTeamOptions}
                onPick={option => setBatchTeam({ id: option?.id || '', name: option?.label || 'Aucune équipe' })}
              />
            </div>
            <button
              type="button"
              disabled={!batchTeam.name || batchBusy === 'team'}
              onClick={() =>
                runBatch('team', () => setTasksTeam(currentProject.id, selectedIds, batchTeam.id, batchTeam.name))
              }
              className="px-2 py-1 rounded-lg text-[11px] font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] shrink-0"
            >
              {batchBusy === 'team' ? '…' : 'OK'}
            </button>
          </div>

          {macroOptions.length > 0 && (
            <div className="flex items-center gap-1 w-[230px]">
              <div className="flex-1">
                <LookupField
                  value={batchMacro}
                  icon={<Target size={11} />}
                  placeholder="macro..."
                  allowClear={false}
                  onSearch={searchMacro}
                  onPick={option => setBatchMacro(option?.id || '')}
                />
              </div>
              <button
                type="button"
                disabled={!batchMacro || batchBusy === 'macro'}
                onClick={() =>
                  runBatch('macro', () => moveTasksToMacro(currentProject.id, selectedIds, batchMacro))
                }
                className="px-2 py-1 rounded-lg text-[11px] font-bold cursor-pointer disabled:opacity-40 text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] shrink-0"
              >
                {batchBusy === 'macro' ? '…' : 'OK'}
              </button>
            </div>
          )}

          <button
            type="button"
            onClick={() => startBatchPickup(selectedIds)}
            className="flex items-center gap-1 px-2.5 py-1 rounded-lg text-[11px] font-semibold cursor-pointer bg-[var(--bg-tertiary)] text-[var(--text-secondary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-secondary)] transition-colors shrink-0 shadow-xs"
            title="Run the selected tasks on the local agent"
          >
            <Sparkles size={12} />
            {t.batchLaunch}
          </button>

          <button
            type="button"
            onClick={() => setChecked({})}
            className="ml-auto text-[10.5px] font-bold text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            Vider la sélection
          </button>
        </div>
      )}

      <div className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] overflow-hidden shadow-xs">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-[var(--border-color)] text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold bg-[var(--bg-tertiary)]/60">
                <th className="py-2 px-2 w-8">
                  <button
                    type="button"
                    onClick={() =>
                      setChecked(prev => {
                        const allSelected = rows.every(t => prev[t.id])
                        if (allSelected) return {}
                        const next: Record<string, boolean> = {}
                        rows.forEach(t => {
                          next[t.id] = true
                        })
                        return next
                      })
                    }
                    className="w-3.5 h-3.5 rounded flex items-center justify-center cursor-pointer"
                    style={{
                      background: rows.length > 0 && rows.every(t => checked[t.id]) ? 'var(--accent-color)' : 'transparent',
                      border: '1px solid var(--border-color)',
                    }}
                    title="Tout sélectionner ou tout désélectionner"
                  >
                    {rows.length > 0 && rows.every(t => checked[t.id]) && <Check size={9} className="text-white" />}
                  </button>
                </th>
                <th className="py-2 px-2">Ticket</th>
                <th className="py-2 px-2">Titre</th>
                <th className="py-2 px-2 w-[190px]">Macro</th>
                <th className="py-2 px-2 w-[170px]">Sprint</th>
                <th className="py-2 px-2 w-[190px]">Équipe</th>
                <th className="py-2 px-2 w-[190px]">Assigné</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-6 px-3 text-center text-[11px] text-[var(--text-muted)]">
                    Rien à trier avec ces critères. Tout est rattaché.
                  </td>
                </tr>
              ) : (
                rows.map(task => (
                  <tr key={task.id} className="border-b border-[var(--border-color)]/60 hover:bg-[var(--bg-tertiary)]/40">
                    <td className="py-1.5 px-2">
                      <button
                        type="button"
                        onClick={() => setChecked(prev => ({ ...prev, [task.id]: !prev[task.id] }))}
                        className="w-3.5 h-3.5 rounded flex items-center justify-center cursor-pointer"
                        style={{
                          background: checked[task.id] ? 'var(--accent-color)' : 'transparent',
                          border: `1px solid ${checked[task.id] ? 'var(--accent-color)' : 'var(--border-color)'}`,
                        }}
                      >
                        {checked[task.id] && <Check size={9} className="text-white" />}
                      </button>
                    </td>

                    <td className="py-1.5 px-2 whitespace-nowrap">
                      {task.externalUrl ? (
                        <a
                          href={task.externalUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex items-center gap-1 text-[10.5px] font-mono font-bold hover:underline"
                          style={{ color: 'var(--status-info)' }}
                          title={`Ouvrir ${task.key} sur le tracker`}
                        >
                          {task.key}
                          <ExternalLink size={9} />
                        </a>
                      ) : (
                        <span className="text-[10.5px] font-mono font-bold text-[var(--accent-color)]">{task.key}</span>
                      )}
                    </td>

                    <td className="py-1.5 px-2 max-w-[320px]">
                      <button
                        type="button"
                        onClick={() => setSelectedTask(task)}
                        className="text-[11.5px] text-left text-[var(--text-secondary)] hover:text-[var(--text-primary)] truncate block w-full cursor-pointer"
                        title={`${task.title} (ouvrir la fiche)`}
                      >
                        {task.title}
                      </button>
                    </td>

                    {/* Macro */}
                    <td className="py-1.5 px-2">
                      <LookupField
                        value={task.parentKey || ''}
                        icon={<Target size={10} />}
                        placeholder="macro..."
                        clearLabel="Aucune macro"
                        emptyHint="Aucune macro ne correspond."
                        disabled={task.source !== 'jira'}
                        onSearch={searchMacro}
                        onPick={option => setTaskMacro(task.id, option?.id || '')}
                      />
                    </td>

                    {/* Sprint */}
                    <td className="py-1.5 px-2">
                      <LookupField
                        value={task.sprint || ''}
                        icon={<CalendarRange size={10} />}
                        placeholder="sprint..."
                        clearLabel="Backlog (aucun sprint)"
                        emptyHint="Aucun sprint ne correspond."
                        disabled={task.source !== 'jira' || sprintOptions.length === 0}
                        onSearch={searchSprint}
                        onPick={option => setTaskSprint(task.id, option?.id || '', option?.label)}
                      />
                    </td>

                    {/* Équipe */}
                    <td className="py-1.5 px-2">
                      {trackerHas(task.source, 'team') ? (
                        <LookupField
                          value={task.team || ''}
                          icon={<Layers size={10} />}
                          placeholder="équipe…"
                          clearLabel="Aucune équipe"
                          onSearch={searchTeamOptions}
                          onPick={option => setTaskTeam(task.id, option?.id || '', option?.label)}
                        />
                      ) : (
                        <span className="text-[10.5px] text-[var(--text-muted)]">-</span>
                      )}
                    </td>

                    {/* Assigné */}
                    <td className="py-1.5 px-2">
                      {trackerHas(task.source, 'assigneeLookup') ? (
                        <LookupField
                          value={task.assignee || ''}
                          icon={<User size={10} />}
                          placeholder="personne…"
                          clearLabel="Non assigné"
                          onSearch={async query => {
                            const people = await searchAssignableUsers(task.id, query)
                            return people.map(m => ({
                              id: m.accountId,
                              label: m.displayName,
                              sublabel: m.email,
                              avatarUrl: m.avatarUrl,
                              muted: !m.active,
                            }))
                          }}
                          onPick={option =>
                            updateTask(task.id, {
                              assignee: option?.label || '',
                              assigneeAccountId: option?.id || '',
                              assigneeAvatar: option?.avatarUrl || '',
                            })
                          }
                        />
                      ) : (
                        <span className="text-[10.5px] text-[var(--text-muted)]">-</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
