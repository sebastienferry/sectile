import { useState } from 'react'
import { useOptionalApp } from '../context/AppContext'
import { mcpConfigText } from '../locales/mcpConfig'
import type { AIProvider } from '../types'
import { mcpProviders, mcpSnippet } from '../../../shared/mcpConfig.mjs'

export interface MCPEngineConfigProps {
  selectedProvider?: AIProvider
  onNavigateToWorkstations?: () => void
}

export function MCPEngineConfig({ selectedProvider = 'claude', onNavigateToWorkstations }: MCPEngineConfigProps) {
  const app = useOptionalApp()
  const text = mcpConfigText[app?.settings.language === 'en' ? 'en' : 'fr']
  const [notice, setNotice] = useState('')
  const provider = mcpProviders[selectedProvider]
  const server = window.location.origin
  if (!provider) return <div className="space-y-2 text-xs text-[var(--text-secondary)]"><p>{text.custom}</p><pre>{`${server}/mcp\nAuthorization: Bearer <SECTILE_API_KEY>\n\nsectile-agent mcp --url ${server}`}</pre></div>
  return <section className="space-y-3 rounded-xl border border-[var(--border-color)] p-4">
    <h3 className="text-sm font-bold">{text.title} · {provider.label}</h3>
    <p className="text-xs text-[var(--text-secondary)]">{text.choose} <code>{provider.path}</code>. {text.merge}</p>
    <p className="text-xs text-[var(--text-secondary)]">{text.key}</p>
    {onNavigateToWorkstations && <button type="button" className="text-xs text-indigo-400 cursor-pointer" onClick={onNavigateToWorkstations}>{text.keys}</button>}
    {(['http', 'stdio'] as const).map(transport => {
      const snippet = mcpSnippet(selectedProvider, transport, server)
      return <div key={transport} className="space-y-2 p-3 rounded-lg bg-[var(--bg-tertiary)]">
        <h4 className="text-xs font-bold">{transport === 'http' ? text.httpTitle : text.stdioTitle}</h4>
        <p className="text-xs text-[var(--text-secondary)]">{transport === 'http'
          ? text.http
          : text.stdio}</p>
        <pre className="p-3 text-[11px] overflow-x-auto rounded bg-[var(--bg-primary)]"><code>{snippet}</code></pre>
        <button type="button" className="text-xs text-indigo-400 cursor-pointer" onClick={async () => {
          try { await navigator.clipboard.writeText(snippet); setNotice(text.copied) }
          catch { setNotice(text.copyError) }
        }}>{text.copy} {transport === 'http' ? 'HTTP' : 'STDIO'}</button>
      </div>
    })}
    <p role="status" className="text-xs">{notice}</p>
  </section>
}
