import React, { useEffect, useRef, useState } from 'react'
import { MessageSquare, Send, RefreshCw, Loader2, User } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { MarkdownEditor, MarkdownView } from './Markdown'
import { format, formatDateTime } from '../lib/i18n'
import type { Task, TaskComment } from '../types'
import { createLatestRequest } from '../lib/latestRequest'

/** Keep drafts and pending requests scoped to one ticket. */
export const TaskComments: React.FC<{ task: Task }> = ({ task }) => (
  <TaskCommentsForTask key={task.id} task={task} />
)

const TaskCommentsForTask: React.FC<{ task: Task }> = ({ task }) => {
  const { getTaskComments, postTaskComment, t, settings } = useApp()
  const strings = t.taskDetail.comments
  const onTracker = Boolean(task.source && task.source !== 'local')

  const [comments, setComments] = useState<TaskComment[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [draft, setDraft] = useState('')
  const [isPosting, setIsPosting] = useState(false)

  const [loadFailed, setLoadFailed] = useState(false)
  const [request] = useState(createLatestRequest)
  const posting = useRef(false)

  const load = async () => {
    if (posting.current) return
    const ticket = request.begin()
    setIsLoading(true)
    setLoadFailed(false)
    try {
      const list = await getTaskComments(task.id)
      if (!request.isLatest(ticket)) return
      if (list === null) setLoadFailed(true)
      else setComments([...list].reverse())
    } finally {
      if (request.isLatest(ticket)) setIsLoading(false)
    }
  }

  useEffect(() => {
    load()
    return () => { request.begin() }
    // Read once per ticket; provider rerenders do not reload tracker comments.
  }, [task.id])

  const submit = async () => {
    const submittedDraft = draft
    const body = submittedDraft.trim()
    if (!body || posting.current) return
    posting.current = true
    const ticket = request.begin()
    setIsLoading(false)
    setIsPosting(true)
    try {
      const updated = await postTaskComment(task.id, body)
      if (!request.isLatest(ticket)) return
      if (updated) {
        setComments([...updated].reverse())
        setLoadFailed(false)
        // Preserve anything written while the previous comment was being sent.
        setDraft(current => current === submittedDraft ? '' : current)
      }
    } finally {
      posting.current = false
      if (request.isLatest(ticket)) setIsPosting(false)
    }
  }

  const formatCommentDate = (iso?: string) =>
    iso
      ? formatDateTime(settings.language, iso, {
          day: '2-digit',
          month: '2-digit',
          year: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
        })
      : ''

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <label className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
          <MessageSquare size={12} className="text-[var(--accent-color)]" />
          <span>
            {strings.title}{comments.length > 0 ? ` (${comments.length})` : ''}
            {onTracker ? ` · ${task.source}` : ''}
          </span>
        </label>
        <button
          type="button"
          onClick={load}
          disabled={isLoading || isPosting}
          className="flex items-center gap-1 px-2 py-1 rounded-md text-[10px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 transition-colors cursor-pointer"
          title={strings.refreshTitle}
        >
          <RefreshCw size={10} className={isLoading ? 'animate-spin' : ''} />
          <span>{strings.refresh}</span>
        </button>
      </div>

      <div className="space-y-1.5">
        <MarkdownEditor
          value={draft}
          onChange={setDraft}
          minHeight={80}
          onKeyDown={e => {
            // Cmd/Ctrl+Enter publishes the comment.
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault()
              submit()
            }
          }}
          placeholder={
            onTracker
              ? format(strings.placeholderTracker, { key: task.key, source: task.source || '' })
              : strings.placeholderLocal
          }
        />
        <div className="flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={submit}
            disabled={!draft.trim() || isPosting}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 transition-all cursor-pointer"
          >
            {isPosting ? <Loader2 size={12} className="animate-spin" /> : <Send size={12} />}
            <span>{isPosting ? strings.posting : strings.post}</span>
          </button>
        </div>
      </div>

      {loadFailed && (
        <p role="alert" className="text-xs text-[var(--text-secondary)]">
          {t.operations.notifications.comments.unavailable}
        </p>
      )}
      {isLoading && comments.length === 0 ? (
        <div className="flex items-center justify-center gap-2 py-6 text-[var(--text-muted)]">
          <Loader2 size={14} className="animate-spin text-[var(--accent-color)]" />
          <span className="text-xs">{strings.loading}</span>
        </div>
      ) : comments.length === 0 && !loadFailed ? (
        <p className="text-[11px] text-[var(--text-muted)] py-2">
          {onTracker ? format(strings.emptyTracker, { key: task.key, source: task.source || '' }) : strings.emptyLocal}
        </p>
      ) : (
        <div className="space-y-2 max-h-[calc(var(--app-h)*0.42)] overflow-y-auto pr-1">
          {comments.map(comment => (
            <div
              key={comment.id}
              className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/60 border border-[var(--border-color)]"
            >
              <div className="flex items-center gap-2 mb-1">
                <span className="w-5 h-5 rounded-full bg-[var(--accent-light)] text-[var(--accent-color)] flex items-center justify-center shrink-0">
                  <User size={11} />
                </span>
                <span className="text-[11px] font-bold text-[var(--text-primary)] truncate">
                  {comment.author || strings.unknownAuthor}
                </span>
                {comment.createdAt && (
                  <span className="text-[10px] font-mono text-[var(--text-muted)] ml-auto shrink-0">
                    {formatCommentDate(comment.createdAt)}
                  </span>
                )}
              </div>
              <MarkdownView compact>{comment.body}</MarkdownView>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
