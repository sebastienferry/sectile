import { useState } from 'react'
import { Check, Copy, Monitor, Terminal } from 'lucide-react'
import { useApp } from '../context/AppContext'

/**
 * Sectile Desktop App Panel
 * Graphical companion app with native PTY terminal streaming and workspace management.
 */
export function SectileDesktopPanel() {
  const { t } = useApp()
  const serverOrigin = window.location.origin
  const [copiedUrl, setCopiedUrl] = useState(false)

  async function copyOrigin() {
    try {
      await navigator.clipboard.writeText(serverOrigin)
      setCopiedUrl(true)
      setTimeout(() => setCopiedUrl(false), 2000)
    } catch {
      // Ignored
    }
  }

  return (
    <section
      className="p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-4"
      aria-labelledby="desktop-app-title"
    >
      <div className="flex items-center gap-2">
        <Monitor size={16} className="text-cyan-400" />
        <h4 id="desktop-app-title" className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
          {t.profileModal.workstations?.desktopAppTitle || 'Sectile Desktop App'}
        </h4>
      </div>

      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        {t.profileModal.workstations?.desktopAppDesc ||
          'Full desktop application with native PTY terminals, live log streaming, and visual workspace management.'}
      </p>

      <div className="p-3.5 rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] space-y-2.5 text-xs text-[var(--text-secondary)]">
        <div className="font-semibold text-[var(--text-primary)] text-[11px]">
          {t.profileModal.workstations?.onWorkstationTitle || 'Quick Setup Guide'} :
        </div>
        <div className="space-y-1.5 pl-1 text-[11px]">
          <div className="flex items-start gap-2">
            <span className="font-bold text-cyan-400">1.</span>
            <span>
              {t.profileModal.workstations?.desktopStep1 || 'Launch Sectile Desktop on your workstation.'}
            </span>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-bold text-cyan-400">2.</span>
            <span>
              {t.profileModal.workstations?.desktopStep2 || 'Set server address to:'}
            </span>
            <div className="inline-flex items-center gap-1.5 bg-[var(--bg-tertiary)] px-2 py-0.5 rounded-lg border border-[var(--border-color)] font-mono text-[10.5px] text-[var(--text-primary)]">
              <span>{serverOrigin}</span>
              <button
                type="button"
                onClick={copyOrigin}
                className="hover:text-cyan-400 transition-colors cursor-pointer"
                title={t.signIn.agent.copyServerUrl}
                aria-label={t.signIn.agent.copyServerUrl}
              >
                {copiedUrl ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
              </button>
            </div>
          </div>
          <div className="flex items-start gap-2">
            <span className="font-bold text-cyan-400">3.</span>
            <span>
              {t.profileModal.workstations?.desktopStep3 ||
                'Enter the temporary pairing code generated in the Workstations section above.'}
            </span>
          </div>
        </div>
      </div>
    </section>
  )
}

/**
 * Headless CLI Agent Panel
 * Background runner for terminals and CI/CD pipelines.
 */
export function HeadlessCliAgentPanel() {
  const { t } = useApp()
  const serverOrigin = window.location.origin
  const [serverUrl, setServerUrl] = useState(serverOrigin)
  const [copyStatus, setCopyStatus] = useState('')
  const [copiedCmd, setCopiedCmd] = useState(false)

  let validUrl = false
  try {
    const url = new URL(serverUrl)
    validUrl = ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash
  } catch {
    // Keep invalid input editable without generating an executable command.
  }

  const quotedUrl = "'" + serverUrl.trim().replace(/\/$/, '').replace(/'/g, "'\\''") + "'"
  const command = 'sectile-agent --url ' + quotedUrl

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(command)
      setCopiedCmd(true)
      setCopyStatus(t.profileModal.workstations?.copiedCommand || 'Command copied.')
      setTimeout(() => setCopiedCmd(false), 2000)
    } catch {
      setCopyStatus(t.signIn.apiKeys.copyUnavailableManual)
    }
  }

  return (
    <section
      className="p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)] space-y-4"
      aria-labelledby="headless-cli-title"
    >
      <div className="flex items-center gap-2">
        <Terminal size={16} className="text-emerald-400" />
        <h4 id="headless-cli-title" className="text-xs font-bold uppercase tracking-wider text-[var(--text-primary)]">
          {t.profileModal.workstations?.headlessCliTitle || 'Headless CLI Agent'}
        </h4>
      </div>

      <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
        {t.profileModal.workstations?.headlessCliDesc ||
          'Lightweight background runner executing autonomous tasks directly in your terminal or CI/CD pipeline.'}
      </p>

      {/* Server URL Input */}
      <div className="space-y-1">
        <label htmlFor="cli-server-url" className="block text-[11px] font-medium text-[var(--text-muted)]">
          {t.profileModal.workstations?.serverUrlLabel || 'Sectile Server URL'}
        </label>
        <input
          id="cli-server-url"
          type="url"
          value={serverUrl}
          onChange={event => {
            setServerUrl(event.target.value)
            setCopyStatus('')
          }}
          placeholder="http://localhost:8090"
          className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-emerald-500 transition-all font-mono"
          aria-invalid={!validUrl}
        />
      </div>

      {/* Executable Command Block */}
      {validUrl ? (
        <div className="flex items-center gap-2">
          <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-xl bg-[var(--bg-secondary)] px-3 py-2 text-xs font-mono text-[var(--text-primary)] select-text border border-[var(--border-color)]">
            <code>{command}</code>
          </pre>
          <button
            type="button"
            onClick={copyCommand}
            className="px-3 py-2 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] bg-[var(--bg-secondary)] hover:bg-[var(--bg-hover)] border border-[var(--border-color)] transition-all cursor-pointer flex items-center gap-1.5 shrink-0"
          >
            {copiedCmd ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
            <span>{copiedCmd ? t.signIn.apiKeys.copied : (t.profileModal.workstations?.copyCommandBtn || 'Copy Command')}</span>
          </button>
        </div>
      ) : (
        <div role="alert" className="p-2.5 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400 text-xs">
          {t.profileModal.workstations?.urlInvalidAlert ||
            'Enter a valid HTTP or HTTPS server URL without credentials or query parameters.'}
        </div>
      )}

      <p className="text-[10px] text-[var(--text-muted)] leading-relaxed">
        {t.profileModal.workstations?.cliPrereqNotice ||
          'Requires the sectile-agent binary in your PATH and a workstation paired once.'}{' '}
        <code className="px-1 py-0.5 rounded bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[9.5px]">
          sectile-agent pair --url … --code …
        </code>
      </p>

      {copyStatus && (
        <p role="status" className="text-[11px] text-emerald-400 font-medium">
          {copyStatus}
        </p>
      )}
    </section>
  )
}

/**
 * LocalExecutionPanel combines both panels for backwards compatibility.
 */
export function LocalExecutionPanel() {
  return (
    <div className="space-y-4" aria-labelledby="local-execution-title">
      <SectileDesktopPanel />
      <HeadlessCliAgentPanel />
    </div>
  )
}

/**
 * LocalAgentSetup export retained for backward compatibility.
 */
export const LocalAgentSetup = LocalExecutionPanel
