// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-grouping.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-grouping.browser.mjs
//
// What it guards (#628): the tab splits into priority or quarter sections,
// empty ones included; a drop sets the section's value and nothing else; a
// Ctrl click selection travels together; a drop on the epic's own section
// saves nothing; the axis and the folded sections survive a reload; the
// Hidden tab stays flat.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
window.saves=[];window.toasts=[];
const meta=(key,title,extra)=>({projectId:'p1',key,title,horizon:'now',description:'',todos:[],updatedAt:'',labelsWritable:true,...extra});
window.metas=[
  meta('PE-1','Platform AI',{priority:'p1'}),
  meta('PE-2','Grafana decision'),
  meta('PE-3','Okta rollout',{priority:'p2',quarter:'2026-Q4'}),
  meta('PE-4','Vault migration',{priority:'p2'}),
  meta('PE-5','Set aside',{horizon:'hidden',priority:'p3'}),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'fr',density:'standard',theme:'dark'},
  t:translations.fr,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async(pid,key,patch,options)=>{
    window.saves.push({key,patch,options:options||null});
    window.metas=window.metas.map(m=>m.key===key?{...m,...patch}:m);
    return window.metas.find(m=>m.key===key);
  },
  addToast:t=>window.toasts.push(t),
  ...Object.fromEntries(['setSelectedTask','setAssigneeFilter','setMyTasksOnly','setSprintFilter','setTeamFilter','setLabelFilter','setPinnedOnly','setSearchQuery'].map(n=>[n,noop])),
};
const app=createRoot(document.getElementById('root'));
app.render(<div style={{height:'100vh',display:'flex'}}><RoadmapView/></div>);`;

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'fixture', enforce: 'pre',
    transform(code, id) {
      // Only the data source is simulated: every component that reads the
      // context gets the fixture, the components under test stay the shipped ones.
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
  await page.getByText('Platform AI').first().waitFor();

  const saves = () => page.evaluate(() => window.saves);
  const lastToast = () => page.evaluate(() => window.toasts[window.toasts.length - 1] || null);
  const sectionIds = () => page.evaluate(() => [...document.querySelectorAll('[data-section]')].map(s => s.dataset.section));
  const keysIn = id => page.evaluate(id => [...document.querySelectorAll(`[data-section="${id}"] [data-epic-key]`)].map(r => r.dataset.epicKey), id);
  const row = key => page.locator(`[data-epic-key="${key}"]`);
  const header = id => page.locator(`[data-section="${id}"] > button`);
  const axis = page.getByRole('combobox', { name: 'Regrouper les macros' });

  // No grouping by default: no section, rows not draggable.
  assert.deepEqual(await sectionIds(), []);
  assert.equal(await row('PE-1').getAttribute('draggable'), null);

  // US1: priority sections, P0 to P3 and "no priority", empty ones included.
  await axis.selectOption('priority');
  await page.locator('[data-section]').first().waitFor();
  assert.deepEqual(await sectionIds(), ['priority:p0', 'priority:p1', 'priority:p2', 'priority:p3', 'priority:none']);
  assert.deepEqual(await keysIn('priority:p2'), ['PE-3', 'PE-4']);
  assert.deepEqual(await keysIn('priority:none'), ['PE-2']);
  assert.match(await header('priority:p0').textContent(), /P0\s*0$/, 'an empty section shows its count');

  // US4: dropping one epic on an empty section sets that value, and only it.
  await row('PE-2').dragTo(header('priority:p0'));
  await page.waitForFunction(() => window.saves.length === 1);
  assert.deepEqual((await saves())[0], { key: 'PE-2', patch: { priority: 'p0' }, options: { quiet: true } });
  await page.waitForFunction(() => document.querySelector('[data-section="priority:p0"] [data-epic-key="PE-2"]') !== null);
  assert.deepEqual(await lastToast(), { type: 'success', title: 'Macros déplacées', description: '1 macro mise à jour' });

  // A drop on the section the epic comes from saves nothing and says nothing.
  const toastsBefore = await page.evaluate(() => window.toasts.length);
  await row('PE-2').dragTo(header('priority:p0'));
  await page.waitForTimeout(200);
  assert.equal((await saves()).length, 1);
  assert.equal(await page.evaluate(() => window.toasts.length), toastsBefore);

  // US5: a Ctrl click selection travels together; a plain click keeps it.
  await row('PE-1').click({ modifiers: ['ControlOrMeta'] });
  await row('PE-3').click({ modifiers: ['ControlOrMeta'] });
  await page.getByText('2 macros sélectionnées').waitFor();
  assert.equal(await row('PE-1').getAttribute('aria-selected'), 'true');
  await row('PE-4').click();
  await page.getByText('2 macros sélectionnées').waitFor();
  await row('PE-3').dragTo(header('priority:none'));
  await page.waitForFunction(() => window.saves.length === 3);
  assert.deepEqual((await saves()).slice(1), [
    { key: 'PE-1', patch: { priority: '' }, options: { quiet: true, bulk: true } },
    { key: 'PE-3', patch: { priority: '' }, options: { quiet: true, bulk: true } },
  ], 'a drop of several epics is a bulk edit');
  await page.waitForFunction(() => document.querySelector('[data-epic-selection]') === null);

  // A selection mixing moved and already-placed epics reports both; the one
  // that did not move stays selected, and Escape clears it.
  await row('PE-1').click({ modifiers: ['ControlOrMeta'] });
  await row('PE-4').click({ modifiers: ['ControlOrMeta'] });
  await row('PE-4').dragTo(header('priority:p2'));
  await page.waitForFunction(() => window.saves.length === 4);
  assert.equal((await saves())[3].key, 'PE-1');
  assert.deepEqual(await lastToast(), { type: 'success', title: 'Macros déplacées', description: '1 macro mise à jour, 1 déjà à cette valeur' });
  await page.getByText('1 macro sélectionnée').waitFor();
  await page.keyboard.press('Escape');
  await page.waitForFunction(() => document.querySelector('[data-epic-selection]') === null);

  // Shift click selects the range in display order: PE-2 (P0), PE-1 and PE-4 (P2).
  await row('PE-2').click({ modifiers: ['ControlOrMeta'] });
  await row('PE-4').click({ modifiers: ['Shift'] });
  await page.getByText('3 macros sélectionnées').waitFor();

  // US3: fold a section, whose epics leave the selection; the axis and the
  // fold survive a reload.
  await header('priority:p2').click();
  await page.getByText('1 macro sélectionnée').waitFor();
  await page.reload();
  await page.locator('[data-section]').first().waitFor();
  assert.equal(await axis.inputValue(), 'priority');
  assert.equal(await header('priority:p2').getAttribute('aria-expanded'), 'false');
  assert.deepEqual(await keysIn('priority:p2'), []);

  // US4.4: a folded section still takes a drop. The reload started the
  // fixture again, PE-2 back without a priority.
  await row('PE-2').dragTo(header('priority:p2'));
  await page.waitForFunction(() => window.saves.length === 1);
  assert.deepEqual((await saves())[0], { key: 'PE-2', patch: { priority: 'p2' }, options: { quiet: true } });

  // US2: the quarter axis, with the current quarter and the next three.
  await axis.selectOption('quarter');
  await page.locator('[data-section="quarter:none"]').waitFor();
  const ids = await sectionIds();
  assert.equal(ids.at(-1), 'quarter:none');
  assert.ok(ids.includes('quarter:2026-Q4'), 'a used quarter shows');
  assert.ok(ids.length >= 5, 'the current quarter and the next three always show');
  assert.deepEqual([...ids.slice(0, -1)].sort(), ids.slice(0, -1), 'quarters are chronological');

  // The Hidden tab stays flat.
  await page.getByRole('button', { name: /Masqu/ }).first().click();
  await page.getByText('Set aside').first().waitFor();
  assert.deepEqual(await sectionIds(), []);
  assert.equal(await row('PE-5').getAttribute('draggable'), null);

  assert.deepEqual(errors, []);
  console.log('roadmap-grouping: ok');
} finally {
  await browser?.close();
  await server.close();
}
