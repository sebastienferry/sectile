import React, { useEffect } from 'react'
import { Pin, Terminal as TerminalIcon, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { priorityColor } from '../lib/priority'
import { format } from '../lib/i18n'

/**
 * Barre des tickets épinglés, sous l'en-tête.
 *
 * Elle sert la bascule à chaud : trois chantiers avancent en parallèle, chacun
 * avec sa session d'agent, et un clic amène la console du bon ticket dans le
 * panneau. Cmd ou Ctrl plus un chiffre fait la même chose au clavier.
 *
 * Les épingles viennent du serveur, donc un ticket reste accessible ici même
 * quand les filtres du board le cachent ou qu'il appartient à un autre projet.
 */
export const PinnedBar: React.FC = () => {
  const { pinnedTasks, togglePin, hotSwitch, selectedTask, setSelectedTask, t } = useApp()

  useEffect(() => {
    if (pinnedTasks.length === 0) return
    const onKey = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return
      const index = Number(e.key) - 1
      if (!Number.isInteger(index) || index < 0 || index > 8) return
      const task = pinnedTasks[index]
      if (!task) return
      e.preventDefault()
      hotSwitch(task.id)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [pinnedTasks, hotSwitch])

  if (pinnedTasks.length === 0) return null

  return (
    <div className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 border-b border-[var(--border-color)] bg-[var(--bg-secondary)] overflow-x-auto scrollbar-none">
      <span className="flex items-center gap-1 text-[9px] font-bold uppercase tracking-[.08em] text-[var(--text-muted)] shrink-0 pr-1">
        <Pin size={10} className="text-[var(--accent-color)]" />
        <span>{t.shell.pinned.title}</span>
      </span>

      {pinnedTasks.map((task, index) => {
        const isCurrent = selectedTask?.id === task.id
        // Same colour as the card's priority dot, so a pin reads as the task it stands for.
        const dot = priorityColor(task.priority)
        return (
          <div
            key={task.id}
            className={`group flex items-center gap-1.5 pl-1.5 pr-1 py-1 rounded-lg border shrink-0 transition-colors ${
              isCurrent
                ? 'bg-[var(--accent-light)] border-[var(--accent-color)]/50'
                : 'bg-[var(--bg-primary)] border-[var(--border-color)] hover:border-[var(--accent-color)]/40'
            }`}
          >
            <span
              className="w-1.5 h-1.5 rounded-full shrink-0"
              style={{ background: dot }}
              title={`${task.priority} · ${task.status}`}
            />
            <button
              type="button"
              onClick={() => hotSwitch(task.id)}
              className="flex items-center gap-1.5 cursor-pointer min-w-0"
              title={format(t.shell.pinned.open, { key: task.key, shortcut: index < 9 ? `Cmd+${index + 1}` : t.shell.pinned.click })}
            >
              <span
                className={`text-[10px] font-mono font-bold ${
                  isCurrent ? 'accent-text' : 'text-[var(--text-primary)]'
                }`}
              >
                {task.key}
              </span>
              <span className="text-[10px] text-[var(--text-secondary)] truncate max-w-[130px]">{task.title}</span>
              {index < 9 && (
                <span className="text-[8px] font-mono text-[var(--text-muted)] shrink-0">⌘{index + 1}</span>
              )}
            </button>

            <button
              type="button"
              onClick={() => setSelectedTask(task)}
              className="p-0.5 rounded text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer shrink-0"
              title={format(t.shell.pinned.openDetail, { key: task.key })}
            >
              <TerminalIcon size={10} />
            </button>
            <button
              type="button"
              onClick={() => togglePin(task.id)}
              className="p-0.5 rounded text-[var(--text-muted)] hover:text-rose-400 cursor-pointer shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
              title={format(t.shell.pinned.unpin, { key: task.key })}
            >
              <X size={10} />
            </button>
          </div>
        )
      })}
    </div>
  )
}
