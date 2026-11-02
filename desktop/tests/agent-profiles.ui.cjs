const {test}=require('node:test')
const assert=require('node:assert/strict')
const {_electron:electron,expect}=require('@playwright/test')
const http=require('node:http'),fs=require('node:fs'),os=require('node:os'),path=require('node:path')

test('project shortcuts create and select repository profiles independently of engines',async()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'sectile-agent-profiles-'))
 const profiles=[{id:'reviewer',name:'reviewer'}],saves=[],launches=[]
 let failSave=true,failLaunch=true,holdSave=false,pendingSave,lateList=false,pendingList,legacy=false,taskWrites=0
 const server=http.createServer((req,res)=>{
  assert.equal(req.headers.authorization,'Bearer private');res.setHeader('Content-Type','application/json')
  if(req.url==='/desktop/status'){res.end(JSON.stringify({connected:true,capabilities:['free-console','task-engines']}));return}
  if(req.url==='/desktop/projects'){res.end(JSON.stringify([{id:'p',name:'Primary',path:'/repo'},{id:'q',name:'Other',path:'/other'},{id:'missing',name:'Missing'}]));return}
  if(req.url.startsWith('/desktop/project?')){res.end(JSON.stringify({configured:true,server:{skills:[]}}));return}
  if(req.url.startsWith('/desktop/task-engines?')){res.end(JSON.stringify({projectDefault:'claude',catalogue:[{id:'claude',name:'Claude',provider:'claude'},{id:'codex',name:'Codex',provider:'codex'}]}));return}
  if(req.url==='/desktop/runs'){res.end(JSON.stringify([{id:'unavailable',kind:'console',provider:'claude',projectId:'missing',taskId:'',skill:'',directory:'',status:'failed'}]));return}
  if(req.url.startsWith('/desktop/tasks?')){if(req.method!=='GET')taskWrites++;res.end('[]');return}
  if(req.url.startsWith('/desktop/agent-profiles?')){
   if(legacy){res.writeHead(404).end('Not found');return}
   const reply=()=>res.end(JSON.stringify({profiles:req.url.includes('projectId=q')?[]:profiles,warnings:req.url.includes('projectId=q')?[]:[{id:'broken',message:'Invalid profile'}]}))
   if(lateList){pendingList=reply;return}reply();return
  }
  if(req.url==='/desktop/agent-profiles'&&req.method==='POST'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{
    const input=JSON.parse(raw);saves.push(input)
    const reply=()=>{if(failSave){res.writeHead(500).end('Read-only checkout');return}const profile={id:input.id,name:input.id};profiles.push(profile);res.writeHead(201).end(JSON.stringify({profile}))}
    if(holdSave){pendingSave=reply;return}reply()
   });return
  }
  if(req.url==='/desktop/consoles'){
   let raw='';req.on('data',chunk=>raw+=chunk);req.on('end',()=>{launches.push(JSON.parse(raw));res.writeHead(failLaunch?422:409).end(failLaunch?'Unsupported profile engine':'Project checkout unavailable')});return
  }
  res.writeHead(404).end()
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 fs.writeFileSync(path.join(root,'agent-connection.json'),JSON.stringify({url:'http://127.0.0.1:'+server.address().port,token:'private'}))
 const env={...process.env,SECTILE_DESKTOP_DATA_DIR:root,SECTILE_DESKTOP_TEST:'1'};delete env.ELECTRON_RUN_AS_NODE
 let app
 try{
  app=await electron.launch({args:[path.resolve(__dirname,'..')],env});const page=await app.firstWindow();page.setDefaultTimeout(8000)
  const open=async name=>{await page.getByRole('button',{name:'Start project prompt for '+name,exact:true}).click();await expect(page.getByLabel('Console agent')).toHaveValue('claude')}
  await expect(page.getByRole('button',{name:'Start project prompt for Missing',exact:true})).toBeDisabled()
  const heading=page.getByRole('button',{name:'Primary',exact:true});const expanded=await heading.getAttribute('aria-expanded')
  await page.getByRole('button',{name:'Start project prompt for Primary',exact:true}).focus();await page.keyboard.press('Enter')
  await expect(page.getByLabel('Agent profile')).toBeEnabled();assert.equal(await heading.getAttribute('aria-expanded'),expanded)
  await expect(page.getByLabel('Agent profile')).toHaveValue('');await expect(page.getByText('broken: Invalid profile')).toBeVisible()
  await page.getByLabel('Agent profile').selectOption('reviewer');await page.getByLabel('Console agent').selectOption('codex')
  await expect(page.getByLabel('Agent profile')).toHaveValue('reviewer')
  await page.getByRole('button',{name:'Create profile',exact:true}).click();await expect(page.getByLabel('Profile name')).toBeFocused()
  await page.getByLabel('Profile name').fill('new-agent');await page.getByLabel('Reusable agent instructions').fill('Review carefully\nKeep repository instructions.')
  await page.screenshot({path:path.join(root,'agent-profile-create.png')})
  await page.getByRole('button',{name:'Save profile',exact:true}).click();await expect(page.getByText('Not saved:',{exact:false})).toBeVisible()
  await expect(page.getByLabel('Profile name')).toHaveValue('new-agent');await expect(page.getByLabel('Reusable agent instructions')).toHaveValue('Review carefully\nKeep repository instructions.')
  assert.equal(launches.length,0)
  failSave=false;holdSave=true
  await page.getByRole('button',{name:'Save profile',exact:true}).click();await expect.poll(()=>saves.length).toBe(2)
  await expect(page.getByRole('button',{name:'Save profile',exact:true})).toBeDisabled();pendingSave();holdSave=false
  await expect(page.getByLabel('Agent profile')).toHaveValue('new-agent');assert.equal(launches.length,0)
  await page.screenshot({path:path.join(root,'agent-profile-select.png')})
  await page.getByRole('button',{name:'Open console',exact:true}).click();await expect(page.getByText('Unsupported profile engine',{exact:false})).toBeVisible()
  assert.deepEqual(launches[0],{projectId:'p',engineId:'codex',agentProfileId:'new-agent'})
  failLaunch=false;await page.getByLabel('Agent profile').selectOption('');await page.getByRole('button',{name:'Open console',exact:true}).click()
  await expect(page.getByText('Project checkout unavailable',{exact:false})).toBeVisible();assert.equal('agentProfileId' in launches[1],false)
  await page.getByRole('button',{name:'Create profile',exact:true}).click();await page.getByLabel('Profile name').fill('canceled');await page.keyboard.press('Escape')
  await expect(page.getByRole('button',{name:'Create profile',exact:true})).toBeFocused();assert.equal(saves.length,2)
  await page.locator('#close-dialog').click();lateList=true;await open('Primary');await expect.poll(()=>!!pendingList).toBe(true)
  await page.locator('#close-dialog').click();lateList=false;await open('Other');await expect(page.getByLabel('Agent profile')).toBeEnabled();pendingList()
  await expect(page.getByLabel('Agent profile').locator('option')).toHaveCount(1)
  await page.locator('#close-dialog').click();legacy=true;await open('Primary')
  await expect(page.getByText('Profiles unavailable:',{exact:false})).toBeVisible();await expect(page.getByRole('button',{name:'Create profile',exact:true})).toBeDisabled()
  await expect(page.getByRole('button',{name:'Open console',exact:true})).toBeEnabled()
  assert.equal(taskWrites,0);assert.equal(saves[0].projectId,'p')
  await page.screenshot({path:path.join(root,'agent-profiles.png')});console.log('Agent profiles screenshot: '+path.join(root,'agent-profiles.png'))
 }finally{if(app)await app.close();await new Promise(resolve=>server.close(resolve))}
})
