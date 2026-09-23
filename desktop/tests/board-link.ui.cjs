const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('header opens the current board safely and survives connection updates',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-board-link-'))
 let status={connected:true,server:'https://example.test/sectile/?task=123&view=board#task'},offline=false,statusReads=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(offline){res.writeHead(503).end('{}');return}
  if(req.url==='/desktop/status'){statusReads++;res.end(JSON.stringify(status));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  res.writeHead(404).end('{}')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const link=page.locator('#connection a'),destination='https://example.test/sectile/?view=board'
  // The row states the state; the address it would open rides in the tooltip.
  await expect(link).toHaveText('Connected')
  await expect(page.locator('#connection')).toHaveAttribute('title','Open the board of '+status.server)
  await expect(page.locator('#connection')).toHaveAttribute('data-state','on')
  await expect(link).toHaveAttribute('href',destination)
  const desktopURL=page.url()
  await link.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([destination])
  await link.focus()
  const reads=statusReads
  await expect.poll(()=>statusReads).toBeGreaterThan(reads)
  assert.equal(await link.evaluate(el=>el===document.activeElement),true)
  await page.keyboard.press('Enter')
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([destination,destination])
  assert.equal(page.url(),desktopURL)
  status.server='http://example.test:8090/other/'
  await expect(page.locator('#connection')).toHaveAttribute('title','Open the board of '+status.server)
  await link.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened.length)).toBe(3)
  assert.equal((await app.evaluate(()=>globalThis.opened))[2],status.server)
  await app.evaluate(({shell})=>{shell.openExternal=async()=>{throw Error('Browser unavailable')}})
  await link.click()
  await expect(page.locator('#error')).toContainText('Browser unavailable')
  for(const serverURL of ['file:///tmp/board','javascript:alert(1)','https://user:secret@example.test/','invalid']){
   status.server=serverURL
   const message=await page.evaluate(()=>window.localAgent.openBoard().then(()=>'',err=>err.message))
   assert.match(message,/Invalid (server )?URL/)
   await expect(link).toHaveCount(0)
  }
  status={connected:false,server:'http://example.test'}
  await expect(page.locator('#connection')).toHaveText('Not connected')
  await expect(page.locator('#connection')).toHaveAttribute('data-state','off')
  assert.match(await page.evaluate(()=>window.localAgent.openBoard().catch(err=>err.message)),/Server disconnected/)
  status.connected=true
  await expect(link).toHaveCount(1)
  offline=true
  await expect(page.locator('#connection')).toHaveText('Not connected')
  await expect(link).toHaveCount(0)
  assert.equal(page.url(),desktopURL)
  assert.equal((await app.evaluate(()=>globalThis.opened)).length,3)
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
