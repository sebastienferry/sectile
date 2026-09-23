import React from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useApp } from '../context/AppContext'

/**
 * Le bandeau qui empêche de prendre une interface en panne pour une interface
 * vide. Tant qu'une lecture dont tout dépend — les projets, les tickets — ne
 * revient pas, il nomme ce que le serveur a répondu et reste en place : un
 * toast, lui, disparaît avant qu'on ait fini de lire un board vide.
 */
export const DegradedReadBanner: React.FC = () => {
  const { coreReadFailures, retryFailedReads, t } = useApp()

  if (coreReadFailures.length === 0) return null

  const detail = coreReadFailures
    .map(failure => `${t.reads.resources[failure.resource]} (${failure.detail})`)
    .join(', ')

  return (
    <div
      data-degraded-reads
      role="alert"
      className="flex items-center gap-2.5 px-4 py-2 border-b border-rose-500/30 bg-rose-500/10 text-rose-200"
    >
      <AlertTriangle size={14} className="shrink-0 text-rose-400" />
      <div className="min-w-0 flex-1">
        <span className="text-xs font-bold">{t.reads.bannerTitle}</span>
        <span className="text-xs text-rose-200/80 ml-2">
          {t.reads.bannerDescription.replace('{detail}', detail)}
        </span>
      </div>
      <button
        type="button"
        onClick={retryFailedReads}
        className="shrink-0 flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold border border-rose-500/40 hover:bg-rose-500/20 cursor-pointer"
      >
        <RefreshCw size={11} />
        {t.reads.retry}
      </button>
    </div>
  )
}
