import { MCPApiKeyForm } from './ApiKeys'
import { useState } from 'react'
import { Network, Copy } from 'lucide-react'
import { useOptionalApp } from '../context/AppContext'
import { mcpConfigText } from '../locales/mcpConfig'
import type { AIProvider } from '../types'
import { mcpProviders, mcpSnippet } from '../../../shared/mcpConfig.mjs'

export interface MCPEngineConfigProps {
  selectedProvider?: AIProvider
  onNavigateToWorkstations?: () => void
  onKeyCreated?: () => void
}

export function MCPEngineConfig({ selectedProvider = 'claude', onNavigateToWorkstations, onKeyCreated }: MCPEngineConfigProps) {
  const app = useOptionalApp()
  const text = mcpConfigText[app?.settings.language === 'en' ? 'en' : 'fr']
  const [notice, setNotice] = useState('')
  const [mode, setMode] = useState<'remote' | 'local' | 'stdio'>('remote')
  const transport = mode === 'stdio' ? 'stdio' : 'http'
  const [localUrl, setLocalUrl] = useState('http://127.0.0.1:8091')
  const local = mode === 'local'
  const server = import.meta.env.VITE_MCP_SERVER_URL || (import.meta.env.DEV ? 'http://localhost:8090' : window.location.origin)
  const snippet = mcpSnippet(selectedProvider, transport, local ? localUrl : server, local)
  const provider = mcpProviders[selectedProvider]
  if (!provider) return <div className="space-y-2 text-xs text-[var(--text-secondary)]"><p>{text.custom}</p><pre>{`${server}/mcp\nAuthorization: Bearer <SECTILE_API_KEY>\n\nsectile-agent mcp --url ${server}`}</pre></div>
  return <section className="space-y-3 p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)]">
    <h3 className="flex items-center gap-2 text-xs font-bold text-[var(--text-primary)]"><Network size={15} className="text-indigo-400" />{text.title} · {provider.label}</h3>
    <p className="text-xs text-[var(--text-secondary)]">{text.choose} <code>{provider.path}</code>. {text.merge}</p>
    {!local && <p className="text-[11px] text-[var(--text-secondary)]">{text.key}</p>}
    {onNavigateToWorkstations && <button type="button" className="text-xs text-indigo-400 cursor-pointer" onClick={onNavigateToWorkstations}>{text.keys}</button>}
    <div role="group" aria-label={text.title} className="inline-flex flex-wrap gap-1 bg-[var(--bg-secondary)] p-0.5 rounded-lg border border-[var(--border-color)]">
      {(['remote', 'local', 'stdio'] as const).map(value => <button key={value} type="button" aria-pressed={mode === value}
        onClick={() => { setMode(value); setNotice('') }}
        className={`px-2.5 py-1 rounded-md text-[10.5px] font-medium transition-colors cursor-pointer ${mode === value ? 'bg-[var(--bg-tertiary)] text-[var(--text-primary)] font-semibold shadow-xs' : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'}`}>
        {value === 'remote' ? text.remote : value === 'local' ? text.proxy : 'STDIO'}
      </button>)}
    </div>
    {local && <label className="block space-y-1 text-[11px] text-[var(--text-secondary)]">{text.proxyUrl}
      <input type="url" value={localUrl} onChange={event => setLocalUrl(event.target.value)} className="w-full px-3 py-2 text-xs rounded-xl bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[var(--text-primary)]" />
    </label>}
    <div className="space-y-2 p-3 rounded-lg bg-[var(--bg-tertiary)]">
        <h4 className="text-xs font-bold">{transport === 'http' ? (local ? text.proxy : text.remote) : text.stdioTitle}</h4>
        <p className="text-xs text-[var(--text-secondary)]">{transport === 'http'
          ? (local ? text.local : text.http)
          : (local ? text.stdioLocal : text.stdio)}</p>
        <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono whitespace-pre-wrap break-all select-text"><code>{snippet}</code></pre>
        <button type="button" className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer" onClick={async () => {
          try { await navigator.clipboard.writeText(snippet); setNotice(text.copied) }
          catch { setNotice(text.copyError) }
        }}><Copy size={11} />{text.copy} {transport === 'http' ? 'HTTP' : 'STDIO'}</button>
      </div>
    {!local && <MCPApiKeyForm onKeyCreated={onKeyCreated} />}
    <p role="status" className="text-xs">{notice}</p>
  </section>
}
