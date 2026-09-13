import { useState } from 'react'
import { Copy, Terminal } from 'lucide-react'

export function LocalAgentSetup() {
  const [serverUrl, setServerUrl] = useState(window.location.origin)
  const [copyStatus, setCopyStatus] = useState('')
  let validUrl = false
  try {
    const url = new URL(serverUrl)
    validUrl = ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash
  } catch {
    // Keep invalid input editable without generating an executable command.
  }
  const quotedUrl = "'" + serverUrl.trim().replace(/\/$/, '').replace(/'/g, "'\\''") + "'"
  const command = 'taskflow agent --url ' + quotedUrl

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(command)
      setCopyStatus('Command copied.')
    } catch {
      setCopyStatus('Copy unavailable. Select the command and copy it manually.')
    }
  }

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="local-agent-title">
      <h3 id="local-agent-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <Terminal size={16} /> Local agent
      </h3>
      <p className="text-[var(--text-muted)]">
        Use TaskFlow Desktop to host your execution consoles. The command below starts an optional headless agent from your repository or project mappings directory. Project settings and skills are downloaded from the server.
      </p>
      <label className="block space-y-1">
        <span className="font-semibold">Server URL</span>
        <input
          type="url"
          value={serverUrl}
          onChange={event => { setServerUrl(event.target.value); setCopyStatus('') }}
          className="w-full rounded-lg border border-[var(--border-color)] bg-[var(--bg-primary)] px-3 py-2 text-[var(--text-primary)]"
          aria-invalid={!validUrl}
        />
      </label>
      {validUrl ? (
        <div className="flex items-start gap-2">
          <pre className="min-w-0 flex-1 whitespace-pre-wrap break-all rounded-lg bg-[var(--bg-primary)] p-3 select-text"><code>{command}</code></pre>
          <button type="button" onClick={copyCommand} className="flex shrink-0 items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]">
            <Copy size={14} /> Copy
          </button>
        </div>
      ) : <p role="alert">Enter an HTTP or HTTPS server URL without credentials, query parameters or a fragment.</p>}
      <p className="text-[var(--text-muted)]">
        Requires the TaskFlow binary in your PATH and TASKFLOW_AGENT_TOKEN set in your terminal.
        Use the token provided by your server administrator; in local mode without a configured server token, any non-empty value is accepted.
        The agent connects to all projects by default.
      </p>
      <p role="status" className="text-[var(--text-muted)]">{copyStatus}</p>
    </section>
  )
}
