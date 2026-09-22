import React, { useEffect, useState } from 'react'
import { X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { getTrackers, initialTracker, type TrackerKind } from '../lib/trackers'
import { TrackerCredentialForm } from './TrackerCredentialForm'
import { useBackdropDismiss } from '../hooks/useBackdropDismiss'
import { useEscapeKey } from '../hooks/useEscapeKey'

/**
 * Premier démarrage : ce qu'il faut savoir avant que quoi que ce soit
 * fonctionne. Sans instance et sans jeton, la synchronisation ne ramène rien et
 * aucune écriture ne part; jusqu'ici on l'apprenait en synchronisant pour rien.
 *
 * Cet écran n'est plus que l'emballage : il choisit le tracker et confie le
 * reste au formulaire partagé, celui que l'onglet Trackers du profil utilise
 * aussi. Deux écrans qui demandent la même chose ont déjà divergé une fois.
 */
export const TrackerSetup: React.FC<{ onClose: () => void }> = ({ onClose }) => {
  const { settings, refreshUserCredentials, t } = useApp()
  const [tracker, setTracker] = useState<TrackerKind>(initialTracker(settings.issueTracker))
  const trackers = getTrackers(t)

  const backdrop = useBackdropDismiss(onClose)

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  useEscapeKey(true, onClose)

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
                    item.id === tracker
                      ? 'accent-bg text-white border-transparent'
                      : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          {/* Le formulaire est remonté à neuf quand le tracker change : ses
              champs appartiennent au tracker choisi, pas à l'écran. */}
          <TrackerCredentialForm key={tracker} tracker={tracker} onSaved={onClose} />
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
