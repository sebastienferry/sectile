import React, { useEffect, useState } from 'react'
import { MessageSquare, Send, RefreshCw, Loader2, User } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { MarkdownEditor, MarkdownView } from './Markdown'
import type { Task, TaskComment } from '../types'

/**
 * Commentaires d'une tâche : lecture et écriture.
 *
 * Sur un ticket suivi par un tracker, le tracker est la source de vérité — les
 * commentaires y sont lus à l'ouverture et publiés dessus, plutôt que recopiés
 * en base où ils divergeraient. Une tâche purement locale les garde en base.
 */
export const TaskComments: React.FC<{ task: Task }> = ({ task }) => {
  const { getTaskComments, postTaskComment } = useApp()

  const [comments, setComments] = useState<TaskComment[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [draft, setDraft] = useState('')
  const [isPosting, setIsPosting] = useState(false)

  const load = async () => {
    setIsLoading(true)
    const list = await getTaskComments(task.id)
    // Le tracker les renvoie du plus ancien au plus récent ; on inverse pour
    // avoir le dernier échange en haut, sans avoir à dérouler.
    setComments([...list].reverse())
    setIsLoading(false)
  }

  useEffect(() => {
    load()
    // Rechargé au changement de tâche uniquement : chaque lecture coûte un appel
    // au tracker.
  }, [task.id])

  const submit = async () => {
    const body = draft.trim()
    if (!body || isPosting) return
    setIsPosting(true)
    const updated = await postTaskComment(task.id, body)
    setIsPosting(false)
    if (updated) {
      setComments([...updated].reverse())
      setDraft('')
    }
  }

  const formatDate = (iso?: string) => {
    if (!iso) return ''
    try {
      return new Date(iso).toLocaleString([], {
        day: '2-digit',
        month: '2-digit',
        year: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
    } catch {
      return ''
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <label className="text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
          <MessageSquare size={12} className="text-[var(--accent-color)]" />
          <span>
            Commentaires{comments.length > 0 ? ` (${comments.length})` : ''}
            {task.source && task.source !== 'local' ? ` · ${task.source}` : ''}
          </span>
        </label>
        <button
          type="button"
          onClick={load}
          disabled={isLoading}
          className="flex items-center gap-1 px-2 py-1 rounded-md text-[10px] font-bold text-[var(--text-secondary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:text-[var(--text-primary)] disabled:opacity-40 transition-colors cursor-pointer"
          title="Relire les commentaires depuis le tracker"
        >
          <RefreshCw size={10} className={isLoading ? 'animate-spin' : ''} />
          <span>Actualiser</span>
        </button>
      </div>

      <div className="space-y-1.5">
        <MarkdownEditor
          value={draft}
          onChange={setDraft}
          minHeight={80}
          onKeyDown={e => {
            // Cmd/Ctrl+Entrée publie, comme partout ailleurs dans l'app.
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault()
              submit()
            }
          }}
          placeholder={
            task.source && task.source !== 'local'
              ? `Commenter ${task.key} dans ${task.source}… (Cmd+Entrée pour publier)`
              : 'Ajouter un commentaire… (Cmd+Entrée pour enregistrer)'
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
            <span>{isPosting ? 'Publication…' : 'Publier'}</span>
          </button>
        </div>
      </div>

      {isLoading && comments.length === 0 ? (
        <div className="flex items-center justify-center gap-2 py-6 text-[var(--text-muted)]">
          <Loader2 size={14} className="animate-spin text-[var(--accent-color)]" />
          <span className="text-xs">Lecture des commentaires…</span>
        </div>
      ) : comments.length === 0 ? (
        <p className="text-[11px] text-[var(--text-muted)] py-2">
          Aucun commentaire{task.source && task.source !== 'local' ? ` sur ${task.key}` : ''}. Le premier ci-dessus partira {task.source && task.source !== 'local' ? `dans ${task.source}` : 'en base locale'}.
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
                  {comment.author || 'Inconnu'}
                </span>
                {comment.createdAt && (
                  <span className="text-[10px] font-mono text-[var(--text-muted)] ml-auto shrink-0">
                    {formatDate(comment.createdAt)}
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
