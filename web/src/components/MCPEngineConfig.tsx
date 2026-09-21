import React, { useState } from 'react'
import { Check, Copy, ExternalLink, Network, Terminal, Code2 } from 'lucide-react'
import { Antigravity, Claude, OpenAI, Cursor } from '@lobehub/icons'
import type { AIProvider } from '../types'
import { useOptionalApp } from '../context/AppContext'
import { translations } from '../locales/translations'

export interface MCPEngineConfigProps {
  selectedProvider?: AIProvider
  onNavigateToWorkstations?: () => void
}

type MCPEngineKey = 'claude' | 'agy' | 'codex' | 'cursor'

export const MCPEngineConfig: React.FC<MCPEngineConfigProps> = ({
  selectedProvider,
  onNavigateToWorkstations,
}) => {
  const app = useOptionalApp()
  const t = app?.t || translations.fr
  const aiT = t?.profileModal?.ai
  const serverOrigin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090'

  const initialEngine: MCPEngineKey =
    selectedProvider === 'claude'
      ? 'claude'
      : selectedProvider === 'agy'
      ? 'agy'
      : selectedProvider === 'codex'
      ? 'codex'
      : 'claude'

  const [activeEngine, setActiveEngine] = useState<MCPEngineKey>(initialEngine)
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  const copyToClipboard = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedKey(id)
      setTimeout(() => setCopiedKey(null), 2500)
    } catch {
      // clipboard access unavailable
    }
  }

  const keyPlaceholder = aiT?.mcpKeyPlaceholder || '<YOUR_WORKSTATION_KEY>'

  const claudeCliCmd = `claude mcp add sectile ${serverOrigin}/mcp --header "Authorization: Bearer ${keyPlaceholder}"`

  const claudeJsonConfig = `{
  "mcpServers": {
    "sectile": {
      "type": "http",
      "url": "${serverOrigin}/mcp",
      "headers": {
        "Authorization": "Bearer ${keyPlaceholder}"
      }
    }
  }
}`

  const agyCliCmd = `agy mcp add --env SECTILE_AGENT_TOKEN="${keyPlaceholder}" sectile sectile-agent mcp --url "${serverOrigin}"`

  const agyCliHttpCmd = `agy mcp add --header "Authorization: Bearer ${keyPlaceholder}" sectile "${serverOrigin}/mcp"`

  const agyJsonConfig = `{
  "mcpServers": {
    "sectile": {
      "command": "sectile-agent",
      "args": ["mcp", "--url", "${serverOrigin}"],
      "env": {
        "SECTILE_AGENT_TOKEN": "${keyPlaceholder}"
      }
    }
  }
}`

  const codexTomlConfig = `[mcp_servers.sectile]
command = "sectile-agent"
args = ["mcp", "--url", "${serverOrigin}"]
env = { SECTILE_AGENT_TOKEN = "${keyPlaceholder}" }`

  const cursorJsonConfig = `{
  "mcpServers": {
    "sectile": {
      "url": "${serverOrigin}/mcp",
      "headers": {
        "Authorization": "Bearer ${keyPlaceholder}"
      }
    }
  }
}`

  return (
    <div className="space-y-3 p-4 rounded-xl bg-[var(--bg-tertiary)]/70 border border-[var(--border-color)]">
      <div className="flex items-start justify-between gap-2">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Network size={15} className="text-indigo-400" />
            <span className="text-xs font-bold text-[var(--text-primary)]">
              {aiT?.mcpConfigWithoutAgent || 'Configuration MCP sans agent local'}
            </span>
          </div>
          <p className="text-[11px] text-[var(--text-secondary)] leading-relaxed">
            {aiT?.mcpConnectDirectlyDesc || "Pour connecter directement votre CLI ou IDE au serveur MCP Sectile sans passer par l'agent local ("}
            <code className="px-1 py-0.5 rounded bg-[var(--bg-secondary)] font-mono text-[10px]">sectile-agent</code>
            {aiT?.mcpConnectDirectlyWorkflow || "). Le moteur accède directement aux outils de gestion des tâches et de suivi du workflow."}
          </p>
        </div>
      </div>

      {/* Info: where to get the workstation API key */}
      <div className="p-2.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border-color)] text-[11px] text-[var(--text-secondary)] flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="text-amber-400 font-bold">{aiT?.mcpKeyRequired || 'Clé requise :'}</span>
          <span>
            {aiT?.mcpKeyRequiredDesc ? (
              <>
                {aiT.mcpKeyRequiredDesc.split('{placeholder}')[0]}
                <code className="text-amber-400 font-mono text-[10px]">{keyPlaceholder}</code>
                {aiT.mcpKeyRequiredDesc.split('{placeholder}')[1]}
              </>
            ) : (
              <>Remplacez <code className="text-amber-400 font-mono text-[10px]">{keyPlaceholder}</code> par une clé d'API.</>
            )}
          </span>
        </div>
        {onNavigateToWorkstations && (
          <button
            type="button"
            onClick={onNavigateToWorkstations}
            className="flex items-center gap-1 text-[10.5px] text-indigo-400 hover:text-indigo-300 font-semibold cursor-pointer shrink-0"
          >
            <span>{aiT?.mcpGenerateKey || 'Générer une clé'}</span>
            <ExternalLink size={11} />
          </button>
        )}
      </div>

      {/* Engine selector buttons */}
      <div className="flex items-center gap-1.5 pt-1 overflow-x-auto">
        {[
          { id: 'claude' as const, label: 'Claude Code', icon: <Claude size={13} className="shrink-0" /> },
          { id: 'agy' as const, label: 'Antigravity (agy)', icon: <Antigravity size={13} className="shrink-0" /> },
          { id: 'codex' as const, label: 'Codex CLI', icon: <OpenAI size={13} className="shrink-0" /> },
          { id: 'cursor' as const, label: 'Cursor / HTTP', icon: <Cursor size={13} className="shrink-0" /> },
        ].map(engine => {
          const isActive = activeEngine === engine.id
          return (
            <button
              key={engine.id}
              type="button"
              onClick={() => setActiveEngine(engine.id)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors cursor-pointer shrink-0 ${
                isActive
                  ? 'accent-bg text-white border-transparent shadow-xs'
                  : 'bg-[var(--bg-secondary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
              }`}
            >
              <span className="shrink-0 flex items-center justify-center">{engine.icon}</span>
              <span>{engine.label}</span>
            </button>
          )
        })}
      </div>

      {/* Claude Code Snippets */}
      {activeEngine === 'claude' && (
        <div className="space-y-2 pt-1">
          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Terminal size={12} className="text-indigo-400" />
                {aiT?.mcpQuickCliCommand ? aiT.mcpQuickCliCommand.replace('{engine}', 'CLI Claude Code') : 'Commande rapide (CLI Claude Code) :'}
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(claudeCliCmd, 'claude-cli')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'claude-cli' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'claude-cli' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap break-all select-text">
              <code>{claudeCliCmd}</code>
            </pre>
          </div>

          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Code2 size={12} className="text-indigo-400" />
                {aiT?.mcpOrInConfig || 'Ou dans'} <code className="font-mono text-[10px]">~/.claude.json</code> :
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(claudeJsonConfig, 'claude-json')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'claude-json' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'claude-json' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap select-text">
              <code>{claudeJsonConfig}</code>
            </pre>
          </div>
        </div>
      )}

      {/* Antigravity Snippets */}
      {activeEngine === 'agy' && (
        <div className="space-y-2 pt-1">
          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Terminal size={12} className="text-indigo-400" />
                {aiT?.mcpQuickCliCommand ? aiT.mcpQuickCliCommand.replace('{engine}', 'agy mcp add') : 'Commande CLI rapide (agy mcp add) :'}
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(agyCliCmd, 'agy-cli')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'agy-cli' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'agy-cli' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap break-all select-text">
              <code>{agyCliCmd}</code>
            </pre>
          </div>

          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Code2 size={12} className="text-indigo-400" />
                {aiT?.mcpOrInConfig || 'Ou dans'} <code className="font-mono text-[10px]">~/.gemini/config/mcp_config.json</code> (passerelle stdio) :
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(agyJsonConfig, 'agy-json')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'agy-json' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'agy-json' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap select-text">
              <code>{agyJsonConfig}</code>
            </pre>
          </div>

          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Network size={12} className="text-indigo-400" />
                {aiT?.mcpOrInConfig ? `${aiT.mcpOrInConfig} HTTP :` : 'Ou en ligne de commande direct HTTP :'}
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(agyCliHttpCmd, 'agy-cli-http')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'agy-cli-http' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'agy-cli-http' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap break-all select-text">
              <code>{agyCliHttpCmd}</code>
            </pre>
          </div>
        </div>
      )}

      {/* Codex CLI Snippets */}
      {activeEngine === 'codex' && (
        <div className="space-y-2 pt-1">
          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Code2 size={12} className="text-indigo-400" />
                {aiT?.mcpOrInConfig ? aiT.mcpOrInConfig.replace('Ou ', '') : 'Dans'} <code className="font-mono text-[10px]">~/.codex/config.toml</code> :
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(codexTomlConfig, 'codex-toml')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'codex-toml' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'codex-toml' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap select-text">
              <code>{codexTomlConfig}</code>
            </pre>
          </div>
        </div>
      )}

      {/* Cursor / HTTP Snippets */}
      {activeEngine === 'cursor' && (
        <div className="space-y-2 pt-1">
          <div className="space-y-1">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="font-semibold text-[var(--text-secondary)] flex items-center gap-1">
                <Code2 size={12} className="text-indigo-400" />
                {aiT?.mcpOrInConfig ? aiT.mcpOrInConfig.replace('Ou ', '') : 'Dans'} <code className="font-mono text-[10px]">.cursor/mcp.json</code> :
              </span>
              <button
                type="button"
                onClick={() => copyToClipboard(cursorJsonConfig, 'cursor-json')}
                className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {copiedKey === 'cursor-json' ? <Check size={11} className="text-emerald-400" /> : <Copy size={11} />}
                <span>{copiedKey === 'cursor-json' ? (aiT?.mcpCopied || 'Copié !') : (aiT?.mcpCopy || 'Copier')}</span>
              </button>
            </div>
            <pre className="p-2.5 rounded-lg bg-[var(--bg-primary)] border border-[var(--border-color)] text-[10px] font-mono text-[var(--text-primary)] whitespace-pre-wrap select-text">
              <code>{cursorJsonConfig}</code>
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}
