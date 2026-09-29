import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Copy } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { resolveTaskStage, skillForStage } from '../lib/workflow'
import { format } from '../lib/i18n'
import { anchoredMenuPosition, type AnchoredMenuPosition } from '../lib/anchoredMenu'
import { SKILL_COMMANDS, taskSkillPrompt } from '../lib/skillPrompt'
import type { Task } from '../types'

const FALLBACK_WIDTH = 280

/**
 * One click copies the prompt of the step the task's column names, to paste
 * into an AI engine's desktop app. The `(...)` menu offers the same prompt and
 * the full chain one; this button is the shortcut for the former (#612).
 */
export function CopyStepPromptButton({ task, className = '' }: { task: Task; className?: string }) {
  const { skillCommand, projects, currentProject, addToast, t } = useApp()
  // What the clipboard refused, shown next to the button to select by hand.
  const [fallback, setFallback] = useState('')
  const [fallbackPos, setFallbackPos] = useState<AnchoredMenuPosition | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!fallback) return
    const close = () => {
      setFallback('')
      buttonRef.current?.focus({ preventScroll: true })
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      close()
    }
    const handleOutside = (e: Event) => {
      const target = e.target as Node
      if (panelRef.current?.contains(target) || buttonRef.current?.contains(target)) return
      close()
    }
    const handleResize = () => setFallback('')
    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('mousedown', handleOutside)
    window.addEventListener('scroll', handleOutside, true)
    window.addEventListener('resize', handleResize)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('mousedown', handleOutside)
      window.removeEventListener('scroll', handleOutside, true)
      window.removeEventListener('resize', handleResize)
    }
  }, [fallback])

  const project = projects.find(item => item.id === task.projectId) || currentProject
  const stageSkill = skillForStage(resolveTaskStage(task, project))
  // A finished task has no step left to copy.
  if (!stageSkill) return null
  const command = skillCommand(stageSkill, SKILL_COMMANDS[stageSkill], task.projectId)
  const label = format(t.taskDetail.copySkill.copyCommand, { command })

  const copy = async () => {
    const prompt = taskSkillPrompt(command, task.id, task.projectId)
    try {
      await navigator.clipboard.writeText(prompt)
      setFallback('')
      addToast({ type: 'info', title: t.shell.card.promptCopied, description: format(t.shell.card.promptCopiedDescription, { command, key: task.key }) })
    } catch {
      // No clipboard (insecure origin) or a refused one: the prompt is still
      // there to select.
      if (buttonRef.current) setFallbackPos(anchoredMenuPosition(buttonRef.current, FALLBACK_WIDTH))
      setFallback(prompt)
    }
  }

  return (
    <>
      <button
        type="button"
        ref={buttonRef}
        onClick={e => {
          e.stopPropagation()
          copy()
        }}
        className={`${className} p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--accent-color)] hover:bg-[var(--accent-light)] border border-transparent hover:border-[var(--accent-color)]/30 transition-colors cursor-pointer`}
        title={label}
        aria-label={label}
      >
        <Copy size={14} />
      </button>
      {fallback && fallbackPos && createPortal(
        <div
          ref={panelRef}
          role="dialog"
          aria-label={t.taskDetail.copySkill.clipboardBlocked}
          onClick={e => e.stopPropagation()}
          onMouseDown={e => e.stopPropagation()}
          style={{ position: 'fixed', left: fallbackPos.left, top: fallbackPos.top, bottom: fallbackPos.bottom, width: FALLBACK_WIDTH, maxHeight: fallbackPos.maxHeight }}
          className="overflow-y-auto rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl p-2 z-[100] text-xs"
        >
          <p className="pb-1.5 text-[10px] text-[var(--text-muted)]">{t.taskDetail.copySkill.clipboardBlocked}</p>
          <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-[var(--bg-primary)] p-2 text-[10px] select-text"><code>{fallback}</code></pre>
        </div>,
        document.body,
      )}
    </>
  )
}
