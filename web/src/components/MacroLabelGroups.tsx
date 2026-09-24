import React, { useCallback, useMemo, useState } from 'react'
import { Check, GripVertical, Layers, Loader2, Plus, Tag } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { issueTypeStyle } from '../lib/issueTypes'
import { Avatar } from './Avatar'
import { LABEL_AXES, axisLabelOf, axisNameOf, axisWords, isAxisLabel, type LabelAxis } from '../lib/labelAxes'
import type { Task } from '../types'

/**
 * Les tickets d'une macro, groupés par un label préfixé.
 *
 * Deux découpes se lisent ainsi, et la même mécanique les porte : les phases
 * disent l'ordre du travail, les objectifs ce qu'on cherche à obtenir. Déplacer
 * un ticket d'un groupe à l'autre retire le label de départ et pose celui
 * d'arrivée, en une écriture qui passe par le chemin habituel.
 */

/**
 * Lignes affichées avant dépliage.
 *
 * Le groupe des tickets non rangés est le plus gros au départ. Tout afficher
 * ferait défiler le panneau longtemps avant d'atteindre le groupe suivant.
 */
const ROWS_SHOWN = 12

const isDone = (task: Task): boolean => task.status === 'finished' || task.status === 'done'

interface AxisGroup {
  label: string
  tasks: Task[]
  done: number
}

