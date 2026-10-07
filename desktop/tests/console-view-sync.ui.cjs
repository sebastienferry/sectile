const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The agent keeps a copy of Claude consoles (#711), so that a launch the web
// app started follows it too: Desktop hands it over on connection and on each
// change, only to an agent that announces it, and never fails a save on it.
test('Claude consoles is handed over to an agent that keeps it',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-console-view-ui-'))
 fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({consoleView:'conversation'}))
 let capable=true,refuse=false
 const sent=[]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:capable?['console-view-default']:[]}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/tasks?')){res.end('[]');return}
  if(req.url==='/desktop/runs'){res.end('[]');return}
  if(req.url==='/desktop/console-view'&&req.method==='PUT'){
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const {view}=JSON.parse(body);sent.push(view)
    if(refuse){res.writeHead(500).end('{}');return}
    res.end(JSON.stringify({view}))
   });return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const saved=()=>JSON.parse(fs.readFileSync(path.join(root,'desktop.json'),'utf8')).consoleView
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(8000)
  // The connection hands the saved setting over.
  await expect.poll(()=>sent[0]).toBe('conversation')
  await page.locator('#settings').click()
  await page.getByRole('tab',{name:'General',exact:true}).first().click()
  const views=page.getByRole('group',{name:'AI consoles'})
  const choose=async name=>{
   await views.getByRole('button',{name,exact:true}).click()
   await expect(views.getByRole('button',{name,exact:true})).toHaveAttribute('aria-pressed','true')
  }
  // A change is handed over as soon as it is saved.
  let count=sent.length
  await choose('Terminal')
  await expect.poll(()=>sent.slice(count)).toEqual(['terminal'])
  assert.equal(saved(),'terminal')
  // An agent that refuses it does not stop the save, nor shows an error.
  refuse=true;count=sent.length
  await choose('Conversation')
  await expect.poll(()=>sent.slice(count)).toEqual(['conversation'])
  assert.equal(saved(),'conversation')
  await expect(page.locator('[data-tone="error"]')).toHaveCount(0)
  // An agent that predates the copy is sent nothing; the setting still saves.
  refuse=false;capable=false;count=sent.length
  await choose('Terminal')
  assert.equal(saved(),'terminal')
  await page.waitForTimeout(300)
  assert.deepEqual(sent.slice(count),[])
  // Every save went to Desktop's own file: the agent's settings.json is left as it was (#746).
  assert.equal(fs.readFileSync(path.join(root,'settings.json'),'utf8'),JSON.stringify({consoleView:'conversation'}))
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve))}
})
