import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mcpCommand, mcpProviders, mcpSnippet, shellQuote} from '../../shared/mcpConfig.mjs'

test('JSON examples use provider-specific HTTP fields and escape URLs',()=>{
 for(const [provider,field] of [['claude','url'],['agy','serverUrl']]) {
  const remote=JSON.parse(mcpSnippet(provider,'http','https://example.test/base/')).mcpServers.sectile
  assert.equal(remote[field],'https://example.test/base/mcp')
  assert.equal(remote.headers.Authorization,'Bearer <SECTILE_API_KEY>')
  const local=JSON.parse(mcpSnippet(provider,'http','http://127.0.0.1:4567',true)).mcpServers.sectile
  assert.equal(local.headers,undefined)
  const stdio=JSON.parse(mcpSnippet(provider,'stdio','https://example.test/"quoted')).mcpServers.sectile
  assert.deepEqual(stdio.args,['mcp','--url','https://example.test/"quoted'])
  assert.equal(stdio.env.SECTILE_AGENT_TOKEN,'<SECTILE_API_KEY>')
 }
})

test('TOML examples distinguish HTTP and STDIO and clear inherited local keys',()=>{
 assert.match(mcpSnippet('codex','http','https://example.test'),/\[mcp_servers.sectile.http_headers\]\nAuthorization =/)
 const local=mcpSnippet('codex','stdio','http://127.0.0.1:4567',true)
 assert.match(local,/"SECTILE_AGENT_TOKEN" = ""/)
 assert.doesNotMatch(local,/<SECTILE_API_KEY>|http_headers/)
})

test('only the supported providers have a configuration file (#614)',()=>{
 assert.deepEqual(Object.keys(mcpProviders).sort(),['agy','claude','codex'])
})

test('Codex remote HTTP example separates headers and explicitly enables the server',()=>{
 assert.equal(mcpSnippet('codex','http','http://localhost:8090'), `[mcp_servers.sectile]
enabled = true
url = "http://localhost:8090/mcp"

[mcp_servers.sectile.http_headers]
Authorization = "Bearer <SECTILE_API_KEY>"`)
 assert.doesNotMatch(mcpSnippet('codex','http','http://127.0.0.1:8091',true), /http_headers|Authorization/)
})

test('Claude commands remove then add the entry of each mode, in the order the CLI parses (#667)',()=>{
 assert.equal(mcpCommand('claude','http','https://sectile.example/'), `claude mcp remove --scope user sectile
claude mcp add --transport http --scope user sectile https://sectile.example/mcp --header 'Authorization: Bearer <SECTILE_API_KEY>'`)
 assert.equal(mcpCommand('claude','http','http://127.0.0.1:8091',true), `claude mcp remove --scope user sectile
claude mcp add --transport http --scope user sectile http://127.0.0.1:8091/mcp`)
 assert.equal(mcpCommand('claude','stdio','https://sectile.example/'), `claude mcp remove --scope user sectile
claude mcp add --scope user sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example`)
 for(const [transport,local] of [['http',false],['http',true],['stdio',false],['stdio',true]]) {
  const lines=mcpCommand('claude',transport,'https://sectile.example',local).split('\n')
  assert.equal(lines.length,2)
  assert.match(lines[0],/^claude mcp remove /)
  assert.doesNotMatch(lines.join('\n'),/&&/)
 }
 const stdio=mcpCommand('claude','stdio','https://sectile.example').split('\n')[1]
 assert.ok(stdio.indexOf(' sectile ')<stdio.indexOf('--env'))
 const http=mcpCommand('claude','http','https://sectile.example').split('\n')[1]
 assert.ok(http.indexOf('https://sectile.example/mcp')<http.indexOf('--header'))
})

test('Codex commands are one add line that reads the key from the environment (#667)',()=>{
 assert.equal(mcpCommand('codex','http','https://sectile.example/'),'codex mcp add sectile --url https://sectile.example/mcp --bearer-token-env-var SECTILE_API_KEY')
 assert.equal(mcpCommand('codex','http','http://127.0.0.1:8091',true),'codex mcp add sectile --url http://127.0.0.1:8091/mcp')
 assert.equal(mcpCommand('codex','stdio','https://sectile.example/'),"codex mcp add sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example")
 for(const transport of ['http','stdio']) assert.doesNotMatch(mcpCommand('codex',transport,'https://sectile.example'),/\n|remove/)
})

test('local commands carry no key (#667)',()=>{
 for(const provider of ['claude','codex']) {
  assert.match(mcpCommand(provider,'stdio','http://127.0.0.1:8091',true),/ --env SECTILE_AGENT_TOKEN= -- sectile-agent mcp --url http:\/\/127\.0\.0\.1:8091$/)
  assert.doesNotMatch(mcpCommand(provider,'http','http://127.0.0.1:8091',true),/--header|--bearer-token-env-var|SECTILE_API_KEY/)
 }
})

test('command arguments are quoted only when a shell would interpret them (#667)',()=>{
 assert.equal(shellQuote('https://sectile.example/mcp'),'https://sectile.example/mcp')
 assert.equal(shellQuote("https://ex.test/a b$c'd"),"'https://ex.test/a b$c'\\''d'")
 assert.equal(shellQuote(''),"''")
 assert.equal(shellQuote('SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>'),"'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>'")
 assert.equal(mcpCommand('codex','stdio',"https://ex.test/a b$c'd"),"codex mcp add sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url 'https://ex.test/a b$c'\\''d'")
})

test('providers without a CLI get no command (#667)',()=>{
 for(const provider of ['agy','custom','']) for(const transport of ['http','stdio']) assert.equal(mcpCommand(provider,transport,'https://sectile.example'),null)
})
