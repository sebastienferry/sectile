const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// A server too old to serve the agent contract is reachable, so reporting it as
// a disconnection sends the user looking at the network. The agent already knows
// the difference; the header has to pass it on.
test('an incompatible server is named, not reported as a disconnection',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-contract-'))
 const diagnosis='agent contract v1 is not served by http://example.test: /api/v1/agent/projects answered HTTP 404. The server runs a build older than this agent; update and restart sectile-server.'
 let status={connected:false,server:'http://example.test',contractError:diagnosis}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify(status));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const connection=page.locator('#connection')
  await expect(connection).toHaveText('Server incompatible')
  await expect(connection).toHaveAttribute('title',diagnosis)

  // An unreachable server keeps its own wording: only a contract failure earns
  // the incompatible label.
  status={connected:false,server:'http://example.test'}
  await expect(connection).toHaveText('Not connected')
  assert.equal(await connection.getAttribute('title'),'The local agent does not reach the Sectile server.')

  // Updating the server clears it, and the board link comes back.
  status={connected:true,server:'http://example.test',contractError:''}
  await expect(page.locator('#connection a')).toHaveText('Connected')
  assert.equal(await connection.getAttribute('title'),'Open '+status.server+' in the default browser')
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
