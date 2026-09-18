import React, { useEffect, useState } from 'react'
import { ChevronDown, ChevronRight, Lock, LockOpen, ShieldCheck, Circle } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { TRACKERS, credentialState, type TrackerKind } from '../lib/trackers'
import { TrackerCredentialForm } from './TrackerCredentialForm'

/**
 * Les trois trackers, chacun dans sa propre zone dépliable.
 *
 * Une personne n'en configure qu'un la plupart du temps, et la liste dit d'un
 * coup d'œil lequel est en place, lequel est verrouillé et lequel n'a rien.
 * Ouvrir une zone donne le formulaire complet de ce tracker, sans changer de
 * fenêtre : c'est une page de réglages, pas une boîte de dialogue.
 */
export const TrackerCredentialsTab: React.FC = () => {
  const { userCredentials, refreshUserCredentials } = useApp()
  const [open, setOpen] = useState<TrackerKind | null>(null)

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  return (
    <div className="space-y-4 animate-in fade-in duration-150">
      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        Vos accès aux trackers. Sur Jira, un commentaire ou un changement de statut est attribué au
        compte du jeton qui l'a écrit : un jeton personnel fait donc porter votre nom à ce que vous
        écrivez, plutôt qu'un compte partagé.
      </p>

      <div className="space-y-2">
        {TRACKERS.map(kind => {
          const mine = userCredentials.find(c => c.tracker === kind.id)
          const isOpen = open === kind.id
          const locked = Boolean(mine?.sealed && !mine.unlocked)
          return (
            <div
              key={kind.id}
              className="rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)]/40 overflow-hidden"
            >
              <button
                type="button"
                onClick={() => setOpen(isOpen ? null : kind.id)}
                className="w-full flex items-center gap-2 px-3 py-2.5 text-left cursor-pointer hover:bg-[var(--bg-tertiary)]/70"
              >
                {isOpen ? (
                  <ChevronDown size={14} className="text-[var(--text-muted)] shrink-0" />
                ) : (
                  <ChevronRight size={14} className="text-[var(--text-muted)] shrink-0" />
                )}
                {!mine ? (
                  <Circle size={13} className="text-[var(--text-muted)] shrink-0" />
                ) : locked ? (
                  <Lock size={13} className="text-amber-400 shrink-0" />
                ) : mine.sealed ? (
                  <LockOpen size={13} className="text-emerald-400 shrink-0" />
                ) : (
                  <ShieldCheck size={13} className="text-emerald-400 shrink-0" />
                )}
                <span className="text-xs font-bold text-[var(--text-primary)]">{kind.label}</span>
                <span className="text-[10px] text-[var(--text-muted)] truncate flex-1 text-right">
                  {mine?.email || credentialState(mine)}
                </span>
              </button>

              {isOpen && (
                <div className="px-3 pb-3 pt-1 border-t border-[var(--border-color)]">
                  <TrackerCredentialForm tracker={kind.id} />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
