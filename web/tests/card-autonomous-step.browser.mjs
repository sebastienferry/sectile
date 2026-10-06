// Isolated browser regression (#637): at or past the project's full-chain stop stage, the card offers the next step,
// autonomously, instead of an idle full chain. Real TaskCard, its children and styles; useApp is mocked in every component.
// Run with Playwright available: node tests/card-autonomous-step.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {TaskCard} from '/src/components/TaskCard.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[]; window.condensed=false;
const spy=(name)=>(...args)=>{window.calls.push([name,...args.map(x=>x?.id||x)])};
window.project={id:'p'};
window.ctx={settings:{density:'standard'}, projects:[window.project],currentProject:window.project,activities:[], t:translations.fr, skillLabel:x=>x,skillCommand:(id,command)=>command,fetchActivities:async()=>{},isPinned:()=>false, parentFilter:null, advanceTask:async(...args)=>{spy('advance')(...args);await new Promise(r=>setTimeout(r,400));},...Object.fromEntries(['setSelectedTask','togglePin','setParentFilter','setChatTask','runSkill','setIsTerminalPanelOpen','addToast'].map(n=>[n,spy(n)]))};
window.task={id:'fixture',key:'#637',title:'Card',projectId:'p',status:'to_close',priority:'medium',source:'github',labels:['reviewed'],description:'Body'};
const app=createRoot(document.getElementById('root'));window.render=()=>{document.body.className='density-'+window.ctx.settings.density;app.render(<div style={{width:300,margin:20}}><TaskCard key={window.condensed?'c':'f'} task={{...window.task}} compact={window.condensed}/></div>)};window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/,'const useApp = () => window.ctx');},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const page=await browser.newPage({viewport:{width:900,height:700}});
  page.setDefaultTimeout(10000);
  const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
  await page.getByRole('button',{name:'Actions',exact:true}).waitFor();

  const fr=await page.evaluate(()=>ctx.t.shell.nextStep);
  const fullChain=stage=>page.getByTitle(fr[stage].autoTooltip,{exact:true});
  const autonomousStep=(skill,stage)=>page.getByRole('button',{name:`Lancer ${skill} en autonome : ${fr[stage].stepDescription}`,exact:true});
  const anyAutonomousStep=()=>page.getByRole('button',{name:/^Lancer .* en autonome/});
  const stepButton=stage=>page.getByTitle(fr[stage].stepTooltip,{exact:true});
  const show=(labels,stop,condensed=false)=>page.evaluate(([l,s,c])=>{task.labels=l;project.fullChainStopStage=s;window.condensed=c;render()},[labels,stop,condensed]);
  const advances=()=>page.evaluate(()=>calls.filter(c=>c[0]==='advance').map(c=>c.slice(0,4)));
  const clearCalls=()=>page.evaluate(()=>{calls.length=0});
  // The shortcut and the full chain share one slot: once the expected one is there, the other one is gone.
  const swapped=async(skill,stage)=>{await autonomousStep(skill,stage).waitFor();assert.equal(await fullChain(stage).count(),0,`${stage}: no full chain`)};
  const notSwapped=async stage=>{await fullChain(stage).waitFor();assert.equal(await anyAutonomousStep().count(),0,`${stage}: no autonomous step`)};

  // AC2: default stop stage (none recorded = reviewed).
  await show(['reviewed'],undefined);
  await swapped('handoff','reviewed');
  await autonomousStep('handoff','reviewed').click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='advance'));
  assert(await autonomousStep('handoff','reviewed').isDisabled(),'the shortcut is disabled while its launch is pending');
  assert(await stepButton('reviewed').isDisabled(),'the other launch controls are disabled too');
  assert.equal(await autonomousStep('handoff','reviewed').locator('.animate-spin').count(),1,'the shortcut shows the spinner');
  assert.equal(await stepButton('reviewed').locator('.animate-spin').count(),0,'the next-step button does not spin');
  await page.waitForFunction(()=>!document.querySelector('.animate-spin'));
  assert.deepEqual(await advances(),[['advance','fixture',false,'autonomous']]);
  assert.equal(await page.evaluate(()=>calls.filter(c=>c[0]==='setSelectedTask').length),0,'the shortcut does not open the task');
  await clearCalls();

  await show(['implemented'],undefined);
  await notSwapped('implemented');
  await fullChain('implemented').click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='advance'));
  await page.waitForFunction(()=>!document.querySelector('.animate-spin'));
  assert.deepEqual((await advances()).map(c=>c.slice(0,3)),[['advance','fixture',true]]);
  await clearCalls();
  await show(['reviewed'],'reviewed');
  await swapped('handoff','reviewed');

  // AC3: stop stage implemented.
  await show(['implemented'],'implemented');
  await swapped('adjust','implemented');
  await autonomousStep('adjust','implemented').click();
  await page.waitForFunction(()=>calls.some(c=>c[0]==='advance'));
  await page.waitForFunction(()=>!document.querySelector('.animate-spin'));
  assert.deepEqual(await advances(),[['advance','fixture',false,'autonomous']]);
  await clearCalls();
  await show(['reviewed'],'implemented');
  await swapped('handoff','reviewed');
  await show(['specified'],'implemented');
  await notSwapped('specified');

  // AC4: the condensed card follows the same rule; the pickup copy entry stays.
  const actions=()=>page.getByRole('button',{name:'Actions',exact:true});
  const menuEntry=name=>page.getByRole('button',{name,exact:true});
  const pickupCopy=()=>page.getByTitle('Copier la commande de chaîne autonome pour cette tâche');
  await show(['implemented'],'implemented',true);
  await actions().click();
  await menuEntry('Avancer en autonome').waitFor();
  await pickupCopy().waitFor();
  assert.equal(await menuEntry('Chaîne complète').count(),0,'a swapped condensed card has no full chain entry');
  await page.keyboard.press('Escape');
  await show(['specified'],'implemented',true);
  await actions().click();
  await menuEntry('Chaîne complète').waitFor();
  await menuEntry('Avancer en autonome').waitFor();
  await pickupCopy().waitFor();
  await page.keyboard.press('Escape');
  await show(['reviewed'],undefined,true);
  await actions().click();
  await menuEntry('Avancer en autonome').waitFor();
  assert.equal(await menuEntry('Chaîne complète').count(),0,'a reviewed condensed card has no full chain entry');
  await page.keyboard.press('Escape');

  // AC5: a finished task keeps its disabled controls.
  await show(['finished'],undefined);
  await notSwapped('finished');
  assert(await fullChain('finished').isDisabled());
  await show(['finished'],undefined,true);
  await actions().click();
  assert(await menuEntry('Chaîne complète').isDisabled());
  await page.keyboard.press('Escape');

  if(process.env.CARD_SCREENSHOT) await page.screenshot({path:process.env.CARD_SCREENSHOT});
  assert.deepEqual(errors,[]);
  console.log('PASS: autonomous step instead of the full chain at and past the stop stage (default, reviewed, implemented), launch arguments, pending guard and spinner, condensed menu, pickup copy entry, finished task.');
} finally {await browser?.close();await server.close();}
