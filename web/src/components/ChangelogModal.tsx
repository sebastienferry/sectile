import React, { useEffect } from 'react'
import { ScrollText, X, Loader2 } from 'lucide-react'
import { MarkdownView } from './Markdown'
import { useChangelog, type VersionInfo } from '../hooks/useVersion'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'

/**
 * The release notes, as the server embeds them.
 *
 * The file arrives as Markdown and is rendered as written: what the release
 * manager proofread before cutting the tag is exactly what the reader sees.
 */
export const ChangelogModal: React.FC<{
  open: boolean
  onClose: () => void
  version: VersionInfo | null
}> = ({ open, onClose, version }) => {
  const { markdown, isLoading, error } = useChangelog(open)
  const backdrop = useBackdropDismiss(onClose)

  useEffect(() => {
    if (!open) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200"
      {...backdrop}
    >
      <div
        className="relative w-full max-w-3xl rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.92)] animate-in zoom-in-95 duration-150"
        role="dialog"
        aria-modal="true"
        aria-labelledby="changelog-modal-title"
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-[var(--border-color)] bg-[var(--bg-tertiary)]/30 shrink-0">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-xl bg-[var(--accent-light)] accent-text flex items-center justify-center shadow-xs">
              <ScrollText size={16} />
            </div>
            <div>
              <h3 id="changelog-modal-title" className="text-sm font-bold text-[var(--text-primary)]">
                Release notes
              </h3>
              <p className="text-[11px] text-[var(--text-muted)]">
                {version
                  ? `This server runs ${version.version}${version.commit ? ` · ${version.commit.slice(0, 12)}` : ''}`
                  : 'Server version unavailable'}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] transition-colors cursor-pointer"
          >
            <X size={17} />
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 text-xs">
          {isLoading ? (
            <div className="flex items-center justify-center gap-2 py-10 text-[var(--text-muted)]">
              <Loader2 size={18} className="animate-spin" />
              <span>Loading the release notes…</span>
            </div>
          ) : error ? (
            <p className="py-10 text-center text-rose-400">
              Release notes unavailable: {error}
            </p>
          ) : (
            <MarkdownView>{markdown || ''}</MarkdownView>
          )}
        </div>
      </div>
    </div>
  )
}
