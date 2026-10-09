const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The desktop task page (#805) against a stand-in agent: creation, edition,
// the description editor, assignees, pull request links and the guard on
// unsaved changes.
const LOADED_DESCRIPTION='Intro paragraph\n\n* first\n* second\n\n__bold__ word ![logo](https://cdn.example/logo.png) ![dot](data:image/gif;base64,R0lGODlhAQABAAAAACw=)\n\n\nAfter two blank lines\n'
const SOURCE_ONLY='<details>\n<summary>More</summary>\n\nHidden\n\n</details>\n\nA note[^1].\n\n[^1]: The footnote.\n'

function agent(){
 const state={taskPage:true,failCreate:false,failUpdate:false,creates:[],updates:[],searches:[],launches:[],detailReads:0,created:0}
 const tasks=new Map([
  ['t1',{id:'t1',key:'#41',projectId:'project-a',source:'github',trackerId:'gh',title:'Loaded title',description:LOADED_DESCRIPTION,assignee:'octocat',prLinks:[{url:'https://github.com/acme/repo/pull/1',state:'merged',branch:'feat/1'}],prUrl:'https://github.com/acme/repo/pull/1',externalUrl:'https://github.com/acme/repo/issues/41'}],
  ['t2',{id:'t2',key:'#42',projectId:'project-a',source:'github',trackerId:'gh',title:'Raw source task',description:SOURCE_ONLY,prLinks:[]}],
  ['j1',{id:'j1',key:'BE-7',projectId:'project-b',source:'jira',trackerId:'jira-a',title:'Jira task',description:'',assignee:'',prLinks:[]}],
 ])
 const projects={
  'project-a':{configured:true,server:{issueTracker:'github',trackers:[{id:'gh',name:'GitHub',provider:'github'}],skills:[{id:'clarify'},{id:'specify'}]}},
  'project-b':{configured:true,server:{issueTracker:'jira',trackers:[{id:'jira-a',name:'Jira BE',provider:'jira'},{id:'jira-b',name:'Jira FE',provider:'jira'}],skills:[{id:'clarify'}]}},
 }
 const people=[{accountId:'5b10',displayName:'Jane Doe',email:'jane@example.com',avatarUrl:'https://avatar.example/jane.png',active:true},{accountId:'7c22',displayName:'John Roe',active:true}]
 const runs=[{id:'run-1',taskId:'t1',taskKey:'#41',projectId:'project-a',skill:'specify',status:'completed',sessionId:'run-1',directory:'/tmp/a',createdAt:'2026-10-01T10:00:00Z'}]
 const read=req=>new Promise(resolve=>{let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>resolve(raw?JSON.parse(raw):{}))})
 const server=http.createServer(async(req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost'),q=key=>url.searchParams.get(key)
  const send=(value,status=200)=>{res.writeHead(status);res.end(JSON.stringify(value))}
  if(url.pathname==='/desktop/status')return send({connected:true,server:'http://example.test',capabilities:['create-task',...state.taskPage?['task-page']:[]]})
  if(url.pathname==='/desktop/projects')return send([{id:'project-a',name:'Project A',path:'/tmp/a',configured:true},{id:'project-b',name:'Project B',path:'/tmp/b',configured:true}])
  if(url.pathname==='/desktop/project')return send(projects[q('id')]||{configured:false})
  if(url.pathname==='/desktop/runs')return send(runs)
  if(url.pathname==='/desktop/run-result')return send({activity:null})
  if(url.pathname==='/desktop/create-task'&&req.method==='POST'){
   const body=await read(req);state.creates.push(body)
   if(state.failCreate)return send({error:'Tracker unavailable'},502)
   const id='created-'+(++state.created),provider=projects[body.projectID].server.trackers.find(item=>item.id===body.trackerID)?.provider||projects[body.projectID].server.issueTracker
   const task={id,key:'#'+(50+state.created),projectId:body.projectID,source:provider,trackerId:body.trackerID||projects[body.projectID].server.trackers[0].id,title:body.title,description:body.description,prLinks:[]}
   tasks.set(id,task);return send(task,201)
  }
  if(url.pathname==='/desktop/tasks/detail'){
   const task=tasks.get(q('taskId'))
   if(!task||task.projectId!==q('projectId'))return send({error:'The task does not belong to this project'},400)
   if(req.method==='GET'){state.detailReads++;return send(task)}
   const body=await read(req);state.updates.push({taskId:task.id,body})
   if(state.failUpdate)return send({error:'Tracker refused the update'},502)
   Object.assign(task,body)
   if(body.prLinks){task.prLinks=body.prLinks;task.prUrl=body.prLinks.at(-1)?.url||''}
   return send(task)
  }
  if(url.pathname==='/desktop/tasks/assignable'){
   state.searches.push({taskId:q('taskId'),query:q('q')})
   return send(people.filter(person=>!q('q')||person.displayName.toLowerCase().includes(q('q').toLowerCase())))
  }
  if(url.pathname==='/desktop/tasks'&&req.method==='POST'){state.launches.push({project:q('projectId'),...await read(req)});return send({status:'queued'})}
  if(url.pathname==='/desktop/tasks')return send([...tasks.values()].filter(task=>task.projectId===q('projectId')))
  res.writeHead(404).end('{}')
 })
 return {state,tasks,server}
}

