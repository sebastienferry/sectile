// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-readiness.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-readiness.browser.mjs
//
// What it guards (#633): the row shows Sectile's suggested readiness followed
// by "?" while nobody decided; a chip click in the panel decides it and the
// badge follows; a second click clears it; a macro whose epic carries no
// labels says the level stays in Sectile; the tickets' stage badge reads
// "Tickets: ..." so it is never mistaken for the readiness.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
window.saves=[];
const meta=(key,title,extra)=>({projectId:'p1',key,title,horizon:'now',description:'',todos:[],updatedAt:'',labelsWritable:true,...extra});
window.metas=[
  meta('PE-1','Empty idea'),
  meta('PE-2','Framed epic',{description:'The framing',todos:[{id:'a',text:'Slice one',done:false}]}),
  meta('PE-3','Milestone epic',{labelsWritable:false}),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[{id:'t1',key:'PE-10',title:'A child',status:'new',priority:'medium',parentKey:'PE-3',projectId:'p1',labels:[]}],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'en',density:'standard',theme:'dark'},
  t:translations.en,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async(pid,key,patch,options)=>{
    window.saves.push({key,patch});
    const current=window.metas.find(m=>m.key===key);
    const next={...current,...patch};
    window.metas=window.metas.map(m=>m.key===key?next:m);
    return next;
  },
  addToast:noop,
  ...Object.fromEntries(['setSelectedTask','setAssigneeFilter','setMyTasksOnly','setSprintFilter','setTeamFilter','setLabelFilter','setPinnedOnly','setSearchQuery'].map(n=>[n,noop])),
};
const app=createRoot(document.getElementById('root'));
app.render(<div style={{height:'100vh',display:'flex'}}><RoadmapView/></div>);`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      if (id.includes('/src/components/') && code.includes("import { useApp } from '../context/AppContext'")) {
        return code.replace("import { useApp } from '../context/AppContext'", 'const useApp = () => window.ctx');
      }
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/roadmap') return next();
        res.setHeader('Content-Type', 'text/html');
        res.end(await s.transformIndexHtml(req.url, `<div id="root"></div><script type="module" src="/roadmap.tsx"></script>`));
      });
    },
    resolveId(id) { if (id === '/roadmap.tsx') return root + id; },
    load(id) { if (id === root + '/roadmap.tsx') return harness; },
  }],
});
await server.listen();
const base = `http://127.0.0.1:${server.httpServer.address().port}`;

let browser;
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' });
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } });
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  await page.goto(`${base}/roadmap`);
  await page.getByText('Empty idea').first().waitFor();

  const lastSave = () => page.evaluate(() => window.saves[window.saves.length - 1] || null);
  const rowOf = key => page.locator('div.rounded-xl.border').filter({ has: page.locator('span', { hasText: new RegExp(`^${key}$`) }) }).first();
  const suggested = key => rowOf(key).locator('[data-readiness-suggested]');
  const decided = key => rowOf(key).locator('[data-readiness]');

  // US2: nobody decided, the row shows the suggestion followed by "?".
  assert.equal((await suggested('PE-1').textContent()).trim(), 'Idea ?');
  assert.equal((await suggested('PE-2').textContent()).trim(), 'Shaping ?');
  assert.equal((await suggested('PE-3').textContent()).trim(), 'Ready ?', 'an epic with a ticket is suggested ready');
  assert.match(await suggested('PE-1').getAttribute('title'), /^Not decided/);

  // AC5: the tickets' stage badge is prefixed and explained.
  const stage = rowOf('PE-3').getByText(/^Tickets: /);
  await stage.waitFor();
  assert.equal(await stage.getAttribute('title'), 'Stage reached by the least advanced open ticket');

  // US3: a chip click decides the level, and the badge follows.
  await rowOf('PE-2').click();
  const chips = page.getByRole('group', { name: 'Readiness' });
  const suggestedChip = chips.getByRole('button', { name: 'Shaping ?' });
  await suggestedChip.waitFor();
  assert.equal(await suggestedChip.getAttribute('aria-pressed'), 'false');
  await chips.getByRole('button', { name: 'Ready', exact: true }).click();
  assert.deepEqual(await lastSave(), { key: 'PE-2', patch: { readiness: 'ready' } });
  await decided('PE-2').waitFor();
  assert.equal((await decided('PE-2').textContent()).trim(), 'Ready');
  assert.equal(await suggested('PE-2').count(), 0);
  assert.equal(await chips.getByRole('button', { name: 'Ready', exact: true }).getAttribute('aria-pressed'), 'true');
  assert.ok(!(await page.getByText('Kept in Sectile').isVisible()), 'a writable epic has no local-only line');

  // US3.3: a second click on the decided chip clears it, and the suggestion is back.
  await chips.getByRole('button', { name: 'Ready', exact: true }).click();
  assert.deepEqual(await lastSave(), { key: 'PE-2', patch: { readiness: '' } });
  await suggested('PE-2').waitFor();
  assert.equal((await suggested('PE-2').textContent()).trim(), 'Shaping ?');

  // US4: an epic that carries no labels keeps the level in Sectile and says so.
  await rowOf('PE-3').click();
  await page.getByText('Kept in Sectile').waitFor();
  await chips.getByRole('button', { name: 'Idea', exact: true }).click();
  assert.deepEqual(await lastSave(), { key: 'PE-3', patch: { readiness: 'idea' } });
  await decided('PE-3').waitFor();
  assert.equal((await decided('PE-3').textContent()).trim(), 'Idea');

  assert.deepEqual(errors, []);
  console.log('roadmap-readiness: ok');
} finally {
  await browser?.close();
  await server.close();
}
