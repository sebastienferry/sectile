const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

function listen(handler){
 const server=http.createServer(handler)
 return new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve(server)))
}
const address=server=>'http://127.0.0.1:'+server.address().port

// A paired desktop opens the server's board in a window of its own, signed in
// by the key it holds, and never hands that key to anybody else.
test('the board opens inside the desktop, signed with the workstation key',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-board-window-'))
 const seen=[],elsewhere=[]
 // Another origin the board page reaches: it must never see the key.
 const other=await listen((req,res)=>{elsewhere.push({url:req.url,authorization:req.headers.authorization||''});res.writeHead(204).end()})
 const board=await listen((req,res)=>{
  seen.push({url:req.url,authorization:req.headers.authorization||'',cookie:req.headers.cookie||''})
  if(req.url==='/api/events'){res.writeHead(200,{'Content-Type':'text/event-stream'});res.write('data: {}\n\n');return}
  if(req.url==='/api/me'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({id:'usr_1'}));return}
  if(req.url.startsWith('/auth/')){res.writeHead(200).end('{}');return}
  res.setHeader('Content-Type','text/html')
  res.end(`<!doctype html><title>Board</title><a id="pr" href="https://github.com/o/r/pull/1" target="_blank">PR</a>
<script>
 fetch('/api/me').then(r=>r.json()).then(me=>{document.title='Board '+me.id})
 new EventSource('/api/events')
 fetch('${address(other)}/probe',{mode:'no-cors'}).catch(()=>{})
 fetch('/auth/logout',{method:'POST'}).then(()=>{document.body.dataset.logout='sent'},()=>{document.body.dataset.logout='refused'})
</script>`)
 })
 const status={connected:true,server:address(board)+'/'}
 const agent=await listen((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify(status));return}
  if(['/desktop/runs','/desktop/projects'].includes(req.url)){res.end('[]');return}
  res.writeHead(404).end('{}')
 })
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:address(agent),token:'agent-secret'}))
 // No OS key store in the test run: the key is kept in clear, as on such hosts.
 fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({server:address(board),apiKey:'sectile_workstation_key'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const link=page.locator('#connection a')
  await expect(link).toHaveText('Connected')
  await expect(page.locator('#connection')).toHaveAttribute('title','Open the board of '+status.server)
  await link.click()

  // The board is a second window of the app, and no browser was launched.
  await expect.poll(()=>app.windows().length).toBe(2)
  const boardPage=app.windows().find(w=>w!==page)
  await expect(boardPage).toHaveTitle('Board usr_1')
  await expect(boardPage.locator('body')).toHaveAttribute('data-logout','refused')
  // The page has none of the desktop's bridge.
  assert.equal(await boardPage.evaluate(()=>typeof window.localAgent),'undefined')

  const signed=url=>seen.filter(r=>r.url===url).every(r=>r.authorization==='Bearer sectile_workstation_key')&&seen.some(r=>r.url===url)
  await expect.poll(()=>signed('/')&&signed('/api/me')&&signed('/api/events')).toBe(true)
  assert.equal(seen.some(r=>r.url.startsWith('/auth/')),false,'sign-in and sign-out never reach the server')
  await expect.poll(()=>elsewhere.length).toBe(1)
  assert.equal(elsewhere[0].authorization,'','another origin never receives the key')

  // A link leaving the server goes to the browser.
  await boardPage.locator('#pr').click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['https://github.com/o/r/pull/1'])
  assert.equal(new URL(boardPage.url()).origin,address(board))

  // Opening the board again reuses the window; opening a task loads it there.
  await link.click()
  await page.waitForTimeout(300)
  assert.equal(app.windows().length,2)
  await page.evaluate(()=>window.localAgent.openTask('#7'))
  await expect.poll(()=>boardPage.url()).toBe(address(board)+'/?task=%237')
  assert.equal(app.windows().length,2)
  assert.deepEqual(await app.evaluate(()=>globalThis.opened),['https://github.com/o/r/pull/1'])

  // A key issued by another server is not sent to this one: the browser opens instead.
  fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({server:'https://elsewhere.example.test',apiKey:'sectile_workstation_key'}))
  await link.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened.length)).toBe(2)
  assert.equal((await app.evaluate(()=>globalThis.opened))[1],address(board)+'/')
 }finally{
  if(app)await app.close()
  for(const server of [agent,board,other]){server.closeAllConnections?.();await new Promise(resolve=>server.close(resolve))}
 }
})