test('the task page creates and edits a task',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-page-'))
 const {state,tasks,server}=agent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  const unloads=[];page.on('dialog',dialog=>unloads.push(dialog.type()))
  const rendererErrors=[];page.on('console',message=>{if(message.type()==='error')rendererErrors.push(message.text())});page.on('pageerror',err=>rendererErrors.push(err.message))
  await page.evaluate(()=>{window.cspViolations=[];document.addEventListener('securitypolicyviolation',event=>window.cspViolations.push(event.violatedDirective+' '+event.blockedURI))})
  const mod=process.platform==='darwin'?'Meta':'Control'
  // A new paragraph after the description's last one.
  const appendLine=async()=>{await page.locator('.md-document > :last-child').click();await page.keyboard.press('End');await page.keyboard.press('Enter')}
  const pane=page.locator('#task-page-pane')
  const heading=pane.locator('.task-page-title > span').first()
  const titleField=page.getByRole('textbox',{name:'Title',exact:true})
  const description=page.getByRole('textbox',{name:'Description',exact:true})
  const statusLine=pane.locator('.task-page-status')
  const save=pane.locator('.task-page-save')
  await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()

  // Cmd+N opens the creation page in the workspace; no dialog opens (AC1).
  await page.keyboard.press(mod+'+n')
  await expect(heading).toHaveText('New task')
  await expect(page.locator('#project-dialog')).not.toHaveAttribute('open','')
  assert.equal(await page.locator('.quick-add,.quick-add-dialog').count(),0)
  await expect(titleField).toBeFocused()
  assert.equal(await page.getByRole('combobox',{name:'Project',exact:true}).inputValue(),'project-a')
  // One tracker: no tracker select. A GitHub tracker takes a free login.
  await expect(page.getByRole('combobox',{name:'Tracker',exact:true})).toBeHidden()
  await expect(pane.locator('.task-page-unsaved')).toBeHidden()

  // A creation without a title does nothing and says what is missing (US1.9).
  await save.click()
  await expect(statusLine).toHaveText('A title is required.')
  assert.equal(state.creates.length,0)

  // Title, rich description through the slash menu and the toolbar, a login and a link.
  await titleField.fill('Created from the page')
  await expect(pane.locator('.task-page-unsaved')).toBeVisible()
  await description.click()
  await page.keyboard.type('/task')
  await expect(page.locator('.md-slash')).toBeVisible()
  await expect(page.locator('.md-slash-option[aria-selected=true]')).toHaveText('Task list')
  await page.keyboard.press('Enter')
  await page.keyboard.type('item')
  await page.keyboard.press('Enter');await page.keyboard.press('Enter')
  await page.keyboard.type('a word here')
  // The caret goes back over " here", then the selection takes "word".
  for(let i=0;i<5;i++)await page.keyboard.press('ArrowLeft')
  for(let i=0;i<4;i++)await page.keyboard.press('Shift+ArrowLeft')
  await expect(page.locator('.md-toolbar')).toBeVisible()
  await page.keyboard.press(mod+'+b')
  await page.getByRole('textbox',{name:'Assignee',exact:true}).fill('octocat')
  const linkInput=page.getByRole('textbox',{name:'Pull request link',exact:true})
  await linkInput.fill('not a link');await page.getByRole('button',{name:'Add',exact:true}).click()
  await expect(pane.getByText('Enter an absolute http or https link.',{exact:true})).toBeVisible()
  await linkInput.fill('https://github.com/acme/repo/pull/9');await linkInput.press('Enter')
  await expect(pane.locator('.task-page-link-current')).toHaveCount(1)
  await linkInput.fill('https://github.com/acme/repo/pull/9');await linkInput.press('Enter')
  await expect(pane.getByText('This pull request is already listed.',{exact:true})).toBeVisible()

  // A failed creation keeps every field (US1.7, AC3).
  state.failCreate=true
  await page.keyboard.press(mod+'+Enter')
  await expect(statusLine).toHaveText('Tracker unavailable')
  await expect(titleField).toHaveValue('Created from the page')
  await expect(heading).toHaveText('New task')
  await expect(save).toBeEnabled()
  state.failCreate=false

  // A creation sends create-task, then one update with the login and the links (AC2).
  await save.click()
  await expect(pane.locator('.task-page-created-text')).toHaveText('Created #51 · Created from the page')
  await page.screenshot({path:path.join(root,'task-page-created.png')})
  console.log('Screenshot:',path.join(root,'task-page-created.png'))
  assert.equal(state.creates.length,2)
  const sent=state.creates.at(-1)
  assert.equal(sent.projectID,'project-a');assert.equal(sent.title,'Created from the page');assert.equal(sent.trackerID,undefined)
  assert.equal(sent.description,'- [ ] item\n\na **word** here\n')
  assert.equal(state.updates.length,1)
  assert.deepEqual(state.updates[0],{taskId:'created-1',body:{assignee:'octocat',assigneeAccountId:'',assigneeAvatar:'',prLinks:[{url:'https://github.com/acme/repo/pull/9'}]}})
  await expect(heading).toHaveText('#51')
  await expect(pane.locator('.task-page-unsaved')).toBeHidden()
  await expect(page.getByRole('button',{name:'Clarify now',exact:true})).toBeFocused()

  // Add another opens a new creation on the same project; Done goes back.
  await page.getByRole('button',{name:'Add another',exact:true}).click()
  await expect(heading).toHaveText('New task')
  assert.equal(await page.getByRole('combobox',{name:'Project',exact:true}).inputValue(),'project-a')

  // Several trackers: the select appears, and a Jira assignee waits for the creation (US1.2, US4.3).
  await page.getByRole('combobox',{name:'Project',exact:true}).selectOption('project-b')
  const trackerSelect=page.getByRole('combobox',{name:'Tracker',exact:true})
  await expect(trackerSelect).toBeVisible()
  await trackerSelect.selectOption('jira-b')
  await expect(page.getByRole('combobox',{name:'Assignee',exact:true})).toBeDisabled()
  await expect(pane.getByText('Search becomes available once the task is created.',{exact:true})).toBeVisible()
  await titleField.fill('Jira creation')
  // A failed follow-up update leaves the created task's links unsaved (US1.8).
  await linkInput.fill('https://gitlab.example/g/p/-/merge_requests/3');await linkInput.press('Enter')
  state.failUpdate=true
  await save.click()
  await expect(statusLine).toHaveText('Created #52, but pull requests could not be saved: Tracker refused the update')
  assert.equal(state.creates.at(-1).trackerID,'jira-b')
  await expect(heading).toHaveText('#52')
  await expect(pane.locator('.task-page-unsaved')).toBeVisible()
  await expect(save).toHaveText('Save')
  state.failUpdate=false
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1).body,{prLinks:[{url:'https://gitlab.example/g/p/-/merge_requests/3'}]})
  await expect(pane.locator('.task-page-unsaved')).toBeHidden()

  // The Jira assignee search runs through the agent and sends name and account id (AC9).
  const picker=page.getByRole('combobox',{name:'Assignee',exact:true})
  await expect(picker).toBeEnabled()
  await picker.click()
  await expect(page.getByRole('option',{name:/Jane Doe/})).toBeVisible()
  await picker.fill('john')
  await expect(page.getByRole('option')).toHaveCount(1)
  assert.equal(state.searches.at(-1).query,'john')
  await page.keyboard.press('Enter')
  await expect(picker).toHaveValue('John Roe')
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1).body,{assignee:'John Roe',assigneeAccountId:'7c22',assigneeAvatar:''})

  // Leaving with unsaved changes asks; Keep editing keeps them (AC11).
  await titleField.fill('Unsaved title')
  await page.getByRole('button',{name:'Done',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Discard unsaved changes to #52?',exact:true})).toBeVisible()
  await expect(page.getByRole('button',{name:'Keep editing',exact:true})).toBeFocused()
  await page.getByRole('button',{name:'Keep editing',exact:true}).click()
  await expect(titleField).toHaveValue('Unsaved title')
  await page.locator('.local-task .run').first().click()
  await expect(page.getByRole('heading',{name:'Discard unsaved changes to #52?',exact:true})).toBeVisible()
  await page.getByRole('button',{name:'Discard',exact:true}).click()
  await expect(pane).toBeHidden()
  assert.equal(tasks.get('created-2').title,'Jira creation')

  // Edit task from the run header reads the task fresh (AC4); a title-only
  // change sends only the title, never the reserialized description (AC5).
  const readsBefore=state.detailReads
  await page.locator('#toolbar').getByRole('button',{name:'Edit task',exact:true}).click()
  await expect(heading).toHaveText('#41')
  await expect(titleField).toHaveValue('Loaded title')
  assert.ok(state.detailReads>readsBefore)
  await expect(pane.locator('.task-page-readonly').first()).toHaveText('Project A')
  await expect(save).toBeDisabled()
  await expect(pane.locator('.task-page-link-state')).toHaveText('Merged')
  // A remote image is never fetched: a placeholder names it; a data: image is drawn (US3.10).
  await expect(pane.locator('.md-image-placeholder')).toHaveText('🖼 logo (https://cdn.example/logo.png)')
  await expect(pane.locator('.md-document img.md-image-data')).toHaveCount(1)
  assert.match(await pane.locator('.md-document img.md-image-data').getAttribute('src'),/^data:image\/gif/)
  await page.screenshot({path:path.join(root,'task-page-edit.png')})
  console.log('Screenshot:',path.join(root,'task-page-edit.png'))
  await titleField.fill('Edited title')
  await page.keyboard.press(mod+'+Enter')
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1),{taskId:'t1',body:{title:'Edited title'}})
  await expect(save).toBeDisabled()
  await expect(page.locator('.local-task').first()).toContainText('Edited title')

  // An edit undone reads as unchanged.
  await description.click();await page.keyboard.press('End');await page.keyboard.type('X')
  await expect(save).toBeEnabled()
  await page.keyboard.press(mod+'+z')
  await expect(save).toBeDisabled()

  // A GitHub assignee is a free login; links are reordered and removed (AC10).
  const login=page.getByRole('textbox',{name:'Assignee',exact:true})
  await expect(login).toHaveValue('octocat')
  await login.fill('hubot')
  await linkInput.fill('https://github.com/acme/repo/pull/2');await linkInput.press('Enter')
  await expect(pane.locator('.task-page-link').last()).toContainText('Current')
  await page.getByRole('button',{name:'Move https://github.com/acme/repo/pull/2 up',exact:true}).click()
  await expect(pane.locator('.task-page-link').last()).toContainText('https://github.com/acme/repo/pull/1')
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1).body,{assignee:'hubot',assigneeAccountId:'',assigneeAvatar:'',prLinks:[{url:'https://github.com/acme/repo/pull/2'},{url:'https://github.com/acme/repo/pull/1',state:'merged',branch:'feat/1'}]})
  await page.getByRole('button',{name:'Remove https://github.com/acme/repo/pull/2',exact:true}).click()
  await page.getByRole('button',{name:'Remove https://github.com/acme/repo/pull/1',exact:true}).click()
  await expect(pane.getByText('No pull request',{exact:true})).toBeVisible()
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1).body,{prLinks:[]})

  // A link in the description opens only on Cmd/Ctrl+click.
  await appendLine()
  // Cmd/Ctrl+K links the selection.
  await page.keyboard.type('see the docs')
  for(let i=0;i<'the docs'.length;i++)await page.keyboard.press('Shift+ArrowLeft')
  await page.keyboard.press(mod+'+k')
  await page.getByRole('textbox',{name:'Link address',exact:true}).fill('https://docs.example/page')
  await page.keyboard.press('Enter')
  const docLink=pane.locator('.md-document a[href="https://docs.example/page"]')
  await docLink.click()
  assert.deepEqual(await app.evaluate(()=>globalThis.opened),[])
  await docLink.click({modifiers:[mod]})
  await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual(['https://docs.example/page'])
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.match(state.updates.at(-1).body.description,/\[the docs\]\(https:\/\/docs\.example\/page\)/)

  // Pasted HTML keeps its supported subset and drops scripts (AC8).
  await appendLine()
  await page.evaluate(()=>{
   const data=new DataTransfer();data.setData('text/html','<p>a<script>window.pasted=1</script><b>b</b></p>');data.setData('text/plain','ab')
   document.querySelector('.md-document').dispatchEvent(new ClipboardEvent('paste',{clipboardData:data,bubbles:true,cancelable:true}))
  })
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.match(state.updates.at(-1).body.description,/\na\*\*b\*\*\n$/)
  assert.equal(await page.evaluate(()=>window.pasted),undefined)
  assert.equal(await pane.locator('.md-document script').count(),0)

  // Edit task from the Tickets row menu; raw HTML and a footnote survive the
  // Markdown toggle and a title-only save (AC7).
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await page.getByRole('button',{name:'Actions for Project A',exact:true}).click();await page.getByRole('menuitem',{name:'Open tasks',exact:true}).click()
  await page.getByRole('button',{name:'More actions for #42',exact:true}).click()
  await page.getByRole('menuitem',{name:'Edit task',exact:true}).click()
  await expect(heading).toHaveText('#42')
  await page.getByRole('button',{name:'Markdown',exact:true}).click()
  const source=page.getByRole('textbox',{name:'Description (Markdown source)',exact:true})
  await expect(source).toHaveValue(SOURCE_ONLY)
  await page.getByRole('button',{name:'Markdown',exact:true}).click()
  await titleField.fill('Raw source task, renamed')
  await save.click()
  await expect(statusLine).toHaveText('Saved')
  assert.deepEqual(state.updates.at(-1).body,{title:'Raw source task, renamed'})
  // Done goes back to the tickets pane the page was opened over.
  await page.getByRole('button',{name:'Close',exact:true}).click()
  await expect(page.locator('#tickets-pane')).toBeVisible()
  await expect(page.locator('.ticket-row[data-task-id="t2"] .ticket-title')).toHaveText('Raw source task, renamed')
  await page.getByRole('button',{name:'Close tickets',exact:true}).click()

  // An agent without the task page: edition says it is outdated, a plain
  // creation still works (AC12).
  state.taskPage=false
  await page.locator('#toolbar').getByRole('button',{name:'Edit task',exact:true}).click()
  await expect(statusLine).toContainText('The running local agent is outdated.')
  await expect(titleField).toHaveCount(0)
  await page.keyboard.press(mod+'+n')
  await expect(heading).toHaveText('New task')
  await expect(page.getByRole('textbox',{name:'Assignee',exact:true})).toHaveCount(0)
  await expect(page.getByRole('textbox',{name:'Pull request link',exact:true})).toHaveCount(0)
  await titleField.fill('Basic creation')
  await save.click()
  await expect(pane.locator('.task-page-created-text')).toHaveText('Created #53 · Basic creation')
  state.taskPage=true

  // Clarify now launches clarify on the created task and leaves the page.
  await page.getByRole('button',{name:'Clarify now',exact:true}).click()
  await expect(pane).toBeHidden()
  assert.deepEqual([state.launches.at(-1).taskID,state.launches.at(-1).skillID],['created-3','clarify'])

  assert.deepEqual(await page.evaluate(()=>window.cspViolations),[])
  assert.deepEqual(rendererErrors,[])
  await page.screenshot({path:path.join(root,'task-page.png')})
 }finally{
  if(app)await app.close()
  await new Promise(resolve=>server.close(resolve))
 }
})

