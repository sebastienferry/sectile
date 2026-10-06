// Isolated browser regression (#672): "AI refine" launches the refine-macro skill on the local agent, as "Realign the
// spec" launches realign-macro. Both buttons share the macro's runs: a run shows on its own skill's button, disables
// the other, and its end makes the panel read the macros again.
// Real RoadmapView, its children and styles; useApp is mocked in every component, and fetch answers the agent, the
// session and the macro run routes.
// Run with Playwright available: node tests/roadmap-macro-skills.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const project={id:'p',name:'Platform',issueTracker:'github',sprints:[]};
const meta=(key,title,description)=>({projectId:'p',key,title,horizon:'now',description,framingComment:'',todos:[],updatedAt:'',status:'open',closed:false});
window.macros=[meta('M-1','Framed macro','Le problème et le périmètre'),meta('M-2','Blank macro','')];
// No agent at all with ?noagent; otherwise one of mine serving every project.
window.agents=location.search.includes('noagent')?[]:[{userId:'me',projectId:'',deviceId:'d',connectedAt:'',lastPingAt:''}];
// The runs the server answers, per macro; a launch records a running one, a stop completes it.
window.runs={};
const json=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
window.fetch=async(url,init={})=>{
  const path=String(url);
  if(path==='/api/agent/status') return json({agents:window.agents});
  if(path==='/api/me') return json({userId:'me',displayName:'Me'});
  const m=path.match(/^\\/api\\/projects\\/p\\/macros\\/([^/]+)\\/(runs|run-skill|cancel-run)$/);
  if(!m) return json({error:'unexpected '+path},404);
  const key=decodeURIComponent(m[1]);
  if(m[2]==='runs') return json({runs:window.runs[key]||[]});
  const body=JSON.parse(init.body||'{}');
  window.calls.push([m[2],key,body]);
  if(m[2]==='run-skill'){
    const run={id:'r'+window.calls.length,skillName:body.skillId,status:'running',summary:'',createdAt:''};
    window.runs[key]=[run,...(window.runs[key]||[])];
    return json({runId:run.id,activity:run},202);
  }
  window.runs[key]=(window.runs[key]||[]).map(r=>r.id===body.runId?{...r,status:'canceled'}:r);
  return json({});
};
const data={tasks:[],projects:[project],currentProject:project,settings:{language:'fr'},t:translations.fr,assigneeFilter:null,myTasksOnly:false,sprintFilter:null,teamFilter:null,labelFilter:null,pinnedOnly:false,searchQuery:'',activeJobCount:0,activities:[],
  fetchProjectMacros:async()=>{window.calls.push(['fetchProjectMacros']);return window.macros.map(m=>({...m}))},pendingHorizonPushes:async()=>[],
  addToast:toast=>window.calls.push(['toast',toast.title,toast.description]),
  saveMacroMeta:async(pid,key,patch)=>{
    window.calls.push(['saveMacroMeta',key,JSON.parse(JSON.stringify(patch))]);
    const saved={...window.macros.find(m=>m.key===key),...patch};
    window.macros=window.macros.map(m=>m.key===key?saved:m);
    return saved;
  }};
