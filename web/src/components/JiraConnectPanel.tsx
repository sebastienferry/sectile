import React, { useState } from 'react'
import { AlertTriangle, Globe, Link2, Loader2, ShieldCheck } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { JiraEntryState } from '../lib/jiraOAuth'
import type { StoredUserCredential } from '../lib/trackers'

/**
 * The Jira entry of the profile when it is not just the API token form
 * (#654): *Connect Jira* first, a connected grant with its account and sites,
 * or a lost one with *Reconnect Jira*. The token form stays one link away.
 */
export const JiraConnectPanel: React.FC<{
  state: Exclude<JiraEntryState, 'form' | 'token-and-connect'>
  credential?: StoredUserCredential
  onUseToken: () => void
}> = ({ state, credential, onUseToken }) => {
  const { connectJira, clearUserCredential, jiraOAuth, t } = useApp()
  const strings = t.trackerCredentials.oauth
  const [isConnecting, setIsConnecting] = useState(false)

  const connect = async () => {
    setIsConnecting(true)
    // On success the browser leaves for Atlassian; the button stays busy.
    if (!(await connectJira())) setIsConnecting(false)
  }

  const disconnect = async () => {
    if (!confirm(strings.disconnectConfirm)) return
    await clearUserCredential('jira')
  }

  const connectButton = (label: string) => (
    <button
      type="button"
      onClick={() => void connect()}
      disabled={isConnecting || !jiraOAuth.configured}
      className="flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
    >
      {isConnecting ? <Loader2 size={13} className="animate-spin" /> : <Link2 size={13} />}
      {isConnecting ? strings.connecting : label}
    </button>
  )

  const tokenLink = (
    <button type="button" onClick={onUseToken} className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--text-primary)] underline cursor-pointer">
      {strings.useTokenInstead}
    </button>
  )

  if (state === 'connect') {
    return (
      <div className="space-y-3">
        <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">{strings.connectHint}</p>
        <div className="flex items-center justify-between gap-2">
          {tokenLink}
          {connectButton(strings.connect)}
        </div>
      </div>
    )
  }

  if (state === 'disconnected') {
    return (
      <div className="space-y-3">
        <div className="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-[11px] text-amber-500 flex items-start gap-2">
          <AlertTriangle size={13} className="shrink-0 mt-0.5" />
          <span>
            <span className="font-bold block">{strings.disconnectedTitle}</span>
            {strings.disconnectedBody}
          </span>
        </div>
        {!jiraOAuth.configured && <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{strings.notConfiguredHint}</p>}
        <div className="flex items-center justify-between gap-2">
          {tokenLink}
          {connectButton(strings.reconnect)}
        </div>
      </div>
    )
  }

  // Connected.
  const sites = credential?.grantedSites || []
  return (
    <div className="space-y-3">
      <div
        className="p-2.5 rounded-xl border text-[11px] leading-relaxed space-y-1"
        style={{ background: 'rgb(var(--status-ok-rgb) / 0.1)', borderColor: 'rgb(var(--status-ok-rgb) / 0.35)', color: 'var(--status-ok)' }}
      >
        <span className="flex items-center gap-1.5 font-bold">
          <ShieldCheck size={13} />
          {credential?.account ? strings.connectedAs.replace('{account}', credential.account) : strings.connectedNoAccount}
        </span>
        {sites.length > 0 && (
          <span className="flex items-start gap-1.5 text-[var(--text-secondary)]">
            <Globe size={12} className="shrink-0 mt-0.5" />
            <span>
              {strings.sites} : {sites.join(', ')}
            </span>
          </span>
        )}
      </div>
      {!jiraOAuth.configured && <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{strings.notConfiguredHint}</p>}
      <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{strings.atlassianNote}</p>
      <div className="flex items-center justify-between gap-2">
        {tokenLink}
        <button type="button" onClick={() => void disconnect()} className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer">
          {strings.disconnect}
        </button>
      </div>
    </div>
  )
}

/** The *Connect Jira* offer shown above the token form while an API token is stored. */
export const JiraConnectOffer: React.FC<{ hint: string; onBack?: () => void }> = ({ hint, onBack }) => {
  const { connectJira, t } = useApp()
  const [isConnecting, setIsConnecting] = useState(false)
  return (
    <div className="p-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] flex items-center justify-between gap-3">
      <span className="text-[10.5px] text-[var(--text-secondary)] leading-relaxed">{hint}</span>
      <div className="flex items-center gap-2 shrink-0">
        {onBack && (
          <button type="button" onClick={onBack} className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--text-primary)] underline cursor-pointer">
            {t.trackerCredentials.oauth.useConnectInstead}
          </button>
        )}
        <button
          type="button"
          onClick={async () => {
            setIsConnecting(true)
            if (!(await connectJira())) setIsConnecting(false)
          }}
          disabled={isConnecting}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
        >
          {isConnecting ? <Loader2 size={13} className="animate-spin" /> : <Link2 size={13} />}
          {isConnecting ? t.trackerCredentials.oauth.connecting : t.trackerCredentials.oauth.connect}
        </button>
      </div>
    </div>
  )
}
