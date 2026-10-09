const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The project board (#806) against a stand-in agent: its entries, its columns,
// its two options, its cards and the drop that moves a task between stages.
async function boardApp(t,{capabilities=['stage-move'],board,tasks:initialTasks}={}){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-board-'))
 const state={tasks:initialTasks,moves:[],launches:[],taskReads:[],refuseMove:false,failRead:false,board,capabilities}
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost'),project=url.searchParams.get('projectId')
  const body=()=>new Promise(resolve=>{let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>resolve(raw?JSON.parse(raw):{}))})
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:state.capabilities}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify(['A','B'].map(name=>({id:'project-'+name.toLowerCase(),name:'Project '+name,path:'/tmp/'+name}))));return}
  if(url.pathname==='/desktop/runs'){res.end('[]');return}
  if(url.pathname==='/desktop/project'){
   const info={configured:true,server:{defaultSkillMode:'interactive',skills:[{id:'clarify'},{id:'pickup',mode:'interactive'},{id:'specify'}]}}
   if(state.board)info.board=state.board
   res.end(JSON.stringify(info));return
  }
  if(url.pathname==='/desktop/tasks/stage-move'&&req.method==='POST'){
   const answer=move=>{
    if(state.refuseMove){res.writeHead(403);res.end(JSON.stringify({error:'You cannot edit this task'}));return}
    const task=state.tasks.find(item=>item.id===move.taskId)
    Object.assign(task,{labels:move.labels,status:move.status},move.trackerStatus?{trackerStatus:move.trackerStatus}:{})
    res.end(JSON.stringify(task))
   }
   // A held move is answered when the test releases it, never on a timer.
   body().then(move=>{state.moves.push({project,...move});if(state.holdMove)state.release=()=>answer(move);else answer(move)});return
  }
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){body().then(launch=>{state.launches.push({project,...launch});res.end(JSON.stringify({status:'queued'}))});return}
   const query=url.searchParams.get('q')||''
   state.taskReads.push({project,query,launchable:url.searchParams.get('launchable')})
   if(state.failRead){res.writeHead(503);res.end(JSON.stringify({error:'Offline'}));return}
   const result=project==='project-b'?[]:state.tasks
   res.end(JSON.stringify(result.filter(task=>!query||(task.key+' '+task.title).includes(query))));return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test-secret'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
 t.after(async()=>{await app.close();await new Promise(resolve=>server.close(resolve))})
 await app.evaluate(({shell})=>{globalThis.opened=[];shell.openExternal=async url=>{globalThis.opened.push(url)}})
 const page=await app.firstWindow();page.setDefaultTimeout(7000)
 await page.getByRole('button',{name:'▾ Project A',exact:true}).waitFor()
 const projectActions=name=>page.getByRole('button',{name:'Actions for Project '+name,exact:true})
 const openBoard=async(name='A')=>{
  await projectActions(name).click()
  await page.getByRole('menuitem',{name:'Open board',exact:true}).click()
  await expect(page.getByRole('heading',{name:'Board · Project '+name,exact:true})).toBeVisible()
 }
 const column=stage=>page.getByRole('region',{name:new RegExp('^'+stage+' column, ')})
 const card=key=>page.locator('.board-card',{has:page.locator('.board-card-key',{hasText:new RegExp('^'+key.replace('#','\\#')+'$')})})
 const cardKeys=stage=>column(stage).locator('.board-card-key').allTextContents()
 // The target is brought into view first: a board scrolling under a drag that
 // has started picks up whatever card the pointer then sits on.
 const drag=async(source,target)=>{await target.scrollIntoViewIfNeeded();await source.dragTo(target)}
 return {page,app,state,openBoard,column,card,cardKeys,projectActions,drag}
}

const mapped={
 epicColors:true,
 trackerColumns:[{name:'Todo',statuses:['Open']},{name:'In Review',statuses:['Code Review']},{name:'Done',statuses:['Closed']}],
 stageColumns:{new:['Todo'],implemented:['In Review'],finished:['Done']},
 trackers:[]
}
const sample=()=>[
 {id:'a1',key:'#1',title:'Clarify the scope',status:'to_clarify',priority:'low',trackerStatus:'Open'},
 {id:'a2',key:'#2',title:'Urgent new task with a rather long title that does not fit',status:'to_clarify',priority:'urgent',parentKey:'#12',labels:['bug','#untouched'],assignee:'Ada',trackerStatus:'Open'},
 {id:'a3',key:'#3',title:'Placed by its column',status:'to_clarify',trackerStatus:'Code Review'},
 {id:'a4',key:'#4',title:'Labelled reviewed',status:'to_clarify',labels:['#reviewed'],trackerStatus:'Code Review',prUrl:'https://github.com/acme/repo/pull/7'},
 {id:'a5',key:'#5',title:'Done long ago',status:'finished',trackerStatus:'Closed'},
 {id:'a6',key:'#6',title:'Specified task',status:'to_implement',labels:['#specified']}
]

