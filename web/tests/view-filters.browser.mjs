// The shared filter controls on the views beyond the board (#866): Triage,
// Sprint timeline, Team and Activities, each rendered for real against a mocked
// context. Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/view-filters-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {TriageView} from '/src/components/TriageView.tsx';
import {SprintTimelineView} from '/src/components/SprintTimelineView.tsx';
import {TeamView} from '/src/components/TeamView.tsx';
import {ActivitiesView} from '/src/components/ActivitiesView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.fetch=async()=>new Response(JSON.stringify({}));
window.EventSource=class {addEventListener(){} close(){}};
const set=key=>value=>{ctx[key]=value;render()};
const noop=()=>{};
const task=(id,sprint,parentKey)=>({id,key:id,title:'Ticket '+id,projectId:'p',status:'to_clarify',priority:'medium',source:'jira',labels:[],issueType:'Story',sprint,parentKey,parentTitle:parentKey?'Macro '+parentKey:undefined});
const project={id:'p',name:'PE',sprints:[{id:'1',name:'S1',state:'active'},{id:'2',name:'S2',state:'future'}]};
const activity=(id,skillId,skillName)=>({id,taskId:'T-1',taskKey:'T-1',taskTitle:'Ticket T-1',skillId,skillName,action:'Run '+skillName,status:'completed',summary:'Summary '+id,output:'',steps:[],createdAt:'2026-10-20T10:00:00Z'});
window.ctx={
 tasks:[task('T-1','S1',undefined),task('T-2',undefined,'M-1'),task('T-3',undefined,undefined)],
 currentProject:project,projects:[project],settings:{language:'fr',density:'compact'},t:translations.fr,
 hideDone:false,toggleHideDone:noop,activeJobCount:0,setSelectedTask:noop,
 parentFilter:null,setParentFilter:set('parentFilter'),availableParents:[],
 taskFacets:{scope:'p',sprints:[],teams:[],macros:[],noMacroCount:0,assignees:[],unassignedCount:0,trackerStatuses:[],statuses:[],sources:[],labels:[],issueTypes:[],total:3},
 fetchProjectMacros:async()=>[{key:'M-1',title:'Macro M-1'}],searchTrackerTeams:async()=>[],searchAssignableUsers:async()=>[],
 setTaskSprint:noop,setTasksSprint:noop,setTaskTeam:noop,setTasksTeam:noop,setTaskMacro:noop,createMacro:noop,moveTasksToMacro:noop,updateTask:noop,migrateTasks:noop,startBatchPickup:noop,
 updateProject:noop,fetchProjects:noop,refreshTasks:noop,addToast:noop,refusalToast:noop,
 teams:[{id:'core',name:'Core',taskCount:3,memberCount:2},{id:'ops',name:'Ops',taskCount:1,memberCount:1}],teamFilter:null,setTeamFilter:set('teamFilter'),
 fetchTeamWorkload:async()=>({members:[],unassigned:[],outside:[]}),refreshTeamMembers:noop,
 activities:[activity('a1','clarify','Clarify'),activity('a2','specify','Specify')],activityStats:{total:2,running:0,queued:0,completed:2,failed:0,canceled:0},
 selectedActivity:null,setSelectedActivity:noop,fetchActivities:async()=>{},fetchActivityStats:async()=>{},retryActivity:noop,deleteActivity:noop,clearCompletedActivities:noop,
 skills:[{id:'clarify',name:'Clarify',command:'/clarify-issue'},{id:'specify',name:'Specify',command:'/specify-issue'}],
};
const views={triage:TriageView,timeline:SprintTimelineView,team:TeamView,activities:ActivitiesView};
const View=views[new URLSearchParams(location.search).get('view')];
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<React.StrictMode><div style={{height:700}}><View/></div></React.StrictMode>);render();`

const server = await createServer({
 root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},
 plugins:[{name:'view-filters-fixture',enforce:'pre',
  transform(code,id){
   if(id.includes('/src/components/'))return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
  },
  configureServer(s){s.middlewares.use(async(req,res,next)=>{
   if(!req.url.startsWith('/fixture'))return next()
   res.setHeader('Content-Type','text/html')
   res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/view-filters-fixture.tsx"></script>'))
  })},
  resolveId(id){if(id==='/view-filters-fixture.tsx')return fixtureId},
  load(id){if(id===fixtureId)return harness},
 }],
})
await server.listen()
let browser
try {
 browser=await chromium.launch({headless:true,channel:'chrome'})
 const page=await browser.newPage({viewport:{width:1300,height:800}})
 page.setDefaultTimeout(8000)
 const errors=[]
 page.on('pageerror',error=>errors.push(error.message))
 const base=`http://127.0.0.1:${server.httpServer.address().port}/fixture`
 const filtersButton=()=>page.getByRole('button',{name:/^Filtres( \d+)?$/})
 const chips=()=>page.getByRole('group',{name:'Filtres actifs'})
 const pick=async(placeholder,label)=>{
  await page.getByPlaceholder(placeholder).click()
  await page.locator('[data-portaled-panel]').getByRole('button',{name:new RegExp('^'+label)}).click()
 }
 // No native <select> is a filter any more (FR2).
 const nativeSelects=()=>page.locator('select').count()

 // ---------- Triage ----------
 await page.goto(base+'?view=triage')
 await page.getByText('Ticket T-3').waitFor()
 const triageRows=async()=>(await page.getByText(/^Ticket T-\d$/).allTextContents()).sort()
 assert.deepEqual(await triageRows(),['Ticket T-1','Ticket T-2','Ticket T-3'])
 assert.equal(await nativeSelects(),0,'triage: no native select')
 await filtersButton().click()
 // The same wording as the board's panel (US2.3).
 await pick('Tous sprints','S1')
 await page.getByText('Ticket T-2').waitFor({state:'detached'})
 assert.deepEqual(await triageRows(),['Ticket T-1'],'the sprint narrows the triage as the select did')
 assert.equal((await filtersButton().innerText()).replace(/\s+/g,' '),'Filtres 1')
 await page.keyboard.press('Escape')
 await chips().getByRole('button',{name:'S1'}).click()
 await page.getByText('Ticket T-3').waitFor()
 assert.deepEqual(await triageRows(),['Ticket T-1','Ticket T-2','Ticket T-3'],'the chip clears the sprint')
 await filtersButton().click()
 await pick('Toutes macros…','Sans macro')
 await page.waitForFunction(()=>ctx.parentFilter==='__no_macro__')
 await page.getByText('Ticket T-2').waitFor({state:'detached'})
 assert.deepEqual(await triageRows(),['Ticket T-1','Ticket T-3'],'"No macro" keeps the tickets without a parent and writes the board filter')
 await page.keyboard.press('Escape')
 await page.getByRole('searchbox').fill('T-3')
 await page.getByText('Ticket T-1').waitFor({state:'detached'})
 await page.getByRole('button',{name:'Effacer la recherche'}).click()
 await page.getByText('Ticket T-1').waitFor()
 await page.evaluate(()=>ctx.setParentFilter(null))

 // ---------- Sprint timeline ----------
 await page.goto(base+'?view=timeline')
 await filtersButton().waitFor()
 await filtersButton().click()
 await pick('Toutes macros…','M-1')
 await page.waitForFunction(()=>ctx.parentFilter==='M-1')
 await page.keyboard.press('Escape')
 assert.deepEqual((await chips().getByRole('button').allInnerTexts()).map(s=>s.trim()),['M-1 · Macro M-1'],'the timeline shows its macro as a chip')
 await chips().getByRole('button',{name:'M-1 · Macro M-1'}).click()
 await page.waitForFunction(()=>ctx.parentFilter===null)
 await filtersButton().click()
 await pick('Toutes macros…','Sans macro')
 await page.waitForFunction(()=>ctx.parentFilter==='__no_macro__','"No macro" is always offered on the timeline')
 await page.keyboard.press('Escape')
 await page.evaluate(()=>ctx.setParentFilter(null))

 // ---------- Team ----------
 await page.goto(base+'?view=team')
 const team=page.getByPlaceholder('Équipe')
 await team.waitFor()
 assert.equal(await team.inputValue(),'Core','the first team is shown')
 assert.equal(await filtersButton().count(),0,'the team is the view subject: no Filters button')
 assert.equal(await nativeSelects(),0,'team: no native select')
 await pick('Équipe','Ops')
 await page.waitForFunction(()=>ctx.teamFilter==='Ops')
 assert.equal(await team.inputValue(),'Ops')
 assert.equal(await page.locator('input[placeholder="Équipe"] ~ button').count(),0,'the team picker cannot be cleared')
 assert.equal(await chips().count(),0,'the team is never a chip')

 // ---------- Activities ----------
 await page.goto(base+'?view=activities')
 await page.getByText('Summary a1').first().waitFor()
 assert.equal(await nativeSelects(),0,'activities: no native select')
 await filtersButton().click()
 await pick('Toutes les skills','Specify')
 await page.getByText('Summary a1').waitFor({state:'detached'})
 assert.ok(await page.getByText('Summary a2').first().isVisible(),'the skill narrows the list as the select did')
 await page.keyboard.press('Escape')
 await chips().getByRole('button',{name:'Specify (/specify-issue)'}).click()
 await page.getByText('Summary a1').first().waitFor()

 assert.deepEqual(errors,[])
 console.log('PASS: triage, timeline, team and activities filter through the shared controls; chips clear them; no native select is a filter')
} finally {
 await browser?.close()
 await server.close()
}