window.ctx=new Proxy(data,{get:(target,key)=>key in target?target[key]:(...args)=>{window.calls.push([String(key),...args]);return Promise.resolve(null)}});
const app=createRoot(document.getElementById('root'));
app.render(<div style={{height:'800px',display:'flex'}}><RoadmapView/></div>);`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url.startsWith('/fixture')&&!req.url.startsWith('/fixture.tsx')){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:1400,height:1000}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  const base=`http://127.0.0.1:${server.httpServer.address().port}/fixture`;
  await page.goto(base);
  await page.evaluate(()=>localStorage.clear());
  await page.reload();

  const row=title=>page.locator('div.cursor-pointer').filter({hasText:title}).first();
  const calls=name=>page.evaluate(n=>calls.filter(c=>c[0]===n),name);
  const refine=()=>page.locator('[data-macro-skill="refine_macro"]');
  const realign=()=>page.locator('[data-macro-skill="realign_macro"]');
  const stops=()=>page.getByRole('button',{name:/^Arrêter le/});
  const enabled=locator=>locator.evaluate(b=>!b.disabled);
  const open=async title=>{
    await row(title).click();
    await page.getByRole('button',{name:'Framing',exact:true}).click();
    await refine().waitFor();
    // The agent status and the session are read before the buttons enable.
    await page.waitForFunction(()=>!document.querySelector('[data-macro-skill="realign_macro"]').disabled);
  };

  // US1: AI refine launches refine_macro on the local agent, and shows the run with its stop button.
  await open('Framed macro');
  assert.equal((await refine().textContent()).trim(),'Raffiner AI');
  await refine().click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='run-skill'));
  assert.deepEqual((await calls('run-skill'))[0],['run-skill','M-1',{skillId:'refine_macro'}]);
  await page.waitForFunction(()=>calls.some(c=>c[0]==='toast'&&c[1]==='Raffinage lancé'));
  await page.waitForFunction(()=>document.querySelector('[data-macro-skill="refine_macro"]').dataset.macroRun==='running');
  assert.equal((await refine().textContent()).trim(),'Raffinage en cours');

  // US3: the running refinement disables Realign with its reason, and only Refine shows a stop button.
  assert.equal(await enabled(realign()),false);
  assert.equal(await realign().getAttribute('title'),'Une autre compétence est en cours sur cette macro.');
  assert.equal(await realign().getAttribute('data-macro-run'),null);
  assert.equal(await stops().count(),1);
  assert.equal(await stops().getAttribute('aria-label'),'Arrêter le raffinage');

  // US4: stopping ends the run, and its end reads the macros again.
  const readsBefore=(await calls('fetchProjectMacros')).length;
  await stops().click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='cancel-run'));
  await page.waitForFunction(n=>calls.filter(c=>c[0]==='fetchProjectMacros').length>n,readsBefore);
  await page.waitForFunction(()=>!document.querySelector('[data-macro-skill="refine_macro"]').dataset.macroRun);
  assert.equal(await stops().count(),0);

  // US3: the reverse, with a realignment recorded under an alias: it shows on Realign, Refine is disabled.
  await page.evaluate(()=>{window.runs['M-1']=[{id:'x',skillName:'sectile:realign-macro',status:'running',summary:'',createdAt:''}]});
  await page.evaluate(()=>{window.calls=[]});
  await row('Blank macro').click();
  await row('Framed macro').click();
  await page.waitForFunction(()=>document.querySelector('[data-macro-skill="realign_macro"]')?.dataset.macroRun==='running');
  assert.equal((await realign().textContent()).trim(),'Réalignement en cours');
  assert.equal(await enabled(refine()),false);
  assert.equal(await refine().getAttribute('title'),'Une autre compétence est en cours sur cette macro.');
  assert.equal(await stops().count(),1);
  assert.equal(await stops().getAttribute('aria-label'),'Arrêter le réalignement');

  // US3: a run of a skill no button launches shows on Realign, named, and stays stoppable.
  await page.evaluate(()=>{window.runs['M-1']=[{id:'y',skillName:'pickup',status:'running',summary:'',createdAt:''}]});
  await row('Blank macro').click();
  await row('Framed macro').click();
  await page.waitForFunction(()=>document.querySelector('[data-macro-skill="realign_macro"]')?.textContent.trim()==='pickup en cours');
  assert.equal(await enabled(refine()),false);
  assert.equal(await enabled(realign()),false);
  assert.equal(await refine().getAttribute('data-macro-run'),null);
  assert.equal(await stops().count(),1);
  await page.evaluate(()=>{window.runs['M-1']=[]});

  // US1: a blank framing warns and launches nothing.
  await page.evaluate(()=>{window.calls=[]});
  await open('Blank macro');
  await refine().click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='toast'));
  assert.equal((await calls('toast'))[0][1],'Cadrage requis');
  assert.equal((await calls('run-skill')).length,0);

  // US1: a dirty draft is saved before the launch.
  await page.getByPlaceholder(/Le problème, le périmètre/).fill('Un cadrage écrit à la main');
  await refine().click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='run-skill'));
  const order=await page.evaluate(()=>calls.filter(c=>c[0]==='saveMacroMeta'||c[0]==='run-skill').map(c=>c[0]));
  assert.deepEqual(order,['saveMacroMeta','run-skill']);
  assert.deepEqual((await calls('saveMacroMeta'))[0],['saveMacroMeta','M-2',{description:'Un cadrage écrit à la main'}]);

  // US5: nothing is left of the line-by-line preview.
  assert.equal(await page.getByText('Raffinage de la macro').count(),0);

  // US2: without an agent, AI refine says why it cannot launch, as Realign does.
  await page.goto(base+'?noagent');
  await row('Framed macro').click();
  await page.getByRole('button',{name:'Framing',exact:true}).click();
  await refine().waitFor();
  await page.waitForFunction(()=>document.querySelector('[data-macro-skill="refine_macro"]').title.includes('agent local'));
  assert.equal(await enabled(refine()),false);
  assert.equal(await refine().getAttribute('title'),"Connectez l'agent local pour lancer le raffinage.");
  assert.equal(await realign().getAttribute('title'),"Connectez l'agent local pour lancer le réalignement.");

  assert.deepEqual(errors,[]);
  console.log('roadmap-macro-skills.browser: ok');
} finally {
  await browser?.close();
  await server.close();
}
