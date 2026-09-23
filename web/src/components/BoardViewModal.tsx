import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Bookmark, Trash2, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { boardViewFormError, normalizeViewLabels } from '../lib/boardViews'
import type { BoardView } from '../types'

/**
 * Creates or edits a saved board view (#387): a name, the projects it spans and
 * the labels a ticket must carry one of. The server validates again and its
 * message is shown as it comes; the checks here only spare a round trip.
 */
export const BoardViewModal: React.FC = () => {
  const { isBoardViewModalOpen, editingBoardView } = useApp()
  if (!isBoardViewModalOpen) return null
  // Keyed on the view, so every opening starts from the view's own values.
  return <BoardViewForm key={editingBoardView?.id ?? 'new'} editingBoardView={editingBoardView} />
}

const BoardViewForm: React.FC<{ editingBoardView: BoardView | null }> = ({ editingBoardView }) => {
  const {
    projects,
    boardViews,
    taskFacets,
    closeBoardViewModal,
    createBoardView,
    updateBoardView,
    deleteBoardView,
    t,
  } = useApp()

  const [name, setName] = useState(editingBoardView?.name ?? '')
  const [projectIds, setProjectIds] = useState<string[]>(editingBoardView?.projectIds ?? [])
  const [labels, setLabels] = useState<string[]>(editingBoardView?.labels ?? [])
  const [labelInput, setLabelInput] = useState('')
  const [showErrors, setShowErrors] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const backdrop = useBackdropDismiss(closeBoardViewModal)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeBoardViewModal()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [closeBoardViewModal])

  // Bookmarked projects first, as in the project switcher.
  const orderedProjects = useMemo(
    () => [...projects.filter(p => p.bookmarked), ...projects.filter(p => !p.bookmarked)],
    [projects]
  )

  const labelSuggestions = useMemo(
    () => (taskFacets.labels || []).map(l => l.value).filter(v => !labels.some(l => l.toLowerCase() === v.toLowerCase())),
    [taskFacets.labels, labels]
  )

  const addLabel = useCallback(() => {
    const next = normalizeViewLabels([...labels, labelInput])
    setLabels(next)
    setLabelInput('')
  }, [labels, labelInput])

  const formError = boardViewFormError(name, projectIds, boardViews, editingBoardView?.id ?? null)
  const errorMessage = formError === 'name'
    ? t.boardViews.errorName
    : formError === 'duplicate'
    ? t.boardViews.errorDuplicate
    : formError === 'projects'
    ? t.boardViews.errorProjects
    : null

  const toggleProject = (id: string) => {
    setProjectIds(prev => (prev.includes(id) ? prev.filter(p => p !== id) : [...prev, id]))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isSubmitting) return
    if (formError) {
      setShowErrors(true)
      return
    }
    // A label typed but not yet confirmed with Enter is still meant.
    const payload = {
      name: name.trim(),
      projectIds,
      labels: normalizeViewLabels([...labels, labelInput]),
    }
    setIsSubmitting(true)
    const saved = editingBoardView
      ? await updateBoardView(editingBoardView.id, payload)
      : await createBoardView(payload)
    setIsSubmitting(false)
    if (saved) closeBoardViewModal()
  }

  const handleDelete = async () => {
    if (!editingBoardView) return
    if (!window.confirm(t.boardViews.deleteConfirm.replace('{name}', editingBoardView.name))) return
    if (await deleteBoardView(editingBoardView.id)) closeBoardViewModal()
  }

  const inputClass = 'w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] placeholder-[var(--text-muted)] focus:outline-none focus:border-[var(--accent-color)]'
  const labelClass = 'block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1'

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150" {...backdrop}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="board-view-modal-title"
        className="relative w-full max-w-lg max-h-[calc(var(--app-h)-2rem)] flex flex-col rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden"
      >
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <div id="board-view-modal-title" className="flex items-center gap-2 font-semibold text-xs text-[var(--text-primary)]">
            <div className="w-5 h-5 rounded-md accent-bg text-white flex items-center justify-center">
              <Bookmark size={12} />
            </div>
            <span>{editingBoardView ? t.boardViews.editTitle : t.boardViews.createTitle}</span>
          </div>
          <button
            type="button"
            onClick={closeBoardViewModal}
            aria-label={t.boardViews.cancel}
            className="p-1 rounded-md text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={16} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4 overflow-y-auto min-h-0">
          <div>
            <label htmlFor="board-view-name" className={labelClass}>{t.boardViews.name} *</label>
            <input
              id="board-view-name"
              autoFocus
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder={t.boardViews.namePlaceholder}
              className={`${inputClass} font-semibold`}
            />
          </div>

          <fieldset>
            <legend className={labelClass}>{t.boardViews.projects} *</legend>
            <div className="max-h-44 overflow-y-auto rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)]/60 p-1.5 space-y-0.5">
              {orderedProjects.map(p => (
                <label key={p.id} className="flex items-center gap-2 px-2 py-1 rounded-lg text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] cursor-pointer">
                  <input
                    type="checkbox"
                    checked={projectIds.includes(p.id)}
                    onChange={() => toggleProject(p.id)}
                    className="accent-[var(--accent-color)]"
                  />
                  <span className="truncate">{p.name}</span>
                </label>
              ))}
            </div>
          </fieldset>

          <div>
            <label htmlFor="board-view-label" className={labelClass}>{t.boardViews.labels}</label>
            {labels.length > 0 && (
              <div className="flex flex-wrap gap-1 mb-1.5">
                {labels.map(label => (
                  <span key={label} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[var(--accent-light)] accent-text">
                    {label}
                    <button
                      type="button"
                      onClick={() => setLabels(labels.filter(l => l !== label))}
                      aria-label={`× ${label}`}
                      className="opacity-70 hover:opacity-100 cursor-pointer"
                    >
                      <X size={10} />
                    </button>
                  </span>
                ))}
              </div>
            )}
            <input
              id="board-view-label"
              type="text"
              list="board-view-label-suggestions"
              value={labelInput}
              onChange={e => setLabelInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' || e.key === ',') {
                  e.preventDefault()
                  addLabel()
                }
              }}
              placeholder={t.boardViews.labelPlaceholder}
              className={inputClass}
            />
            <datalist id="board-view-label-suggestions">
              {labelSuggestions.map(v => <option key={v} value={v} />)}
            </datalist>
            <p className="mt-1 text-[10px] text-[var(--text-muted)] leading-snug">{t.boardViews.labelsHint}</p>
          </div>

          {showErrors && errorMessage && (
            <p role="alert" className="text-[11px] font-semibold text-rose-400">{errorMessage}</p>
          )}

          <div className="flex items-center justify-between gap-2 pt-1">
            {editingBoardView ? (
              <button
                type="button"
                onClick={handleDelete}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
              >
                <Trash2 size={13} />
                <span>{t.boardViews.delete}</span>
              </button>
            ) : <span />}
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={closeBoardViewModal}
                className="px-3 py-1.5 rounded-lg text-xs font-semibold text-[var(--text-secondary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
              >
                {t.boardViews.cancel}
              </button>
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-3.5 py-1.5 rounded-lg text-xs font-bold accent-bg text-white disabled:opacity-50 cursor-pointer"
              >
                {editingBoardView ? t.boardViews.save : t.boardViews.create}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
