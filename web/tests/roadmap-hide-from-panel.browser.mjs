// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-hide-from-panel.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-hide-from-panel.browser.mjs
//
// What it guards (#808): the condensed row offers no HIDDEN, so the side panel
// header carries the NOW / NEXT / LATER / HIDDEN selector. Hiding a macro from
// it lands the macro in the Hidden tab, still selected, and the same selector
// brings it back. An unclassified macro shows its suggestion with " ?".
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
  meta('PE-1','Platform AI'),
  meta('PE-2','Okta rollout',{horizon:'later'}),
  meta('PE-3','Not yet sorted',{horizon:''}),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'en',density:'standard',theme:'dark'},
  t:translations.en,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async(pid,key,patch)=>{
    window.saves.push({key,patch});
    window.metas=window.metas.map(m=>m.key===key?{...m,...patch}:m);
    return window.metas.find(m=>m.key===key);
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
  const context = await browser.newContext({ viewport: { width: 1400, height: 900 } });
  // The condensed shape is the one whose rows leave HIDDEN out.
  await context.addInitScript(() => localStorage.setItem('sectile_roadmap_display_mode', 'condensed'));
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  const errors = [];
  page.on('pageerror', e => errors.push(e.message));

  await page.goto(`${base}/roadmap`);
  const row = key => page.locator(`[data-epic-key="${key}"]`);
  const panel = page.locator('aside');
  const panelButton = label => panel.getByRole('group', { name: 'Horizon' }).getByRole('button', { name: new RegExp(`^${label}( \\?)?$`) });
  const tabButton = name => page.getByRole('button', { name: new RegExp(`^${name} \\d+$`) });
  // The horizon tabs carry an inline font weight; the row and panel buttons a class.
  const activeTab = () => page.evaluate(() => [...document.querySelectorAll('button')].find(b => b.style.fontWeight === '700')?.textContent?.replace(/\d+$/, '').trim());
  const saves = () => page.evaluate(() => window.saves);

  await tabButton('NOW').click();
  await row('PE-1').waitFor();
  await row('PE-1').click();
  await panel.getByText('Platform AI').first().waitFor();

  // US1: the condensed row has no HIDDEN, the panel has the four horizons.
  assert.equal(await row('PE-1').getByRole('button', { name: 'HIDDEN' }).count(), 0, 'the condensed row leaves HIDDEN out');
  assert.deepEqual(
    await panel.getByRole('group', { name: 'Horizon' }).getByRole('button').allTextContents(),
    ['NOW', 'NEXT', 'LATER', 'HIDDEN'],
  );
  assert.equal(await panelButton('NOW').getAttribute('aria-pressed'), 'true');
  assert.equal(await panelButton('HIDDEN').getAttribute('aria-pressed'), 'false');

  // Hide it from the panel: one save, the Hidden tab, still selected.
  await panelButton('HIDDEN').click();
  await page.waitForFunction(() => window.saves.length === 1);
  assert.deepEqual((await saves())[0], { key: 'PE-1', patch: { horizon: 'hidden' } });
  await page.waitForFunction(() => [...document.querySelectorAll('button')].some(b => b.style.fontWeight === '700' && /^Hidden/.test(b.textContent || '')));
  assert.equal(await activeTab(), 'Hidden');
  await row('PE-1').waitFor();
  assert.equal(await panelButton('HIDDEN').getAttribute('aria-pressed'), 'true');
  assert.equal(await panelButton('NOW').getAttribute('aria-pressed'), 'false');

  // US2: bring it back from the panel of the Hidden tab.
  await panelButton('NOW').click();
  await page.waitForFunction(() => window.saves.length === 2);
  assert.deepEqual((await saves())[1], { key: 'PE-1', patch: { horizon: 'now' } });
  await page.waitForFunction(() => [...document.querySelectorAll('button')].some(b => b.style.fontWeight === '700' && /^NOW/.test(b.textContent || '')));
  await row('PE-1').waitFor();
  assert.equal(await panelButton('NOW').getAttribute('aria-pressed'), 'true');

  // US3: an unclassified macro shows its suggestion with " ?", nothing pressed.
  await tabButton('Unclassified').click();
  await row('PE-3').click();
  await panel.getByText('Not yet sorted').first().waitFor();
  const labels = await panel.getByRole('group', { name: 'Horizon' }).getByRole('button').allTextContents();
  assert.equal(labels.filter(l => l.endsWith(' ?')).length, 1, `one suggested horizon, got ${labels}`);
  assert.equal(await panel.getByRole('group', { name: 'Horizon' }).locator('[aria-pressed="true"]').count(), 0);

  assert.deepEqual(errors, []);
  console.log('roadmap-hide-from-panel: ok');
} finally {
  await browser?.close();
  await server.close();
}
