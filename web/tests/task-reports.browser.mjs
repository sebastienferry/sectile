// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// Real task reports remain accessible independently of tracker comment failures.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/task-reports-fixture.tsx'
const harness = `
import React from 'react'; import {createRoot} from 'react-dom/client';
import {TaskComments} from '/src/components/TaskComments.tsx';
import {translations} from '/src/locales/translations.ts'; import '/src/index.css';
window.ctx={getTaskComments:async()=>[],postTaskComment:async()=>[],t:translations.fr,settings:{language:'fr'}};
const app=createRoot(document.getElementById('root'));
window.render=(id='one')=>app.render(<TaskComments task={{id,key:'PE-1',source:'jira',title:'Report',updatedAt:id}}/>);render();`
const server = await createServer({ root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' }, plugins: [{
  name: 'task-reports-fixture', enforce: 'pre',
  transform(code, id) { if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx') },
  configureServer(s) { s.middlewares.use(async (req, res, next) => {
    if (req.url !== '/fixture') return next()
    res.setHeader('Content-Type', 'text/html')
    res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/task-reports-fixture.tsx"></script>'))
  }) },
  resolveId(id) { if (id === '/task-reports-fixture.tsx') return fixtureId },
  load(id) { if (id === fixtureId) return harness },
}] })
await server.listen()
let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const page = await browser.newPage({ viewport: { width: 900, height: 900 } })
  const errors = []; page.on('pageerror', error => errors.push(error.message))
  let failHistory = false
  await page.route('**/api/tasks/*/reports', route => route.fulfill(failHistory ? {status:503,body:'{}'} : {
    contentType:'application/json',body:JSON.stringify(route.request().url().includes('/two/') ? [] : [
      {reportId:'round-1',stage:'clarified',round:1,summary:'Decision one accepted.',body:'# Complete first round\n\n**bold `code`**\n\n| Goal | Status |\n| --- | --- |\n| Retain history | Complete |',createdAt:'2026-10-14T03:00:00Z',attachment:{filename:'PE-1-clarification-round-1.md',content:'https://jira.example/attachment/1'}},
      {reportId:'round-2',stage:'clarified',round:2,summary:'Question two remains open.',body:'# Complete fallback report\n\nAll report text remains here.',createdAt:'2026-10-14T03:10:00Z',warning:'Attachments disabled'},
    ]),
  }))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  await page.getByRole('heading', { name: 'Rapports d’étape' }).waitFor()
  assert.equal(await page.locator('article').count(), 2)
  assert.match(await page.locator('article').first().textContent(), /Tour 1/)
  assert.match(await page.locator('article').last().textContent(), /Tour 2/)
  await page.getByText('Lire le rapport complet', { exact: true }).first().click()
  await page.getByRole('heading', { name: 'Complete first round' }).waitFor()
  assert.equal(await page.getByRole('link', { name: 'PE-1-clarification-round-1.md' }).getAttribute('href'), 'https://jira.example/attachment/1')
  await page.getByText('Lire le rapport complet', { exact: true }).last().click()
  await page.getByRole('heading', { name: 'Complete fallback report' }).waitFor()
  await page.screenshot({path:'/tmp/issue803-report-history.png',fullPage:true})
  await page.evaluate(() => window.render('two'))
  await page.waitForFunction(() => document.querySelectorAll('article').length === 0)
  failHistory = true; await page.evaluate(() => window.render('three'))
  await page.getByRole('button', { name: 'Historique des rapports indisponible · Réessayer' }).waitFor()
  failHistory = false; await page.getByRole('button', { name: 'Historique des rapports indisponible · Réessayer' }).click()
  await page.getByRole('heading', { name: 'Rapports d’étape' }).waitFor()
  assert.deepEqual(errors, [])
  console.log('Report history survives tracker failure, retains rounds, switches tasks safely, and retries history reads.')
} finally { await browser?.close(); await server.close() }
