// Isolated browser regression: real TaskCard and styles, mocked useApp operations.
// Run with Playwright available: node tests/condensed-card.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const root=fileURLToPath(new URL('..', import.meta.url)).replace(/\/$/, '');
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {TaskCard} from '/src/components/TaskCard.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.calls=[];
const spy=(name)=>(...args)=>{window.calls.push([name,...args.map(x=>x?.id||x)])};
window.ctx={settings:{density:'compact'}, projects:[],activities:[], t:translations.fr, skillLabel:x=>x,isPinned:()=>false, parentFilter:null, advanceTask:async(...args)=>{spy('advance')(...args);await new Promise(r=>setTimeout(r,500));},...Object.fromEntries(['setSelectedTask','togglePin','setParentFilter','setChatTask','runSkill','setIsTerminalPanelOpen'].map(n=>[n,spy(n)]))};
window.task={id:'fixture',key:'#39',title:'A long title '.repeat(20),projectId:'p',status:'to_clarify',priority:'high',source:'github',externalUrl:'https://example.test/39',parentKey:'#1',prUrl:'https://example.test/pr/1',description:'Hidden description',labels:['new'],issueType:'Story',assignee:'Alice',branchName:'feature/39'};
const app=createRoot(document.getElementById('root'));window.render=()=>{document.body.className='density-'+window.ctx.settings.density;app.render(<div style={{width:280,margin:20}}><TaskCard task={{...window.task}}/></div>)};window.render();`;
const server=await createServer({root,configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.endsWith('/TaskCard.tsx')) return code.replace("import { useApp } from '../context/AppContext'","const useApp = () => window.ctx");},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
browser=await chromium.launch({headless:true,channel:'chrome'});const page=await browser.newPage({viewport:{width:900,height:700}});page.setDefaultTimeout(10000);let errors=[];page.on('pageerror',e=>errors.push(e.message));await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);await page.getByRole('button',{name:'Actions',exact:true}).waitFor();
const title=page.locator('button[title]').filter({hasText:'A long title'});
assert.equal(await page.getByText('Hidden description').count(),0);
assert.equal(await title.evaluate(e=>getComputedStyle(e).textOverflow),'ellipsis');
assert(await title.evaluate(e=>e.scrollWidth>e.clientWidth));
await title.focus();await page.keyboard.press('Enter');assert.equal(await page.evaluate(()=>calls.filter(c=>c[0]==='setSelectedTask').length),1);
await page.getByRole('button',{name:'Actions',exact:true}).click();await page.getByRole('button',{name:'Épingler',exact:true}).click();assert.equal(await page.evaluate(()=>calls.filter(c=>c[0]==='togglePin').length),1);
await page.getByRole('button',{name:'Actions',exact:true}).click();await page.keyboard.press('Escape');assert.equal(await page.getByRole('button',{name:'Actions',exact:true}).evaluate(e=>e===document.activeElement),true);
await page.getByRole('button',{name:'Actions',exact:true}).click();assert.equal(await page.getByRole('link',{name:'Ouvrir la PR / MR'}).getAttribute('href'),'https://example.test/pr/1');await page.getByRole('button',{name:'Filtrer par parent #1'}).click();
assert.equal(await page.evaluate(()=>calls.filter(c=>c[0]==='setParentFilter').length),1);
await page.getByRole('button',{name:'Actions',exact:true}).click();await page.getByRole('button',{name:'Avancer une étape',exact:true}).click();await page.getByRole('button',{name:'Actions',exact:true}).click();assert(await page.getByRole('button',{name:'Avancer automatiquement'}).isDisabled());await page.keyboard.press('Escape');await page.waitForTimeout(600);
await page.getByRole('button',{name:'Actions',exact:true}).click();await page.getByRole('button',{name:'Avancer automatiquement'}).click();assert.deepEqual(await page.evaluate(()=>calls.filter(c=>c[0]==='advance')),[['advance','fixture',false],['advance','fixture',true]]);
await page.waitForTimeout(600);
await page.evaluate(()=>{ctx.activities=[{taskId:'fixture',status:'running'}];render()});assert(await page.locator('[draggable]').evaluate(e=>e.className.includes('border-indigo-500/60')));assert.equal(await page.getByText('Live',{exact:true}).count(),0);
await page.evaluate(()=>{task.status='finished';task.labels=['finished'];render()});await page.getByRole('button',{name:'Actions',exact:true}).click();assert(await page.getByRole('button',{name:'Avancer une étape'}).isDisabled());await page.keyboard.press('Escape');
for(const density of ['standard','comfortable']) {await page.evaluate(d=>{ctx.settings.density=d;render()},density);await page.getByText('Hidden description').waitFor();}
// The current link controls the icon in both detailed and compact surfaces.
for (const [state,label] of [['open','PR ouverte'],['conflicting','PR en conflits'],['merged','PR fusionnée'],['closed','PR fermée sans fusion']]) {
 await page.evaluate(state=>{ctx.settings.density='standard';task.prLinks=[{url:'https://example.test/pr/old',state:'merged'},{url:task.prUrl,state}];render()},state);
 await page.getByRole('img',{name:label,exact:true}).waitFor();
 await page.evaluate(()=>{ctx.settings.density='compact';render()});
 await page.getByRole('button',{name:'Actions',exact:true}).click();
 await page.getByRole('img',{name:label,exact:true}).waitFor();
 await page.keyboard.press('Escape');
}
await page.evaluate(()=>{ctx.settings.density='compact';task.externalUrl=undefined;task.source='local';task.parentKey=undefined;task.prUrl=undefined;render()});assert.equal(await page.getByRole('link',{name:'#39',exact:true}).count(),0);assert.equal(await page.getByText('Hidden description').count(),0);
await page.evaluate(()=>{task.status='to_clarify';task.labels=['new'];ctx.settings.density='compact';render()});
assert.deepEqual(await page.locator('[draggable]').evaluate(e=>{const d=new DataTransfer();e.dispatchEvent(new DragEvent('dragstart',{bubbles:true,dataTransfer:d}));return d.getData('text/plain')}),'fixture');
for (const zoom of [0.8,1,1.25]) {
 await page.evaluate(z=>{document.documentElement.style.zoom=z;document.documentElement.style.setProperty('--ui-zoom',z);document.body.classList.add('light');},zoom);
 await page.getByRole('button',{name:'Actions',exact:true}).click();
 await page.getByRole('button',{name:'Épingler',exact:true}).waitFor();
 await page.getByRole('button',{name:'Supprimer la tâche'}).evaluate(e=>e.scrollIntoView());
 assert(await page.getByRole('button',{name:'Supprimer la tâche'}).isVisible());
 await page.keyboard.press('Escape');
}
await page.evaluate(()=>{document.documentElement.style.zoom='1';document.documentElement.style.setProperty('--ui-zoom','1');document.body.classList.remove('light');});
if(process.env.CARD_SCREENSHOT) await page.screenshot({path:process.env.CARD_SCREENSHOT});
assert.deepEqual(errors,[]);console.log('PASS: compact metadata, ellipsis, keyboard details, action isolation, pin/parent/PR, Escape focus, advance guards and arguments, finished state, activity border, detailed densities, local reference, drag payload.');
} finally {await browser?.close();await server.close();}
