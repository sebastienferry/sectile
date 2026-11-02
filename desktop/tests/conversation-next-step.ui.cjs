const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

// The launches and stops are recorded by the stub agent, not by the page, so
// waiting on the window alone can read them one poll behind.
async function until(condition,message,timeout=8000){
 const deadline=Date.now()+timeout
 while(!condition()){
  if(Date.now()>deadline)throw Error(message)
  await new Promise(resolve=>setTimeout(resolve,25))
 }
}

// A stand-in agent for one task conversation: a Specify conversation whose
// skill, task stage, approvals and launches the test drives (#875).
function agent(provider){
 const state={
  stage:'clarified',activity:'running',approvals:[],refuse:true,launches:[],stops:[],log:[],
  runs:[{id:'chat',taskId:'task',taskKey:'#7',projectId:'project',kind:'task',provider,skill:'specify',conversation:true,headless:true,status:'running',directory:'/tmp/project',createdAt:'2026-10-28T09:00:00Z'}],
  readOnly:new Set()
 }
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost'),id=url.searchParams.get('id')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,server:'http://example.test',capabilities:['claude-conversation','codex-conversation','agy-conversation','conversation-controls']}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{prCreationStage:'implemented',skills:['clarify','specify','implement','adjust','handoff','pickup'].map(id=>({id}))}}));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(state.runs));return}
  if(url.pathname==='/desktop/run-result'){
   const run=state.runs.find(item=>item.id===id)
   if(!run){res.writeHead(404).end('{}');return}
   const status=id==='chat'?state.activity:'running'
   res.end(JSON.stringify({activity:{id,taskId:'task',skillId:run.skill,status},task:{status:'open',labels:['#'+state.stage]}}));return
  }
  if(url.pathname==='/desktop/conversation'){
   const run=state.runs.find(item=>item.id===id)
   if(!run){res.writeHead(404).end('{}');return}
   const events=[{kind:'assistant',text:id==='chat'?'Specification written.':'Implementing.'}]
   res.end(JSON.stringify({id,provider,events:events.map(event=>JSON.stringify(event)),version:1,busy:false,readOnly:state.readOnly.has(id),approvals:id==='chat'?state.approvals:[]}));return
  }
  if(url.pathname==='/desktop/stop'){
   state.stops.push(id);state.log.push('stop:'+id);state.readOnly.add(id)
   const run=state.runs.find(item=>item.id===id);if(run)run.status='canceled'
   res.writeHead(204).end();return
  }
  if(url.pathname==='/desktop/tasks'){
   if(req.method==='POST'){
    let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
     const body=JSON.parse(raw);state.launches.push(body);state.log.push('launch:'+body.skillID)
     // The answer comes a moment later, so the launch is seen in flight.
     setTimeout(()=>{
      if(state.refuse){res.writeHead(409);res.end(JSON.stringify({error:'A run of implement is still active on this task.',activeRunId:'elsewhere'}));return}
      state.runs.push({id:'next-'+state.launches.length,taskId:'task',taskKey:'#7',projectId:'project',kind:'task',provider,skill:body.skillID,conversation:true,headless:true,status:'running',directory:'/tmp/project',createdAt:'2026-10-28T10:00:00Z'})
      res.end(JSON.stringify({status:'running'}))
     },400)
    });return
   }
   res.end(JSON.stringify([{id:'task',key:'#7',title:'Offer the next step',status:'open',labels:['#'+state.stage]}]));return
  }
  res.writeHead(404).end('{}')
 })
 return new Promise(resolve=>server.listen(0,'127.0.0.1',()=>resolve({server,state})))
}

