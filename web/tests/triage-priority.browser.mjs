// The priority column of the Triage view (#882): one dot per row after the
// title, a pick written through updateTask, a refused write leaving the dot on
// the stored level. Rendered for real against a mocked context. Run with
// PLAYWRIGHT_MODULE pointing to an installed Playwright module.
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/triage-priority-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {TriageView} from '/src/components/TriageView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.fetch=async()=>new Response(JSON.stringify({}));
window.EventSource=class {addEventListener(){} close(){}};
const noop=()=>{};
const task=(id,priority)=>({id,key:id,title:'Ticket '+id,projectId:'p',status:'to_clarify',priority,source:'github',labels:[],issueType:'Story'});
const project={id:'p',name:'PE',sprints:[]};
const language=new URLSearchParams(location.search).get('lang')||'fr';
window.updates=[];
window.refuse=false;
window.ctx={
 tasks:[task('T-1','medium'),task('T-2','medium'),task('T-3','urgent')],
 currentProject:project,projects:[project],settings:{language,density:'compact'},t:translations[language],
 hideDone:false,toggleHideDone:noop,activeJobCount:0,setSelectedTask:noop,
 parentFilter:null,setParentFilter:noop,availableParents:[],
 taskFacets:{scope:'p',sprints:[],teams:[],macros:[],noMacroCount:0,assignees:[],unassignedCount:0,trackerStatuses:[],statuses:[],sources:[],labels:[],issueTypes:[],total:3},
 fetchProjectMacros:async()=>[],searchTrackerTeams:async()=>[],searchAssignableUsers:async()=>[],
 setTaskSprint:noop,setTasksSprint:noop,setTaskTeam:noop,setTasksTeam:noop,setTaskMacro:noop,createMacro:noop,moveTasksToMacro:noop,migrateTasks:noop,startBatchPickup:noop,
 updateTask:async(id,req)=>{
  window.updates.push([id,req]);
  if(window.refuse)return;
  ctx.tasks=ctx.tasks.map(t=>t.id===id?{...t,...req}:t);render();
 },
};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<React.StrictMode><div style={{height:700}}><TriageView/></div></React.StrictMode>);render();`

const server = await createServer({
 root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},
 plugins:[{name:'triage-priority-fixture',enforce:'pre',
  transform(code,id){
   if(id.includes('/src/components/'))return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
  },
  configureServer(s){s.middlewares.use(async(req,res,next)=>{
   if(!req.url.startsWith('/fixture'))return next()
   res.setHeader('Content-Type','text/html')
   res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/triage-priority-fixture.tsx"></script>'))
  })},
  resolveId(id){if(id==='/triage-priority-fixture.tsx')return fixtureId},
  load(id){if(id===fixtureId)return harness},
 }],
})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1400,height:800}})
 page.setDefaultTimeout(8000)
 const errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/fixture`
 const headers=()=>page.locator('thead th').allTextContents()
 const row=key=>page.locator('tbody tr',{hasText:'Ticket '+key})
 const prioritySelect=key=>row(key).locator('select')
 const dotColor=key=>row(key).locator('select + span').evaluate(el=>el.style.backgroundColor)

 await page.goto(base)
 await page.getByText('Ticket T-1').waitFor()
 const fr=await headers()
 assert.equal(fr[fr.indexOf('Titre')+1],'Priorité','the priority column follows the title')
 for(const key of ['T-1','T-2','T-3'])assert.equal(await prioritySelect(key).count(),1,`one priority control on ${key}`)
 assert.equal(await prioritySelect('T-1').inputValue(),'medium')
 assert.equal(await dotColor('T-1'),'var(--status-info)','medium has the List view colour')
 assert.equal(await dotColor('T-3'),'var(--status-danger)','urgent has the List view colour')
 assert.equal(await row('T-1').locator('select + span').getAttribute('title'),'Priorité : Moyenne','the dot names its level')
 assert.deepEqual(await prioritySelect('T-1').locator('option').allTextContents(),['Urgent','Haute','Moyenne','Basse'])

 // A pick is written through the task update and recolours the dot.
 await prioritySelect('T-1').selectOption('high')
 assert.deepEqual(await page.evaluate(()=>window.updates),[['T-1',{priority:'high'}]])
 assert.equal(await dotColor('T-1'),'var(--status-warn)','the dot takes the new level')
 assert.equal(await prioritySelect('T-1').inputValue(),'high')

 // A refused write (a Jira refusal, #679) leaves the stored level shown.
 await page.evaluate(()=>{window.refuse=true})
 await prioritySelect('T-2').selectOption('urgent')
 assert.deepEqual(await page.evaluate(()=>window.updates.at(-1)),['T-2',{priority:'urgent'}])
 assert.equal(await prioritySelect('T-2').inputValue(),'medium','a refused write leaves the select on the stored level')
 assert.equal(await dotColor('T-2'),'var(--status-info)','a refused write leaves the dot unchanged')

 // The empty state still spans every column.
 await page.evaluate(()=>{ctx.tasks=[];render()})
 const emptyCell=page.locator('tbody td[colspan]')
 assert.equal(Number(await emptyCell.getAttribute('colspan')),(await headers()).length,'the empty state spans every column')

 await page.goto(base+'?lang=en')
 await page.getByText('Ticket T-1').waitFor()
 const en=await headers()
 assert.equal(en[en.indexOf('Title')+1],'Priority','the column reads in English')
 assert.equal(await row('T-1').locator('select + span').getAttribute('title'),'Priority : Medium')

 assert.deepEqual(errors,[])
 console.log('PASS: the triage shows each priority after the title, writes a pick through updateTask, and keeps the stored level on a refusal')
} finally {
 await browser?.close()
 await server.close()
}
