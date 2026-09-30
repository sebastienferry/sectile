// Isolated browser regression (#634): a macro's slicing lines are selected and turned into stories in one gesture,
// the panel locks the slicing while the batch runs, then reports each line and a summary; each line shows its origin.
// Real RoadmapView, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/roadmap-batch-stories.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Sectile',issueTracker:'local',sprints:[]};
const meta=(key,title,todos)=>({projectId:'p',key,title,horizon:'now',description:'',framingComment:'',todos,updatedAt:'',status:'open',closed:false});
window.slicing=[
  {id:'a',text:'Ligne A',done:false,sourceKind:'tasks',sourceEntry:'1. Setup'},
  {id:'b',text:'Ligne B',done:true,storyKey:'#12',sourceKind:'stories',sourceEntry:'#12'},
  {id:'c',text:'Ligne C',done:true},
  {id:'d',text:'Ligne D',done:false,sourceKind:'spec'},
  {id:'e',text:'Ligne E',done:false,sourceKind:'scenarios'},
];
window.macros=[meta('M-1','Macro un',window.slicing),meta('M-2','Macro deux',[{id:'z',text:'Autre',done:false}])];
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>window.macros.map(m=>({...m})),pendingHorizonPushes:async()=>[],addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),
  // The answer is held until the test releases it, so the locked panel can be read.
  createStoriesFromMacroTodos:(...args)=>{window.calls.push(['createStoriesFromMacroTodos',...args]);return new Promise(resolve=>{window.release=resolve})}};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve(null)}});
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{height:'800px',display:'flex'}}><RoadmapView/></div>);
window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:1400,height:1000}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.evaluate(()=>localStorage.clear());
  await page.reload();

  const panel=()=>page.locator('aside');
  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const selectBox=text=>panel().getByRole('checkbox',{name:`Sélectionner « ${text} » pour la création groupée`});
  const batchButton=()=>panel().getByRole('button',{name:/^Créer les stories \(\d+\)$|^Création…$/});
  const summary='1 créée, 1 passée, 2 en échec';
  // The slicing lives in the Framing mode of the panel.
  await page.getByRole('button',{name:'Framing',exact:true}).click();
  await panel().getByText('Ligne A').waitFor();

  // US5: every line carries its origin, an unknown kind as written, the entry as tooltip.
  const badge=text=>panel().locator('span.rounded-full').filter({hasText:new RegExp('^'+text+'$')}).first();
  for (const label of ['tâches','story existante','saisie à la main','spécification','scenarios']) {
    assert.equal(await badge(label).count(),1,`badge ${label}`);
  }
  assert.equal(await badge('tâches').getAttribute('title'),'1. Setup');
  assert.equal(await badge('saisie à la main').getAttribute('title'),'Origine de la ligne : saisie à la main');

  // US1: only unattached lines can be selected; the button counts them; select all skips the attached line.
  assert.equal(await panel().getByRole('checkbox').count(),4,'the attached line has no selection box');
  assert.equal(await batchButton().textContent(),'Créer les stories (0)');
  assert.ok(await batchButton().isDisabled(),'nothing selected, nothing to create');
  await selectBox('Ligne A').check();
  assert.equal(await batchButton().textContent(),'Créer les stories (1)');
  await panel().getByRole('button',{name:'Tout sélectionner'}).click();
  assert.equal(await batchButton().textContent(),'Créer les stories (4)');
  await panel().getByRole('button',{name:'Tout désélectionner'}).waitFor();

  // US4.5: while the batch runs, nothing of the slicing can be edited.
  await batchButton().click();
  await panel().getByRole('button',{name:'Création…'}).waitFor();
  const [sent]=await calls('createStoriesFromMacroTodos');
  assert.deepEqual(sent.slice(1,3),['p','M-1']);
  assert.deepEqual([...sent[3]].sort(),['a','c','d','e']);
  const editable=await panel().evaluate(aside=>{
    const heading=[...aside.querySelectorAll('div')].find(d=>/^Checklist TODOs/.test(d.textContent||'')&&d.children.length===0);
    const section=heading.parentElement.parentElement;
    return [...section.querySelectorAll('button, input, select')].filter(el=>!el.disabled).map(el=>el.getAttribute('aria-label')||el.title||el.textContent);
  });
  // The story key of the attached line opens its ticket; it edits nothing.
  assert.deepEqual(editable,['Story créée : #12'],'every control of the slicing is locked while the batch runs');
  assert.equal((await calls('saveMacroMeta')).length,0,'nothing saved the slicing, done states included');

  // US4: the report, per line and as a summary.
  await page.evaluate(()=>{
    const todos=window.slicing.map(t=>t.id==='a'?{...t,storyKey:'#30'}:t.id==='d'?{...t,storyKey:'#20'}:t);
    window.release({macro:{...window.macros[0],todos},created:1,skipped:1,failed:2,results:[
      {todoId:'a',status:'created',storyKey:'#30',notice:'milestone M-1 non posé'},
      {todoId:'c',status:'failed',error:"le projet cible gone n'existe plus"},
      {todoId:'d',status:'skipped',storyKey:'#20'},
      {todoId:'e',status:'failed',error:'boom'},
    ]});
  });
  await panel().getByText(summary).waitFor();
  await panel().getByText("le projet cible gone n'existe plus").waitFor();
  await panel().getByText('déjà rattachée à #20').waitFor();
  await panel().getByText('milestone M-1 non posé').waitFor();
  assert.equal(await panel().getByRole('button',{name:'#30'}).count(),1,'a created line shows its key');
  assert.equal(await panel().getByRole('checkbox').count(),2,'the attached lines left the selection boxes');
  assert.equal(await panel().getByRole('checkbox',{checked:true}).count(),0,'the selection is cleared');
  assert.equal(await batchButton().textContent(),'Créer les stories (0)');

  // US1.7, US4.4: another macro starts clean, and so does coming back.
  await row('Macro deux').click();
  await panel().getByText('Autre').waitFor();
  assert.equal(await panel().getByText(summary).count(),0,'the report belongs to its macro');
  await row('Macro un').click();
  await panel().getByText('Ligne A').waitFor();
  assert.equal(await panel().getByText(summary).count(),0,'switching macro cleared the report');

  assert.deepEqual(errors,[]);
  console.log('roadmap-batch-stories.browser: ok');
} finally {
  await browser?.close();
  await server.close();
}
