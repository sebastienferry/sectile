import React, { useEffect, useState } from 'react'
import { Check, Key, Globe, Mail, Loader2, ShieldCheck, X, AlertCircle, FolderGit2, Lock } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { TrackerCredentials } from '../types'
import {
  TRACKERS,
  canCheck,
  credentialState,
  initialTracker,
  prefillFromCredential,
  saveBlockedReason,
  scopesFor,
  sealingConsequence,
  SEALING_INVITATION,
  storedFor,
  trackerFields,
  type CredentialScope,
  type TrackerKind,
} from '../lib/trackers'

/**
 * Premier démarrage : ce qu'il faut savoir avant que quoi que ce soit fonctionne.
 *
 * Sans instance et sans jeton, la synchronisation ne ramène rien, les équipes
 * restent vides et aucune écriture ne part. Jusqu'ici on l'apprenait en
 * synchronisant pour rien. Cet écran demande les valeurs du tracker choisi, les
 * vérifie auprès de l'instance, et dit à quel compte elles donnent accès avant
 * d'enregistrer quoi que ce soit.
 *
 * Les champs suivent le tracker : demander un site Jira et un e-mail Atlassian
 * pour configurer GitHub n'a jamais eu de sens.
 */
export const TrackerSetup: React.FC<{ onClose: () => void }> = ({ onClose }) => {
  const {
    checkTrackerCredentials,
    saveTrackerCredentials,
    settings,
    addToast,
    userCredentials,
    refreshUserCredentials,
    saveUserCredential,
    unlockUserCredential,
    clearUserCredential,
  } = useApp()

  const initial = initialTracker(settings.issueTracker)
  const [tracker, setTracker] = useState<TrackerKind>(initial)
  const kind = trackerFields(tracker)
  // Pour un tracker dont l'accès est personnel, l'état à montrer est celui de
  // la personne, pas celui du serveur : sinon l'écran annonce « aucun jeton »
  // à quelqu'un qui en a enregistré un.
  const serverStored = storedFor(settings, tracker)

  const [siteUrl, setSiteUrl] = useState(storedFor(settings, initial).siteUrl)
  const [project, setProject] = useState(storedFor(settings, initial).project)
  const [email, setEmail] = useState(settings.jiraEmail || '')
  const [token, setToken] = useState('')
  // Pour qui le jeton est enregistré. Sur Jira une écriture est attribuée au
  // compte du jeton, donc un jeton partagé fait signer toute l'équipe par un
  // même compte d'intégration.
  const [scope, setScope] = useState<CredentialScope>(scopesFor(initial)[0])
  const [passphrase, setPassphrase] = useState('')
  const [unlockPhrase, setUnlockPhrase] = useState('')

  useEffect(() => {
    void refreshUserCredentials()
  }, [refreshUserCredentials])

  const mine = userCredentials.find(c => c.tracker === tracker)
  const stored = kind.personalOnly
    ? { ...serverStored, tokenIsSet: Boolean(mine), tokenFromEnv: false }
    : serverStored

  // Le site enregistré avec l'accès personnel est celui de la personne : il
  // reprend la main sur la valeur serveur dès qu'il existe.
  useEffect(() => {
    const prefill = prefillFromCredential(mine, { siteUrl, email })
    if (prefill.siteUrl !== siteUrl) setSiteUrl(prefill.siteUrl)
    if (prefill.email !== email) setEmail(prefill.email)
  }, [mine, siteUrl, email])

  const selectTracker = (next: TrackerKind) => {
    const values = storedFor(settings, next)
    setTracker(next)
    setScope(scopesFor(next)[0])
    setSiteUrl(values.siteUrl)
    setProject(values.project)
    setToken('')
    setCheck(null)
  }
  const [isChecking, setIsChecking] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [check, setCheck] = useState<{
    ok: boolean
    error?: string
    account?: string
    identity?: { displayName: string; email?: string; siteUrl: string }
    projects?: { id: string; name: string }[]
  } | null>(null)

  const credentials = (): TrackerCredentials => ({ tracker, siteUrl, project, email, token })

  // Dire pourquoi l'enregistrement est bloqué, plutôt que de laisser un bouton
  // gris sans explication visible.
  const blockedReason = saveBlockedReason(tracker, { siteUrl, email }, Boolean(check?.ok))

  const runCheck = async () => {
    setIsChecking(true)
    setCheck(await checkTrackerCredentials(credentials()))
    setIsChecking(false)
  }

  const save = async () => {
    setIsSaving(true)
    const saved =
      scope === 'personal'
        ? await saveUserCredential({ tracker, siteUrl, email, token, passphrase })
        : await saveTrackerCredentials(credentials())
    setIsSaving(false)
    if (saved) {
      addToast({
        type: 'success',
        title: `${kind.label} configuré`,
        description:
          scope === 'personal'
            ? sealingConsequence(Boolean(passphrase.trim()))
            : 'Le jeton est enregistré dans la configuration du serveur, pour tout le monde.',
      })
      onClose()
    }
  }

  const fieldClass =
    'w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-60 flex items-center justify-center p-4 bg-black/70 backdrop-blur-xs">
      <div className="relative w-full max-w-lg rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.9)]">
        <div className="flex items-start justify-between px-5 py-4 border-b border-[var(--border-color)]">
          <div className="min-w-0">
            <h2 className="text-sm font-bold text-[var(--text-primary)]">Connecter votre tracker</h2>
            <p className="text-[11px] text-[var(--text-secondary)] mt-0.5 leading-relaxed">
              Sans ces valeurs, la synchronisation ne ramène rien et aucune écriture ne part. Elles
              sont vérifiées auprès de l'instance avant d'être enregistrées.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer shrink-0"
            title="Configurer plus tard"
          >
            <X size={16} />
          </button>
        </div>

        <div className="p-5 space-y-3 overflow-y-auto">
          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Tracker
            </label>
            <div className="flex items-center gap-1.5">
              {TRACKERS.map(t => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => selectTracker(t.id)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-semibold border cursor-pointer ${
                    t.id === tracker
                      ? 'accent-bg text-white border-transparent'
                      : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              {kind.siteLabel}
            </label>
            <div className="relative">
              <input
                type="text"
                value={siteUrl}
                onChange={e => setSiteUrl(e.target.value)}
                placeholder={kind.sitePlaceholder}
                className={fieldClass}
              />
              <Globe size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
            </div>
            {kind.siteIsPersonal && (
              <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
                Votre compte appartient à cette instance. Les projets que vous posez sur ce
                tracker la reprennent.
              </span>
            )}
          </div>

          {kind.wantsEmail && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                E-mail Atlassian
              </label>
              <div className="relative">
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="prenom.nom@exemple.com"
                  className={fieldClass}
                />
                <Mail size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
              </div>
            </div>
          )}

          {kind.projectLabel && !kind.personalOnly && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                {kind.projectLabel}
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={project}
                  onChange={e => setProject(e.target.value)}
                  placeholder={kind.projectPlaceholder}
                  className={fieldClass}
                />
                <FolderGit2 size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
              </div>
            </div>
          )}

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Personal Access Token
            </label>
            <div className="relative">
              <input
                type="password"
                value={token}
                onChange={e => setToken(e.target.value)}
                placeholder={
                  stored.tokenIsSet
                    ? 'Déjà configuré, laissez vide pour le garder'
                    : stored.tokenFromEnv
                      ? "Fourni par l'environnement du serveur, laissez vide pour le garder"
                      : 'Collez le jeton'
                }
                className={fieldClass}
              />
              <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
            </div>
            <span className="text-[9.5px] text-[var(--text-muted)] block mt-1">
              {kind.tokenHint}
              {stored.tokenFromEnv && ' Un jeton vient déjà de l’environnement du serveur ; celui saisi ici prime.'}
            </span>
          </div>

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Enregistrer ce jeton
            </label>
            {scopesFor(tracker).length > 1 ? (
              <div className="flex items-center gap-1.5">
                {scopesFor(tracker).map(choice => (
                  <button
                    key={choice}
                    type="button"
                    onClick={() => setScope(choice)}
                    className={`px-3 py-1.5 rounded-xl text-xs font-semibold border cursor-pointer ${
                      choice === scope
                        ? 'accent-bg text-white border-transparent'
                        : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                    }`}
                  >
                    {choice === 'personal' ? 'Pour moi seulement' : 'Pour le serveur'}
                  </button>
                ))}
              </div>
            ) : (
              <div className="text-[11px] font-semibold text-[var(--text-primary)]">Pour moi seulement</div>
            )}
            <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
              {scope === 'personal'
                ? "Ce que vous écrivez porte votre compte plutôt que celui du serveur. " + credentialState(mine)
                : 'Le serveur utilise ce jeton pour tout le monde, y compris pour les écritures parties en file de fond.'}
            </span>
          </div>

          {scope === 'personal' && (
            <div className="space-y-2">
              <div>
                <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                  Phrase de scellement (facultative)
                </label>
                <span className="text-[10px] text-[var(--text-secondary)] block mb-1 leading-relaxed">
                  {SEALING_INVITATION}
                </span>
                <div className="relative">
                  <input
                    type="password"
                    value={passphrase}
                    onChange={e => setPassphrase(e.target.value)}
                    placeholder="Laissez vide pour laisser le serveur l'ouvrir"
                    className={fieldClass}
                  />
                  <Lock size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
                </div>
                <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
                  {sealingConsequence(Boolean(passphrase.trim()))}
                </span>
              </div>

              {mine?.sealed && !mine.unlocked && (
                <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-1.5">
                  <span className="text-[10.5px] text-[var(--text-secondary)] block">
                    Votre jeton est scellé et verrouillé. Déverrouillez-le pour cette session.
                  </span>
                  <div className="flex items-center gap-1.5">
                    <input
                      type="password"
                      value={unlockPhrase}
                      onChange={e => setUnlockPhrase(e.target.value)}
                      placeholder="Phrase de scellement"
                      className="flex-1 px-2.5 py-1.5 text-xs rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]"
                    />
                    <button
                      type="button"
                      onClick={async () => {
                        if (await unlockUserCredential(tracker, unlockPhrase)) setUnlockPhrase('')
                      }}
                      className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
                    >
                      Déverrouiller
                    </button>
                  </div>
                </div>
              )}

              {mine && (
                <button
                  type="button"
                  onClick={() => void clearUserCredential(tracker)}
                  className="text-[10px] text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer"
                >
                  Oublier mon accès {kind.label}
                </button>
              )}
            </div>
          )}

          {check && (
            <div
              className="p-3 rounded-xl border text-[11px] leading-relaxed"
              style={{
                background: check.ok ? 'rgb(var(--status-ok-rgb) / 0.1)' : 'rgb(var(--status-danger-rgb) / 0.1)',
                borderColor: check.ok ? 'rgb(var(--status-ok-rgb) / 0.35)' : 'rgb(var(--status-danger-rgb) / 0.35)',
                color: check.ok ? 'var(--status-ok)' : 'var(--status-danger)',
              }}
            >
              {check.ok ? (
                <>
                  <div className="flex items-center gap-1.5 font-bold">
                    <ShieldCheck size={13} />
                    Connecté comme {check.identity?.displayName ?? check.account}
                  </div>
                  {check.projects && check.projects.length > 0 && (
                    <div className="mt-1 text-[var(--text-secondary)]">
                      {check.projects.length} projet(s) visibles, dont{' '}
                      <span className="font-mono">
                        {check.projects.slice(0, 5).map(p => p.id).join(', ')}
                      </span>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex items-start gap-1.5">
                  <AlertCircle size={13} className="shrink-0 mt-0.5" />
                  <span>{check.error}</span>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 px-5 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            Plus tard
          </button>
          {blockedReason && (
            <span className="text-[10px] text-[var(--text-muted)] leading-tight flex-1 text-right pr-1">{blockedReason}</span>
          )}
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={runCheck}
              disabled={isChecking || !canCheck(tracker, { siteUrl, email })}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
            >
              {isChecking ? <Loader2 size={13} className="animate-spin" /> : <ShieldCheck size={13} />}
              Vérifier
            </button>
            <button
              type="button"
              onClick={save}
              disabled={isSaving || !check?.ok}
              title={check?.ok ? 'Enregistrer ces accès' : "Vérifiez d'abord les accès"}
              className="flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
            >
              {isSaving ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
              Enregistrer
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
