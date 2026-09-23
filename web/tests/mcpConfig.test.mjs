import {test} from 'node:test'
import assert from 'node:assert/strict'
import {mcpSnippet} from '../../shared/mcpConfig.mjs'

test('JSON examples use provider-specific HTTP fields and escape URLs',()=>{
 for(const [provider,field] of [['claude','url'],['cursor','url'],['agy','serverUrl'],['gemini','httpUrl']]) {
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
 assert.match(mcpSnippet('vibe','http','https://example.test'),/transport = "streamable-http"/)
 for(const provider of ['codex','vibe']) {
  const local=mcpSnippet(provider,'stdio','http://127.0.0.1:4567',true)
  assert.match(local,/"SECTILE_AGENT_TOKEN" = ""/)
  assert.doesNotMatch(local,/<SECTILE_API_KEY>|http_headers/)
 }
})

test('Codex remote HTTP example separates headers and explicitly enables the server',()=>{
 assert.equal(mcpSnippet('codex','http','http://localhost:8090'), `[mcp_servers.sectile]
enabled = true
url = "http://localhost:8090/mcp"

[mcp_servers.sectile.http_headers]
Authorization = "Bearer <SECTILE_API_KEY>"`)
 assert.doesNotMatch(mcpSnippet('codex','http','http://127.0.0.1:8091',true), /http_headers|Authorization/)
})
