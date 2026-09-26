const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('a task row is renamed in place through its pencil',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-rename-'))
 const base={projectId:'project',directory:'/tmp/repo',status:'completed'}
 const runs=[
  {...base,id:'run-42',taskId:'task-42',taskKey:'#42',skill:'implement',createdAt:'2026-09-25T06:00:03Z'},
  {...base,id:'run-macro',taskId:'',macroKey:'M-7',taskKey:'M-7',skill:'refine',createdAt:'2026-09-25T06:00:02Z'},
  {...base,id:'console',kind:'console',provider:'claude',taskId:'',skill:'',sessionId:'console',createdAt:'2026-09-25T06:00:01Z'},
 ]
 let taskRequests=0
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'https://example.test/sectile/',capabilities:['free-console']}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/repo'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/tasks'){taskRequests++;res.end(JSON.stringify([{id:'task-42',key:'#42',title:'Tracker title',labels:[]}]));return}
  if(url.pathname==='/desktop/run-result'){res.end(JSON.stringify({activity:null}));return}
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}),{mode:0o600})
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(10000)
  const rows=page.locator('#runs .local-task')
  const row=id=>rows.filter({has:page.locator(`.run-state[data-run-id="${id}"]`)})
  const field=page.getByRole('textbox',{name:'Local task name',exact:true})
  const localNames=()=>page.evaluate(()=>Object.values(JSON.parse(localStorage.getItem('localTasks')||'{}')).map(task=>task.name).filter(Boolean).sort())
  const rename=async(id,name)=>{await row(id).hover();await page.getByRole('button',{name:'Rename '+name,exact:true}).click();await expect(field).toBeFocused()}
  await expect(rows).toHaveCount(3)
  await expect(row('run-42').locator('strong')).toHaveText('Tracker title')

  // The "…" menu is gone; every kind of row carries a pencil, shown on hover like the archive button.
  assert.equal(await page.locator('#runs .task-menu').count(),0)
  assert.equal(await rows.getByRole('button',{name:/^Actions for /}).count(),0)
  await page.locator('#terminal').hover()
  for(const [id,name] of [['run-42','Tracker title'],['run-macro','refine'],['console','Claude console']]){
   const pencil=row(id).getByRole('button',{name:'Rename '+name,exact:true})
   await expect(pencil).toHaveAttribute('title','Rename '+name)
   assert.equal(await pencil.evaluate(el=>getComputedStyle(el).opacity),'0')
   const order=await row(id).evaluate(el=>[...el.children].map(child=>child.className.split(' ')[0]).slice(-2))
   assert.deepEqual(order,['task-archive','task-rename-button'],id+': the pencil follows the archive button')
  }
  await row('console').hover()
  await expect.poll(()=>row('console').locator('.task-rename-button').evaluate(el=>getComputedStyle(el).opacity)).toBe('1')

  // The pencil opens a field holding the whole displayed name, selected; Enter saves.
  await rename('run-42','Tracker title')
  await expect(field).toHaveValue('Tracker title')
  assert.deepEqual(await field.evaluate(el=>[el.selectionStart,el.selectionEnd,el.maxLength]),[0,'Tracker title'.length,120])
  await page.keyboard.type('Renamed task')
  await page.keyboard.press('Enter')
  await expect(field).toHaveCount(0)
  await expect(row('run-42').locator('strong')).toHaveText('Renamed task')
  await expect(page.locator('#title')).toHaveText('#42 · Renamed task · implement')
  await expect(page.getByRole('button',{name:'Rename Renamed task',exact:true})).toBeFocused()
  assert.deepEqual(await localNames(),['Renamed task'])

  // Leaving the field saves too.
  await rename('run-42','Renamed task')
  await field.fill('Blurred name')
  await page.locator('#terminal').click()
  await expect(field).toHaveCount(0)
  await expect(row('run-42').locator('strong')).toHaveText('Blurred name')
  assert.deepEqual(await localNames(),['Blurred name'])

  // Escape cancels, and is not also taken by the ticket pane.
  await page.getByRole('button',{name:'Actions for Project',exact:true}).click();await page.getByRole('menuitem',{name:'Open tasks',exact:true}).click()
  await expect(page.locator('#tickets-pane')).toBeVisible()
  await rename('run-42','Blurred name')
  await page.keyboard.type('Ignored')
  await page.keyboard.press('Escape')
  await expect(field).toHaveCount(0)
  await expect(row('run-42').locator('strong')).toHaveText('Blurred name')
  await expect(page.locator('#tickets-pane')).toBeVisible()
  await expect(page.getByRole('button',{name:'Rename Blurred name',exact:true})).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.locator('#tickets-pane')).toBeHidden()

  // A blank or unchanged value saves nothing, and the row keeps following its default name.
  await rename('run-42','Blurred name')
  await field.fill('   ')
  await page.keyboard.press('Enter')
  await expect(row('run-42').locator('strong')).toHaveText('Blurred name')
  await rename('console','Claude console')
  await page.keyboard.press('Enter')
  await expect(row('console').locator('strong')).toHaveText('Claude console')
  assert.deepEqual(await localNames(),['Blurred name'])
  await expect(row('console')).not.toHaveClass(/selected/)

  // Pressing another row's pencil saves the open field and edits the other row.
  await rename('console','Claude console')
  await field.fill('My console')
  await row('run-macro').hover()
  await page.getByRole('button',{name:'Rename refine',exact:true}).click()
  await expect(row('console').locator('strong')).toHaveText('My console')
  await expect(row('run-macro').getByRole('textbox',{name:'Local task name',exact:true})).toBeFocused()
  await expect(row('console')).not.toHaveClass(/selected/)
  await expect(row('run-macro')).not.toHaveClass(/selected/)

  // A sidebar rebuild keeps the open field, its focus and what was typed.
  await field.fill('Half typ')
  const before=taskRequests
  await page.evaluate(()=>{window.renameTestTime=Date.now()+16000;Date.now=()=>window.renameTestTime})
  await field.evaluate(el=>{el.dataset.previous='1'})
  await expect.poll(()=>taskRequests).toBeGreaterThan(before)
  // The field is a new element: the rebuild did happen under it.
  await expect(page.locator('.task-rename[data-previous]')).toHaveCount(0)
  await expect(field).toBeFocused()
  await expect(field).toHaveValue('Half typ')
  await page.keyboard.type('ed')
  await page.keyboard.press('Enter')
  await expect(row('run-macro').locator('strong')).toHaveText('Half typed')

  // The field takes 120 characters at most.
  await rename('run-macro','Half typed')
  await page.keyboard.insertText('x'.repeat(130))
  await expect(field).toHaveValue('x'.repeat(120))
  await page.keyboard.press('Enter')
  await expect(row('run-macro').locator('strong')).toHaveText('x'.repeat(120))
  assert.deepEqual(await localNames(),['Blurred name','My console','x'.repeat(120)])
 }finally{
  await app?.close()
  await new Promise(resolve=>server.close(resolve))
  fs.rmSync(root,{recursive:true,force:true})
 }
})