export const MacroLabelGroups: React.FC<{ axis: LabelAxis; tasks: Task[] }> = ({ axis, tasks }) => {
  const { setSelectedTask, updateTask, addToast } = useApp()
  const meta = LABEL_AXES[axis]
  const words = axisWords(axis)

  const carries = useCallback((label: string) => isAxisLabel(axis, label), [axis])
  const nameOf = useCallback((label: string) => axisNameOf(axis, label), [axis])
  const labelOf = useCallback((name: string) => axisLabelOf(axis, name), [axis])

  const [filter, setFilter] = useState('')
  const [newGroup, setNewGroup] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  // Groupes nommés mais encore sans ticket : ils vivent ici jusqu'au premier
  // dépôt, puisqu'un label sans porteur n'existe pas côté tracker.
  const [pendingGroups, setPendingGroups] = useState<string[]>([])
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [dragged, setDragged] = useState<{ taskId: string; from: string } | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)

  const groups = useMemo<AxisGroup[]>(() => {
    const byLabel = new Map<string, Task[]>()
    // Les groupes nommés s'affichent même vides : sans cela on ne pourrait rien
    // y déposer, et un groupe se nomme avant d'être rempli.
    //
    // Ils sont filtrés par l'axe, comme les labels des tickets. C'est une
    // ceinture par-dessus la clé de montage : si le composant est un jour
    // réutilisé sans remonter, un groupe nommé pour un axe ne doit pas
    // réapparaître dans l'autre.
    pendingGroups.filter(carries).forEach(label => byLabel.set(label, []))
    tasks.forEach(task => {
      ;(task.labels || []).filter(carries).forEach(label => {
        const list = byLabel.get(label)
        if (list) list.push(task)
        else byLabel.set(label, [task])
      })
    })

    const q = filter.trim().toLowerCase()
    return Array.from(byLabel.entries())
      .filter(([label]) => (q ? nameOf(label).toLowerCase().includes(q) : true))
      // Les groupes les plus chargés d'abord : c'est là que le travail est.
      .sort((a, b) => b[1].length - a[1].length || a[0].localeCompare(b[0]))
      .map(([label, list]) => ({ label, tasks: list, done: list.filter(isDone).length }))
  }, [tasks, filter, pendingGroups, carries, nameOf])

  // Un ticket sans aucun label de cet axe n'est rangé nulle part : c'est
  // précisément ce qu'on vient voir pour le ranger.
  const unplaced = useMemo(
    () => tasks.filter(task => !(task.labels || []).some(carries)),
    [tasks, carries]
  )

  /**
   * Déplace un ticket d'un groupe à l'autre.
   *
   * La liste entière des labels est réécrite, le seul chemin d'écriture que
   * l'interface expose. Seul le label de cet axe bouge : les autres labels du
   * ticket, étapes de workflow comprises, sont reportés tels quels.
   */
  const move = async (taskId: string, from: string, to: string) => {
    if (from === to) return
    const task = tasks.find(t => t.id === taskId)
    if (!task) return
    setBusy(taskId)
    try {
      const current = task.labels || []
      const kept = from
        ? current.filter(l => l.trim().toLowerCase() !== from.trim().toLowerCase())
        : [...current]
      const next =
        to && !kept.some(l => l.trim().toLowerCase() === to.trim().toLowerCase())
          ? [...kept, to]
          : kept
      await updateTask(taskId, { labels: next })
    } catch (err: any) {
      addToast({ type: 'error', title: 'Déplacement refusé', description: err.message })
    } finally {
      setBusy(null)
    }
  }

  /**
   * Nomme un groupe, sans rien étiqueter.
   *
   * Poser le label sur tous les tickets non rangés en marquerait des dizaines
   * d'un seul clic. Le groupe est donc annoncé vide et sert de cible : le label
   * ne devient réel qu'au premier ticket qu'on y dépose, ce qui est aussi la
   * vérité côté Jira, où un label n'existe que porté.
   */
  const createGroup = () => {
    const label = labelOf(newGroup)
    if (!label) return
    setPendingGroups(prev => (prev.includes(label) ? prev : [...prev, label]))
    setNewGroup('')
  }

  const dragProps = (task: Task, from: string) => ({
    draggable: true,
    onDragStart: (e: React.DragEvent) => {
      e.dataTransfer.setData('text/plain', task.id)
      e.dataTransfer.effectAllowed = 'move'
      setDragged({ taskId: task.id, from })
    },
    onDragEnd: () => {
      setDragged(null)
      setHovered(null)
    },
  })

  const dropProps = (target: string) => ({
    onDragOver: (e: React.DragEvent) => {
      e.preventDefault()
      e.dataTransfer.dropEffect = 'move'
    },
    onDragEnter: () => setHovered(target),
    onDragLeave: () => setHovered(prev => (prev === target ? null : prev)),
    onDrop: (e: React.DragEvent) => {
      e.preventDefault()
      setHovered(null)
      const id = e.dataTransfer.getData('text/plain') || dragged?.taskId
      if (id) move(id, dragged?.from || '', target)
      setDragged(null)
    },
  })

  /**
   * Un ticket par ligne, avec son titre.
   *
   * La clé seule, le titre en infobulle, ne permet pas de répartir une macro :
   * on ne lit pas « PE-1841, PE-1853, PE-1862 » en survolant chaque pastille.
   * Répartir demande de savoir de quoi on parle.
   */
  const renderRow = (task: Task, from: string) => {
    const type = issueTypeStyle(task.issueType || '')
    const done = isDone(task)
    const working = busy === task.id
    return (
      <div
        key={`${from}:${task.id}`}
        {...dragProps(task, from)}
        className={`flex items-center gap-1.5 px-2 py-1 rounded-lg border transition-colors ${
          working ? 'opacity-50' : 'cursor-grab active:cursor-grabbing'
        }`}
        style={{
          background: done ? 'rgb(var(--status-ok-rgb) / 0.09)' : 'var(--bg-primary)',
          borderColor: done ? 'rgb(var(--status-ok-rgb) / 0.3)' : 'var(--border-color)',
        }}
        title={`${task.key} - ${task.title}${done ? ' (terminé)' : ''} · Glisser vers un groupe`}
      >
        <GripVertical size={10} className="text-[var(--text-muted)] opacity-60 shrink-0" />
        {done && <Check size={10} className="shrink-0" style={{ color: 'var(--status-ok)' }} />}
        <button
          type="button"
          onClick={() => setSelectedTask(task)}
          className="text-[10px] font-mono font-bold shrink-0 cursor-pointer hover:underline"
          style={{ color: done ? 'var(--status-ok)' : 'var(--accent-color)' }}
        >
          {task.key}
        </button>
        {task.issueType && (
          <span
            className="text-[9px] px-1 rounded font-bold shrink-0"
            style={{ color: type.color, background: type.background }}
          >
            {type.short}
          </span>
        )}
        <span
          className="text-[11px] truncate flex-1 min-w-0"
          style={{
            color: done ? 'var(--text-muted)' : 'var(--text-primary)',
            textDecoration: done ? 'line-through' : 'none',
          }}
        >
          {task.title}
        </span>
        {task.sprint && (
          <span
            className="text-[9px] font-mono px-1 rounded shrink-0 max-w-[110px] truncate text-[var(--text-muted)] bg-[var(--bg-tertiary)] border border-[var(--border-color)]"
            title={`Sprint : ${task.sprint}`}
          >
            {task.sprint}
          </span>
        )}
        {task.assignee && <Avatar name={task.assignee} url={task.assigneeAvatar} size={16} />}
        {working && <Loader2 size={10} className="animate-spin shrink-0" />}
      </div>
    )
  }

  const renderGroup = (label: string, list: Task[], done: number, isUnplaced: boolean) => {
    const target = hovered === (isUnplaced ? '' : label)
    const groupKey = label || '__none__'
    const isOpen = Boolean(expanded[groupKey])
    return (
      <div
        key={label || '__unplaced__'}
        {...dropProps(isUnplaced ? '' : label)}
        className="rounded-xl border p-2.5 transition-colors"
        style={{
          borderColor: target ? 'var(--accent-color)' : 'var(--border-color)',
          background: target ? 'var(--accent-light)' : 'var(--bg-primary)',
        }}
      >
        <div className="flex items-center gap-1.5 mb-2 flex-wrap">
          {isUnplaced ? (
            <span className="text-[10.5px] font-bold text-[var(--text-muted)]">{words.none}</span>
          ) : (
            <span
              className="inline-flex items-center gap-1 text-[10.5px] font-bold px-1.5 py-0.5 rounded"
              style={{
                color: 'var(--accent-color)',
                background: 'var(--accent-light)',
                border: '1px solid rgb(var(--accent-rgb) / 0.35)',
              }}
              title={`Label sur le tracker : ${label}`}
            >
              <Tag size={10} />
              {nameOf(label)}
            </span>
          )}
          <span className="text-[9.5px] font-mono text-[var(--text-muted)]">
            {list.length} {list.length > 1 ? 'tickets' : 'ticket'}
            {done > 0 && ` · ${done} terminé${done > 1 ? 's' : ''}`}
          </span>
          {!isUnplaced && done === list.length && list.length > 0 && (
            <span
              className="text-[9px] font-bold px-1.5 rounded inline-flex items-center gap-1"
              style={{
                color: 'var(--status-ok)',
                background: 'rgb(var(--status-ok-rgb) / 0.13)',
                border: '1px solid rgb(var(--status-ok-rgb) / 0.32)',
              }}
            >
              <Check size={9} /> Groupe terminé
            </span>
          )}
        </div>
        <div className="flex flex-col gap-1">
          {list.length === 0 ? (
            <span className="text-[10.5px] text-[var(--text-muted)] italic">
              Déposer un ticket ici pour le placer.
            </span>
          ) : (
            <>
              {(isOpen ? list : list.slice(0, ROWS_SHOWN)).map(task =>
                renderRow(task, isUnplaced ? '' : label)
              )}
              {/* Le reste est annoncé, jamais coupé en silence : un groupe
                  tronqué sans le dire se lirait comme un groupe complet. */}
              {list.length > ROWS_SHOWN && (
                <button
                  type="button"
                  onClick={() => setExpanded(prev => ({ ...prev, [groupKey]: !isOpen }))}
                  className="self-start px-2 py-0.5 rounded-lg border border-dashed border-[var(--border-color)] text-[10px] font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  {isOpen ? 'Replier' : `+ ${list.length - ROWS_SHOWN} autres`}
                </button>
              )}
            </>
          )}
        </div>
      </div>
    )
  }

  if (tasks.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-10 text-center px-4">
        <Layers size={22} className="text-[var(--text-muted)]" />
        <p className="text-[11.5px] text-[var(--text-secondary)]">
          Aucun ticket sous cette macro : rien à répartir pour l'instant.
        </p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex items-center gap-1.5 flex-wrap">
        <span className="text-[10px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)]">
          {words.plural} ({groups.length})
        </span>
        <input
          type="text"
          value={filter}
          onChange={e => setFilter(e.target.value)}
          placeholder="Filtrer…"
          className="px-2 py-0.5 text-[10.5px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] w-[130px]"
          title="Ne garder que les groupes dont le nom contient ce texte"
        />
        <span className="ml-auto flex items-center gap-1">
          <input
            type="text"
            value={newGroup}
            onChange={e => setNewGroup(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter') {
                e.preventDefault()
                createGroup()
              }
            }}
            placeholder={words.placeholder}
            className="px-2 py-0.5 text-[10.5px] rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)] w-[130px]"
          />
          {/* Le label obtenu, montré avant d'être posé : « migration A » devient
              « phase:migration-a », Jira refusant les espaces dans un label. */}
          {newGroup.trim() && (
            <span
              className="text-[9.5px] font-mono text-[var(--text-muted)] shrink-0"
              title="Label qui sera posé sur le tracker"
            >
              {labelOf(newGroup)}
            </span>
          )}
          <button
            type="button"
            onClick={createGroup}
            disabled={!labelOf(newGroup)}
            className="flex items-center gap-1 px-2 py-0.5 rounded-lg text-[10.5px] font-bold text-white accent-bg disabled:opacity-40 cursor-pointer shrink-0"
            title="Nommer le groupe sans étiqueter de ticket : il sert de cible, le label n'est posé qu'au premier dépôt"
          >
            <Plus size={10} />
            Nommer
          </button>
        </span>
      </div>

      {groups.length === 0 && (
        <p className="text-[11px] text-[var(--text-muted)]">
          {filter.trim()
            ? `Aucun groupe ne correspond à « ${filter.trim()} ».`
            : `Aucun groupe : nommez-en un, ou posez un label « ${meta.prefix}nom » sur un ticket.`}
        </p>
      )}

      {groups.map(group => renderGroup(group.label, group.tasks, group.done, false))}

      {/* Toujours en dernier, et toujours affiché tant qu'il reste des tickets à
          ranger : c'est la file d'attente de cette vue. */}
      {unplaced.length > 0 && renderGroup('', unplaced, unplaced.filter(isDone).length, true)}
    </div>
  )
}
