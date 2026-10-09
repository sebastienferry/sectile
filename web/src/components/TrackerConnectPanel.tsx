import React, { useState } from 'react'
import { AlertCircle, AlertTriangle, CheckCircle2, Globe, Info, Link2, Loader2, ShieldCheck, X } from 'lucide-react'
import { useApp } from '../context/AppContext'
import { forgeOAuthStrings, oauthOutcomeMessage, oauthOutcomeTone, oauthStrings, type EntryState } from '../lib/trackerOAuth'
import type { OAuthTracker, StoredUserCredential } from '../lib/trackers'

/**
 * A tracker's entry of the profile when it is not just the token form: Jira
 * through Atlassian (#654), GitHub and GitLab through their OAuth app (#804).
 * *Connect* first, a connected grant with its account (and for Jira its
 * sites), or a lost one with *Reconnect*. The token form stays one link away.
 */
export const TrackerConnectPanel: React.FC<{
  tracker: OAuthTracker
  state: Exclude<EntryState, 'form' | 'token-and-connect'>
  credential?: StoredUserCredential
  onUseToken: () => void
}> = ({ tracker, state, credential, onUseToken }) => {
  const { connectTracker, clearUserCredential, trackerOAuth, addToast, t } = useApp()
  const strings = oauthStrings(t, tracker)
  const forge = forgeOAuthStrings(t, tracker)
  const configured = trackerOAuth[tracker].configured
  const [isConnecting, setIsConnecting] = useState(false)

  const connect = async () => {
    setIsConnecting(true)
    // On success the browser leaves for the provider; the button stays busy.
    if (!(await connectTracker(tracker))) setIsConnecting(false)
  }

  const disconnect = async () => {
    if (!confirm(strings.disconnectConfirm)) return
    if (!(await clearUserCredential(tracker))) return
    // Atlassian has no revocation endpoint: the note outlives the panel that
    // showed it, so the person still reads it once the grant is forgotten.
    // A forge grant is revoked by the server itself.
    if (tracker === 'jira') addToast({ type: 'info', title: strings.disconnect, description: t.trackerCredentials.oauth.atlassianNote })
  }

  const connectButton = (label: string) => (
    <button
      type="button"
      onClick={() => void connect()}
      disabled={isConnecting || !configured}
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

  const instanceNote = forge && <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{forge.instanceNote}</p>

  if (state === 'connect') {
    return (
      <div className="space-y-3">
        <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">{strings.connectHint}</p>
        {instanceNote}
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
        {!configured && <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{strings.notConfiguredHint}</p>}
        <div className="flex items-center justify-between gap-2">
          {tokenLink}
          {connectButton(strings.reconnect)}
        </div>
      </div>
    )
  }

  // Connected. Only a Jira grant names the sites it covers.
  const sites = tracker === 'jira' ? credential?.grantedSites || [] : []
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
              {t.trackerCredentials.oauth.sites} {sites.join(', ')}
            </span>
          </span>
        )}
      </div>
      {!configured && <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{strings.notConfiguredHint}</p>}
      {tracker === 'jira' ? <p className="text-[10.5px] text-[var(--text-muted)] leading-relaxed">{t.trackerCredentials.oauth.atlassianNote}</p> : instanceNote}
      <div className="flex items-center justify-between gap-2">
        {tokenLink}
        <button type="button" onClick={() => void disconnect()} className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--status-danger)] cursor-pointer">
          {strings.disconnect}
        </button>
      </div>
    </div>
  )
}

/** The *Connect* offer shown above the token form while a token is stored. */
export const TrackerConnectOffer: React.FC<{ tracker: OAuthTracker; hint: string; onBack?: () => void }> = ({ tracker, hint, onBack }) => {
  const { connectTracker, trackerOAuth, t } = useApp()
  const strings = oauthStrings(t, tracker)
  const [isConnecting, setIsConnecting] = useState(false)
  return (
    <div className="p-2.5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] flex items-center justify-between gap-3">
      <span className="text-[10.5px] text-[var(--text-secondary)] leading-relaxed">{hint}</span>
      <div className="flex items-center gap-2 shrink-0">
        {onBack && (
          <button type="button" onClick={onBack} className="text-[10.5px] text-[var(--text-muted)] hover:text-[var(--text-primary)] underline cursor-pointer">
            {strings.useConnectInstead}
          </button>
        )}
        <button
          type="button"
          onClick={async () => {
            setIsConnecting(true)
            if (!(await connectTracker(tracker))) setIsConnecting(false)
          }}
          disabled={isConnecting || !trackerOAuth[tracker].configured}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
        >
          {isConnecting ? <Loader2 size={13} className="animate-spin" /> : <Link2 size={13} />}
          {isConnecting ? strings.connecting : strings.connect}
        </button>
      </div>
    </div>
  )
}

/**
 * What the last consent came back with, kept in its tracker's entry until the
 * person dismisses it: a notification alone disappears before a refusal and
 * the sites it names can be read.
 */
export const OAuthOutcomeBanner: React.FC<{ tracker: OAuthTracker }> = ({ tracker }) => {
  const { oauthOutcome, dismissOAuthOutcome, trackerOAuth, t } = useApp()
  if (!oauthOutcome || oauthOutcome.tracker !== tracker) return null
  const { outcome } = oauthOutcome
  const strings = oauthStrings(t, tracker)
  const tone = oauthOutcomeTone(outcome)
  const rgb = tone === 'success' ? '--status-ok-rgb' : tone === 'error' ? '--status-danger-rgb' : '--accent-rgb'
  const color = tone === 'success' ? 'var(--status-ok)' : tone === 'error' ? 'var(--status-danger)' : 'var(--text-secondary)'
  const Icon = tone === 'success' ? CheckCircle2 : tone === 'error' ? AlertCircle : Info
  return (
    <div
      role="status"
      data-oauth-outcome={outcome}
      data-oauth-tracker={tracker}
      className="p-2.5 rounded-xl border text-[11px] leading-relaxed flex items-start gap-2"
      style={{ background: `rgb(var(${rgb}) / 0.1)`, borderColor: `rgb(var(${rgb}) / 0.35)`, color }}
    >
      <Icon size={13} className="shrink-0 mt-0.5" />
      <span className="flex-1">
        <span className="font-bold block">{strings.outcomeTitle}</span>
        {oauthOutcomeMessage(outcome, strings.outcomes, trackerOAuth[tracker].sites)}
      </span>
      <button type="button" onClick={dismissOAuthOutcome} title={strings.dismiss} aria-label={strings.dismiss} className="shrink-0 opacity-70 hover:opacity-100 cursor-pointer">
        <X size={13} />
      </button>
    </div>
  )
}
