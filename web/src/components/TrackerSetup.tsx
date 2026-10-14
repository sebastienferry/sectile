import React, { useEffect, useState } from 'react'
import { X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { getTrackers, initialTracker, type TrackerKind, type TrackerSetupTarget } from '../lib/trackers'
import { TrackerCredentialForm } from './TrackerCredentialForm'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'

/**
 * Connects a tracker so the user acts on it under their own account (#795).
 * Opened after a project save for one specific tracker, the dialog starts on
 * that tracker's provider and site with no provider picker; the shared form,
 * also used by the profile, handles the rest.
 */
export const TrackerSetup: React.FC<{ onClose: () => void; target?: TrackerSetupTarget | null }> = ({ onClose, target }) => {
  const { settings, refreshUserCredentials, trackerSetupTarget, t } = useApp()
  const chosen = target ?? trackerSetupTarget ?? null
  // Only used when no tracker was chosen: a chosen one fixes the provider below.
  const [tracker, setTracker] = useState<TrackerKind>(initialTracker(settings.issueTracker))
  const trackers = getTrackers(t)

  const backdrop = useBackdropDismiss(onClose)

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  useEscapeKey(true, onClose)

  const effectiveTracker = chosen ? chosen.provider : tracker

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-60 flex items-center justify-center p-4 bg-black/70 backdrop-blur-xs" {...backdrop}>
      <div className="relative w-full max-w-lg rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.9)]">
        <div className="flex items-start justify-between px-5 py-4 border-b border-[var(--border-color)]">
          <div className="min-w-0">
            <h2 className="text-sm font-bold text-[var(--text-primary)]">{t.trackerCredentials.setup.title}</h2>
            <p className="text-[11px] text-[var(--text-secondary)] mt-0.5 leading-relaxed">
              {t.trackerCredentials.setup.description}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer shrink-0"
            title={t.trackerCredentials.setup.closeTitle}
          >
            <X size={16} />
          </button>
        </div>

        <div className="p-5 space-y-3 overflow-y-auto">
          {!chosen && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                {t.trackerCredentials.setup.trackerLabel}
              </label>
              <div className="flex items-center gap-1.5">
                {trackers.map(item => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setTracker(item.id)}
                    className={`px-3 py-1.5 rounded-xl text-xs font-semibold border cursor-pointer ${
                      item.id === effectiveTracker
                        ? 'accent-bg text-white border-transparent'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                    }`}
                  >
                    {item.label}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Le formulaire est remonté à neuf quand le tracker change : ses
              champs appartiennent au tracker choisi, pas à l'écran. */}
          <TrackerCredentialForm
            key={`${effectiveTracker}:${chosen?.site || ''}`}
            tracker={effectiveTracker}
            initialSiteUrl={chosen?.site}
            onSaved={onClose}
          />
        </div>

        <div className="flex items-center justify-end px-5 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            {t.trackerCredentials.setup.later}
          </button>
        </div>
      </div>
    </div>
  )
}
