import { useEffect, useState } from 'react'
import { MarkdownView } from './Markdown'

type TaskReport = {
  reportId: string
  stage: string
  round?: number
  summary: string
  body: string
  createdAt: string
  warning?: string
  attachment?: { filename: string; content: string }
}

/** Retained reports load independently so tracker failures cannot hide them. */
export function TaskReports({ taskId, revision }: { taskId: string; revision?: string }) {
  const [history, setHistory] = useState<{ taskId: string; reports: TaskReport[]; error: boolean }>({ taskId: '', reports: [], error: false })
  const [refresh, setRefresh] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    fetch(`/api/tasks/${encodeURIComponent(taskId)}/reports`, { signal: controller.signal })
      .then(async response => {
        if (!response.ok) throw new Error('Report history unavailable')
        return response.json() as Promise<TaskReport[]>
      })
      .then(reports => setHistory({ taskId, reports, error: false }))
      .catch(() => { if (!controller.signal.aborted) setHistory({ taskId, reports: [], error: true }) })
    return () => controller.abort()
  }, [taskId, revision, refresh])

  const reports = history.taskId === taskId ? history.reports : []
  if (history.taskId === taskId && history.error) return <button type="button" onClick={() => setRefresh(value => value + 1)} className="text-xs">Historique des rapports indisponible · Réessayer</button>
  if (!reports.length) return null
  const stages: Record<string, string> = { clarified: 'Clarification', specified: 'Spécification', implemented: 'Implémentation', reviewed: 'Revue', finished: 'Clôture' }
  return <section aria-label="Historique des rapports" className="space-y-2">
    <h3 className="text-xs font-bold">Rapports d’étape</h3>
    {reports.map(report => <article key={report.reportId} className="p-2.5 rounded-xl border border-[var(--border-color)]">
      <time className="text-[10px]" dateTime={report.createdAt}>{new Date(report.createdAt).toLocaleString()}</time>
      <p className="text-xs font-bold">{stages[report.stage] || report.stage}{report.round ? ` · Tour ${report.round}` : ''}</p>
      <p className="text-xs whitespace-pre-wrap">{report.summary}</p>
      {report.warning && <p className="text-xs text-[var(--text-muted)]">Pièce jointe indisponible : {report.warning}</p>}
      {report.attachment && /^https?:\/\//i.test(report.attachment.content) && <a href={report.attachment.content} target="_blank" rel="noopener noreferrer" className="text-xs underline">{report.attachment.filename}</a>}
      <details>
        <summary className="text-xs cursor-pointer">Lire le rapport complet</summary>
        <MarkdownView compact>{report.body}</MarkdownView>
      </details>
    </article>)}
  </section>
}
