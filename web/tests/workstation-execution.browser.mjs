// Real AppContext launch preparation and recovery controls; only API responses are fixtures.
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || '../..//desktop/node_modules/playwright/index.mjs')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {AppProvider,useApp} from '/src/context/AppContext.tsx';
import {WorkstationPicker} from '/src/components/WorkstationPicker.tsx';
import {RunProjectPicker} from '/src/components/RunProjectPicker.tsx';
import {ExecutionRecovery} from '/src/components/ExecutionRecovery.tsx';
import {WorkstationAttribution} from '/src/components/WorkstationAttribution.tsx';
import '/src/index.css';
function Fixture(){const app=useApp();window.app=app;return <><button onClick={()=>app.runSkill('one','implement', 'keep prompt',{projectId:'p',mode:'autonomous',model:'sonnet',withComments:true})}>Launch</button><button onClick={()=>app.retryActivity('past')}>Retry</button><button onClick={()=>app.startBatchPickup(['one','two'])}>Batch</button><WorkstationPicker/><RunProjectPicker/>{app.activities.map(run=><div key={run.id}><WorkstationAttribution run={run}/><ExecutionRecovery run={run}/></div>)}</>}
createRoot(document.getElementById('root')).render(<AppProvider><Fixture/></AppProvider>);`
const server = await createServer({
 root, resolve:{preserveSymlinks}, configFile:root+'/vite.config.ts', server:{port:0,host:'127.0.0.1'},
 plugins:[{name:'workstation-fixture',enforce:'pre',
  transform(code,id){if(id.endsWith('/useCurrentUser.ts'))return 'export const useCurrentUser=()=>({user:{userId:"owner",role:"member",signedIn:true},loading:false,reload:async()=>{}})'},
  configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url!='/fixture')return next();res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))})},
  resolveId(id){if(id==='/fixture.tsx'||id===root+'/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}
 }]
})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1400,height:1000}})
 page.setDefaultTimeout(10000)
 const errors=[];page.on('pageerror',error=>{errors.push(error.message);console.error(error.stack)})
 await page.addInitScript(()=>{localStorage.setItem('sectile_selected_project_id','p');window.EventSource=class{addEventListener(){}close(){}};window.confirm=()=>true})
 const task=id=>({id,key:'#'+id,title:id,projectId:'p',projectIds:['p'],status:'to_implement',labels:['#specified'],priority:'medium',createdAt:'2026-10-11T12:00:00Z'})
 const tasks=[task('one'),task('two')]
 const stamp='2026-10-11T12:00:00Z'
 let devices=[{deviceId:'a',label:'Mac'},{deviceId:'b',label:'Mac'}]
 let online=true, deny=false, unconfirmed=false
 let activities=[{id:'past',taskId:'one',runProjectId:'p',skillId:'remote_run',skillName:'implement',status:'completed',action:'Agent-owned remote execution',deviceId:'a',deviceLabel:'Historical Mac',userId:'owner',summary:'',createdAt:stamp,steps:[]}]
 const launches=[],retries=[],cancels=[]
 await page.route('**/api/**',async route=>{
  const req=route.request(),url=new URL(req.url());let body=[];let status=200
  if(url.pathname==='/api/me')body={userId:'owner',role:'member',signedIn:true}
  else if(url.pathname==='/api/projects')body=[{id:'p',slug:'p',name:'Project',isDefault:true,enabledViews:[],issueTracker:'github'}]
  else if(url.pathname==='/api/tasks')body=tasks
  else if(url.pathname==='/api/skills')body=[]
  else if(url.pathname==='/api/settings')body={language:'en',userName:'Owner'}
  else if(url.pathname==='/api/agent/status')body={availableWorkstations:devices,agents:online?devices.map(d=>({...d,userId:'owner'})):[]}
  else if(url.pathname==='/api/activities')body=activities
  else if(/stats|settings|status|facets/.test(url.pathname))body={}
  else if(url.pathname.endsWith('/run-skill')){
   const input=req.postDataJSON();launches.push(input)
   if(!devices.some(d=>d.deviceId===input.deviceId)){status=400;body={error:'Chosen workstation disappeared'}}
   else body={task:tasks[0],activity:{id:'launched',taskId:'one',skillId:'agent_launch',status:'completed',steps:[],createdAt:stamp}}
  } else if(url.pathname.endsWith('/retry')){retries.push(req.postDataJSON());body={}}
  else if(url.pathname.endsWith('/cancel')){
   const input=req.postDataJSON();cancels.push(input)
   if(deny){status=403;body={error:'Not the owner'}}
   else if(unconfirmed&&!input.force){status=502;body={error:'Process stop unconfirmed'}}
   else {activities=activities.map(a=>({...a,status:'canceled'}));body={}}
  }
  await route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)})
 })
 const url=`http://127.0.0.1:${server.httpServer.address().port}/fixture`
 await page.goto(url)
 await page.waitForFunction(()=>window.app?.tasks.length===2)
 await page.getByRole('button',{name:'Launch',exact:true}).click()
 const picker=page.locator('[data-workstation-picker]')
 await picker.waitFor()
 if(process.env.WORKSTATION_SCREENSHOT)await page.screenshot({path:process.env.WORKSTATION_SCREENSHOT})
 assert.equal(launches.length,0)
 assert.equal(await page.evaluate(()=>document.activeElement?.getAttribute('data-workstation')),'a')
 await page.keyboard.press('Shift+Tab')
 assert.equal(await page.evaluate(()=>document.activeElement?.textContent),'Cancel')
 await page.keyboard.press('Tab')
 assert.equal(await page.evaluate(()=>document.activeElement?.getAttribute('data-workstation')),'a')
 await picker.getByRole('button',{name:'Mac (b)',exact:true}).click()
 await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches[0].deviceId,'b')
 assert.equal(launches[0].projectId,'p')
 assert.equal(launches[0].prompt,'keep prompt')
 assert.equal(launches[0].mode,'autonomous')
 assert.equal(launches[0].model,'sonnet')
 assert.equal(launches[0].withComments,true)
 await page.getByRole('button',{name:'Launch',exact:true}).click();await picker.waitFor();await page.keyboard.press('Escape');await picker.waitFor({state:'detached'})
 assert.equal(launches.length,1,'dismissal makes no launch request')
 await page.getByRole('button',{name:'Launch',exact:true}).click();await picker.waitFor()
 devices=[{deviceId:'a',label:'Only Mac'}]
 await picker.getByRole('button',{name:'Refresh',exact:true}).click()
 await picker.getByRole('button',{name:'Only Mac',exact:true}).click()
 await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches.at(-1).deviceId,'a','refresh replaces a vanished choice')
 await page.getByRole('button',{name:'Launch',exact:true}).click();await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches.length,3,'one device launches automatically')
 devices=[]
 await page.getByRole('button',{name:'Launch',exact:true}).click();await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches.length,3,'zero devices sends nothing')
 devices=[{deviceId:'b',label:'New Mac'}]
 await page.getByRole('button',{name:'Retry',exact:true}).click()
 await page.waitForFunction(()=>app.activities.length>0)
 await page.waitForTimeout(150)
 assert.deepEqual(retries.at(-1),{projectId:'p',deviceId:'b'})
 // Attribution is a durable snapshot; historical devices are never marked unavailable.
 await page.reload();await page.getByText('Historical Mac',{exact:true}).waitFor()
 assert.equal(await page.getByText('Workstation unavailable',{exact:true}).count(),0)
 for(const state of ['queued','pending','running']){
  activities=[{...activities[0],id:'active',status:state,deviceId:'b',deviceLabel:'Executing Mac'}]
  await page.evaluate(()=>app.fetchActivities())
  const action=page.getByRole('button',{name:'Cancel Execution',exact:true})
  await action.waitFor();await action.click();await action.waitFor({state:'detached'})
 }
 activities=[{...activities[0],id:'denied',status:'running',deviceId:'b',deviceLabel:'Executing Mac'}]
 deny=true;await page.evaluate(()=>app.fetchActivities())
 await page.getByRole('button',{name:'Cancel Execution',exact:true}).click()
 await page.waitForTimeout(150)
 assert.equal(await page.getByRole('button',{name:'Force close',exact:true}).count(),0,'permission refusal never offers force closure')
 assert.equal(activities[0].status,'running')
 deny=false
 activities=[{...activities[0],id:'stuck',status:'running',deviceId:'b',deviceLabel:'Executing Mac'}]
 online=false;unconfirmed=true
 await page.reload();await page.getByText('Executing Mac · Workstation unavailable',{exact:true}).waitFor()
 await page.getByRole('button',{name:'Cancel Execution',exact:true}).click()
 const force=page.getByRole('button',{name:'Force close',exact:true});await force.waitFor()
 assert.equal(activities[0].status,'running')
 await force.click();await force.waitFor({state:'detached'})
 assert.equal(cancels.at(-1).force,true)
 await page.getByRole('button',{name:'Launch',exact:true}).click();await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches.at(-1).deviceId,'b','force closure allows another launch')
 const beforeRefusal=launches.length
 devices=[{deviceId:'a',label:'Mac'},{deviceId:'b',label:'Mac'}]
 await page.getByRole('button',{name:'Launch',exact:true}).click();await picker.waitFor()
 devices=[{deviceId:'a',label:'Mac'}]
 await picker.getByRole('button',{name:'Mac (b)',exact:true}).click()
 await page.waitForFunction(()=>!app.isSkillRunning)
 assert.equal(launches.length,beforeRefusal+1,'stale selection is refused without a rerouted request')
 assert.equal(launches.at(-1).deviceId,'b')
 assert.deepEqual(errors,[])
 console.log('PASS: workstation selection, dismissal, refresh, retry, snapshots, active-state cancellation and force-close/relaunch')
} finally {await browser?.close();await server.close()}
