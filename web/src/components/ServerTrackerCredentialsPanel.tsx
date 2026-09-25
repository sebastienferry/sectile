import React, { useCallback, useEffect, useState } from 'react'
import { CheckCircle2, Key, Mail, RefreshCw, Trash2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { fill, relativeTime } from '../lib/adminStats'
import {
  checkServerCredential,
  clearServerCredential,
  fetchServerCredentials,
  saveServerCredential,
  serverCredentialLabel,
  serverCredentialVariables,
  type ServerCredentialState,
  type ServerCredentialTracker,
} from '../lib/serverCredentials'

const TRACKER_NAMES: Record<ServerCredentialTracker, string> = {
  github: 'GitHub',
  jira: 'Jira',
  gitlab: 'GitLab',
}

const fieldClass =
  'w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'
const buttonClass =
  'flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed'

interface CardProps {
  state: ServerCredentialState
  onChange: (state: ServerCredentialState) => void
  onRefresh: () => void
  /** When the states were read, which the relative dates count from. */
  loadedAt: number
}

/**
 * One provider's server credential. The token is write-only: the card shows
 * where the credential comes from and whom it authenticates as, and a new
 * token is saved only after a check the instance accepted, the same rule as
 * the personal credential form.
 */
function ServerCredentialCard({ state, onChange, onRefresh, loadedAt }: CardProps) {
  const { t, addToast, settings } = useApp()
  const labels = t.admin.serverCredentials
  const name = TRACKER_NAMES[state.tracker]
  const [email, setEmail] = useState(state.email || '')
  const [token, setToken] = useState('')
  const [checkedToken, setCheckedToken] = useState<string | null>(null)
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)

  const wantsEmail = state.tracker === 'jira'
  const typed = token.trim()
  // A save needs the very token and e-mail the instance just accepted.
  const checkedKey = `${email.trim()}|${typed}`
  const canSave = typed !== '' && (!wantsEmail || email.trim() !== '') && checkedToken === checkedKey

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    try {
      await action()
    } catch (err) {
      setMessage({ ok: false, text: err instanceof Error ? err.message : String(err) })
    } finally {
      setBusy(false)
    }
  }

  const check = () => run(async () => {
    const account = await checkServerCredential(state.tracker, typed ? email.trim() : '', typed)
    setMessage({ ok: true, text: fill(labels.checked, { account }) })
    if (typed) {
      setCheckedToken(checkedKey)
    } else {
      // A check of the credential in use dates it on the server.
      onRefresh()
    }
  })

  const save = () => run(async () => {
    const saved = await saveServerCredential(state.tracker, email.trim(), typed)
    onChange(saved)
    setToken('')
    setCheckedToken(null)
    setMessage(null)
    addToast({ type: 'success', title: fill(labels.saved, { tracker: name }) })
  })

  const clear = () => {
    if (!window.confirm(fill(labels.confirmClear, { tracker: name }))) return
    void run(async () => {
      onChange(await clearServerCredential(state.tracker))
      setMessage(null)
      addToast({ type: 'success', title: fill(labels.cleared, { tracker: name }) })
    })
  }

  const label = serverCredentialLabel(state)
  const variables = serverCredentialVariables(state.tracker).join(', ')
  const stateText = fill(labels[label], { variables })
  const stateTone = label === 'stored'
    ? 'text-emerald-400'
    : label === 'environment' ? 'text-cyan-400' : 'text-amber-400'

  return (
    <div className="space-y-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-primary)] p-3" data-server-credential={state.tracker}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-bold text-[var(--text-primary)]">{name}</span>
        <span className={`text-[11px] ${stateTone}`}>{stateText}</span>
      </div>
      {state.source === 'database' && (
        <div className="text-[11px] text-[var(--text-muted)] space-x-2">
          <span>
            {labels.account} <span className="text-[var(--text-primary)]">{state.account || '-'}</span>
            {' · '}
            {state.checkedAt
              ? `${labels.checkedAt} ${relativeTime(state.checkedAt, loadedAt, settings.language) ?? ''}`
              : labels.notChecked}
          </span>
          {state.updatedAt && (
            <span>· {labels.updatedAt} {relativeTime(state.updatedAt, loadedAt, settings.language) ?? ''}</span>
          )}
        </div>
      )}

      {wantsEmail && (
        <div className="relative">
          <input
            type="email"
            value={email}
            onChange={e => { setEmail(e.target.value); setCheckedToken(null) }}
            placeholder={labels.email}
            aria-label={labels.email}
            className={fieldClass}
          />
          <Mail size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
        </div>
      )}
      <div className="relative">
        <input
          type="password"
          autoComplete="new-password"
          name={`server-token-${state.tracker}`}
          value={token}
          onChange={e => { setToken(e.target.value); setCheckedToken(null) }}
          placeholder={labels.tokenPlaceholder}
          aria-label={`${labels.token} ${name}`}
          className={fieldClass}
        />
        <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
      </div>

      {message && (
        <div className={`text-[11px] ${message.ok ? 'text-emerald-400' : 'text-red-400'}`} role="status">
          {message.text}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => void check()}
          disabled={busy || (!typed && state.source === 'none')}
          className={buttonClass}
        >
          <CheckCircle2 size={12} />
          <span>{typed ? labels.check : labels.checkCurrent}</span>
        </button>
        <button
          type="button"
          onClick={() => void save()}
          disabled={busy || !canSave}
          title={canSave ? undefined : labels.saveNeedsCheck}
          className={buttonClass}
        >
          <Key size={12} />
          <span>{labels.save}</span>
        </button>
        {state.source === 'database' && (
          <button type="button" onClick={clear} disabled={busy} className={`${buttonClass} ml-auto text-red-400`}>
            <Trash2 size={12} />
            <span>{labels.clear}</span>
          </button>
        )}
      </div>
    </div>
  )
}

/**
 * The Administration page's section for the server tracker credentials (#464):
 * one card per provider, admin-only like the rest of the page.
 */
export const ServerTrackerCredentialsPanel: React.FC = () => {
  const { t } = useApp()
  const labels = t.admin.serverCredentials
  const [states, setStates] = useState<ServerCredentialState[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loadedAt, setLoadedAt] = useState(0)

  const load = useCallback(async () => {
    try {
      setStates(await fetchServerCredentials())
      setLoadedAt(Date.now())
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const replace = (next: ServerCredentialState) => {
    setStates(current => current.map(state => (state.tracker === next.tracker ? next : state)))
    setLoadedAt(Date.now())
  }

  return (
    <section
      className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4"
      aria-labelledby="admin-server-credentials-title"
      data-server-credentials
    >
      <div className="flex items-center gap-2">
        <h3 id="admin-server-credentials-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
          <Key size={14} /> {labels.title}
        </h3>
        <button type="button" onClick={() => void load()} className={`${buttonClass} ml-auto text-[11px]`}>
          <RefreshCw size={12} />
          <span>{t.admin.refresh}</span>
        </button>
      </div>
      <p className="text-[11px] text-[var(--text-muted)] leading-relaxed">{labels.intro}</p>
      {error && <p className="text-[11px] text-amber-400">{labels.loadFailed} ({error})</p>}
      <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
        {states.map(state => (
          <ServerCredentialCard
            key={state.tracker}
            state={state}
            onChange={replace}
            onRefresh={() => void load()}
            loadedAt={loadedAt}
          />
        ))}
      </div>
    </section>
  )
}
