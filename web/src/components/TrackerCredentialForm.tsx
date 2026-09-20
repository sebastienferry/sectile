import React, { useEffect, useRef, useState } from 'react'
import { AlertCircle, Check, Globe, Key, Loader2, Lock, LockOpen, Mail, ShieldCheck } from 'lucide-react'
import { useApp } from '../context/AppContext'
import {
  canCheck,
  credentialState,
  prefillFromCredential,
  saveBlockedReason,
  sealingConsequence,
  storedFor,
  trackerFields,
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
export interface TrackerCredentialFormProps {
  tracker: TrackerKind
  onSaved?: () => void
  passphrase?: string
}

export const TrackerCredentialForm: React.FC<TrackerCredentialFormProps> = ({
  tracker,
  onSaved,
  passphrase = '',
}) => {
  const {
    checkTrackerCredentials,
    settings,
    addToast,
    userCredentials,
    saveUserCredential,
    clearUserCredential,
    t,
  } = useApp()

  const kind = trackerFields(tracker, t)
  const serverStored = storedFor(settings, tracker)
  const mine = userCredentials.find(c => c.tracker === tracker)
  const stored = { ...serverStored, tokenIsSet: Boolean(mine), tokenFromEnv: false }

  const [siteUrl, setSiteUrl] = useState(mine?.siteUrl || serverStored.siteUrl)
  const [email, setEmail] = useState(mine?.email || settings.jiraEmail || '')
  const [token, setToken] = useState('')

  const [isChecking, setIsChecking] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [check, setCheck] = useState<{ ok: boolean; error?: string; account?: string } | null>(null)

  // Ce qui est déjà enregistré revient dans le formulaire : sans cela l'écran
  // redemande ce que la personne a déjà donné, et la vérification reste grise
  // faute d'un champ obligatoire.
  const applied = useRef<string | null>(null)
  useEffect(() => {
    const identity = `${mine?.siteUrl || ''}|${mine?.email || ''}`
    if (applied.current === identity) return
    applied.current = identity
    const prefill = prefillFromCredential(mine, { siteUrl, email })
    if (prefill.siteUrl !== siteUrl) setSiteUrl(prefill.siteUrl)
    if (prefill.email !== email) setEmail(prefill.email)
  }, [mine, siteUrl, email])

  const blockedReason = saveBlockedReason(tracker, { siteUrl, email }, Boolean(check?.ok), t)

  const runCheck = async () => {
    setIsChecking(true)
    try {
      setCheck(await checkTrackerCredentials({ tracker, siteUrl, email, token }))
    } catch (err) {
      setCheck({ ok: false, error: err instanceof Error ? err.message : t.trackerCredentials.form.checkErrorDefault })
    } finally {
      setIsChecking(false)
    }
  }

  const save = async () => {
    setIsSaving(true)
    let saved = false
    try {
      saved = await saveUserCredential({ tracker, siteUrl, email, token, passphrase })
    } catch (err) {
      addToast({
        type: 'error',
        title: t.trackerCredentials.form.saveErrorTitle,
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setIsSaving(false)
    }
    if (!saved) return
    addToast({
      type: 'success',
      title: t.trackerCredentials.form.configuredTitle.replace('{tracker}', kind.label),
      description: sealingConsequence(Boolean(passphrase.trim()), t),
    })
    setToken('')
    setCheck(null)
    onSaved?.()
  }

  const forget = async () => {
    if (!confirm(t.trackerCredentials.form.forgetConfirm.replace('{tracker}', kind.label))) return
    if (await clearUserCredential(tracker)) {
      setToken('')
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
            {t.trackerCredentials.form.siteIsPersonalNotice}
          </span>
        )}
      </div>

      {kind.wantsEmail && (
        <div>
          <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
            {t.trackerCredentials.form.accountEmail}
          </label>
          <div className="relative">
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder={t.trackerCredentials.form.accountEmailPlaceholder}
              className={fieldClass}
            />
            <Mail size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </div>
        </div>
      )}

      <div>
        <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
          {t.trackerCredentials.form.personalAccessToken}
        </label>
        <div className="relative">
          <input
            type="password"
            autoComplete="new-password"
            name={`tracker-token-${tracker}`}
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder={
              stored.tokenIsSet
                ? t.trackerCredentials.form.tokenPlaceholderSet
                : stored.tokenFromEnv
                  ? t.trackerCredentials.form.tokenPlaceholderEnv
                  : t.trackerCredentials.form.tokenPlaceholderEmpty
            }
            className={fieldClass}
          />
          <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
        </div>
        <span className="text-[9.5px] text-[var(--text-muted)] block mt-1">{kind.tokenHint}</span>
      </div>

      {mine?.sealed ? (
        locked ? (
          <div className="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-[11px] text-amber-500 flex items-center gap-2">
            <Lock size={13} className="shrink-0" />
            <span>
              {credentialState(mine, t)}
              {t.trackerCredentials.form.lockedNoticeSuffix}
            </span>
          </div>
        ) : (
          <div className="p-2 rounded-xl bg-emerald-500/10 border border-emerald-500/25 text-[11px] text-emerald-400 flex items-center gap-2">
            <LockOpen size={13} className="shrink-0" />
            <span>{t.trackerCredentials.form.sealedUnlockedNotice}</span>
          </div>
        )
      ) : passphrase.trim() ? (
        <span className="text-[9.5px] text-[var(--text-muted)] block leading-relaxed">
          {t.trackerCredentials.form.willBeSealedNotice}
        </span>
      ) : null}

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
              {t.trackerCredentials.form.connectedAs} {check.account}
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
            {t.trackerCredentials.form.forgetAccess}
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
            {t.trackerCredentials.form.verify}
          </button>
          <button
            type="button"
            onClick={save}
            disabled={isSaving || !check?.ok}
            title={check?.ok ? t.trackerCredentials.form.saveTitleReady : t.trackerCredentials.form.saveTitleNotChecked}
            className="flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
          >
            {isSaving ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
            {t.trackerCredentials.form.save}
          </button>
        </div>
      </div>
    </div>
  )
}
