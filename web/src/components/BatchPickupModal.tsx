import { useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, FolderGit2, Loader2, X } from 'lucide-react'
import type { Task } from '../types'
import type { TranslationSchema } from '../locales/translations'
import { isValidBatchWorktreeName, suggestedBatchWorktreeName } from '../lib/batchPickup'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'

interface BatchPickupModalProps {
  tasks: Task[]
  labels: TranslationSchema['batchDialog']
  onCancel: () => void
  onConfirm: (taskIds: string[], worktreeName: string) => Promise<boolean>
}

export function BatchPickupModal({ tasks, labels, onCancel, onConfirm }: BatchPickupModalProps) {
  const [orderedTasks, setOrderedTasks] = useState(tasks)
  const [worktreeName, setWorktreeName] = useState(() => suggestedBatchWorktreeName(tasks.map(task => task.key)))
  const [launching, setLaunching] = useState(false)
  const [failed, setFailed] = useState(false)
  const launchingRef = useRef(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const validName = isValidBatchWorktreeName(worktreeName)
  const cancel = () => { if (!launchingRef.current) onCancel() }
  const backdrop = useBackdropDismiss(cancel)
  useEscapeKey(true, cancel)

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    inputRef.current?.focus()
    inputRef.current?.select()
    return () => { if (previous?.isConnected) previous.focus() }
  }, [])

  const move = (index: number, offset: number) => {
    setOrderedTasks(previous => {
      const next = [...previous]
      const destination = index + offset
      if (destination < 0 || destination >= next.length) return previous
      ;[next[index], next[destination]] = [next[destination], next[index]]
      return next
    })
  }

  const launch = async () => {
    if (launchingRef.current || !validName) return
    launchingRef.current = true
    setLaunching(true)
    setFailed(false)
    try {
      if (!await onConfirm(orderedTasks.map(task => task.id), worktreeName.trim())) setFailed(true)
    } catch {
      setFailed(true)
    } finally {
      launchingRef.current = false
      setLaunching(false)
    }
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs" {...backdrop}>
      <section
        role="dialog" aria-modal="true" aria-labelledby="batch-dialog-title"
        className="w-full max-w-xl max-h-[85vh] flex flex-col rounded-2xl border border-[var(--border-color)] bg-[var(--bg-secondary)] shadow-2xl text-[var(--text-primary)]"
        onKeyDown={event => {
          // Keep the app's global shortcuts and focus outside the dialog inactive.
          event.stopPropagation()
          if (event.key !== 'Tab') return
          const elements = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)'))
          const first = elements[0]
          const last = elements[elements.length - 1]
          if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
          else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
        }}
      >
        <header className="flex items-center justify-between px-5 py-4 border-b border-[var(--border-color)]">
          <h2 id="batch-dialog-title" className="flex items-center gap-2 text-sm font-bold"><FolderGit2 size={17} />{labels.title}</h2>
          <button type="button" onClick={cancel} disabled={launching} aria-label={labels.cancel} className="p-1 rounded hover:bg-[var(--bg-tertiary)] disabled:opacity-50"><X size={17} /></button>
        </header>
        <form className="flex flex-col min-h-0" onSubmit={event => { event.preventDefault(); void launch() }}>
          <div className="p-5 space-y-4 overflow-y-auto">
            <div>
              <label htmlFor="batch-worktree-name" className="block text-xs font-semibold mb-1.5">{labels.worktree}</label>
              <input ref={inputRef} id="batch-worktree-name" value={worktreeName} onChange={event => setWorktreeName(event.target.value)} disabled={launching} maxLength={80}
                aria-invalid={!validName} aria-describedby="batch-worktree-help"
                className="w-full rounded-lg px-3 py-2 text-sm bg-[var(--bg-primary)] border border-[var(--border-color)] focus:outline-[var(--accent-color)]" />
              <p id="batch-worktree-help" className={`mt-1 text-xs ${validName ? 'text-[var(--text-muted)]' : 'text-rose-400'}`}>{validName ? labels.worktreeHint : labels.invalidName}</p>
            </div>
            <div>
              <h3 className="text-xs font-semibold mb-1">{labels.order}</h3>
              <p className="text-xs text-[var(--text-muted)] mb-2">{labels.orderHint}</p>
              <ol className="space-y-2">
                {orderedTasks.map((task, index) => (
                  <li key={task.id} className="flex items-center gap-2 p-2 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)]">
                    <span className="text-xs text-[var(--text-muted)] w-5 text-center">{index + 1}</span>
                    <div className="flex-1 min-w-0"><span className="text-xs font-mono text-[var(--text-muted)]">{task.key}</span><p className="text-sm truncate" title={task.title}>{task.title}</p></div>
                    <button type="button" disabled={launching || index === 0} onClick={() => move(index, -1)} aria-label={`${labels.moveUp} ${task.key}`} className="p-1.5 rounded hover:bg-[var(--bg-tertiary)] disabled:opacity-30"><ArrowUp size={15} /></button>
                    <button type="button" disabled={launching || index === orderedTasks.length - 1} onClick={() => move(index, 1)} aria-label={`${labels.moveDown} ${task.key}`} className="p-1.5 rounded hover:bg-[var(--bg-tertiary)] disabled:opacity-30"><ArrowDown size={15} /></button>
                  </li>
                ))}
              </ol>
            </div>
            {failed && <p role="alert" className="text-xs text-rose-400">{labels.failed}</p>}
          </div>
          <footer className="flex items-center justify-end gap-2 px-5 py-3 border-t border-[var(--border-color)]">
            <button type="button" disabled={launching} onClick={cancel} className="px-3 py-1.5 text-xs font-semibold rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] disabled:opacity-50">{labels.cancel}</button>
            <button type="submit" disabled={launching || !validName} className="flex items-center gap-2 px-3 py-1.5 text-xs font-semibold rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] hover:bg-[var(--bg-primary)] disabled:opacity-50 disabled:cursor-not-allowed">{launching && <Loader2 size={13} className="animate-spin" />}{launching ? labels.launching : labels.launch}</button>
          </footer>
        </form>
      </section>
    </div>
  )
}
