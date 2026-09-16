import { useCallback, useEffect, useState } from 'react'
import { Copy, Laptop, Trash2 } from 'lucide-react'

type Device = {
  ID: string
  UserID: string
  Label: string
  CreatedAt: string
  LastSeen: string
}

type PairingCode = {
  code: string
  expiresAt: string
}

function expiry(expiresAt: string): string {
  const date = new Date(expiresAt)
  return Number.isNaN(date.getTime()) ? 'expiry unknown' : `Valid until ${date.toLocaleTimeString()}`
}

export function WorkstationPairing() {
  // The address the workstation must be pointed at is the one serving this page.
  const serverOrigin = window.location.origin
  const [devices, setDevices] = useState<Device[]>([])
  const [code, setCode] = useState<PairingCode | null>(null)
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)

  const loadDevices = useCallback(async () => {
    try {
      const res = await fetch('/api/devices')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setDevices(body.devices ?? [])
    } catch {
      setStatus('Could not load paired workstations.')
    }
  }, [])

  useEffect(() => { void loadDevices() }, [loadDevices])

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

  async function copyCode() {
    if (!code) return
    try {
      await navigator.clipboard.writeText(code.code)
      setStatus('Pairing code copied.')
    } catch {
      setStatus('Copy unavailable. Select the code and copy it manually.')
    }
  }

  async function revoke(device: Device) {
    setStatus('')
    try {
      const res = await fetch(`/api/devices?id=${encodeURIComponent(device.ID)}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setStatus(`${device.Label || device.ID} revoked.`)
      await loadDevices()
    } catch {
      setStatus('Could not revoke that workstation.')
    }
  }

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="pairing-title">
      <h3 id="pairing-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <Laptop size={16} /> Workstations
      </h3>
      <p className="text-[var(--text-muted)]">
        Pair Sectile Desktop with this account. The code is single use, expires within ten minutes,
        and is exchanged once for a credential stored on that machine. Revoking a workstation
        leaves your other machines connected.
      </p>

      <button
        type="button"
        onClick={requestCode}
        disabled={busy}
        className="rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)] disabled:opacity-50"
      >
        {busy ? 'Issuing…' : 'Generate a pairing code'}
      </button>

      {code ? (
        <div className="flex items-start gap-2">
          <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-lg bg-[var(--bg-primary)] p-3 select-text"><code>{code.code}</code></pre>
          <button type="button" onClick={copyCode} className="flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]">
            <Copy size={14} /> Copy
          </button>
        </div>
      ) : null}
      {code ? <p className="text-[var(--text-muted)]">{expiry(code.expiresAt)}</p> : null}

      {/* A code on its own says nothing about where it is typed: the steps belong
          next to it, on the screen that issues it. */}
      <div className="space-y-2 rounded-lg border border-[var(--border-color)] bg-[var(--bg-primary)] p-3">
        <h4 className="font-semibold text-[var(--text-primary)]">On the workstation</h4>
        <ol className="list-decimal space-y-1 pl-5 text-[var(--text-muted)]">
          <li>Install and open Sectile Desktop on the machine you want to pair.</li>
          <li>
            In its connection screen, enter this server address:{' '}
            <code className="rounded bg-[var(--bg-secondary)] px-1 py-0.5 select-text">{serverOrigin}</code>
          </li>
          <li>Paste the pairing code above and confirm.</li>
          <li>
            The workstation appears in the list below once paired. Its credential is stored on that
            machine, so the code is never needed again.
          </li>
        </ol>
        <p className="text-[var(--text-muted)]">
          A code that expired before it was used is not reusable: generate a new one.
        </p>
      </div>

      {devices.length > 0 ? (
        <ul className="space-y-1">
          {devices.map(device => (
            <li key={device.ID} className="flex items-center justify-between gap-2 rounded-lg bg-[var(--bg-primary)] px-3 py-2">
              <span className="min-w-0 truncate">
                <span className="font-semibold">{device.Label || 'Unnamed workstation'}</span>
                <span className="text-[var(--text-muted)]"> · last seen {new Date(device.LastSeen).toLocaleString()}</span>
              </span>
              <button
                type="button"
                onClick={() => revoke(device)}
                aria-label={`Revoke ${device.Label || device.ID}`}
                className="flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 hover:bg-[var(--bg-hover)]"
              >
                <Trash2 size={14} /> Revoke
              </button>
            </li>
          ))}
        </ul>
      ) : <p className="text-[var(--text-muted)]">No workstation is paired yet.</p>}

      <p role="status" className="text-[var(--text-muted)]">{status}</p>
    </section>
  )
}
