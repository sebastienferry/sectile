import React from 'react'
import { Check, ExternalLink, GripVertical, Loader2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { issueTypeStyle } from '../lib/issueTypes'
import { isTaskDone } from '../lib/workflow'
import { Avatar } from './Avatar'
import type { Task } from '../types'

/**
 * Un ticket d'une macro, sur une ligne.
 *
 * La même ligne sert les groupes par phase ou par objectif et la liste des
 * tickets du cadrage, parce que « comme dans les objectifs » est une demande
 * qu'un composant partagé tient et que deux rendus jumeaux finissent toujours
 * par diverger.
 *
 * Une colonne et le titre en clair : les pastilles ne portaient que la clé, le
 * titre restant en infobulle, et on ne répartit pas une macro en lisant
 * « PE-1841, PE-1853, PE-1862 » sans survoler chacune.
 */

interface MacroTaskRowProps {
  task: Task
  /** Ouvre le ticket dans Sectile. */
  onOpen: (task: Task) => void
  /** Propriétés de glissement, quand la ligne est déplaçable. */
  dragProps?: React.HTMLAttributes<HTMLDivElement> & { draggable?: boolean }
  /** Une écriture est en cours sur ce ticket. */
  busy?: boolean
  /** Infobulle de la ligne entière. */
  title?: string
}

export const MacroTaskRow: React.FC<MacroTaskRowProps> = ({ task, onOpen, dragProps, busy, title }) => {
  const type = issueTypeStyle(task.issueType || '')
  const done = isTaskDone(task)
  const draggable = Boolean(dragProps)
  const { t } = useApp()
  const strings = t.planning.macro

  return (
    <div
      {...dragProps}
      className={`flex items-center gap-1.5 px-2 py-1 rounded-lg border transition-colors ${
        busy ? 'opacity-50' : draggable ? 'cursor-grab active:cursor-grabbing' : ''
      }`}
      style={{
        background: done ? 'rgb(var(--status-ok-rgb) / 0.09)' : 'var(--bg-primary)',
        borderColor: done ? 'rgb(var(--status-ok-rgb) / 0.3)' : 'var(--border-color)',
      }}
      title={title || format(done ? strings.taskTitleDone : strings.taskTitle, { key: task.key, title: task.title })}
    >
      {draggable && <GripVertical size={10} className="text-[var(--text-muted)] opacity-60 shrink-0" />}
      {done && <Check size={10} className="shrink-0" style={{ color: 'var(--status-ok)' }} />}
      <button
        type="button"
        onClick={() => onOpen(task)}
        className="text-[10px] font-mono font-bold shrink-0 cursor-pointer hover:underline"
        style={{ color: done ? 'var(--status-ok)' : 'var(--accent-color)' }}
      >
        {task.key}
      </button>
      {/* Le lien vers le tracker est distinct de l'ouverture dans Sectile :
          consulter la fiche et aller commenter le ticket ne sont pas le même
          geste, et confondre les deux oblige à ressortir par le mauvais bout. */}
      {task.externalUrl && (
        <a
          href={task.externalUrl}
          target="_blank"
          rel="noreferrer"
          onClick={e => e.stopPropagation()}
          className="shrink-0 text-[var(--text-muted)] hover:text-[var(--accent-color)] transition-colors"
          title={format(strings.openOnTracker, { key: task.key })}
        >
          <ExternalLink size={9} />
        </a>
      )}
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
          title={format(strings.sprintTitle, { sprint: task.sprint })}
        >
          {task.sprint}
        </span>
      )}
      {task.assignee && <Avatar name={task.assignee} url={task.assigneeAvatar} size={16} />}
      {busy && <Loader2 size={10} className="animate-spin shrink-0" />}
    </div>
  )
}
