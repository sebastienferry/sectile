const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')
const {WebSocketServer}=require('ws')

// A fake agent serving runs, a conversation and /desktop/run-folder (#676).
// answer(input) decides the reply to each attach: {status, body}.
async function fakeAgent({runs,capabilities,answer}){
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-run-folders-')),posted=[]
 const state={busy:false,readOnly:false}
 const titles=runs.filter(run=>run.taskId).map(run=>({id:run.taskId,key:run.taskKey,title:run.title,labels:['#specified']}))
 const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type','application/json')
  const url=new URL(req.url,'http://localhost')
  if(url.pathname==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities}));return}
  if(url.pathname==='/desktop/projects'){res.end(JSON.stringify([{id:'project',name:'Project',path:'/tmp/project'}]));return}
  if(url.pathname==='/desktop/project'){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(url.pathname==='/desktop/tasks'){res.end(JSON.stringify(titles));return}
  if(url.pathname==='/desktop/runs'){res.end(JSON.stringify(runs));return}
  if(url.pathname==='/desktop/conversation'&&req.method==='POST'&&!url.searchParams.get('id')){
   const chat={id:'chat',taskId:'',projectId:'project',kind:'console',provider:'claude',conversation:true,headless:true,status:'running',directory:'/tmp/project'}
   runs.push(chat);res.writeHead(201).end(JSON.stringify(chat));return
  }
  if(url.pathname==='/desktop/conversation'){
   res.end(JSON.stringify({id:'chat',events:[],version:1,busy:state.busy,readOnly:state.readOnly}));return
  }
  if(url.pathname==='/desktop/run-folder'){
   let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{
    const input=JSON.parse(body);posted.push(input)
    const reply=answer(input)
    if(reply.status>=400){res.statusCode=reply.status;res.setHeader('Content-Type','text/plain');res.end(reply.body);return}
    res.end(JSON.stringify(reply.body))
   });return
  }
  if(url.pathname==='/desktop/stop'){state.readOnly=true;state.busy=false;res.writeHead(204).end();return}
  res.end('{}')
 })
 const ws=new WebSocketServer({server});ws.on('connection',socket=>socket.send('console\r\n'))
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'test'}))
 // The conversation view is opt-in from Appearance.
 fs.writeFileSync(path.join(root,'settings.json'),JSON.stringify({consoleView:'conversation'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 const app=await electron.launch({args:[path.resolve(__dirname,'..')],env})
 const page=await app.firstWindow();page.setDefaultTimeout(8000)
 const choose=async folder=>app.evaluate(({dialog},chosen)=>{dialog.showOpenDialog=async()=>chosen?{canceled:false,filePaths:[chosen]}:{canceled:true,filePaths:[]}},folder)
 const close=async()=>{await app.close();ws.close();await new Promise(resolve=>server.close(resolve))}
 return {page,choose,posted,state,close}
}

const source={id:'source',taskId:'source',taskKey:'#1',title:'Source execution',projectId:'project',skill:'implement',status:'completed',directory:'/tmp/project',sessionId:'source'}

test('a conversation attaches a folder from its composer',async()=>{
 const agent=await fakeAgent({runs:[{...source}],capabilities:['claude-conversation','run-folders'],answer:input=>input.path==='/dup'?{status:409,body:'/dup is already attached\n'}:{body:{typed:false,appliesAt:'next-turn'}}})
 const {page,choose,posted,state}=agent
 try{
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await page.getByRole('button',{name:'Claude chat (test)',exact:true}).click()
  const add=page.locator('.conversation').getByRole('button',{name:'Add folder…',exact:true})
  const status=page.locator('.conversation-status')
  await expect(add).toBeEnabled()
  // The execution toolbar offers its own action only on a ticket discussion.
  await expect(page.locator('#add-run-folder')).toBeHidden()

  // Closing the picker changes nothing.
  await choose(null);await add.click()
  await expect(status).toHaveText('Ready')
  assert.equal(posted.length,0)

  // Claude may be working: the folder is still added, for the next turn.
  state.busy=true
  await expect(status).toHaveText('Claude Code is working…')
  await expect(add).toBeEnabled()
  await choose('/notes');await add.click()
  await expect(status).toHaveText('Attached /notes: Claude sees it from your next message')
  assert.deepEqual(posted.at(-1),{runId:'chat',path:'/notes'})
  // The outcome is not overwritten by the next poll.
  await page.waitForTimeout(1600)
  await expect(status).toHaveText('Attached /notes: Claude sees it from your next message')

  await choose('/dup');await add.click()
  await expect(status).toHaveText('/dup is already attached')

  // Read-only history offers nothing to attach to.
  await page.getByRole('button',{name:'Stop execution',exact:true}).click()
  await expect(add).toBeDisabled()
 }finally{await agent.close()}
})

test('an agent without the capability shows no add-folder action',async()=>{
 const discussion={id:'disc',taskId:'disc',taskKey:'#2',title:'Ticket discussion',projectId:'project',skill:'discuss',status:'running',directory:'/tmp/project',sessionId:'disc'}
 const agent=await fakeAgent({runs:[{...source},discussion],capabilities:['claude-conversation'],answer:()=>({body:{}})})
 const {page}=agent
 try{
  await page.locator('.run').filter({hasText:'Ticket discussion'}).click()
  await expect(page.locator('#title')).toContainText('#2')
  await expect(page.locator('#add-run-folder')).toBeHidden()
  await page.locator('.run').filter({hasText:'Source execution'}).click()
  await page.getByRole('button',{name:'Claude chat (test)',exact:true}).click()
  await expect(page.locator('.conversation-status')).toHaveText('Ready')
  await expect(page.locator('.conversation-add-folder')).toBeHidden()
 }finally{await agent.close()}
})

test('a running ticket discussion attaches a folder from its toolbar',async()=>{
 const discussion={id:'disc',taskId:'disc',taskKey:'#2',title:'Ticket discussion',projectId:'project',skill:'discuss',status:'running',directory:'/tmp/project',sessionId:'disc'}
 const skill={id:'skill',taskId:'skill',taskKey:'#3',title:'Skill run',projectId:'project',skill:'implement',status:'running',directory:'/tmp/project',sessionId:'skill'}
 const ended={id:'ended',taskId:'ended',taskKey:'#4',title:'Ended discussion',projectId:'project',skill:'discuss',status:'completed',directory:'/tmp/project',sessionId:''}
 const answers={'/now':{typed:true,appliesAt:'now'},'/later':{typed:false,appliesAt:'next-launch'},'/src/c':{mappedAs:'github.com/o/c',typed:true,appliesAt:'now'}}
 const agent=await fakeAgent({runs:[discussion,skill,ended],capabilities:['run-folders'],answer:input=>answers[input.path]?{body:answers[input.path]}:{status:409,body:input.path+' is already the project\'s local repository\n'}})
 const {page,choose,posted}=agent
 try{
  const add=page.locator('#add-run-folder'),status=page.locator('#add-run-folder-status')
  await page.locator('.run').filter({hasText:'Ticket discussion'}).click()
  await expect(add).toBeVisible()
  for(const [folder,text] of [
   ['/now','Attached /now and typed /add-dir into the session'],
   ['/later','Attached /later: the discussion sees it at its next launch'],
   ['/src/c',"/src/c is a checkout of github.com/o/c: it is now that repository's folder; typed /add-dir into the session"],
   ['/test/repo',"/test/repo is already the project's local repository"],
  ]){
   await choose(folder);await add.click()
   await expect(status).toHaveText(text)
   assert.deepEqual(posted.at(-1),{runId:'disc',path:folder})
  }
  await page.locator('.run').filter({hasText:'Skill run'}).click()
  await expect(page.locator('#title')).toContainText('#3')
  await expect(add).toBeHidden()
  await expect(status).toBeHidden()
  await page.locator('.run').filter({hasText:'Ended discussion'}).click()
  await expect(page.locator('#title')).toContainText('#4')
  await expect(add).toBeHidden()
 }finally{await agent.close()}
})