for(const [provider,name] of [['claude','Claude Code'],['codex','Codex'],['agy','Antigravity']]){
 test(name+' conversation offers the next step once its skill is done and hands over to a fresh conversation',async()=>{
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-conversation-next-step-'))
  fs.writeFileSync(path.join(root,'desktop.json'),JSON.stringify({consoleView:'conversation'}))
  const {server,state}=await agent(provider)
  fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
  const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
  let app
  try{
   app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
   const page=await app.firstWindow();page.setDefaultTimeout(10000)
   const offer=page.getByRole('group',{name:'Next workflow step'}),launch=offer.getByRole('button')
   const badge=page.locator('#skill-result'),status=page.locator('.conversation-status')
   const toolbar=()=>page.evaluate(()=>{const button=document.querySelector('#next-step');return [button.hidden,button.disabled,button.getAttribute('aria-label'),document.querySelector('#next-step-status').textContent,document.querySelector('#pickup-chain').disabled]})
   await expect(page.locator('.conversation-assistant')).toContainText('Specification written.')
   await expect(status).toHaveText('Ready')

   // While the skill runs, nothing is offered.
   await expect(page.locator('#next-step-status')).toContainText('Conversation in progress')
   await expect(offer).toHaveCount(0)

   // A completed skill whose task has not reached its stage yet offers nothing (AC3).
   state.activity='completed'
   await expect(badge).toHaveAttribute('data-kind','verify')
   await expect(offer).toHaveCount(0)

   // Nor does a conversation waiting for a tool approval (AC4).
   state.approvals=[{id:'ask-1',tool:'Bash',toolUseId:'t-1',input:{command:'make test'}}]
   state.stage='specified'
   await expect(status).toHaveText('Waiting for your approval')
   await expect(badge).toHaveAttribute('data-kind','done')
   await page.waitForTimeout(1500)
   await expect(offer).toHaveCount(0)
   await expect(page.locator('#next-step-status')).toContainText('specified')
   const withoutOffer=await toolbar()

   // Once it is idle again, the next step is offered after the last message (AC1).
   state.approvals=[]
   await expect(launch).toHaveText('Next: Implement')
   await expect(offer).toContainText('Starts a new conversation for this step and closes this one.')
   assert.equal(await page.locator('.conversation-events > :last-child').getAttribute('class'),'conversation-offer')
   // The toolbar reads the same with and without the offer (AC8).
   assert.deepEqual(await toolbar(),withoutOffer)

   // A refused launch keeps the predecessor open and selected, says why
   // beside the offer and leaves forcing to the toolbar (AC5).
   await launch.click()
   await expect(launch).toBeDisabled()
   await expect(offer).toContainText('Launching Implement…')
   await expect(offer.getByRole('status')).toHaveText('A run is already active on this task.')
   await expect(launch).toBeEnabled()
   await expect(page.locator('#force-next-step')).toBeVisible()
   await expect(page.locator('.conversation-assistant')).toContainText('Specification written.')
   await expect(status).toHaveText('Ready')
   assert.deepEqual(state.stops,[])
   assert.equal(state.launches.length,1)

   // An accepted launch is made once however often the offer is pressed
   // (US3.3), in the conversation view, and only then is the predecessor
   // closed (FR4, FR5).
   state.refuse=false
   await launch.dblclick()
   await until(()=>state.stops.length===1,'the predecessor was never closed')
   assert.equal(state.launches.length,2,'a double click launches once')
   assert.equal(state.launches[1].skillID,'implement')
   assert.equal(state.launches[1].view,'conversation')
   assert.equal(state.launches[1].force,undefined)
   assert.deepEqual(state.log.slice(-2),['launch:implement','stop:chat'])
   assert.deepEqual(state.stops,['chat'])

   // The successor is selected, and the predecessor reopens as read-only history (US3.5).
   await expect(page.locator('.conversation-assistant')).toContainText('Implementing.')
   await expect(offer).toHaveCount(0)
   await page.locator('#execution-history').selectOption('chat')
   await expect(page.locator('.conversation-assistant')).toContainText('Specification written.')
   await expect(status).toHaveText('Read-only history')
   await expect(offer).toHaveCount(0)
  }finally{
   await app?.close();server.close();fs.rmSync(root,{recursive:true,force:true})
  }
 })
}
