// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// Exercises batch configuration and submission without executing a skill.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/batch-pickup-context-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {AppProvider,useApp} from '/src/context/AppContext.tsx';
import '/src/index.css';
window.result=null;
function Entry(){const {tasks,startBatchPickup}=useApp();return <button disabled={!tasks.length} onClick={async()=>{window.result=await startBatchPickup(['one','two'])}}>Prepare</button>}
createRoot(document.getElementById('root')).render(<AppProvider><Entry/></AppProvider>);`

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'batch-pickup-context-fixture', enforce: 'pre',
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/batch-pickup-context-fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/batch-pickup-context-fixture.tsx' || id === fixtureId) return fixtureId },
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
  const tasks = ['one', 'two'].map((id, i) => ({ id, key: '#' + (i + 1), title: id, projectId: 'p', status: 'to_clarify', labels: ['new'], priority: 'medium' }))
  const requests = []
  let accept = false
  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url())
    let body
    if (url.pathname === '/api/tasks') body = tasks
    else if (url.pathname === '/api/projects') body = [{ id: 'p', name: 'Fixture', bookmarked: true }]
    else if (url.pathname === '/api/activities') body = []
    else if (url.pathname === '/api/activities/stats') body = {}
    else if (url.pathname.endsWith('/run-skill')) {
      requests.push({ url: url.pathname, body: route.request().postDataJSON() })
      if (!accept) return route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'Fixture launch refused' }) })
      body = { task: tasks[1], activity: { id: 'fixture-run', taskId: 'two', status: 'queued' } }
    } else return route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(body) })
  })
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  await page.getByRole('button', { name: 'Prepare', exact: true }).click()
  await page.getByRole('dialog').waitFor()
  assert.equal(requests.length, 0)
  if (process.env.BATCH_SCREENSHOT) await page.screenshot({ path: process.env.BATCH_SCREENSHOT })
  await page.keyboard.press('Escape')
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  assert.equal(await page.evaluate(() => result), false, 'cancellation resolves false to retain the caller selection')
  await page.getByRole('button', { name: 'Prepare', exact: true }).click()
  await page.getByRole('button', { name: 'Monter #2', exact: true }).click()
  await page.getByLabel('Nom du worktree').fill('batch-chosen')
  await page.getByRole('button', { name: 'Lancer le lot', exact: true }).click()
  await page.getByRole('alert').waitFor()
  assert.equal(requests.length, 1)
  assert.equal(requests[0].url, '/api/tasks/two/run-skill')
  assert.equal(requests[0].body.skillId, 'pickup_issues')
  assert.ok(requests[0].body.prompt.startsWith('/pickup-issues two one\n'))
  assert.ok(requests[0].body.prompt.includes('.tasks/worktrees/batch-chosen'))
  assert.deepEqual(requests[0].body.batchTaskIds, ['two', 'one'], 'the batch tickets reach the server in the chosen order')
  assert.equal(await page.getByLabel('Nom du worktree').inputValue(), 'batch-chosen')
  assert.equal(await page.evaluate(() => result), false, 'failed launch leaves the pending caller unresolved')
  accept = true
  await page.getByRole('button', { name: 'Lancer le lot', exact: true }).click()
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  assert.equal(await page.evaluate(() => result), true, 'accepted launch resolves true to clear the caller selection')
  assert.deepEqual(requests[1], requests[0])
  assert.deepEqual(errors, [])
  console.log('PASS: real AppProvider opens before dispatch, cancellation resolves false, selected order/name and batch tickets reach run-skill, failed launch can retry, acceptance resolves true')

} finally {
  await browser?.close()
  await server.close()
}
