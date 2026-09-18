import React, { useEffect } from 'react'
import { KeyRound, Lock, LockOpen, ShieldCheck } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { credentialState, trackerFields, type TrackerKind } from '../lib/trackers'

/**
 * L'accès tracker, dans le profil, à côté des clés d'API et des postes
 * appairés : c'est du même ordre, un identifiant qui appartient à une personne.
 *
 * Ce panneau existe surtout pour une raison d'accès : l'écran de connexion ne
 * s'ouvrait que lors de la création d'un projet Jira sans accès enregistré,
 * donc une fois n'importe quelle valeur en base, plus personne ne pouvait y
 * revenir. Un jeton personnel a besoin d'un chemin durable.
 */
export const TrackerCredentialPanel: React.FC = () => {
  const { userCredentials, refreshUserCredentials, unlockUserCredential, clearUserCredential, setIsTrackerSetupOpen } =
    useApp()

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="block text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)] flex items-center gap-1.5">
          <KeyRound size={14} className="text-emerald-400" />
          <span>Mes accès tracker</span>
        </label>
        <button
          type="button"
          onClick={() => setIsTrackerSetupOpen(true)}
          className="px-3 py-1.5 rounded-xl text-[11px] font-semibold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          Connecter un tracker
        </button>
      </div>

      <p className="text-[10.5px] text-[var(--text-secondary)] leading-relaxed">
        Sur Jira, un commentaire ou un changement de statut est attribué au compte du jeton qui l'a
        écrit. Un jeton personnel fait donc porter votre nom à ce que vous écrivez, au lieu du compte
        partagé du serveur.
      </p>

      {userCredentials.length === 0 ? (
        <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/60 border border-[var(--border-color)] text-[10.5px] text-[var(--text-muted)]">
          {credentialState(undefined)}
        </div>
      ) : (
        <div className="space-y-1.5">
          {userCredentials.map(credential => {
            // trackerFields retombe sur le premier tracker pour un nom inconnu,
            // donc une valeur venue de la base ne casse pas l'affichage.
            const label = trackerFields(credential.tracker as TrackerKind).label
            const locked = credential.sealed && !credential.unlocked
            return (
              <div
                key={credential.tracker}
                className="p-2.5 rounded-xl bg-[var(--bg-tertiary)]/60 border border-[var(--border-color)] flex items-start gap-2"
              >
                {locked ? (
                  <Lock size={13} className="text-amber-400 shrink-0 mt-0.5" />
                ) : credential.sealed ? (
                  <LockOpen size={13} className="text-emerald-400 shrink-0 mt-0.5" />
                ) : (
                  <ShieldCheck size={13} className="text-emerald-400 shrink-0 mt-0.5" />
                )}
                <div className="min-w-0 flex-1">
                  <div className="text-[11px] font-semibold text-[var(--text-primary)]">
                    {label}
                    {credential.email ? <span className="font-normal text-[var(--text-muted)]"> · {credential.email}</span> : null}
                  </div>
                  <div className="text-[10px] text-[var(--text-secondary)] leading-relaxed">
                    {credentialState(credential)}
                  </div>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  {locked && (
                    <button
                      type="button"
                      onClick={() => {
                        const phrase = window.prompt('Phrase de scellement')
                        if (phrase) void unlockUserCredential(credential.tracker, phrase)
                      }}
                      className="px-2.5 py-1 rounded-lg text-[10px] font-semibold bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                    >
                      Déverrouiller
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      if (confirm(`Oublier votre accès ${label} ? Vous devrez saisir votre jeton à nouveau.`)) {
                        void clearUserCredential(credential.tracker)
                      }
                    }}
                    className="px-2.5 py-1 rounded-lg text-[10px] font-semibold bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer"
                  >
                    Oublier
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
