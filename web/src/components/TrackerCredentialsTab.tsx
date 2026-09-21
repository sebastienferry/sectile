import React, { useEffect, useState } from 'react'
import {
  ChevronDown,
  ChevronRight,
  Lock,
  LockOpen,
  ShieldCheck,
  Circle,
  KeyRound,
  Check,
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { personalTrackers, credentialState, type TrackerKind } from '../lib/trackers'
import { TrackerCredentialForm } from './TrackerCredentialForm'

/**
 * Onglet des accès personnels aux trackers distants.
 * Une seule section dédiée au sommet gère la phrase de scellement unique
 * pour l'ensemble des jetons de l'utilisateur.
 */
export const TrackerCredentialsTab: React.FC = () => {
  const {
    userCredentials,
    refreshUserCredentials,
    unlockAllUserCredentials,
    lockAllUserCredentials,
    saveUserCredential,
    addToast,
    t,
  } = useApp()

  const [open, setOpen] = useState<TrackerKind | null>(null)
  const [unlockPhrase, setUnlockPhrase] = useState('')
  const [sharedPassphrase, setSharedPassphrase] = useState('')
  const [isUnlocking, setIsUnlocking] = useState(false)
  const [isLocking, setIsLocking] = useState(false)
  const [isApplying, setIsApplying] = useState(false)
  const [showChangePassphrase, setShowChangePassphrase] = useState(false)
  const [newPassphraseInput, setNewPassphraseInput] = useState('')

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  const anySealed = userCredentials.some(c => c.sealed)
  const anyLocked = userCredentials.some(c => c.sealed && !c.unlocked)
  const allUnlocked = anySealed && !anyLocked

  const handleUnlockAll = async () => {
    if (!unlockPhrase.trim()) return
    setIsUnlocking(true)
    const phrase = unlockPhrase.trim()
    const success = await unlockAllUserCredentials(phrase)
    setIsUnlocking(false)
    if (success) {
      setSharedPassphrase(phrase)
      setUnlockPhrase('')
    }
  }

  const handleLockAll = async () => {
    setIsLocking(true)
    await lockAllUserCredentials()
    setIsLocking(false)
    setSharedPassphrase('')
    setShowChangePassphrase(false)
  }

  const handleUpdateAllPassphrase = async (targetPhrase: string) => {
    setIsApplying(true)
    try {
      const configured = userCredentials.filter(c => c.siteUrl || c.email || c.sealed)
      for (const cred of configured) {
        await saveUserCredential({
          tracker: cred.tracker,
          siteUrl: cred.siteUrl,
          email: cred.email,
          token: '', // garde le jeton existant en base
          passphrase: targetPhrase.trim(),
        })
      }
      setSharedPassphrase(targetPhrase.trim())
      setShowChangePassphrase(false)
      setNewPassphraseInput('')
      addToast({
        type: 'success',
        title: targetPhrase.trim()
          ? t.trackerCredentials.toastUpdatedTitle
          : t.trackerCredentials.toastRemovedTitle,
        description: targetPhrase.trim()
          ? t.trackerCredentials.toastUpdatedDesc
          : t.trackerCredentials.toastRemovedDesc,
      })
      await refreshUserCredentials()
    } catch (err: any) {
      addToast({
        type: 'error',
        title: t.trackerCredentials.toastErrorTitle,
        description: err?.message || t.trackerCredentials.toastErrorDefault,
      })
    } finally {
      setIsApplying(false)
    }
  }

  return (
    <div className="space-y-4 animate-in fade-in duration-150">
      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        {t.trackerCredentials.description}
      </p>

      {/* Section Globale Dédiée : Phrase de scellement unique */}
      <div className="p-4 rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)]/50 space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <KeyRound size={15} className="text-[var(--accent-color)]" />
              <span className="text-xs font-bold text-[var(--text-primary)]">
                {t.trackerCredentials.masterPassphraseTitle}
              </span>
              {anyLocked ? (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-500 border border-amber-500/30">
                  <Lock size={10} /> {t.trackerCredentials.statusLocked}
                </span>
              ) : allUnlocked ? (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  <LockOpen size={10} /> {t.trackerCredentials.statusActive}
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-[var(--bg-secondary)] text-[var(--text-muted)] border border-[var(--border-color)]">
                  {t.trackerCredentials.statusNotConfigured}
                </span>
              )}
            </div>
            <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
              {t.trackerCredentials.passphraseDescription}
            </p>
          </div>
          {allUnlocked && (
            <button
              type="button"
              onClick={() => void handleLockAll()}
              disabled={isLocking}
              className="px-2.5 py-1 rounded-lg text-xs font-medium border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-amber-400 hover:border-amber-400/40 cursor-pointer shrink-0"
            >
              {isLocking ? t.trackerCredentials.locking : t.trackerCredentials.lock}
            </button>
          )}
        </div>

        {/* Cas A : Jetons verrouillés */}
        {anyLocked && (
          <div className="pt-2 border-t border-[var(--border-color)]/60 space-y-2">
            <p className="text-[11px] text-amber-400 font-medium">
              {t.trackerCredentials.unlockPrompt}
            </p>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <input
                  type="password"
                  autoComplete="new-password"
                  value={unlockPhrase}
                  onChange={e => setUnlockPhrase(e.target.value)}
                  placeholder={t.trackerCredentials.unlockPlaceholder}
                  className="w-full pl-8 pr-3 py-1.5 text-xs rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                  onKeyDown={e => {
                    if (e.key === 'Enter') void handleUnlockAll()
                  }}
                />
                <KeyRound size={13} className="absolute left-2.5 top-2 text-amber-400" />
              </div>
              <button
                type="button"
                onClick={() => void handleUnlockAll()}
                disabled={isUnlocking || !unlockPhrase.trim()}
                className="px-3.5 py-1.5 rounded-lg text-xs font-semibold accent-bg text-white shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer shrink-0"
              >
                {isUnlocking ? t.trackerCredentials.unlocking : t.trackerCredentials.unlockAll}
              </button>
            </div>
          </div>
        )}

        {/* Cas B : Jetons déverrouillés */}
        {allUnlocked && (
          <div className="pt-2 border-t border-[var(--border-color)]/60 space-y-2">
            <div className="flex items-center justify-between text-[11px]">
              <span className="text-emerald-400 font-medium flex items-center gap-1.5">
                <Check size={13} /> {t.trackerCredentials.operationalBanner}
              </span>
              <button
                type="button"
                onClick={() => setShowChangePassphrase(!showChangePassphrase)}
                className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--text-primary)] underline cursor-pointer"
              >
                {showChangePassphrase
                  ? t.trackerCredentials.cancelChangePassphrase
                  : t.trackerCredentials.changePassphrase}
              </button>
            </div>
            {showChangePassphrase && (
              <div className="p-2.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] space-y-2">
                <span className="text-[10px] text-[var(--text-muted)] block">
                  {t.trackerCredentials.changePassphrasePrompt}
                </span>
                <div className="flex items-center gap-2">
                  <input
                    type="password"
                    autoComplete="new-password"
                    value={newPassphraseInput}
                    onChange={e => setNewPassphraseInput(e.target.value)}
                    placeholder={t.trackerCredentials.newPassphrasePlaceholder}
                    className="flex-1 px-2.5 py-1.5 text-xs rounded-lg bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                  />
                  <button
                    type="button"
                    onClick={() => void handleUpdateAllPassphrase(newPassphraseInput)}
                    disabled={isApplying}
                    className="px-3 py-1.5 rounded-lg text-xs font-semibold accent-bg text-white shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer shrink-0"
                  >
                    {isApplying ? t.trackerCredentials.applying : t.trackerCredentials.apply}
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Cas C : Aucun jeton scellé pour le moment */}
        {!anySealed && (
          <div className="pt-2 border-t border-[var(--border-color)]/60 space-y-2">
            <span className="text-[10.5px] text-[var(--text-muted)] block">
              {t.trackerCredentials.definePassphrasePrompt}
            </span>
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <input
                  type="password"
                  autoComplete="new-password"
                  value={sharedPassphrase}
                  onChange={e => setSharedPassphrase(e.target.value)}
                  placeholder={t.trackerCredentials.definePassphrasePlaceholder}
                  className="w-full pl-8 pr-3 py-1.5 text-xs rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                />
                <Lock size={13} className="absolute left-2.5 top-2 text-[var(--accent-color)]" />
              </div>
              {userCredentials.some(c => Boolean(c.siteUrl || c.email)) && sharedPassphrase.trim() && (
                <button
                  type="button"
                  onClick={() => void handleUpdateAllPassphrase(sharedPassphrase)}
                  disabled={isApplying}
                  className="px-3 py-1.5 rounded-lg text-xs font-semibold accent-bg text-white shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer shrink-0"
                >
                  {isApplying ? t.trackerCredentials.applying : t.trackerCredentials.sealMyTokens}
                </button>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Liste des Trackers */}
      <div className="space-y-2">
        {personalTrackers(t).map(kind => {
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
                  {mine?.email || credentialState(mine, t)}
                </span>
              </button>

              {isOpen && (
                <div className="px-3 pb-3 pt-1 border-t border-[var(--border-color)]">
                  <TrackerCredentialForm
                    tracker={kind.id}
                    passphrase={sharedPassphrase}
                  />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
