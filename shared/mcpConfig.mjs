export const mcpProviders = {
 claude: {label:'Claude', path:'~/.claude.json'},
 agy: {label:'Antigravity', path:'~/.gemini/config/mcp_config.json'},
 codex: {label:'Codex', path:'~/.codex/config.toml'},
}

const token = '<SECTILE_API_KEY>'
const trimServer = server => server.replace(/\/+$/, '')

// JSON string escaping is also valid for these TOML basic strings.
export function mcpSnippet(provider, transport, server, local = false) {
 const quote = JSON.stringify
 const base = trimServer(server)
 let entry
 if (transport === 'stdio') {
  entry = {command:'sectile-agent', args:['mcp','--url',base], env:{SECTILE_AGENT_TOKEN:local?'':token}}
 } else {
  const urlField = provider === 'agy' ? 'serverUrl' : 'url'
  entry = {[urlField]:base+'/mcp'}
  if (provider === 'claude') entry.type = 'http'
  if (!local) entry[provider === 'codex' ? 'http_headers' : 'headers'] = {Authorization:'Bearer '+token}
 }
 if (provider === 'codex') {
  const lines = ['[mcp_servers.sectile]', 'enabled = true']
  for (const [key,value] of Object.entries(entry)) {
   if (key === 'http_headers') {
    lines.push('', '[mcp_servers.sectile.http_headers]', 'Authorization = '+quote(value.Authorization))
    continue
   }
   const encoded = Array.isArray(value) ? '['+value.map(quote).join(', ')+']'
    : typeof value === 'object' ? '{ '+Object.entries(value).map(([k,v])=>quote(k)+' = '+quote(v)).join(', ')+' }'
    : quote(value)
   lines.push(key+' = '+encoded)
  }
  return lines.join('\n')
 }
 return JSON.stringify({mcpServers:{sectile:entry}},null,2)
}

// POSIX single quotes, left out when the value holds only characters no shell interprets.
export function shellQuote(value) {
 return /^[A-Za-z0-9_./:=@%+-]+$/.test(value) ? value : "'"+value.replaceAll("'", "'\\''")+"'"
}

// The CLI line registering the same entry as mcpSnippet, or null when the provider has no CLI for it.
// --env and --header are variadic in `claude mcp add`: the name goes before --env and --header after the URL.
export function mcpCommand(provider, transport, server, local = false) {
 if (provider !== 'claude' && provider !== 'codex') return null
 const base = trimServer(server)
 const stdio = ['--env', 'SECTILE_AGENT_TOKEN='+(local ? '' : token), '--', 'sectile-agent', 'mcp', '--url', base]
 let lines
 if (provider === 'claude') {
  const add = transport === 'stdio' ? ['claude', 'mcp', 'add', '--scope', 'user', 'sectile', ...stdio]
   : ['claude', 'mcp', 'add', '--transport', 'http', '--scope', 'user', 'sectile', base+'/mcp', ...(local ? [] : ['--header', 'Authorization: Bearer '+token])]
  // `claude mcp add` refuses an existing name, so the entry is removed first.
  lines = [['claude', 'mcp', 'remove', '--scope', 'user', 'sectile'], add]
 } else {
  lines = [transport === 'stdio' ? ['codex', 'mcp', 'add', 'sectile', ...stdio]
   : ['codex', 'mcp', 'add', 'sectile', '--url', base+'/mcp', ...(local ? [] : ['--bearer-token-env-var', 'SECTILE_API_KEY'])]]
 }
 return lines.map(args => args.map(shellQuote).join(' ')).join('\n')
}
