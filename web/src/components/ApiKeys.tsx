import React, { useCallback, useEffect, useState } from 'react'
import { Check, Copy, Cpu, Laptop, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { Antigravity, Claude, OpenAI, Cursor } from './icons'
import { useApp } from '../context/AppContext'
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

/**
 * Section 1: Workstations Panel
 * Handles pairing development machines via temporary codes and managing paired devices.
 */
export function WorkstationsPanel({
  refreshTrigger = 0,
  onDeviceChange,
}: {
  refreshTrigger?: number
  onDeviceChange?: () => void
}) {
  const { t } = useApp()
  const serverOrigin = window.location.origin
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [code, setCode] = useState<PairingCode | null>(null)
  const [sharedServerToken, setSharedServerToken] = useState(false)
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [copiedCode, setCopiedCode] = useState(false)

  const loadKeys = useCallback(async () => {
    try {
      const res = await fetch('/api/devices')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setKeys(body.devices ?? [])
    } catch {
      setStatus(t.profileModal.workstations?.noWorkstations || 'Could not load workstations.')
    }
  }, [t])

  useEffect(() => {
    void loadKeys()
  }, [loadKeys, refreshTrigger])

  useEffect(() => {
    fetch('/api/me')
      .then(res => (res.ok ? res.json() : null))
      .then(body => setSharedServerToken(Boolean(body?.sharedServerToken)))
      .catch(() => {})
  }, [])

  async function copy(value: string, doneMsg: string) {
    try {
      await navigator.clipboard.writeText(value)
      setCopiedCode(true)
      setStatus(doneMsg)
      setTimeout(() => setCopiedCode(false), 2000)
    } catch {
      setStatus(t.profileModal.workstations?.copyCommandBtn ? 'Copy unavailable.' : 'Copy unavailable.')
    }
  }

  async function requestCode() {
    setBusy(true)
    setStatus('')
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
      onDeviceChange?.()
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
      onDeviceChange?.()
    } catch {
      setStatus('Could not revoke that workstation.')
    }
  }

  const pairCommand = `sectile-agent pair --url ${serverOrigin} --code <CODE>`

  return (
    <section
      className="p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-4"
      aria-labelledby="workstations-title"
    >
      {/* Header */}
      <div className="flex items-center gap-2">
        <Laptop size={16} className="text-blue-400" />
        <h4 id="workstations-title" className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
          {t.profileModal.workstations?.title || 'Workstations'}
        </h4>
      </div>

      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        {t.profileModal.workstations?.desc ||
          `Pair each machine once with a code. The workstation receives its own credential and keeps it; you never handle it. It expires after ${DEFAULT_KEY_TTL_DAYS} days unless renewed here.`}
      </p>

      {/* Shared Server Token Deprecation Warning */}
      {sharedServerToken && (
        <div role="alert" className="p-3 rounded-xl border border-amber-500/30 bg-amber-500/10 text-amber-300 text-xs space-y-1">
          <div className="font-semibold">SECTILE_SERVER_TOKEN</div>
          <p className="text-[11px] text-amber-300/90 leading-relaxed">
            {t.profileModal.workstations?.sharedTokenWarning ||
              'This server still runs with SECTILE_SERVER_TOKEN, which is deprecated and will be removed in the next release. Pair your workstations and drop the variable.'}
          </p>
        </div>
      )}

      {/* Action Button: Pair a Workstation */}
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={requestCode}
          disabled={busy}
          className="px-3.5 py-1.5 rounded-lg text-xs font-semibold text-white accent-bg shadow-xs hover:opacity-90 active:scale-95 transition-all cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
        >
          <Plus size={13} />
          <span>{busy ? (t.profileModal.workstations?.workingBtn || 'Working…') : (t.profileModal.workstations?.pairBtn || 'Pair a workstation')}</span>
        </button>
      </div>

      {/* Temporary Pairing Code Card */}
      {code && (
        <div className="p-3.5 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] space-y-3">
          <div className="flex items-center justify-between text-[11px]">
            <span className="font-semibold text-[var(--text-primary)]">
              {t.profileModal.workstations?.onWorkstationTitle || 'Temporary Pairing Code'}
            </span>
            <span className="text-[10px] text-[var(--text-muted)] font-mono">
              {codeExpiry(code.expiresAt)}
            </span>
          </div>

          <div className="flex items-center gap-2">
            <code className="min-w-0 flex-1 font-mono text-xs text-[var(--text-primary)] bg-[var(--bg-tertiary)] px-3 py-2 rounded-lg border border-[var(--border-color)] select-text break-all">
              {code.code}
            </code>
            <button
              type="button"
              onClick={() => copy(code.code, t.profileModal.workstations?.copiedCommand || 'Pairing code copied.')}
              className="px-3 py-2 rounded-lg text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] border border-[var(--border-color)] transition-all cursor-pointer flex items-center gap-1.5 shrink-0"
            >
              {copiedCode ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
              <span>{copiedCode ? 'Copied' : 'Copy'}</span>
            </button>
          </div>

          <div className="space-y-1.5 text-[11px] text-[var(--text-secondary)] pt-1">
            <div className="font-semibold text-[var(--text-primary)]">
              {t.profileModal.workstations?.onWorkstationTitle || 'On the workstation'} :
            </div>
            <div className="space-y-1 pl-1">
              <div className="flex items-start gap-1.5">
                <span className="font-medium text-[var(--text-muted)]">1.</span>
                <span>
                  {t.profileModal.workstations?.terminalStep || 'In a terminal:'}{' '}
                  <code className="rounded bg-[var(--bg-tertiary)] px-1.5 py-0.5 text-[10px] font-mono select-text border border-[var(--border-color)]">
                    {pairCommand.replace('<CODE>', code.code)}
                  </code>
                </span>
              </div>
              <div className="flex items-start gap-1.5">
                <span className="font-medium text-[var(--text-muted)]">2.</span>
                <span>
                  {t.profileModal.workstations?.desktopStep || 'Or in Sectile Desktop, paste the code in the connection screen.'}
                </span>
              </div>
            </div>
            <p className="text-[10px] text-[var(--text-muted)] italic pt-1">
              {t.profileModal.workstations?.singleUseNotice || 'Single use. Non-reusable once expired: generate a new one if needed.'}
            </p>
          </div>
        </div>
      )}

      {/* Paired Machines List */}
      <div className="space-y-2 pt-1">
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
            {t.profileModal.workstations?.pairedListTitle || 'Connected Machines'}
          </span>
          <span className="text-[10px] font-mono text-[var(--text-muted)]">
            {keys.length} {keys.length === 1 ? 'machine' : 'machines'}
          </span>
        </div>

        {keys.length > 0 ? (
          <ul className="space-y-2">
            {keys.map(key => {
              const state = expiryState(key.ExpiresAt)
              const badgeClass =
                state === 'expired'
                  ? 'bg-red-500/15 text-red-400 border-red-500/30'
                  : state === 'soon'
                    ? 'bg-amber-500/15 text-amber-400 border-amber-500/30'
                    : 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'

              return (
                <li
                  key={key.ID}
                  className="flex flex-wrap sm:flex-nowrap items-center justify-between gap-3 p-3 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)]"
                >
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Laptop size={14} className="text-blue-400 shrink-0" />
                      <span className="font-semibold text-xs text-[var(--text-primary)] truncate">
                        {key.Label || 'Unnamed Workstation'}
                      </span>
                      <span className={`px-1.5 py-0.5 rounded text-[9.5px] font-medium border ${badgeClass}`}>
                        {describeExpiry(key.ExpiresAt)}
                      </span>
                    </div>
                    <div className="text-[10.5px] text-[var(--text-muted)]">
                      {t.profileModal.workstations?.lastSeen || 'last seen'}{' '}
                      {new Date(key.LastSeen).toLocaleString()}
                    </div>
                  </div>

                  <div className="flex items-center gap-1.5 shrink-0">
                    <button
                      type="button"
                      onClick={() => renew(key, DEFAULT_KEY_TTL_DAYS)}
                      aria-label={`Renew ${key.Label || key.ID}`}
                      className="px-2.5 py-1 rounded-lg text-[11px] font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] border border-[var(--border-color)] transition-all cursor-pointer flex items-center gap-1"
                    >
                      <RefreshCw size={11} />
                      <span>
                        {key.ExpiresAt
                          ? (t.profileModal.workstations?.renewBtn || 'Renew')
                          : (t.profileModal.workstations?.renewNever || 'Never expires')}
                      </span>
                    </button>
                    <button
                      type="button"
                      onClick={() => revoke(key)}
                      aria-label={`Revoke ${key.Label || key.ID}`}
                      className="px-2.5 py-1 rounded-lg text-[11px] font-medium text-red-400 hover:text-red-300 bg-red-500/10 hover:bg-red-500/20 border border-red-500/20 transition-all cursor-pointer flex items-center gap-1"
                    >
                      <Trash2 size={11} />
                      <span>{t.profileModal.workstations?.revokeBtn || 'Revoke'}</span>
                    </button>
                  </div>
                </li>
              )
            })}
          </ul>
        ) : (
          <div className="p-4 rounded-xl border border-dashed border-[var(--border-color)] text-center text-xs text-[var(--text-muted)]">
            {t.profileModal.workstations?.noWorkstations || 'No workstation is paired yet.'}
          </div>
        )}
      </div>

      {status && (
        <p role="status" className="text-[11px] text-[var(--text-secondary)] font-medium">
          {status}
        </p>
      )}
    </section>
  )
}

/**
 * Section 2: Direct MCP Integration Panel
 * For connecting AI provider desktop apps (Cursor, Claude Desktop, Antigravity, etc.)
 * directly to Sectile's MCP endpoint without a local agent daemon.
 */
export function DirectMcpPanel({
  onKeyCreated,
}: {
  onKeyCreated?: () => void
}) {
  const { t } = useApp()
  const serverOrigin = window.location.origin
  const [label, setLabel] = useState('')
  const [ttlDays, setTtlDays] = useState<number>(DEFAULT_KEY_TTL_DAYS)
  const [issued, setIssued] = useState<IssuedKey | null>(null)
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState('')
  const [copiedKey, setCopiedKey] = useState(false)
  const [copiedConfig, setCopiedConfig] = useState(false)
  const [activeConfigTab, setActiveConfigTab] = useState<'claude' | 'agy' | 'codex' | 'cursor'>('claude')

  async function createKey(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setStatus('')
    try {
      const res = await fetch('/api/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label: label.trim(), ttlDays }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const keyData: IssuedKey = await res.json()
      setIssued(keyData)
      setLabel('')
      onKeyCreated?.()
    } catch {
      setStatus('Could not create an API key.')
    } finally {
      setBusy(false)
    }
  }

  async function copyToClipboard(val: string, type: 'key' | 'config') {
    try {
      await navigator.clipboard.writeText(val)
      if (type === 'key') {
        setCopiedKey(true)
        setTimeout(() => setCopiedKey(false), 2000)
      } else {
        setCopiedConfig(true)
        setTimeout(() => setCopiedConfig(false), 2000)
      }
    } catch {
      setStatus('Copy unavailable. Please copy manually.')
    }
  }

  const effectiveToken = issued ? issued.token : '<YOUR_API_KEY>'
  const mcpEndpoint = `${serverOrigin}/mcp`

  const claudeConfigJson = JSON.stringify(
    {
      mcpServers: {
        sectile: {
          type: 'http',
          url: mcpEndpoint,
          headers: {
            Authorization: `Bearer ${effectiveToken}`,
          },
        },
      },
    },
    null,
    2
  )

  const agyConfigJson = JSON.stringify(
    {
      mcpServers: {
        sectile: {
          url: mcpEndpoint,
          headers: {
            Authorization: `Bearer ${effectiveToken}`,
          },
        },
      },
    },
    null,
    2
  )

  const codexConfigToml = `[mcp_servers.sectile]
url = "${mcpEndpoint}"
headers = { Authorization = "Bearer ${effectiveToken}" }`

  const cursorConfigJson = JSON.stringify(
    {
      mcpServers: {
        sectile: {
          url: mcpEndpoint,
          headers: {
            Authorization: `Bearer ${effectiveToken}`,
          },
        },
      },
    },
    null,
    2
  )

  const displayedConfig =
    activeConfigTab === 'claude'
      ? claudeConfigJson
      : activeConfigTab === 'agy'
      ? agyConfigJson
      : activeConfigTab === 'codex'
      ? codexConfigToml
      : cursorConfigJson

  return (
    <section
      className="p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-4"
      aria-labelledby="direct-mcp-title"
    >
      {/* Header */}
      <div className="flex items-center gap-2">
        <Cpu size={16} className="text-purple-400" />
        <h4 id="direct-mcp-title" className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
          {t.profileModal.workstations?.directMcpTitle || 'Direct MCP Integration'}
        </h4>
      </div>

      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        {t.profileModal.workstations?.directMcpDesc ||
          'Connect AI desktop applications (Cursor, Claude Desktop, Antigravity, etc.) directly to Sectile\'s MCP endpoint (/mcp) via an API key without running a local agent daemon.'}
      </p>

      {/* API Key Generation Form */}
      <form onSubmit={createKey} className="grid grid-cols-1 sm:grid-cols-12 gap-2.5 items-end">
        <div className="sm:col-span-6 space-y-1">
          <label htmlFor="mcp-key-label" className="block text-[11px] font-medium text-[var(--text-muted)]">
            {t.profileModal.workstations?.labelInput || 'Client Label'}
          </label>
          <input
            id="mcp-key-label"
            value={label}
            onChange={e => setLabel(e.target.value)}
            placeholder={t.profileModal.workstations?.labelPlaceholder || 'e.g. Claude Desktop on MacBook Pro…'}
            className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-all"
            autoComplete="off"
          />
        </div>

        <div className="sm:col-span-3 space-y-1">
          <label htmlFor="mcp-key-ttl" className="block text-[11px] font-medium text-[var(--text-muted)]">
            {t.profileModal.workstations?.expiresInput || 'Expiration'}
          </label>
          <select
            id="mcp-key-ttl"
            value={ttlDays}
            onChange={e => setTtlDays(Number(e.target.value))}
            className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-purple-500 transition-all cursor-pointer"
          >
            <option value={DEFAULT_KEY_TTL_DAYS}>
              {t.profileModal.workstations?.ttl90 || `in ${DEFAULT_KEY_TTL_DAYS} days`}
            </option>
            <option value={30}>{t.profileModal.workstations?.ttl30 || 'in 30 days'}</option>
            <option value={365}>{t.profileModal.workstations?.ttl365 || 'in 1 year'}</option>
            <option value={0}>{t.profileModal.workstations?.ttl0 || 'never'}</option>
          </select>
        </div>

        <div className="sm:col-span-3">
          <button
            type="submit"
            disabled={busy}
            className="w-full px-3.5 py-2 rounded-xl text-xs font-semibold text-white accent-bg shadow-xs hover:opacity-90 active:scale-95 transition-all cursor-pointer disabled:opacity-50 flex items-center justify-center gap-1.5"
          >
            <Plus size={13} />
            <span>{busy ? (t.profileModal.workstations?.workingBtn || 'Working…') : (t.profileModal.workstations?.createKeyBtn || 'Create API Key')}</span>
          </button>
        </div>
      </form>

      {/* Key Created Reveal Banner */}
      {issued && (
        <div className="p-3.5 rounded-xl border border-emerald-500/30 bg-emerald-500/10 space-y-2.5">
          <div className="flex items-center justify-between">
            <span className="text-xs font-bold text-emerald-400">
              {t.profileModal.workstations?.keyCreatedSuccess?.replace('{label}', issued.device.Label || 'Client') ||
                `API key generated for ${issued.device.Label || 'Client'}. Copy it now: it will never be displayed again.`}
            </span>
            <span className="text-[10px] text-emerald-300/80 font-mono">
              {describeExpiry(issued.device.ExpiresAt)}
            </span>
          </div>

          <div className="flex items-center gap-2">
            <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-lg bg-[var(--bg-secondary)] px-3 py-2 text-xs font-mono text-[var(--text-primary)] select-text border border-[var(--border-color)]">
              <code>{issued.token}</code>
            </pre>
            <button
              type="button"
              onClick={() => copyToClipboard(issued.token, 'key')}
              className="px-3 py-2 rounded-lg text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-secondary)] hover:bg-[var(--bg-hover)] border border-[var(--border-color)] transition-all cursor-pointer flex items-center gap-1.5 shrink-0"
            >
              {copiedKey ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
              <span>{copiedKey ? 'Copied' : (t.profileModal.workstations?.copyKeyBtn || 'Copy Key')}</span>
            </button>
          </div>
        </div>
      )}

      {/* Desktop App Configuration Presets */}
      <div className="space-y-2 pt-1">
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-bold uppercase tracking-wider text-[var(--text-muted)]">
            {t.profileModal.workstations?.desktopConfigsTitle || 'Desktop App Configurations'}
          </span>
          <div className="flex items-center gap-1 bg-[var(--bg-secondary)] p-0.5 rounded-lg border border-[var(--border-color)] overflow-x-auto">
            <button
              type="button"
              onClick={() => setActiveConfigTab('claude')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10.5px] font-medium transition-colors cursor-pointer shrink-0 ${
                activeConfigTab === 'claude'
                  ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)] font-semibold shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
              }`}
            >
              <Claude size={12} className="shrink-0" />
              <span>{t.profileModal.workstations?.tabClaude || 'Claude Desktop'}</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveConfigTab('agy')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10.5px] font-medium transition-colors cursor-pointer shrink-0 ${
                activeConfigTab === 'agy'
                  ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)] font-semibold shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
              }`}
            >
              <Antigravity size={12} className="shrink-0" />
              <span>{t.profileModal.workstations?.tabAgy || 'Antigravity'}</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveConfigTab('codex')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10.5px] font-medium transition-colors cursor-pointer shrink-0 ${
                activeConfigTab === 'codex'
                  ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)] font-semibold shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
              }`}
            >
              <OpenAI size={12} className="shrink-0" />
              <span>{t.profileModal.workstations?.tabCodex || 'Codex / ChatGPT'}</span>
            </button>
            <button
              type="button"
              onClick={() => setActiveConfigTab('cursor')}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[10.5px] font-medium transition-colors cursor-pointer shrink-0 ${
                activeConfigTab === 'cursor'
                  ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)] font-semibold shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
              }`}
            >
              <Cursor size={12} className="shrink-0" />
              <span>{t.profileModal.workstations?.tabCursor || 'Cursor'}</span>
            </button>
          </div>
        </div>

        <div className="relative">
          <pre className="p-3 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] font-mono text-[11px] text-[var(--text-primary)] select-text overflow-x-auto">
            <code>{displayedConfig}</code>
          </pre>
          <div className="absolute top-2.5 right-2.5">
            <button
              type="button"
              onClick={() => copyToClipboard(displayedConfig, 'config')}
              className="px-2.5 py-1 rounded-lg text-[10.5px] font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-tertiary)] hover:bg-[var(--bg-hover)] border border-[var(--border-color)] transition-all cursor-pointer flex items-center gap-1 shadow-xs"
            >
              {copiedConfig ? <Check size={12} className="text-emerald-400" /> : <Copy size={12} />}
              <span>{copiedConfig ? 'Copied' : (t.profileModal.workstations?.copyConfigBtn || 'Copy Config')}</span>
            </button>
          </div>
        </div>
      </div>

      {status && (
        <p role="status" className="text-[11px] text-purple-400 font-medium">
          {status}
        </p>
      )}
    </section>
  )
}

/**
 * ApiKeysPanel combines both panels for backwards compatibility.
 */
export function ApiKeysPanel() {
  const [devicesVersion, setDevicesVersion] = useState(0)

  return (
    <div className="space-y-6">
      <WorkstationsPanel
        refreshTrigger={devicesVersion}
        onDeviceChange={() => setDevicesVersion(v => v + 1)}
      />
      <DirectMcpPanel onKeyCreated={() => setDevicesVersion(v => v + 1)} />
    </div>
  )
}
