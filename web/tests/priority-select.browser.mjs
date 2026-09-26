// Isolated browser regression: real PrioritySelect and styles, mocked useApp.
// Run with Playwright available: node tests/priority-select.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const {root,preserveSymlinks}=browserRoot(import.meta.url);
const harness=`import React from 'react'; import {createRoot} from 'react-dom/client'; import {PrioritySelect} from '/src/components/PrioritySelect.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.ctx={t:translations.fr}; window.changes=[]; window.value='high';
const app=createRoot(document.getElementById('root'));window.render=()=>app.render(<div style={{width:240,margin:20}}><label>Priorité<PrioritySelect value={window.value} onChange={p=>{window.changes.push(p);window.value=p;window.render()}} className="w-full px-2.5 py-1.5 text-xs"/></label></div>);window.render();`;
const server=await createServer({root,resolve:{preserveSymlinks},configFile:root+'/vite.config.ts',server:{port:0,host:'127.0.0.1'},plugins:[{name:'fixture',enforce:'pre',transform(code,id){if(id.endsWith('/PrioritySelect.tsx')) return code.replace("import { useApp } from '../context/AppContext'","const useApp = () => window.ctx");},configureServer(s){s.middlewares.use(async(req,res,next)=>{if(req.url==='/fixture'){res.setHeader('Content-Type','text/html');res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))}else next()})},resolveId(id){if(id==='/fixture.tsx')return root+'/fixture.tsx'},load(id){if(id===root+'/fixture.tsx')return harness}}]});
await server.listen();
let browser;
try {
browser=await chromium.launch({headless:true,channel:'chrome'});const page=await browser.newPage({viewport:{width:600,height:300}});page.setDefaultTimeout(10000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`);
const select=page.locator('select');await select.waitFor();
const dot=page.locator('[data-priority-dot]');
// The colour the dot resolves to, compared with the CSS variable it must use.
const dotMatches=async(variable)=>page.evaluate(v=>{const probe=document.createElement('span');probe.style.backgroundColor=`var(${v})`;document.body.append(probe);const want=getComputedStyle(probe).backgroundColor;probe.remove();return getComputedStyle(document.querySelector('[data-priority-dot]')).backgroundColor===want},variable);
assert.deepEqual(await select.locator('option').evaluateAll(os=>os.map(o=>[o.value,o.textContent])),[['urgent','Urgent'],['high','Haute'],['medium','Moyenne'],['low','Basse']]);
assert.equal(await select.inputValue(),'high');
assert(await dotMatches('--status-warn'),'high shows the warning colour');
assert.equal(await dot.getAttribute('aria-hidden'),'true');
assert.equal(await page.getByLabel('Priorité').evaluate(e=>e.tagName),'SELECT');
await select.selectOption('urgent');
assert(await dotMatches('--status-danger'),'urgent shows the danger colour');
await select.selectOption('high');
assert(await dotMatches('--status-warn'),'back to high');
for (const [value,variable] of [['medium','--status-info'],['low','--text-muted']]) {await select.selectOption(value);assert(await dotMatches(variable),value);}
assert.deepEqual(await page.evaluate(()=>changes),['urgent','high','medium','low']);
// The label must not run under the dot.
const [dotBox,selectBox]=[await dot.boundingBox(),await select.boundingBox()];
assert(dotBox.x>selectBox.x && dotBox.y>selectBox.y && dotBox.y+dotBox.height<selectBox.y+selectBox.height,'the dot sits inside the field');
assert(await select.evaluate(e=>parseFloat(getComputedStyle(e).paddingLeft))>=dotBox.x+dotBox.width-selectBox.x,'the label starts after the dot');
if(process.env.PRIORITY_SCREENSHOT) await page.screenshot({path:process.env.PRIORITY_SCREENSHOT});
assert.deepEqual(errors,[]);console.log('PASS: four ordered options, dot colour per level, follows every change, decorative dot, label association, dot inside the field before the label.');
} finally {await browser?.close();await server.close();}
