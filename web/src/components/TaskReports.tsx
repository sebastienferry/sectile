import { useEffect, useState } from 'react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
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
  const strings = useApp().t.taskDetail.reports
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
  if (history.taskId === taskId && history.error) return <button type="button" onClick={() => setRefresh(value => value + 1)} className="text-xs">{strings.unavailable}</button>
  if (!reports.length) return null
  return <section aria-label={strings.region} className="space-y-2">
    <h3 className="text-xs font-bold">{strings.title}</h3>
    {reports.map(report => <article key={report.reportId} className="p-2.5 rounded-xl border border-[var(--border-color)]">
      <time className="text-[10px]" dateTime={report.createdAt}>{new Date(report.createdAt).toLocaleString()}</time>
      <p className="text-xs font-bold">{strings.stages[report.stage] || report.stage}{report.round ? ` · ${format(strings.round, { round: report.round })}` : ''}</p>
      <p className="text-xs whitespace-pre-wrap">{report.summary}</p>
      {report.warning && <p className="text-xs text-[var(--text-muted)]">{format(strings.attachmentUnavailable, { warning: report.warning })}</p>}
      {report.attachment && /^https?:\/\//i.test(report.attachment.content) && <a href={report.attachment.content} target="_blank" rel="noopener noreferrer" className="text-xs underline">{report.attachment.filename}</a>}
      <details>
        <summary className="text-xs cursor-pointer">{strings.readFull}</summary>
        <MarkdownView compact>{report.body}</MarkdownView>
      </details>
    </article>)}
  </section>
}
