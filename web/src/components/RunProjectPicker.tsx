import React from 'react'
import { FolderGit2, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { format } from '../lib/i18n'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'

/**
 * Asks which project a run works for when its ticket belongs to several and
 * the launch named none (#741). The server answered with the candidates; the
 * launch is made again for the one picked. Closing gives the launch up.
 */
export const RunProjectPicker: React.FC = () => {
  const { runProjectChoice, chooseRunProject, t } = useApp()
  const strings = t.runProjectPicker
  const cancel = () => chooseRunProject(null)
  const backdrop = useBackdropDismiss(cancel)
  useEscapeKey(Boolean(runProjectChoice), cancel)

  if (!runProjectChoice) return null

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-60 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs"
      {...backdrop}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="run-project-picker-title"
        className="relative w-full max-w-md rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden"
        data-run-project-picker
      >
        <div className="flex items-start justify-between gap-3 px-5 py-4 border-b border-[var(--border-color)]">
          <div className="min-w-0">
            <h2 id="run-project-picker-title" className="text-sm font-bold text-[var(--text-primary)]">{strings.title}</h2>
            <p className="text-[11px] text-[var(--text-secondary)] mt-0.5 leading-relaxed">
              {format(strings.description, { key: runProjectChoice.taskKey })}
            </p>
          </div>
          <button
            type="button"
            onClick={cancel}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer shrink-0"
            title={strings.cancel}
            aria-label={strings.cancel}
          >
            <X size={16} />
          </button>
        </div>
        <ul className="p-3 space-y-1.5">
          {runProjectChoice.candidates.map(candidate => (
            <li key={candidate.id}>
              <button
                type="button"
                onClick={() => chooseRunProject(candidate.id)}
                className="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-semibold text-left text-[var(--text-primary)] bg-[var(--bg-tertiary)] border border-[var(--border-color)] hover:border-[var(--accent-color)] cursor-pointer"
                data-run-project={candidate.id}
              >
                <FolderGit2 size={14} className="text-[var(--accent-color)] shrink-0" />
                <span className="truncate">{candidate.name}</span>
              </button>
            </li>
          ))}
        </ul>
        <div className="flex justify-end px-5 py-3 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <button
            type="button"
            onClick={cancel}
            className="px-3 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            {strings.cancel}
          </button>
        </div>
      </div>
    </div>
  )
}
