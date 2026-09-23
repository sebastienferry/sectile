// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// Exercises the real Backlog against mocked context operations; no skill is executed.
import { createServer } from 'vite'
import { fileURLToPath } from 'node:url'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const root = fileURLToPath(new URL('../', import.meta.url)).replace(/\/$/, '')
const fixtureId = root + '/backlog-batch-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {ListView} from '/src/components/ListView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.calls=[]; window.accept=false; window.pending=false;
const task=(id,stage,priority)=>({id,key:id,title:'Ticket '+id,projectId:'p',status:stage==='clarified'?'clarified':'to_clarify',priority,source:'local',labels:[stage],issueType:'Story'});
window.allTasks=[task('new-low','new','low'),task('clarified','clarified','urgent'),{...task('specified','specified','medium'),parentKey:'#1'},task('new-high','new','high')];
const project={id:'p',epicColors:true};
window.ctx={tasks:allTasks,projects:[project],currentProject:project,activities:[],activeTasks:new Set(),boardGrouping:'workflow',hideDone:false,isPinned:()=>false,t:translations.fr,
startBatchPickup:async ids=>{calls.push(ids);if(pending)return new Promise(resolve=>window.finish=resolve);return accept},
setBoardGrouping:value=>{ctx.boardGrouping=value;render()}};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<ListView/>);render();`
const server = await createServer({
  root, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'backlog-batch-fixture', enforce: 'pre',
    transform(code, id) {
      if (id.endsWith('/TaskFilters.tsx')) return 'export const TaskFilters=()=>null'
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/backlog-batch-fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/backlog-batch-fixture.tsx') return fixtureId },
    load(id) { if (id === fixtureId) return harness },
  }],
})
await server.listen()
let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const page = await browser.newPage({ viewport: { width: 1600, height: 1000 } })
  page.setDefaultTimeout(6000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  const box = id => page.locator('tbody tr').filter({ hasText: 'Ticket ' + id }).getByRole('checkbox')
  const launch = page.getByRole('button', { name: 'Lot', exact: true })
  await page.locator('tbody tr').filter({ hasText: 'Ticket specified' }).locator('[data-epic-bar]').waitFor()
  assert.equal(await page.locator('[data-epic-bar]').count(), 1, 'only the task with a parent carries an epic bar')
  await box('clarified').check()
  await box('new-low').check()
  await box('new-high').check()
  await launch.click()
  assert.deepEqual(await page.evaluate(() => calls[0]), ['new-high', 'new-low', 'clarified'])
  assert.equal(await box('new-low').isChecked(), true, 'failed launch preserves selection')
  await box('specified').check()
  assert.equal(await launch.isDisabled(), true, 'later workflow stages block the whole batch')
  await box('specified').uncheck()
  await page.evaluate(() => { ctx.tasks = allTasks.map(t => t.id === 'clarified' ? { ...t, projectId: 'other' } : t); render() })
  await page.getByText('Sélectionnez des tâches d’un seul projet.').waitFor()
  assert.equal(await launch.isDisabled(), true)
  await page.evaluate(() => { ctx.tasks = allTasks.filter(t => t.id !== 'new-low'); render() })
  await page.getByText('Sélectionnez uniquement des tâches visibles dans le backlog.').waitFor()
  assert.equal(await launch.isDisabled(), true)
  await page.evaluate(() => { ctx.tasks = allTasks; render() })
  await page.getByText('Sélectionnez uniquement des tâches visibles dans le backlog.').waitFor({ state: 'hidden' })
  await page.getByLabel('Grouper par étape workflow').uncheck()
  await launch.click()
  assert.deepEqual(await page.evaluate(() => calls.at(-1)), ['clarified', 'new-high', 'new-low'], 'flat view follows the visible sort')
  await page.evaluate(() => { ctx.boardGrouping = 'status'; render() })
  await page.getByLabel('Grouper par statut').check()
  await launch.click()
  assert.deepEqual(await page.evaluate(() => calls.at(-1)), ['new-high', 'new-low', 'clarified'], 'status groups follow displayed order')
  await page.evaluate(() => { pending = true })
  await launch.click()
  assert.equal(await launch.isDisabled(), true, 'cannot launch again while awaiting acceptance')
  await page.evaluate(() => finish(true))
  await launch.waitFor({ state: 'hidden' })
  assert.equal(await box('new-low').isChecked(), false, 'accepted launch clears selection')
  assert.deepEqual(errors, [])
  console.log('PASS: Backlog eligibility, epic bar, visible group/sort order, failure retention, accepted clearing, pending guard, hidden tasks, cross-project selection')
} finally {
  await browser?.close()
  await server.close()
}
