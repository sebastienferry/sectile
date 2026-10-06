// Isolated browser regression (#671): the Framing / Execution / Phases / Goals switcher is a tab strip of the macro
// panel, and the list's sprint check (placement badges, the "À corriger" toggle and filter, the sprint strip) follows
// the Now and Next tabs whatever the panel shows.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-mode-tabs.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Sectile',issueTracker:'github',sprints:[]};
const meta=(key,title,horizon,extra={})=>({projectId:'p',key,title,horizon,description:'',framingComment:'',todos:[],updatedAt:'',status:'open',closed:false,...extra});
const todos=[{id:'a',text:'Premier',done:true},{id:'b',text:'Second',done:false}];
const macros=[meta('M-1','Macro un','now'),meta('M-2','Macro deux','now'),meta('M-3','Macro trois','later',{todos}),meta('M-4','Macro quatre','later',{todos})];
// An open ticket with no sprint: M-1 has one placement issue, M-2 none.
const task=(id,parentKey)=>({id,key:'#'+id,title:'Ticket '+id,status:'todo',priority:'medium',parentKey,parentTitle:'',projectId:'p',source:'github',labels:[]});
const data={tasks:[task('10','M-1'),task('11','M-3')],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description])};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve([])}});
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'700px',display:'flex'}}><RoadmapView/></div>);
window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:1400,height:900}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.evaluate(()=>localStorage.clear());
  await page.reload();

  const tabButton=name=>page.getByRole('button',{name:new RegExp('^'+name+' \\d+$')});
  const panel=()=>page.locator('aside');
  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const modeTabs=()=>page.getByRole('tab');
  const modeTab=name=>page.getByRole('tab',{name,exact:true});
  const selectedMode=()=>page.evaluate(()=>[...document.querySelectorAll('[role="tab"][aria-selected="true"]')].map(t=>t.textContent));
  // The default mode per horizon is set by an effect, after the tab's own render: wait for it.
  const expectMode=async(name,message)=>{await page.locator('[role="tab"][aria-selected="true"]').filter({hasText:new RegExp('^'+name+'$')}).waitFor().catch(()=>{});assert.deepEqual(await selectedMode(),[name],message)};
  const onlyIssues=()=>page.getByRole('button',{name:'À corriger',exact:true});
  const sprintStrip=()=>page.getByText('Sprints actifs :',{exact:true});
  const absent=async(locator,message)=>{await locator.first().waitFor({state:'detached'});assert.equal(await locator.count(),0,message)};
  const strings=await page.evaluate(()=>{const r=window.ctx.t.planning.roadmap;return {description:r.framing.descriptionHeading,compose:r.execution.compose,label:r.modes.label}});

  // US1: four tabs at the bottom of the panel header, none in the toolbar.
  await panel().waitFor();
  const tablist=panel().getByRole('tablist',{name:strings.label});
  await tablist.waitFor();
  assert.equal(await page.getByRole('tablist').count(),1,'the panel holds the only mode switcher');
  assert.deepEqual(await tablist.getByRole('tab').allTextContents(),['Framing','Execution','Phases','Objectifs']);
  await expectMode('Execution','Now opens Execution');
  assert.equal(await page.getByRole('button',{name:'Framing',exact:true}).count(),0,'no mode button is left in the toolbar');
  assert.ok(await page.evaluate(()=>{const header=document.querySelector('aside > div');return header?.lastElementChild?.getAttribute('role')==='tablist'}),'the tabs close the panel header');

  // US2 on Now: the sprint check is shown in Execution mode...
  await row('Macro un').getByText('1 à corriger').waitFor();
  await row('Macro deux').getByText('tout placé').waitFor();
  await onlyIssues().waitFor();
  await sprintStrip().waitFor();

  // ...and stays when the panel switches to Framing: only the body changes.
  await modeTab('Framing').click();
  await expectMode('Framing');
  await panel().getByText(strings.description,{exact:true}).first().waitFor();
  await absent(panel().getByText(strings.compose,{exact:true}),'the Execution body is gone');
  await row('Macro un').getByText('1 à corriger').waitFor();
  await row('Macro deux').getByText('tout placé').waitFor();
  assert.equal(await onlyIssues().count(),1,'the toggle stays in the toolbar in Framing mode');
  assert.equal(await sprintStrip().count(),1,'the sprint strip stays in Framing mode');

  // The filter applies whatever the mode.
  await onlyIssues().click();
  await absent(row('Macro deux'),'the macro with nothing to fix leaves the list');
  await row('Macro un').waitFor();
  await modeTab('Objectifs').click();
  await expectMode('Objectifs');
  assert.equal(await row('Macro deux').count(),0,'the filter still applies in Goals mode');

  // Selecting another macro keeps the mode.
  await onlyIssues().click();
  await row('Macro deux').click();
  await page.waitForFunction(()=>document.querySelector('aside span.font-mono')?.textContent==='M-2');
  await expectMode('Objectifs','a macro change keeps the mode');

  // US2 on Later: TODO counts, no toggle, no strip, whatever the mode; "À corriger" left on hides nothing.
  await onlyIssues().click();
  await tabButton('LATER').click();
  await row('Macro trois').waitFor();
  await expectMode('Framing','Later opens Framing');
  await row('Macro trois').getByText('1/2 todos').waitFor();
  assert.equal(await row('Macro trois').getByText('à corriger').count(),0,'Later has no placement badge, even with an unscheduled ticket');
  await absent(onlyIssues(),'Later has no "À corriger" toggle');
  assert.equal(await sprintStrip().count(),0,'Later has no sprint strip');
  assert.equal(await row('Macro quatre').count(),1,'the filter left on is not applied on Later');
  await modeTab('Execution').click();
  await panel().getByText(strings.compose,{exact:true}).first().waitFor();
  await row('Macro trois').getByText('1/2 todos').waitFor();
  assert.equal(await onlyIssues().count(),0,'Execution mode does not bring the toggle on Later');
  assert.equal(await sprintStrip().count(),0,'Execution mode does not bring the sprint strip on Later');

  // US3: back on Now, Execution again, and the kept filter applies again.
  await modeTab('Phases').click();
  await tabButton('NOW').click();
  await row('Macro un').waitFor();
  await expectMode('Execution','Now resets the panel to Execution');
  await absent(row('Macro deux'),'"À corriger" was kept across tabs');
  await onlyIssues().click();
  await row('Macro deux').waitFor();

  // The expanded panel keeps the same tabs at the same place.
  await panel().getByRole('button',{name:'Plein écran'}).click();
  await absent(tabButton('NOW'),'the expanded panel takes the whole view');
  assert.equal(await page.getByRole('tablist').count(),1);
  assert.ok(await page.evaluate(()=>document.querySelector('aside > div')?.lastElementChild?.getAttribute('role')==='tablist'),'the expanded header ends with the tabs');
  await panel().getByRole('button',{name:'Réduire'}).click();
  await tabButton('NOW').waitFor();

  // A hidden panel, or no macro selected, leaves no switcher anywhere.
  await panel().getByRole('button',{name:'Masquer le panneau'}).click();
  await absent(panel(),'the panel is hidden');
  assert.equal(await modeTabs().count(),0,'no mode tab with the panel hidden');
  assert.equal(await page.getByRole('button',{name:'Framing',exact:true}).count(),0,'and none in the toolbar');
  await page.getByRole('button',{name:'Afficher le panneau'}).click();
  await panel().waitFor();
  await page.getByRole('button',{name:/^Non classés/}).click();
  await absent(panel(),'an empty tab selects no macro');
  assert.equal(await modeTabs().count(),0,'no mode tab with no macro selected');

  assert.deepEqual(errors,[]);
  console.log('roadmap-mode-tabs: ok');
} finally {
  await browser?.close();
  await server.close();
}
