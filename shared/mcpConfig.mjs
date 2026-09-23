export const mcpProviders = {
 claude: {label:'Claude', path:'~/.claude.json'},
 agy: {label:'Antigravity', path:'~/.gemini/config/mcp_config.json'},
 codex: {label:'Codex', path:'~/.codex/config.toml'},
 cursor: {label:'Cursor', path:'~/.cursor/mcp.json'},
 gemini: {label:'Gemini', path:'~/.gemini/settings.json'},
 vibe: {label:'Mistral Vibe', path:'~/.vibe/config.toml'},
}

// JSON string escaping is also valid for these TOML basic strings.
export function mcpSnippet(provider, transport, server, local = false) {
 const quote = JSON.stringify
 const base = server.replace(/\/+$/, '')
 const token = '<SECTILE_API_KEY>'
 let entry
 if (transport === 'stdio') {
  entry = {command:'sectile-agent', args:['mcp','--url',base], env:{SECTILE_AGENT_TOKEN:local?'':token}}
 } else {
  const urlField = provider === 'agy' ? 'serverUrl' : provider === 'gemini' ? 'httpUrl' : 'url'
  entry = {[urlField]:base+'/mcp'}
  if (provider === 'claude') entry.type = 'http'
  if (!local) entry[provider === 'codex' ? 'http_headers' : 'headers'] = {Authorization:'Bearer '+token}
 }
 if (provider === 'codex' || provider === 'vibe') {
  const lines = provider === 'vibe'
   ? ['[[mcp_servers]]','name = "sectile"',`transport = ${quote(transport === 'http' ? 'streamable-http' : 'stdio')}`]
   : ['[mcp_servers.sectile]']
  for (const [key,value] of Object.entries(entry)) {
   const encoded = Array.isArray(value) ? '['+value.map(quote).join(', ')+']'
    : typeof value === 'object' ? '{ '+Object.entries(value).map(([k,v])=>quote(k)+' = '+quote(v)).join(', ')+' }'
    : quote(value)
   lines.push(key+' = '+encoded)
  }
  return lines.join('\n')
 }
 return JSON.stringify({mcpServers:{sectile:entry}},null,2)
}
