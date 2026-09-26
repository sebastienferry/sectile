import React, { useEffect, useRef } from 'react'
import { CheckCircle2, AlertTriangle, AlertCircle, Info, X, ExternalLink } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { createDismissTimer, toastDuration, type DismissTimer } from '../lib/toastTimer'
import type { ToastMessage } from '../types'

const ToastItem: React.FC<{ toast: ToastMessage; onRemove: (id: string) => void }> = ({ toast, onRemove }) => {
  const { t } = useApp()
  const timerRef = useRef<DismissTimer | null>(null)
  // Only a toast offering a link waits for the user; the others keep their pace.
  const holdable = toast.link ? timerRef : null

  const duration = toastDuration(toast)

  useEffect(() => {
    const timer = createDismissTimer(duration, () => onRemove(toast.id))
    timerRef.current = timer
    return () => timer.cancel()
  }, [toast.id, duration, onRemove])

  // Focus moving between the link and the close button stays inside the toast.
  const handleBlur = (e: React.FocusEvent<HTMLDivElement>) => {
    if (e.currentTarget.contains(e.relatedTarget as Node | null)) return
    holdable?.current?.resume('focus')
  }

  const getIcon = (type: ToastMessage['type']) => {
    switch (type) {
      case 'success':
        return <CheckCircle2 size={16} className="text-emerald-400 shrink-0" />
      case 'warning':
        return <AlertTriangle size={16} className="text-amber-400 shrink-0" />
      case 'error':
        return <AlertCircle size={16} className="text-rose-400 shrink-0" />
      default:
        return <Info size={16} className="text-blue-400 shrink-0" />
    }
  }

  return (
    <div
      className="pointer-events-auto flex items-start gap-2.5 p-3 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-xl text-xs animate-in slide-in-from-bottom-2 fade-in duration-200"
      onMouseEnter={() => holdable?.current?.pause('hover')}
      onMouseLeave={() => holdable?.current?.resume('hover')}
      onFocus={() => holdable?.current?.pause('focus')}
      onBlur={handleBlur}
    >
      <div className="mt-0.5">{getIcon(toast.type)}</div>
      <div className="flex-1 min-w-0">
        <div className="font-semibold text-[var(--text-primary)]">
          {toast.title}
        </div>
        {toast.description && (
          <div className="text-[11px] text-[var(--text-muted)] mt-0.5 line-clamp-2">
            {toast.description}
          </div>
        )}
        {toast.link && (
          <div className="flex items-center gap-2 mt-1.5">
            <button
              type="button"
              onClick={() => {
                toast.link?.onOpen()
                onRemove(toast.id)
              }}
              className="text-[11px] font-medium text-[var(--accent-color)] hover:underline"
            >
              {toast.link.label}
            </button>
            {toast.link.externalUrl && (
              <a
                href={toast.link.externalUrl}
                target="_blank"
                rel="noopener noreferrer"
                title={t.toasts.openInTracker}
                aria-label={t.toasts.openInTracker}
                className="p-0.5 text-[var(--text-muted)] hover:text-[var(--text-primary)] rounded transition-colors"
              >
                <ExternalLink size={12} />
              </a>
            )}
          </div>
        )}
      </div>
      <button
        onClick={() => onRemove(toast.id)}
        title={t.operations.notifications.dismiss}
        aria-label={t.operations.notifications.dismiss}
        className="p-0.5 text-[var(--text-muted)] hover:text-[var(--text-primary)] rounded transition-colors"
      >
        <X size={14} />
      </button>
    </div>
  )
}

export const ToastContainer: React.FC = () => {
  const { toasts, removeToast } = useApp()

  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm pointer-events-none">
      {toasts.map(toast => (
        <ToastItem key={toast.id} toast={toast} onRemove={removeToast} />
      ))}
    </div>
  )
}
