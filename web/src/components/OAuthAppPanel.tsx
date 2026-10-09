import React, { useCallback, useEffect, useState } from 'react'
import { Globe, Key, Link2, Trash2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import {
  OAUTH_APP_SCOPES,
  clearOAuthApp,
  fetchOAuthApp,
  oauthAppRedirectPlaceholder,
  oauthAppSourceLabel,
  saveOAuthApp,
  type OAuthAppState,
} from '../lib/oauthApp'
import type { OAuthTracker } from '../lib/trackers'
import type { OAuthAppStrings } from '../locales/translations'

const fieldClass =
  'w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'
const buttonClass =
  'flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed'

/**
 * The Administration page's section for a tracker's OAuth app people connect
 * through: Atlassian's for Jira (#654), GitHub's and GitLab's (#804). The
 * secret field starts empty and is emptied again after a save: the page never
 * holds the secret beyond what the admin types.
 */
export const OAuthAppPanel: React.FC<{ tracker: OAuthTracker }> = ({ tracker }) => {
  const { t, addToast } = useApp()
  const labels: OAuthAppStrings = t.admin[`${tracker}OAuth`]
  const [state, setState] = useState<OAuthAppState | null>(null)
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [redirectUrl, setRedirectUrl] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const apply = useCallback((next: OAuthAppState) => {
    setState(next)
    setClientId(next.clientId || '')
    setRedirectUrl(next.redirectUrl || '')
    setClientSecret('')
  }, [])

  useEffect(() => {
    fetchOAuthApp(tracker)
      .then(apply)
      .catch(err => setError(err instanceof Error ? err.message : String(err)))
  }, [tracker, apply])

  const run = async (action: () => Promise<void>, failure: string) => {
    setBusy(true)
    try {
      await action()
      setError(null)
    } catch (err) {
      addToast({ type: 'error', title: failure, description: err instanceof Error ? err.message : String(err) })
    } finally {
      setBusy(false)
    }
  }

  const save = () => run(async () => {
    apply(await saveOAuthApp(tracker, { clientId, clientSecret, redirectUrl }))
    addToast({ type: 'success', title: labels.saved })
  }, labels.saveFailed)

  const clear = () => {
    if (!window.confirm(labels.confirmClear)) return
    void run(async () => {
      apply(await clearOAuthApp(tracker))
      addToast({ type: 'success', title: labels.cleared })
    }, labels.saveFailed)
  }

  // Only a readable secret saved on this page can be kept by leaving the field
  // empty: one from the environment is not copied into the database.
  const keepsSecret = Boolean(state?.secretSet && state.source === 'database' && !state.unreadable)
  const canSave = clientId.trim() !== '' && redirectUrl.trim() !== '' && (clientSecret.trim() !== '' || keepsSecret)
  const incomplete = state?.source === 'environment' && !state.configured
  const tone = incomplete ? 'text-amber-400' : state?.source === 'database' ? 'text-emerald-400' : state?.source === 'environment' ? 'text-cyan-400' : 'text-amber-400'
  const titleId = `admin-${tracker}-oauth-title`

  return (
    <section
      className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4"
      aria-labelledby={titleId}
      data-oauth-app={tracker}
    >
      <div className="flex flex-wrap items-center gap-2">
        <h3 id={titleId} className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
          <Link2 size={14} /> {labels.title}
        </h3>
        {state && <span className={`text-[11px] ${tone}`}>{incomplete ? labels.sourceEnvironmentIncomplete : oauthAppSourceLabel(state.source, labels)}</span>}
      </div>
      <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">{labels.intro}</p>
      {labels.note && <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">{labels.note}</p>}
      {error && <p className="text-[11px] text-amber-400">{labels.loadFailed} ({error})</p>}
      {state?.unreadable && <p className="text-[11px] text-amber-400">{labels.unreadable}</p>}

      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <label className="space-y-1">
          <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{labels.clientId}</span>
          <span className="relative block">
            <input type="text" value={clientId} onChange={e => setClientId(e.target.value)} className={fieldClass} />
            <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </span>
        </label>
        <label className="space-y-1">
          <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{labels.clientSecret}</span>
          <span className="relative block">
            <input
              type="password"
              autoComplete="new-password"
              value={clientSecret}
              onChange={e => setClientSecret(e.target.value)}
              placeholder={keepsSecret ? labels.secretSet : labels.secretPlaceholder}
              className={fieldClass}
            />
            <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </span>
        </label>
        <label className="space-y-1">
          <span className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)]">{labels.redirectUrl}</span>
          <span className="relative block">
            <input type="url" value={redirectUrl} onChange={e => setRedirectUrl(e.target.value)} placeholder={oauthAppRedirectPlaceholder(tracker)} className={fieldClass} />
            <Globe size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
          </span>
          <span className="block text-[9.5px] text-[var(--text-muted)]">{labels.redirectHint}</span>
        </label>
      </div>

      <details className="text-[11px] text-[var(--text-muted)]">
        <summary className="cursor-pointer">{labels.scopes}</summary>
        <code className="block mt-1 break-words text-[10.5px] text-[var(--text-secondary)]">{OAUTH_APP_SCOPES[tracker].join(' ')}</code>
      </details>

      <div className="flex items-center gap-2 text-[11px]">
        <button type="button" onClick={() => void save()} disabled={busy || !canSave} className={buttonClass}>
          <Key size={12} />
          <span>{labels.save}</span>
        </button>
        {state?.source === 'database' && (
          <button type="button" onClick={clear} disabled={busy} className={`${buttonClass} ml-auto text-red-400`}>
            <Trash2 size={12} />
            <span>{labels.clear}</span>
          </button>
        )}
      </div>
    </section>
  )
}
