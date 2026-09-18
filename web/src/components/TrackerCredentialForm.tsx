import React, { useEffect, useRef, useState } from 'react'
import { AlertCircle, Check, Globe, Key, Loader2, Lock, Mail, ShieldCheck } from 'lucide-react'
import { useApp } from '../context/AppContext'
import {
  SEALING_INVITATION,
  canCheck,
  credentialState,
  prefillFromCredential,
  saveBlockedReason,
  scopesFor,
  sealingConsequence,
  storedFor,
  trackerFields,
  type CredentialScope,
  type TrackerKind,
} from '../lib/trackers'

/**
 * Tout ce qu'on fait avec l'accès d'un tracker : le saisir, le vérifier auprès
 * de l'instance, l'enregistrer, le desceller, l'oublier.
 *
 * Un seul endroit pour ces règles, parce qu'elles sont subtiles et qu'elles ont
 * déjà divergé une fois : l'enregistrement ne se débloque qu'après une
 * vérification réussie, un jeton déjà enregistré ne revient jamais du serveur,
 * et un tracker qui attribue ses écritures à un compte n'accepte que le
 * personnel.
 */
export const TrackerCredentialForm: React.FC<{ tracker: TrackerKind; onSaved?: () => void }> = ({
  tracker,
  onSaved,
}) => {
  const {
    checkTrackerCredentials,
    saveTrackerCredentials,
    settings,
    addToast,
    userCredentials,
    saveUserCredential,
    unlockUserCredential,
    clearUserCredential,
  } = useApp()

  const kind = trackerFields(tracker)
  const serverStored = storedFor(settings, tracker)
  const mine = userCredentials.find(c => c.tracker === tracker)
  const stored = kind.personalOnly ? { ...serverStored, tokenIsSet: Boolean(mine), tokenFromEnv: false } : serverStored

  const [siteUrl, setSiteUrl] = useState(serverStored.siteUrl)
  const [project, setProject] = useState(serverStored.project)
  const [email, setEmail] = useState(settings.jiraEmail || '')
  const [token, setToken] = useState('')
  const [scope, setScope] = useState<CredentialScope>(scopesFor(tracker)[0])
  const [passphrase, setPassphrase] = useState('')
  const [unlockPhrase, setUnlockPhrase] = useState('')
  const [isChecking, setIsChecking] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [check, setCheck] = useState<{ ok: boolean; error?: string; account?: string } | null>(null)

  // Ce qui est déjà enregistré revient dans le formulaire : sans cela l'écran
  // redemande ce que la personne a déjà donné, et la vérification reste grise
  // faute d'un champ obligatoire.
  //
  // Une seule fois par valeur enregistrée, et non à chaque frappe : `mine` est
  // un objet neuf à chaque rendu, donc l'effet se rejouait sans cesse et
  // remplissait à nouveau le champ qu'on venait de vider. On ne pouvait plus
  // effacer un site pour en saisir un autre sans que l'ancien revienne devant
  // ce qu'on tapait.
  const applied = useRef<string | null>(null)
  useEffect(() => {
    const identity = `${mine?.siteUrl || ''}|${mine?.email || ''}`
    if (applied.current === identity) return
    applied.current = identity
    const prefill = prefillFromCredential(mine, { siteUrl, email })
    if (prefill.siteUrl !== siteUrl) setSiteUrl(prefill.siteUrl)
    if (prefill.email !== email) setEmail(prefill.email)
  }, [mine, siteUrl, email])

  const blockedReason = saveBlockedReason(tracker, { siteUrl, email }, Boolean(check?.ok))

  // Le bouton doit revenir à son état normal quoi qu'il arrive. Sans cela une
  // exception levée avant l'envoi laisse une attente sans fin, sans message et
  // sans requête : rien à l'écran ne dit qu'il s'est passé quelque chose, et on
  // ne peut même pas réessayer.
  const runCheck = async () => {
    setIsChecking(true)
    try {
      setCheck(await checkTrackerCredentials({ tracker, siteUrl, project, email, token }))
    } catch (err) {
      setCheck({ ok: false, error: err instanceof Error ? err.message : 'Vérification impossible' })
    } finally {
      setIsChecking(false)
    }
  }

  const save = async () => {
    setIsSaving(true)
    let saved = false
    try {
      saved =
        scope === 'personal'
          ? await saveUserCredential({ tracker, siteUrl, email, token, passphrase })
          : await saveTrackerCredentials({ tracker, siteUrl, project, email, token })
    } catch (err) {
      addToast({
        type: 'error',
        title: 'Enregistrement impossible',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setIsSaving(false)
    }
    if (!saved) return
    addToast({
      type: 'success',
      title: `${kind.label} configuré`,
      description:
        scope === 'personal'
          ? sealingConsequence(Boolean(passphrase.trim()))
          : 'Le jeton est enregistré dans la configuration du serveur, pour tout le monde.',
    })
    setToken('')
    setCheck(null)
    onSaved?.()
  }

  const forget = async () => {
    if (!confirm(`Oublier votre accès ${kind.label} ? Vous devrez saisir votre jeton à nouveau.`)) return
    if (await clearUserCredential(tracker)) {
      setToken('')
      setPassphrase('')
      setUnlockPhrase('')
      setSiteUrl('')
      setEmail('')
      setCheck(null)
    }
  }

  const fieldClass =
    'w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'
  const locked = Boolean(mine?.sealed && !mine.unlocked)

  return (
    <div className="space-y-3">
      <div>
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
          {kind.siteLabel}
        </label>
        <div className="relative">
          <input type="text" value={siteUrl} onChange={e => setSiteUrl(e.target.value)} placeholder={kind.sitePlaceholder} className={fieldClass} />
          <Globe size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
        </div>
        {kind.siteIsPersonal && (
          <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
            Votre compte appartient à cette instance. Les projets que vous posez sur ce tracker la reprennent.
          </span>
        )}
      </div>

      {kind.wantsEmail && (
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            E-mail du compte
          </label>
          <div className="relative">
            <input type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="prenom.nom@exemple.com" className={fieldClass} />
            <Mail size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </div>
        </div>
      )}

      {kind.projectLabel && !kind.personalOnly && (
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {kind.projectLabel}
          </label>
          <input type="text" value={project} onChange={e => setProject(e.target.value)} placeholder={kind.projectPlaceholder} className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]" />
        </div>
      )}

      <div>
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
          Personal Access Token
        </label>
        <div className="relative">
          <input
            type="password"
            autoComplete="new-password"
            name={`tracker-token-${tracker}`}
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder={stored.tokenIsSet ? 'Déjà configuré, laissez vide pour le garder' : stored.tokenFromEnv ? "Fourni par l'environnement du serveur" : 'Collez le jeton'}
            className={fieldClass}
          />
          <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
        </div>
        <span className="text-[9.5px] text-[var(--text-muted)] block mt-1">{kind.tokenHint}</span>
      </div>

      {scopesFor(tracker).length > 1 && (
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Enregistrer ce jeton
          </label>
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
          <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
            {scope === 'personal'
              ? 'Ce que vous écrivez porte votre compte plutôt que celui du serveur.'
              : 'Le serveur utilise ce jeton pour tout le monde, y compris pour les écritures parties en file de fond.'}
          </span>
        </div>
      )}

      {scope === 'personal' && (
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            Phrase de scellement (facultative)
          </label>
          <span className="text-[10px] text-[var(--text-secondary)] block mb-1 leading-relaxed">{SEALING_INVITATION}</span>
          <div className="relative">
            <input
              type="password"
              autoComplete="new-password"
              name={`tracker-sealing-${tracker}`}
              value={passphrase}
              onChange={e => setPassphrase(e.target.value)}
              placeholder="Laissez vide pour ne pas sceller"
              className={fieldClass}
            />
            <Lock size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </div>
          <span className="text-[9.5px] text-[var(--text-muted)] block mt-1 leading-relaxed">
            {sealingConsequence(Boolean(passphrase.trim()))}
          </span>
        </div>
      )}

      {locked && (
        <div className="p-2.5 rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] space-y-1.5">
          <span className="text-[10.5px] text-[var(--text-secondary)] block">{credentialState(mine)}</span>
          <div className="flex items-center gap-1.5">
            <input
              type="password"
              autoComplete="new-password"
              name={`tracker-unlock-${tracker}`}
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

      {check && (
        <div
          className="p-2.5 rounded-xl border text-[11px] leading-relaxed"
          style={{
            background: check.ok ? 'rgb(var(--status-ok-rgb) / 0.1)' : 'rgb(var(--status-danger-rgb) / 0.1)',
            borderColor: check.ok ? 'rgb(var(--status-ok-rgb) / 0.35)' : 'rgb(var(--status-danger-rgb) / 0.35)',
            color: check.ok ? 'var(--status-ok)' : 'var(--status-danger)',
          }}
        >
          {check.ok ? (
            <span className="flex items-center gap-1.5 font-bold">
              <ShieldCheck size={13} />
              Connecté comme {check.account}
            </span>
          ) : (
            <span className="flex items-start gap-1.5">
              <AlertCircle size={13} className="shrink-0 mt-0.5" />
              {check.error}
            </span>
          )}
        </div>
      )}

      <div className="flex items-center justify-between gap-2">
        {mine ? (
          <button type="button" onClick={forget} className="text-[10px] text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer">
            Oublier mon accès
          </button>
        ) : (
          <span className="text-[9.5px] text-[var(--text-muted)] leading-tight pr-1">{blockedReason}</span>
        )}
        <div className="flex items-center gap-2 shrink-0">
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
  )
}
