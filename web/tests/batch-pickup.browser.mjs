// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// Exercises batch configuration and submission without executing a skill.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/batch-pickup-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {BatchPickupModal} from '/src/components/BatchPickupModal.tsx';
import {translations} from '/src/locales/translations.ts';
import {buildBatchPickupPrompt} from '/src/lib/batchPickup.ts';
import '/src/index.css';
window.calls=[];window.accept=false;window.pending=false;window.cancelled=0;window.escaped=0;window.language='fr';
window.addEventListener('keydown',e=>{if(e.key==='Escape')escaped++});
const tasks=[{id:'one',key:'#1',title:'First task'},{id:'two',key:'#2',title:'Second task'},{id:'three',key:'#3',title:'Third task'}];
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<BatchPickupModal tasks={tasks} labels={translations[language].batchDialog} onCancel={()=>cancelled++} onConfirm={async(ids,name)=>{calls.push({ids,name,prompt:buildBatchPickupPrompt(ids,name)});if(pending)return new Promise(resolve=>window.finish=resolve);return accept}}/>);render();`

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'batch-pickup-fixture', enforce: 'pre',
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/batch-pickup-fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/batch-pickup-fixture.tsx') return fixtureId },
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
  const dialog = page.getByRole('dialog')
  const input = page.getByLabel('Nom du worktree')
  const launch = page.getByRole('button', { name: 'Lancer le lot', exact: true })
  await dialog.waitFor()
  assert.equal(await input.inputValue(), 'batch-1-2-3')
  assert.equal(await input.evaluate(element => element === document.activeElement), true)
  await page.getByRole('button', { name: 'Monter #1', exact: true }).waitFor()
  assert.equal(await page.getByRole('button', { name: 'Monter #1', exact: true }).isDisabled(), true)
  await page.getByRole('button', { name: 'Monter #3', exact: true }).click()
  await page.getByRole('button', { name: 'Monter #3', exact: true }).click()
  assert.deepEqual(await page.locator('ol li p').allTextContents(), ['Third task', 'First task', 'Second task'])
  await input.fill('../bad')
  assert.equal(await launch.isDisabled(), true)
  await input.fill('batch-custom')
  assert.deepEqual(await page.evaluate(() => calls), [], 'opening and editing never launches work')
  await page.keyboard.press('Escape')
  assert.equal(await page.evaluate(() => cancelled), 1)
  assert.equal(await page.evaluate(() => escaped), 0, 'Escape must not reach the underlying board')
  await launch.click()
  await page.getByRole('alert').waitFor()
  assert.equal(await input.inputValue(), 'batch-custom')
  assert.deepEqual(await page.evaluate(() => calls[0].ids), ['three', 'one', 'two'])
  assert.ok((await page.evaluate(() => calls[0].prompt)).includes('.tasks/worktrees/batch-custom'))
  await page.evaluate(() => { language = 'en'; render() })
  await page.getByRole('heading', { name: 'Prepare batch' }).waitFor()
  assert.equal(await page.getByLabel('Worktree name').inputValue(), 'batch-custom')
  await page.evaluate(() => { pending = true })
  await page.getByRole('button', { name: 'Launch batch', exact: true }).click()
  assert.equal(await page.getByRole('button', { name: 'Launching…' }).isDisabled(), true)
  assert.equal(await page.getByLabel('Worktree name').isDisabled(), true)
  await page.keyboard.press('Escape')
  assert.equal(await page.evaluate(() => cancelled), 1, 'pending launch cannot be dismissed')
  await page.evaluate(() => finish(true))
  await page.getByRole('button', { name: 'Launch batch', exact: true }).waitFor()
  assert.equal(await page.getByRole('alert').count(), 0)
  assert.deepEqual(errors, [])
  console.log('PASS: batch dialog order, worktree validation, cancel/Escape isolation, failure retry, pending guard, French/English labels and submitted prompt')

} finally {
  await browser?.close()
  await server.close()
}
