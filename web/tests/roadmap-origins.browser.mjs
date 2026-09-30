// Isolated browser regression: the real RoadmapView, mocked useApp.
// Run with Playwright available: node tests/roadmap-origins.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
// It must name the ESM entry point, not the package directory, e.g.
//   PLAYWRIGHT_MODULE=/absolute/path/to/desktop/node_modules/playwright/index.mjs node tests/roadmap-origins.browser.mjs
//
// What it guards (#632): the roadmap opens on the project's own epics; the
// toolbar offers the own key, the declared keys and any other origin still
// carried, each with its count; ticking one shows its epics and a chip; an
// empty selection falls back to the own key; the choice survives a reload; an
// epic of a roadmap project is marked read only, offers no label edit and says
// why its priority and quarter stay in Sectile.
import { createServer } from 'vite';
import { browserRoot } from './browserRoot.mjs';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
import assert from 'node:assert/strict';
const { root, preserveSymlinks } = browserRoot(import.meta.url);

const harness = `import React from 'react'; import {createRoot} from 'react-dom/client'; import {RoadmapView} from '/src/components/RoadmapView.tsx'; import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
const noop=()=>{};
const meta=(key,title,extra)=>({projectId:'p1',key,title,horizon:'now',description:'',todos:[],updatedAt:'',labelsWritable:true,axesWritable:true,labels:[],origin:key.split('-')[0],...extra});
const foreign={foreign:true,labelsWritable:false,axesWritable:false};
window.metas=[
  meta('PE-1','Billing'),
  meta('PE-2','Okta rollout'),
  meta('DATA-12','Data export',foreign),
  meta('OLD-3','Legacy import',foreign),
];
const project={id:'p1',name:'Demo',slug:'demo',color:'indigo',icon:'Folder',issueTracker:'jira',jiraProject:'PE',roadmapProjects:['DATA','OPS'],isDefault:true,githubRepo:'',description:'',repoPath:'',enabledViews:['roadmap'],sprints:[]};
window.ctx={
  tasks:[],
  projects:[project],currentProject:project,settings:{userName:'Alice',language:'fr',density:'standard',theme:'dark'},
  t:translations.fr,activeJobCount:0,searchQuery:'',
  fetchProjectMacros:async()=>window.metas,
  pendingHorizonPushes:async()=>[],
  saveMacroMeta:async()=>null,
  editMacroLabels:async()=>true,
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

  const rowOf = key => page.locator('div.rounded-xl.border').filter({ has: page.locator('span', { hasText: new RegExp(`^${key}$`) }) }).first();
  const visibleKeys = () => page.evaluate(() => {
    const seen = [];
    for (const row of document.querySelectorAll('div.rounded-xl.border')) {
      const key = [...row.querySelectorAll('span')].map(s => s.textContent.trim()).find(t => /^[A-Z]+-\d+$/.test(t));
      if (key && !seen.includes(key)) seen.push(key);
    }
    return seen.sort();
  });
  const entries = () => page.getByRole('menuitemcheckbox').evaluateAll(items =>
    items.map(i => [...i.querySelectorAll('span')].map(s => s.textContent.trim()).filter(Boolean).join(' '))
  );

  await page.goto(`${base}/roadmap`);
  await page.getByText('Billing').first().waitFor();

  // US2.2: the own key alone on a first visit.
  assert.deepEqual(await visibleKeys(), ['PE-1', 'PE-2']);

  // US2.1, US2.5, US2.6: own key first, declared keys, then the carried one.
  await page.getByRole('button', { name: 'Projets', exact: true }).click();
  assert.deepEqual(await entries(), ['PE ce projet 2', 'DATA 1', 'OPS 0', 'OLD 1']);

  // US2.3, US2.10: ticking DATA shows its epic and a chip.
  await page.getByRole('menuitemcheckbox', { name: /^DATA/ }).click();
  await page.getByText('Data export').first().waitFor();
  assert.deepEqual(await visibleKeys(), ['DATA-12', 'PE-1', 'PE-2']);
  await page.keyboard.press('Escape');
  assert.ok(await page.getByRole('button', { name: 'projets PE, DATA', exact: true }).isVisible(), 'a chip names the selection');

  // US2.8: the choice survives a reload.
  await page.reload();
  await page.getByText('Data export').first().waitFor();
  assert.deepEqual(await visibleKeys(), ['DATA-12', 'PE-1', 'PE-2']);

  // US2.4: unticking everything falls back to the own key.
  await page.getByRole('button', { name: /^Projets · 2$/ }).click();
  await page.getByRole('menuitemcheckbox', { name: /^PE/ }).click();
  await page.getByRole('menuitemcheckbox', { name: /^DATA/ }).click();
  await page.waitForFunction(() => !document.body.textContent.includes('Data export'));
  assert.deepEqual(await visibleKeys(), ['PE-1', 'PE-2']);
  await page.keyboard.press('Escape');
  assert.equal(await page.getByRole('button', { name: /^projets / }).count(), 0, 'the default selection shows no chip');

  // US3: a foreign epic is marked read only, with no label edit.
  await page.getByRole('button', { name: 'Projets', exact: true }).click();
  await page.getByRole('menuitemcheckbox', { name: /^DATA/ }).click();
  await page.keyboard.press('Escape');
  const data = rowOf('DATA-12');
  assert.ok(await data.getByLabel(/Épic du projet Jira DATA/).isVisible(), 'the row carries the read-only mark');
  await data.click();
  await page.getByText('Lecture seule', { exact: true }).waitFor();
  assert.equal(await page.getByPlaceholder('Ajouter un label…').count(), 0, 'no label edit on a foreign epic');
  await page.getByText(/cette épic appartient au projet DATA/).first().waitFor();

  // An own epic keeps its label editor.
  await rowOf('PE-1').click();
  await page.getByPlaceholder('Ajouter un label…').waitFor();
  assert.equal(await page.getByText('Lecture seule', { exact: true }).count(), 0);

  assert.deepEqual(errors, []);
  console.log('roadmap-origins: ok');
} finally {
  await browser?.close();
  await server.close();
}
