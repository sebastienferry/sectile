import { mcpProviders, mcpSnippet } from '../../shared/mcpConfig.mjs'

export function mcpSettings(api, providerSelect) {
 const section = document.createElement('section')
 section.className = 'mcp-settings'
 const heading = document.createElement('h3'); heading.textContent = 'MCP connection'
 const target = document.createElement('select'); target.setAttribute('aria-label', 'MCP connection target')
 for (const [value, text] of [['remote','Remote HTTP server · API key'],['local','Local proxy · no client authentication']]) {
  const option = document.createElement('option'); option.value = value; option.textContent = text; target.append(option)
 }
 const explanation = document.createElement('p'); explanation.className = 'hint'
 const path = document.createElement('p'); path.className = 'hint'
 const cards = document.createElement('div'); cards.className = 'mcp-options'; cards.setAttribute('role', 'radiogroup'); cards.setAttribute('aria-label', 'MCP transport')
 const apply = document.createElement('button'); apply.type = 'button'; apply.textContent = 'Update provider configuration'
 const notice = document.createElement('p'); notice.setAttribute('role', 'status')
 section.append(heading,target,explanation,path,cards,apply,notice)
 let info, transport = 'http', revision = 0, busy = false
 const render = () => {
  cards.replaceChildren()
  const provider = providerSelect.value
  apply.disabled = busy || !info || !mcpProviders[provider]
  target.disabled = busy || !info
  if (!info) return
  path.textContent = info.path
  const local = target.value === 'local'
  explanation.textContent = local
   ? 'The running local agent forwards MCP calls using its paired identity. No API key is stored in the provider configuration. Any native process on this workstation can use this proxy while a provider selects this mode. The desktop agent must stay running.'
   : 'Connect to the paired Sectile server. The pairing API key is written to the provider’s user configuration. This connection works while the desktop agent is stopped.'
  for (const mode of ['http','stdio']) {
   const card = document.createElement('div'); card.className = 'mcp-option'
   const label = document.createElement('label')
   const radio = document.createElement('input'); radio.type = 'radio'; radio.name = 'mcp-transport'; radio.value = mode; radio.checked = transport === mode; radio.disabled = busy
   radio.onchange = () => {transport = mode; notice.textContent = ''}
   label.append(radio, mode === 'http' ? ' Streamable HTTP' : ' STDIO')
   const help = document.createElement('p'); help.className = 'hint'
   help.textContent = mode === 'http' ? 'The AI engine calls the MCP endpoint directly over HTTP.' : 'The AI engine starts the bundled sectile-agent bridge and exchanges MCP over stdin/stdout. The bridge forwards to the selected endpoint.'
   const preview = document.createElement('pre'); preview.textContent = mcpSnippet(provider, mode, local ? info.localURL : info.server, local)
   const copy = document.createElement('button'); copy.type = 'button'; copy.textContent = 'Copy '+mode.toUpperCase()+' example'; copy.disabled = busy
   copy.onclick = async () => {try {await api.copyText(preview.textContent); notice.textContent = 'Example copied. Replace the key placeholder for remote connections and use the installed binary path for STDIO.'} catch (e) {notice.textContent = e.message}}
   card.append(label,help,preview,copy); cards.append(card)
  }
 }
 const load = async () => {
  const current = ++revision
  info = null; path.textContent = ''; explanation.textContent = ''; render(); notice.textContent = ''
  if (!mcpProviders[providerSelect.value]) {path.textContent = ''; explanation.textContent = 'Configure MCP manually in your custom provider. Choose a supported provider to preview and update its configuration.'; return}
  notice.textContent = 'Loading MCP configuration…'
  try {
   const result = await api.mcpConfig(providerSelect.value)
   if (current !== revision || !section.isConnected) return
   info = result; target.value = info.choice.target; transport = info.choice.transport
   notice.textContent = ''; render()
  } catch (e) {if (current === revision) notice.textContent = 'Connect to an up-to-date local agent to configure MCP. '+e.message}
 }
 target.onchange = () => {notice.textContent = ''; render()}
 apply.onclick = async () => {
  const provider = providerSelect.value
  busy = true; providerSelect.disabled = true; render(); notice.textContent = 'Updating configuration…'
  try {
   const result = await api.configureMCP(provider,{target:target.value,transport})
   notice.textContent = 'Updated '+result.path+'. Restart the AI engine to reconnect. Other servers and tool permissions are preserved.'
  } catch (e) {notice.textContent = 'Configuration update failed: '+e.message}
  finally {busy = false; providerSelect.disabled = false; render()}
 }
 providerSelect.addEventListener('change',load)
 return {section, load}
}
