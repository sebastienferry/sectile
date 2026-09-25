const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('desktop sidebar rows put the run states and the titles in columns',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-sidebar-alignment-'))
 const base={projectId:'project',skill:'implement',directory:'/tmp/repo',status:'running'}
 const runs=[
  {...base,id:'run-42',taskId:'task-42',taskKey:'#42',createdAt:'2026-09-25T06:00:05Z'},
  {...base,id:'run-446',taskId:'task-446',taskKey:'#446',status:'completed',createdAt:'2026-09-25T06:00:04Z'},
  {...base,id:'run-macro',taskId:'',macroKey:'M-7',taskKey:'M-7',status:'failed',createdAt:'2026-09-25T06:00:03Z'},
  {...base,id:'console',kind:'console',provider:'claude',taskId:'',skill:'',sessionId:'console',createdAt:'2026-09-25T06:00:02Z'},
  {...base,id:'run-long',taskId:'task-long',taskKey:'#123456',status:'queued',createdAt:'2026-09-25T06:00:01Z'},
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test/sectile/',capabilities:['free-console']}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/repo'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){res.end('[]');return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const rows=page.locator('#runs .local-task')
  await expect(rows).toHaveCount(runs.length)

  const layout=await rows.evaluateAll(elements=>elements.map(row=>{
   const state=row.querySelector('.run-state'),slot=row.querySelector('.task-key-slot'),key=row.querySelector('.task-number')
   return {
    runId:state?.dataset.runId,
    first:row.firstElementChild?.className,
    order:[...row.children].map(child=>child.className.split(' ')[0]),
    stateLeft:state?.getBoundingClientRect().left,
    titleLeft:row.querySelector('strong').getBoundingClientRect().left,
    slotWidth:slot?.getBoundingClientRect().width,
    key:key?.textContent,
    keyFits:key?key.scrollWidth<=key.clientWidth:null,
   }
  }))
  const byRun=Object.fromEntries(layout.map(item=>[item.runId,item]))
  assert.deepEqual(Object.keys(byRun).sort(),runs.map(run=>run.id).sort())
  for(const item of layout){
   assert.equal(item.first,'run-state',`${item.runId}: the glyph is the first element of the row`)
   assert.deepEqual(item.order.slice(0,3),['run-state','task-key-slot','run'],`${item.runId}: glyph, key, then title`)
   assert.equal(item.stateLeft,layout[0].stateLeft,`${item.runId}: glyphs share one column`)
  }
  const aligned=['run-42','run-446','run-macro','console']
  for(const id of aligned)assert.equal(byRun[id].titleLeft,byRun['run-42'].titleLeft,`${id}: titles share one column`)
  for(const id of aligned)assert.equal(byRun[id].slotWidth,byRun['run-42'].slotWidth,`${id}: the key column has one width`)

  // A long key is shown in full and only pushes its own title.
  assert.equal(byRun['run-long'].key,'#123456')
  assert.equal(byRun['run-long'].keyFits,true,'The long key is not truncated')
  assert.ok(byRun['run-long'].titleLeft>byRun['run-42'].titleLeft,'The long key pushes its own title to the right')

  // The free console reserves the width but shows no key control.
  assert.equal(byRun.console.key,undefined)
  assert.equal(await rows.filter({has:page.locator('.run-state[data-run-id="console"]')}).locator('.task-number').count(),0)

  // The key keeps its own behaviour: it opens the task without selecting the run.
  // The first row is selected on launch, so the clicks target #446.
  const row=id=>rows.filter({has:page.locator(`.run-state[data-run-id="${id}"]`)})
  const key=row('run-446').locator('.task-number')
  await expect(key).toHaveAttribute('title','Open task in Sectile')
  await expect(key).toHaveAttribute('aria-label','Open #446 in Sectile')
  await expect(row('run-macro').locator('.task-number')).toBeDisabled()
  await expect(row('run-446')).not.toHaveClass(/selected/)
  await key.click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['https://example.test/sectile/?task=task-446'])
  await expect(row('run-446')).not.toHaveClass(/selected/)

  // The glyph, out of the row button, still selects the run.
  const glyph=row('run-446').locator('.run-state')
  await expect(glyph).toHaveAttribute('title',/^Process: /)
  await glyph.click()
  await expect(row('run-446')).toHaveClass(/selected/)
  await expect(row('run-42')).not.toHaveClass(/selected/)
 }finally{
  await app?.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
