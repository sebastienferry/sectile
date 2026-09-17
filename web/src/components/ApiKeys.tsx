import { useCallback, useEffect, useState } from 'react'
import { Copy, Laptop, RefreshCw, Trash2 } from 'lucide-react'
import { DEFAULT_KEY_TTL_DAYS, describeExpiry, expiryState, type ApiKey } from '../lib/apiKeys'

type PairingCode = {
  code: string
  expiresAt: string
}

type IssuedKey = {
  token: string
  device: ApiKey
}

function codeExpiry(expiresAt: string): string {
  const date = new Date(expiresAt)
  return Number.isNaN(date.getTime()) ? 'expiry unknown' : `Valid until ${date.toLocaleTimeString()}`
}

// Workstations are paired, not configured: the user carries a short-lived code,
// the machine receives and keeps its API key, and the key itself stays out of
// sight. Showing a key in clear is the advanced case of an MCP client that has
// no agent to pair for it, and it is folded away as such.
export function ApiKeysPanel() {
  // The address every client must be pointed at is the one serving this page.
  const serverOrigin = window.location.origin
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [code, setCode] = useState<PairingCode | null>(null)
  const [label, setLabel] = useState('')
  const [ttlDays, setTtlDays] = useState<number>(DEFAULT_KEY_TTL_DAYS)
  const [issued, setIssued] = useState<IssuedKey | null>(null)
  const [sharedServerToken, setSharedServerToken] = useState(false)
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)

  const loadKeys = useCallback(async () => {
    try {
      const res = await fetch('/api/devices')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setKeys(body.devices ?? [])
    } catch {
      setStatus('Could not load workstations.')
    }
  }, [])

  useEffect(() => { void loadKeys() }, [loadKeys])
  useEffect(() => {
    fetch('/api/me')
      .then(res => (res.ok ? res.json() : null))
      .then(body => setSharedServerToken(Boolean(body?.sharedServerToken)))
      .catch(() => {})
  }, [])

  async function copy(value: string, done: string) {
    try {
      await navigator.clipboard.writeText(value)
      setStatus(done)
    } catch {
      setStatus('Copy unavailable. Select the value and copy it manually.')
    }
  }

  async function requestCode() {
    setBusy(true)
    setStatus('')
    setIssued(null)
    try {
      const res = await fetch('/api/pairing-codes', { method: 'POST' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setCode(await res.json())
    } catch {
      setStatus('Could not issue a pairing code.')
    } finally {
      setBusy(false)
    }
  }

  async function createKey() {
    setBusy(true)
    setStatus('')
    setCode(null)
    try {
      const res = await fetch('/api/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label: label.trim(), ttlDays }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setIssued(await res.json())
      setLabel('')
      await loadKeys()
    } catch {
      setStatus('Could not create an API key.')
    } finally {
      setBusy(false)
    }
  }

  async function renew(key: ApiKey, days: number) {
    setStatus('')
    try {
      const res = await fetch(`/api/devices?id=${encodeURIComponent(key.ID)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ttlDays: days }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setStatus(days > 0 ? `${key.Label || key.ID} renewed for ${days} days.` : `${key.Label || key.ID} no longer expires.`)
      await loadKeys()
    } catch {
      setStatus('Could not renew that workstation.')
    }
  }

  async function revoke(key: ApiKey) {
    setStatus('')
    try {
      const res = await fetch(`/api/devices?id=${encodeURIComponent(key.ID)}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setStatus(`${key.Label || key.ID} revoked.`)
      await loadKeys()
    } catch {
      setStatus('Could not revoke that workstation.')
    }
  }

  const inputClass = 'rounded-lg border border-[var(--border-color)] bg-[var(--bg-primary)] px-3 py-2 text-[var(--text-primary)]'
  const buttonClass = 'rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)] disabled:opacity-50'
  const pairCommand = `sectile-agent pair --url ${serverOrigin} --code <code>`

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="workstations-title">
      <h3 id="workstations-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <Laptop size={16} /> Workstations
      </h3>
      <p className="text-[var(--text-muted)]">
        Pair each machine once with a code. The workstation receives its own credential and keeps it;
        you never handle it. It expires after {DEFAULT_KEY_TTL_DAYS} days unless renewed here, and
        revoking a workstation leaves your other machines connected.
      </p>

      {sharedServerToken ? (
        <p role="alert" className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-[var(--text-primary)]">
          This server still runs with <code>SECTILE_SERVER_TOKEN</code>, which is deprecated and will be
          removed in the next release. Pair your workstations and drop the variable.
        </p>
      ) : null}

      <button type="button" onClick={requestCode} disabled={busy} className={buttonClass}>
        {busy ? 'Working…' : 'Pair a workstation'}
      </button>

      {code ? (
        <div className="space-y-2 rounded-lg border border-[var(--border-color)] bg-[var(--bg-primary)] p-3">
          <div className="flex items-start gap-2">
            <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-lg bg-[var(--bg-secondary)] p-3 select-text"><code>{code.code}</code></pre>
            <button type="button" onClick={() => copy(code.code, 'Pairing code copied.')} className={`flex shrink-0 items-center gap-1 ${buttonClass}`}>
              <Copy size={14} /> Copy
            </button>
          </div>
          <p className="text-[var(--text-muted)]">{codeExpiry(code.expiresAt)}. Single use.</p>
          <h4 className="font-semibold text-[var(--text-primary)]">On the workstation</h4>
          <ol className="list-decimal space-y-1 pl-5 text-[var(--text-muted)]">
            <li>
              In a terminal: <code className="rounded bg-[var(--bg-secondary)] px-1 py-0.5 select-text">{pairCommand}</code>,
              then start the agent with <code className="rounded bg-[var(--bg-secondary)] px-1 py-0.5 select-text">sectile-agent --url {serverOrigin}</code>.
            </li>
            <li>Or, in Sectile Desktop, enter this server address and paste the code in the connect screen.</li>
            <li>The workstation appears in the list below once paired. The code is never needed again.</li>
          </ol>
          <p className="text-[var(--text-muted)]">A code that expired before it was used is not reusable: generate a new one.</p>
        </div>
      ) : null}

      {keys.length > 0 ? (
        <ul className="space-y-1">
          {keys.map(key => {
            const state = expiryState(key.ExpiresAt)
            const tone = state === 'expired' ? 'text-red-500' : state === 'soon' ? 'text-amber-500' : 'text-[var(--text-muted)]'
            return (
              <li key={key.ID} className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-[var(--bg-primary)] px-3 py-2">
                <span className="min-w-0 flex-1">
                  <span className="font-semibold">{key.Label || 'Unnamed workstation'}</span>
                  <span className="text-[var(--text-muted)]"> · last seen {new Date(key.LastSeen).toLocaleString()}</span>
                  <span className={`block ${tone}`}>{describeExpiry(key.ExpiresAt)}</span>
                </span>
                <span className="flex shrink-0 items-center gap-1">
                  <button
                    type="button"
                    onClick={() => renew(key, DEFAULT_KEY_TTL_DAYS)}
                    aria-label={`Renew ${key.Label || key.ID} for ${DEFAULT_KEY_TTL_DAYS} days`}
                    className={`flex items-center gap-1 ${buttonClass} px-2 py-1`}
                  >
                    <RefreshCw size={14} /> {key.ExpiresAt ? 'Renew' : `Expire in ${DEFAULT_KEY_TTL_DAYS} days`}
                  </button>
                  <button
                    type="button"
                    onClick={() => revoke(key)}
                    aria-label={`Revoke ${key.Label || key.ID}`}
                    className={`flex items-center gap-1 ${buttonClass} px-2 py-1`}
                  >
                    <Trash2 size={14} /> Revoke
                  </button>
                </span>
              </li>
            )
          })}
        </ul>
      ) : <p className="text-[var(--text-muted)]">No workstation is paired yet.</p>}

      {/* The advanced case: a client that cannot pair because no agent runs
          beside it. Only here is a credential ever shown in clear. */}
      <details className="rounded-lg border border-[var(--border-color)] bg-[var(--bg-primary)] p-3">
        <summary className="cursor-pointer font-semibold text-[var(--text-primary)]">
          Advanced: an MCP client without a local agent
        </summary>
        <div className="mt-2 space-y-2">
          <p className="text-[var(--text-muted)]">
            A paired agent registers Sectile in your coding CLI by itself. Use this only for a client you
            configure by hand on a machine with no agent: create a key, copy it once, and set it as the
            bearer header on <code>{serverOrigin}/mcp</code>. It is listed above like any workstation.
          </p>
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={event => { event.preventDefault(); void createKey() }}
          >
            <label className="flex min-w-0 flex-1 flex-col gap-1 text-[var(--text-muted)]">
              Label
              <input
                value={label}
                onChange={event => setLabel(event.target.value)}
                placeholder="claude on the build server…"
                className={inputClass}
                autoComplete="off"
              />
            </label>
            <label className="flex flex-col gap-1 text-[var(--text-muted)]">
              Expires
              <select value={ttlDays} onChange={event => setTtlDays(Number(event.target.value))} className={inputClass}>
                <option value={DEFAULT_KEY_TTL_DAYS}>in {DEFAULT_KEY_TTL_DAYS} days</option>
                <option value={30}>in 30 days</option>
                <option value={365}>in a year</option>
                <option value={0}>never</option>
              </select>
            </label>
            <button type="submit" disabled={busy} className={buttonClass}>
              {busy ? 'Working…' : 'Create a key'}
            </button>
          </form>
          {issued ? (
            <div className="space-y-2">
              <p className="font-semibold text-[var(--text-primary)]">
                Key for {issued.device.Label || 'unnamed client'}. Copy it now: it is not shown again.
              </p>
              <div className="flex items-start gap-2">
                <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-lg bg-[var(--bg-secondary)] p-3 select-text"><code>{issued.token}</code></pre>
                <button type="button" onClick={() => copy(issued.token, 'API key copied.')} className={`flex shrink-0 items-center gap-1 ${buttonClass}`}>
                  <Copy size={14} /> Copy
                </button>
              </div>
              <p className="text-[var(--text-muted)]">{describeExpiry(issued.device.ExpiresAt)}.</p>
            </div>
          ) : null}
        </div>
      </details>

      <p role="status" className="text-[var(--text-muted)]">{status}</p>
    </section>
  )
}
