const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('the tickets pane lists, sorts and launches a project\'s open tasks',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-open-tasks-'))
 const requests=[],launches=[],pending=[]
 let failRead=false,failLaunch=false,configured=true,empty=false,offline=false,runs=[]
 const tasks=[
  {id:'a1',key:'#1',title:'First open task',status:'to_clarify',priority:'medium'},
  {id:'a2',key:'#2',title:'Second open task',status:'to_implement',priority:'urgent',prUrl:'https://github.com/acme/repo/pull/42'},
  {id:'a3',key:'#3',title:'Finished by status',status:'finished'},
  {id:'a4',key:'#4',title:'Finished by label',labels:['#finished']},
  {id:'a5',key:'#5',title:'Done task',status:'done'},
  {id:'a9',key:'#9',title:'Ninth open task',status:'to_clarify',priority:'medium'},
  {id:'a100',key:'#100',title:'Hundredth open task',status:'to_clarify',priority:'medium'}
 ]
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  if(offline){res.writeHead(503).end('{}');return}
  const url=new URL(req.url,'http://localhost'),project=url.searchParams.get('projectId')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test'}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify(['A','B'].map(name=>({id:'project-'+name.toLowerCase(),name:'Project '+name,path:'/tmp/'+name}))));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured,server:{skills:[{id:'clarify'},{id:'pickup'},{id:'specify'}]}}));return}
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     if(failLaunch){res.writeHead(500);res.end(JSON.stringify({error:'Launch rejected'}));return}
     launches.push({project,...JSON.parse(raw)});res.end(JSON.stringify({status:'queued'}))
    });return
   }
   const query=url.searchParams.get('q');requests.push({project,query,launchable:url.searchParams.get('launchable')})
   if(query==='slow'){pending.push(res);return}
   if(failRead){res.writeHead(503);res.end(JSON.stringify({error:'Offline'}));return}
   const result=empty?[]:project==='project-b'?[{id:'b1',key:'#1',title:'Other project task',status:'to_clarify',priority:'low'}]:tasks
   res.end(JSON.stringify(result.filter(task=>!query||(task.key+' '+task.title).includes(query))));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const open=name=>page.getByRole('button',{name:'Open tasks in Project '+name,exact:true})
  const pane=page.locator('#tickets-pane'),rows=page.locator('.ticket-row'),query=page.getByRole('textbox',{name:'Search server tasks'})
  const search=page.getByRole('button',{name:'Search',exact:true})
  const close=()=>page.getByRole('button',{name:'Close tickets',exact:true}).click()
  const keys=()=>rows.locator('.ticket-key').allTextContents()
  const sortOf=column=>page.locator('.tickets-table th[data-column="'+column+'"]').getAttribute('aria-sort')
  const heading=page.getByRole('button',{name:'▾ Project A',exact:true})
  await heading.waitFor()
  // The command palette offers the list too; with no project selected it asks which one.
  await page.keyboard.press('Control+k')
  await page.getByRole('textbox',{name:'Search commands'}).fill('tasks')
  await expect(page.getByRole('button',{name:'Quick add task',exact:true})).toBeHidden()
  await page.getByRole('button',{name:'Tasks list',exact:true}).click()
  await page.getByText('Choose the project whose tasks you want to browse.',{exact:true}).waitFor()
  await page.getByRole('button',{name:'Project B',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Tickets · Project B',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Close tickets',exact:true}).click()
  await expect(open('B')).toBeFocused()
  await page.mouse.move(700,400)
  await expect(open('A')).toHaveCSS('opacity','0')
  await heading.hover()
  await expect(open('A')).toHaveCSS('opacity','1')
  await heading.click()
  await page.getByRole('button',{name:'▸ Project A',exact:true}).focus();await page.keyboard.press('Tab')
  await expect(open('A')).toBeFocused();await expect(open('A')).toHaveCSS('opacity','1')
  await page.keyboard.press('Enter')
  // The pane takes the console's place, the project stays collapsed, nothing launches.
  await expect(pane).toBeVisible()
  await expect(page.locator('#workspace article')).toBeHidden()
  await expect(page.getByRole('heading',{name:'Tickets · Project A',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'▸ Project A',exact:true}).waitFor()
  await expect(rows).toHaveCount(4)
  assert.deepEqual(requests.at(-1),{project:'project-a',query:'',launchable:'true'})
  assert.equal(await query.inputValue(),'')
  assert.equal(await query.evaluate(el=>document.activeElement===el),true)
  assert.equal(launches.length,0)
  // Default order: priority descending, then natural identity ascending.
  assert.deepEqual(await keys(),['#2','#1','#9','#100'])
  assert.equal(await sortOf('priority'),'descending');assert.equal(await sortOf('key'),'none')
  await expect(rows.nth(0).locator('.ticket-stage')).toHaveText('specified')
  await expect(rows.nth(0).getByRole('button',{name:'Open PR #42 for #2 — State unknown',exact:true})).toBeVisible()
  await expect(rows.nth(1).locator('.ticket-title')).toHaveText('First open task')
  // The key opens that task in Sectile, like the sidebar task number.
  await page.getByRole('button',{name:'Open #1 in Sectile',exact:true}).click()
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['http://example.test/?task=a1'])
  await page.getByRole('button',{name:'Open #9 in Sectile',exact:true}).focus()
  await page.keyboard.press('Enter')
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['http://example.test/?task=a1','http://example.test/?task=a9'])
  assert.equal(launches.length,0)
  // The next step is resolved per row; a skill the project lacks disables Run with the reason.
  const runFirst=page.getByRole('button',{name:'Run: Clarify',exact:true}).first()
  await expect(runFirst).toBeEnabled()
  const runSecond=rows.nth(0).locator('.ticket-run')
  await expect(runSecond).toBeDisabled();await expect(runSecond).toHaveAttribute('title','Next skill is unavailable: Implement')
  // Sorting by a column, then reversing it, keeps identity as the tie-break.
  const titleHeader=page.getByRole('button',{name:'Title',exact:true})
  await titleHeader.click()
  assert.deepEqual(await keys(),['#1','#100','#9','#2'])
  assert.equal(await sortOf('title'),'ascending');assert.equal(await sortOf('priority'),'none')
  await expect(titleHeader).toBeFocused()
  await titleHeader.click()
  assert.deepEqual(await keys(),['#2','#9','#100','#1'])
  assert.equal(await sortOf('title'),'descending')
  await page.getByRole('button',{name:'Key',exact:true}).click()
  assert.deepEqual(await keys(),['#1','#2','#9','#100'])
  await page.getByRole('button',{name:'Priority',exact:true}).click()
  assert.deepEqual(await keys(),['#2','#1','#9','#100'])
  assert.equal(await sortOf('priority'),'descending')
  // The sort lasts for the pane only.
  await titleHeader.click();assert.deepEqual(await keys(),['#1','#100','#9','#2'])
  await close();await expect(pane).toBeHidden();await expect(open('A')).toBeFocused()
  await page.keyboard.press('Enter');await expect(rows).toHaveCount(4)
  assert.deepEqual(await keys(),['#2','#1','#9','#100'])
  // Search narrows and restores.
  await query.fill('Second');await search.click();await expect(rows).toHaveCount(1)
  await expect(rows).toContainText('Second open task')
  await query.fill('missing');await search.click();await page.getByText('No matching open tasks',{exact:true}).waitFor()
  await query.fill('');await search.click();await expect(rows).toHaveCount(4)
  await page.screenshot({path:path.join(root,'project-open-tasks.png')})
  console.log('Tickets pane screenshot: '+path.join(root,'project-open-tasks.png'))
  // Hold an older request, complete a newer one, then reject the old request.
  await query.fill('slow');await search.click();await expect.poll(()=>pending.length).toBe(1)
  await page.getByText('Loading open tasks…',{exact:true}).waitFor()
  await query.fill('Second');await search.click();await expect(rows).toHaveCount(1)
  pending.shift().writeHead(503).end(JSON.stringify({error:'Stale failure'}))
  await page.waitForTimeout(150)
  await expect(rows).toContainText('Second open task')
  await expect(page.getByRole('alert').filter({hasText:'Stale failure'})).toHaveCount(0)
  // An old successful response cannot populate another project's pane.
  await query.fill('slow');await search.click();await expect.poll(()=>pending.length).toBe(1)
  await close();await open('B').click();await expect(rows).toContainText('Other project task')
  await expect(page.getByRole('heading',{name:'Tickets · Project B',exact:true})).toBeVisible()
  pending.shift().end(JSON.stringify(tasks))
  await page.waitForTimeout(150);await expect(rows).toHaveCount(1);await expect(rows).toContainText('Other project task')
  assert.equal(requests.at(-1).project,'project-b')
  // A failed launch stays visible and the control comes back.
  const run=page.getByRole('button',{name:'Run: Clarify',exact:true})
  failLaunch=true;await run.click()
  await expect(page.locator('.tickets-status')).toContainText('Could not launch #1: ')
  await expect(page.locator('.tickets-status')).toContainText('Launch rejected')
  await expect(run).toBeEnabled()
  assert.equal(launches.length,0)
  failLaunch=false
  // Run submits the next step once, with no mode override, and the pane stays open.
  await run.click()
  await expect(page.getByRole('status').filter({hasText:'Execution submitted for #1'})).toBeVisible()
  await expect(pane).toBeVisible()
  assert.deepEqual(launches,[{project:'project-b',taskID:'b1',skillID:'clarify',prompt:''}])
  // The menu launches the full chain and a discussion with no instructions at all.
  const more=page.getByRole('button',{name:'More actions for #1',exact:true})
  await more.click()
  await expect(more).toHaveAttribute('aria-expanded','true')
  assert.deepEqual(await page.getByRole('menuitem').allTextContents(),['Pickup (full chain)','clarify','specify','Discussion (no skill)','Discussion in native terminal','Custom instructions…'])
  await page.getByRole('menuitem',{name:'Pickup (full chain)',exact:true}).click()
  await expect.poll(()=>launches.length).toBe(2)
  assert.deepEqual(launches.at(-1),{project:'project-b',taskID:'b1',skillID:'pickup',prompt:''})
  await expect(more).toHaveAttribute('aria-expanded','false')
  await more.click();await page.keyboard.press('Escape');await expect(page.getByRole('menu')).toBeHidden();await expect(more).toBeFocused()
  await expect(pane).toBeVisible()
  await more.click();await page.getByRole('menuitem',{name:'Discussion (no skill)',exact:true}).click()
  await expect.poll(()=>launches.length).toBe(3)
  assert.deepEqual(launches.at(-1),{project:'project-b',taskID:'b1',skillID:'discuss',prompt:''})
  // Custom instructions need text and carry the one-off execution mode.
  await more.click();await page.getByRole('menuitem',{name:'Custom instructions…',exact:true}).click()
  const instructions=page.getByRole('textbox',{name:'Custom instructions',exact:true})
  await expect(instructions).toBeFocused()
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await expect(page.getByRole('status').filter({hasText:'Enter custom instructions.'})).toBeVisible()
  assert.equal(launches.length,3)
  await instructions.fill('Explain the ticket')
  await page.getByRole('combobox',{name:'Execution mode for #1',exact:true}).selectOption('autonomous')
  // Sorting rebuilds the rows; what the user is typing is their work, not render state.
  await page.getByRole('button',{name:'Title',exact:true}).click()
  await expect(instructions).toHaveValue('Explain the ticket')
  await expect(page.getByRole('combobox',{name:'Execution mode for #1',exact:true})).toHaveValue('autonomous')
  await expect(page.getByRole('button',{name:'Title',exact:true})).toBeFocused()
  await page.getByRole('button',{name:'Launch',exact:true}).click()
  await expect.poll(()=>launches.length).toBe(4)
  assert.deepEqual(launches.at(-1),{project:'project-b',taskID:'b1',skillID:'custom',prompt:'Explain the ticket',mode:'autonomous'})
  await expect(instructions).toHaveCount(0)
  // An active execution disables Run, shows the shared state and leaves the menu open to use;
  // the poll updates the row in place without moving focus.
  await run.focus()
  runs=[{id:'run-b1',taskId:'b1',taskKey:'#1',projectId:'project-b',skill:'clarify',status:'running',directory:'/tmp/B',createdAt:new Date().toISOString()}]
  const state=rows.first().locator('.run-state')
  await expect(state).toHaveAttribute('aria-label','Running')
  await expect(run).toBeDisabled();await expect(run).toHaveAttribute('title','An execution is active on this task')
  // Disabling the focused control hands focus to the row's menu, never to the body.
  await expect(more).toBeFocused();await expect(more).toBeEnabled()
  runs=[{...runs[0],status:'completed'}]
  await expect(state).toHaveAttribute('aria-label','Finished')
  await expect(run).toBeEnabled()
  // A control the refresh does not touch keeps focus.
  await expect(more).toBeFocused()
  assert.deepEqual(await keys(),['#1'])
  // An archived execution is hidden here as it is in the sidebar.
  await page.getByRole('button',{name:'Archive #1',exact:true}).click()
  await expect(state).toBeEmpty()
  // Selecting an execution leaves the pane instead of changing it behind a hidden view.
  runs=[{...runs[0],status:'running'}]
  await expect(state).toHaveAttribute('aria-label','Running')
  await page.locator('#runs .local-task .run').first().click()
  await expect(pane).toBeHidden()
  await expect(page.locator('#workspace article')).toBeVisible()
  await open('B').click();await expect(rows).toHaveCount(1)
  runs=[]
  // Loading errors, empty lists and an unconfigured project.
  failRead=true;await open('A').click();await page.getByRole('alert').filter({hasText:'Could not load open tasks'}).waitFor()
  failRead=false;await search.click();await expect(rows).toHaveCount(4)
  empty=true;await search.click();await page.getByText('No open tasks in this project',{exact:true}).waitFor()
  configured=false;await search.click()
  await page.getByText('Configure a local repository before launching tasks.',{exact:true}).waitFor()
  await page.getByText('No open tasks in this project',{exact:true}).waitFor()
  empty=false;await search.click();await expect(rows).toHaveCount(4)
  await page.getByText('Configure a local repository before launching tasks.',{exact:true}).waitFor()
  for(const button of await page.locator('.ticket-run').all())assert.equal(await button.isDisabled(),true)
  const disabledMenu=page.getByRole('button',{name:'More actions for #1',exact:true})
  await disabledMenu.click()
  for(const item of await page.getByRole('menuitem').all())assert.equal(await item.isDisabled(),true)
  // Every item is disabled, so focus stays on the opener: Escape must still
  // dismiss the menu and leave the pane open, and an outside press must too.
  await page.keyboard.press('Escape')
  await expect(page.getByRole('menu')).toBeHidden();await expect(pane).toBeVisible()
  await expect(disabledMenu).toBeFocused()
  await disabledMenu.click();await expect(page.getByRole('menu')).toBeVisible()
  await page.locator('.tickets-toolbar h2').click()
  await expect(page.getByRole('menu')).toBeHidden();await expect(pane).toBeVisible()
  // Escape closes the pane and returns to the console view.
  await page.keyboard.press('Escape');await expect(pane).toBeHidden()
  await expect(page.locator('#workspace article')).toBeVisible()
  // With a project already selected the palette opens its list without asking.
  await page.keyboard.press('Control+k')
  await page.getByRole('button',{name:'Tasks list',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Tickets · Project A',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Close tickets',exact:true}).click()
  await page.getByRole('button',{name:'▸ Project A',exact:true}).waitFor()
  // The icon fits alongside the existing project controls at minimum width.
  await page.getByRole('separator',{name:'Resize sidebar'}).focus()
  for(let i=0;i<4;i++)await page.keyboard.press('ArrowLeft')
  const row=page.locator('.project-row').first(),bounds=await row.boundingBox()
  for(const button of await row.getByRole('button').all()){
   const control=await button.boundingBox()
   assert.ok(control.x>=bounds.x-1&&control.x+control.width<=bounds.x+bounds.width+1,'project control overflows the row')
  }
  await open('A').click();await expect(pane).toBeVisible()
  offline=true
  await expect(page.locator('#setup')).toBeVisible()
  await expect(pane).toBeHidden()
  offline=false
  await expect(page.locator('#workspace')).toBeVisible()
 }finally{
  for(const response of pending)response.end('[]')
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})