test('closing the window with unsaved changes asks first',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-task-page-close-'))
 const {server}=agent()
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
  const page=await app.firstWindow();page.setDefaultTimeout(7000)
  await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()
  // Electron turns the refused unload into will-prevent-unload; a listener
  // keeps Playwright from answering the page's dialog itself.
  page.on('dialog',()=>{})
  await app.evaluate(({dialog})=>{globalThis.asked=0;globalThis.answer=0;dialog.showMessageBoxSync=()=>{globalThis.asked++;return globalThis.answer}})
  const mod=process.platform==='darwin'?'Meta':'Control'
  await page.keyboard.press(mod+'+n')
  await page.getByRole('textbox',{name:'Title',exact:true}).fill('Not saved yet')
  // Keep editing cancels the close (US6.3).
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].close())
  await expect.poll(()=>app.evaluate(()=>globalThis.asked)).toBe(1)
  await expect(page.getByRole('textbox',{name:'Title',exact:true})).toHaveValue('Not saved yet')
  // Discard lets it close.
  await app.evaluate(()=>{globalThis.answer=1})
  const closed=new Promise(resolve=>page.once('close',resolve))
  await app.evaluate(({BrowserWindow})=>BrowserWindow.getAllWindows()[0].close())
  await closed
  assert.equal(await app.evaluate(()=>globalThis.asked),2)
 }finally{
  if(app)await app.close().catch(()=>{})
  await new Promise(resolve=>server.close(resolve))
 }
})