test('the board opens from the project menu and the palette, one page at a time',async t=>{
 const {page,state,openBoard,column,projectActions}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard('A')
 // Finished tasks are read too: the board has a column for them.
 assert.deepEqual(state.taskReads.at(-1),{project:'project-a',query:'',launchable:'false'})
 for(const [stage,count] of [['New',2],['Clarified',0],['Specified',1],['Implemented',1],['Reviewed',1]]){
  await expect(column(stage)).toHaveAttribute('aria-label',stage+' column, '+count+(count===1?' task':' tasks'))
 }
 // Opening the tickets list replaces the board, and the other way round.
 await projectActions('A').click();await page.getByRole('menuitem',{name:'Open tasks',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Tickets · Project A',exact:true})).toBeVisible()
 await expect(page.locator('.board')).toHaveCount(0)
 await openBoard('A')
 await expect(page.locator('.tickets-table')).toHaveCount(0)
 // Close gives the focus back to what opened it.
 await page.getByRole('button',{name:'Close board',exact:true}).click()
 await expect(page.locator('#tickets-pane')).toBeHidden()
 await expect(projectActions('A')).toBeFocused()
 // The palette opens the selected project's board.
 await page.keyboard.press('Control+k')
 await page.getByRole('combobox',{name:'Search commands'}).fill('board')
 await page.locator('.palette-option',{hasText:'Project board'}).click()
 await expect(page.getByRole('heading',{name:'Board · Project A',exact:true})).toBeVisible()
 // Escape closes it as it closes the tickets list.
 await page.keyboard.press('Escape')
 await expect(page.locator('#tickets-pane')).toBeHidden()
})

test('the palette asks which project when none is selected',async t=>{
 const {page}=await boardApp(t,{board:mapped,tasks:sample()})
 await page.keyboard.press('Control+k')
 await page.getByRole('combobox',{name:'Search commands'}).fill('board')
 await page.locator('.palette-option',{hasText:'Project board'}).click()
 await page.getByText('Choose the project whose board you want to see.',{exact:true}).waitFor()
 await page.getByRole('button',{name:'Project B',exact:true}).click()
 await expect(page.getByRole('heading',{name:'Board · Project B',exact:true})).toBeVisible()
 await expect(page.getByText('This project has no tasks.',{exact:true})).toBeVisible()
 await expect(page.locator('.board-column')).toHaveCount(5)
 await expect(page.locator('.board-collapsed')).toHaveAttribute('aria-label','Show finished tasks (0)')
})

test('columns place tasks as the web board does, ordered by priority then key',async t=>{
 const {page,cardKeys,openBoard,column}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 assert.deepEqual(await cardKeys('New'),['#2','#1'])
 assert.deepEqual(await cardKeys('Specified'),['#6'])
 assert.deepEqual(await cardKeys('Implemented'),['#3'])
 // An explicit label wins over the column.
 assert.deepEqual(await cardKeys('Reviewed'),['#4'])
 await expect(column('Clarified').locator('.board-card')).toHaveCount(0)
 // Each column carries its stage tint.
 const tints=await page.locator('.board-column').evaluateAll(columns=>columns.map(column=>getComputedStyle(column).backgroundColor))
 assert.equal(new Set(tints).size,tints.length)
})

test('without board data from the agent the board still opens, placed by status',async t=>{
 const {cardKeys,openBoard,card}=await boardApp(t,{tasks:sample()})
 await openBoard()
 // #3 has no workflow label: without the mapping its status places it.
 assert.deepEqual(await cardKeys('New'),['#2','#1','#3'])
 // And no colour bar without the project's say-so.
 await expect(card('#2')).not.toHaveClass(/has-epic/)
})

test('the finished column starts collapsed, expands, and the choice is remembered',async t=>{
 const {page,openBoard,column,cardKeys}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 const strip=page.locator('.board-collapsed')
 await expect(strip).toHaveAttribute('aria-label','Show finished tasks (1)')
 await expect(strip.locator('.board-count')).toHaveText('1')
 await strip.click()
 assert.deepEqual(await cardKeys('Finished'),['#5'])
 await expect(page.getByRole('button',{name:'Hide finished',exact:true})).toBeFocused()
 await page.reload()
 await openBoard()
 await expect(column('Finished')).toBeVisible()
 await page.getByRole('button',{name:'Hide finished',exact:true}).click()
 await expect(strip).toBeFocused()
 await page.reload()
 await openBoard()
 await expect(page.locator('.board-collapsed')).toBeVisible()
})

test('cards are condensed by default, full on demand, and the choice is remembered',async t=>{
 const {page,openBoard,card}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 const condensed=page.getByRole('radio',{name:'Condensed',exact:true}),full=page.getByRole('radio',{name:'Full',exact:true})
 await expect(page.getByRole('radiogroup',{name:'Card display'})).toBeVisible()
 await expect(condensed).toHaveAttribute('aria-checked','true')
 const second=card('#2')
 await expect(second.locator('.board-card-title')).toHaveAttribute('title','Urgent new task with a rather long title that does not fit')
 await expect(second.locator('.board-card-details')).toHaveCount(0)
 // A card keeps its own height: the workspace's article rule must not stretch
 // it to fill the column.
 const heights=await page.locator('.board-card').evaluateAll(cards=>cards.map(card=>card.getBoundingClientRect().height))
 assert.ok(Math.max(...heights)<60,'condensed cards stretched: '+heights.join(', '))
 await full.click()
 await expect(full).toHaveAttribute('aria-checked','true')
 const details=card('#2').locator('.board-card-details')
 await expect(details.locator('.board-card-parent')).toHaveText('#12')
 await expect(details.locator('.board-card-priority')).toHaveText('urgent')
 assert.deepEqual(await details.locator('.board-card-label').allTextContents(),['bug'])
 await expect(details.locator('.board-card-assignee')).toHaveText('Ada')
 // A full card gives the title a line of its own, below the key.
 const [keyBox,titleBox]=await Promise.all([card('#2').locator('.board-card-key').boundingBox(),card('#2').locator('.board-card-title').boundingBox()])
 assert.ok(titleBox.y>=keyBox.y+keyBox.height-1,'title beside the key in a full card')
 await expect(card('#4').locator('.pr-indicator')).toHaveCount(1)
 await page.reload()
 await openBoard()
 await expect(page.getByRole('radio',{name:'Full',exact:true})).toHaveAttribute('aria-checked','true')
 // The arrow keys move within the group.
 await page.getByRole('radio',{name:'Full',exact:true}).focus();await page.keyboard.press('ArrowLeft')
 await expect(page.getByRole('radio',{name:'Condensed',exact:true})).toHaveAttribute('aria-checked','true')
 await expect(page.getByRole('radio',{name:'Condensed',exact:true})).toBeFocused()
})

test('the macro colour bar is the web board’s colour, only with the project setting',async t=>{
 const {openBoard,card}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 // web/tests/epicColor.test.mjs pins #12 to violet, #8b5cf6.
 await expect(card('#2')).toHaveClass(/has-epic/)
 await expect(card('#2')).toHaveCSS('border-left-color','rgb(139, 92, 246)')
 await expect(card('#1')).not.toHaveClass(/has-epic/)
})

test('the colour bar stays off when the project does not enable it',async t=>{
 const {openBoard,card}=await boardApp(t,{board:{...mapped,epicColors:false},tasks:sample()})
 await openBoard()
 await expect(card('#2')).not.toHaveClass(/has-epic/)
})

test('a card offers the tickets row actions, and its title launches nothing',async t=>{
 const {page,app,state,openBoard,card}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 await card('#1').getByRole('button',{name:'More actions for #1',exact:true}).click()
 assert.deepEqual(await page.getByRole('menuitem').allTextContents(),['Pickup (full chain)','clarify','specify','Discussion (no skill)','Discussion in native terminal','Custom instructions…','Launch…','Edit task'])
 await page.getByRole('menuitem',{name:'clarify',exact:true}).click()
 await expect.poll(()=>state.launches.length).toBe(1)
 assert.deepEqual(state.launches[0],{project:'project-a',taskID:'a1',skillID:'clarify',prompt:''})
 // Custom instructions open under the card.
 await card('#1').getByRole('button',{name:'More actions for #1',exact:true}).click()
 await page.getByRole('menuitem',{name:'Custom instructions…',exact:true}).click()
 await expect(page.getByRole('textbox',{name:'Custom instructions'})).toBeFocused()
 await page.getByRole('button',{name:'Cancel',exact:true}).click()
 await card('#1').getByRole('button',{name:'Open #1 in Sectile',exact:true}).click()
 assert.equal(state.launches.length,1)
 await expect.poll(()=>app.evaluate(()=>globalThis.opened)).toEqual([expect.stringContaining('a1')])
})

test('dropping a card moves the task as the web board does and reloads',async t=>{
 const {page,state,openBoard,card,column,cardKeys,drag}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 await expect(card('#1')).toHaveAttribute('draggable','true')
 const reads=state.taskReads.length
 await drag(card('#1'),column('Implemented'))
 await expect.poll(()=>state.moves.length).toBe(1)
 assert.deepEqual(state.moves[0],{project:'project-a',taskId:'a1',labels:['#implemented'],status:'to_test',trackerStatus:'Code Review'})
 await expect.poll(()=>state.taskReads.length).toBeGreaterThan(reads)
 assert.deepEqual(await cardKeys('Implemented'),['#1','#3'])
 await expect(page.locator('.tickets-status')).toHaveText('#1 moved to Implemented')
 // Backwards, onto an unmapped stage: the tracker status stays, other labels too.
 await drag(card('#2'),column('Clarified'))
 await expect.poll(()=>state.moves.length).toBe(2)
 assert.deepEqual(state.moves[1],{project:'project-a',taskId:'a2',labels:['bug','#clarified'],status:'clarified',trackerStatus:'Open'})
 // Onto its own column: nothing is sent.
 await drag(card('#6'),column('Specified'))
 await page.waitForTimeout(300)
 assert.equal(state.moves.length,2)
})

test('a card being moved is busy and cannot be dragged again',async t=>{
 const {page,state,openBoard,card,column,cardKeys,drag}=await boardApp(t,{board:mapped,tasks:sample()})
 state.holdMove=true
 await openBoard()
 await drag(card('#1'),column('Specified'))
 await expect.poll(()=>state.moves.length).toBe(1)
 await expect(card('#1')).toHaveAttribute('aria-busy','true')
 await expect(card('#1')).toHaveAttribute('draggable','false')
 await expect(page.locator('.tickets-status')).toHaveText('Moving #1 to Specified…')
 state.release()
 await expect.poll(()=>cardKeys('Specified')).toEqual(['#1','#6'])
 await expect(card('#1')).toHaveAttribute('draggable','true')
})

test('a drop onto the collapsed finished strip finishes the task and keeps it collapsed',async t=>{
 const {page,state,openBoard,card,drag}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 const strip=page.locator('.board-collapsed')
 await drag(card('#6'),strip)
 await expect.poll(()=>state.moves.length).toBe(1)
 assert.deepEqual(state.moves[0],{project:'project-a',taskId:'a6',labels:['#finished'],status:'finished',trackerStatus:'Closed'})
 await expect(page.locator('.board-collapsed')).toHaveAttribute('aria-label','Show finished tasks (2)')
})

test('a refused drop keeps the card and shows the server message',async t=>{
 const {page,state,openBoard,card,cardKeys,column,drag}=await boardApp(t,{board:mapped,tasks:sample()})
 state.refuseMove=true
 await openBoard()
 await drag(card('#1'),column('Reviewed'))
 await expect.poll(()=>state.moves.length).toBe(1)
 await expect(page.locator('.tickets-status')).toHaveText('Could not move #1 to Reviewed: You cannot edit this task')
 assert.deepEqual(await cardKeys('New'),['#2','#1'])
 await expect(card('#1')).toHaveAttribute('draggable','true')
 await expect(card('#1')).not.toHaveAttribute('aria-busy','true')
})

test('an agent without the stage move leaves cards in place',async t=>{
 const {page,openBoard,card}=await boardApp(t,{capabilities:[],board:mapped,tasks:sample()})
 await openBoard()
 await expect(card('#1')).toBeVisible()
 await expect(page.locator('.board-card[draggable=true]')).toHaveCount(0)
 await expect(page.locator('.board-hint')).toHaveText('Moving cards between stages needs a newer local agent. Restart the agent from Settings to update it.')
 await expect(card('#1')).not.toHaveAttribute('draggable','true')
})

test('a search narrows the board, counts included, and a failed load offers a retry',async t=>{
 const {page,state,openBoard,column}=await boardApp(t,{board:mapped,tasks:sample()})
 await openBoard()
 await page.getByRole('textbox',{name:'Search server tasks'}).fill('Clarify')
 await page.getByRole('button',{name:'Search',exact:true}).click()
 await expect(column('New')).toHaveAttribute('aria-label','New column, 1 task')
 await expect(column('Reviewed')).toHaveAttribute('aria-label','Reviewed column, 0 tasks')
 assert.equal(state.taskReads.at(-1).query,'Clarify')
 state.failRead=true
 await page.getByRole('button',{name:'Search',exact:true}).click()
 await expect(page.locator('.board-area [role=alert]')).toHaveText('Could not load the board: Offline. Use Search to retry.')
 state.failRead=false
 await page.getByRole('button',{name:'Search',exact:true}).click()
 await expect(column('New')).toBeVisible()
})
