// Browser regressions for draft preservation and delayed comment responses.
// PLAYWRIGHT_MODULE=/absolute/path/to/playwright/index.mjs node tests/draft-safety.browser.mjs
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixture = root + '/draft-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {QuickAddModal} from '/src/components/QuickAddModal.tsx';
import {TaskComments} from '/src/components/TaskComments.tsx';
import {SkillsView} from '/src/components/SkillsView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.skillReads=[];window.skillWrites=[];window.reads=[];window.posts=[];window.creations=[];
window.task={id:'a',key:'A',source:'local'};
window.mode='quick';
window.state={t:translations.fr,settings:{language:'fr'},isQuickAddOpen:true,
  currentProject:{id:'p'},fetchSkillEditor:()=>new Promise(resolve=>skillReads.push(resolve)),
  saveSkillMode:(id,mode)=>new Promise(resolve=>skillWrites.push({id,mode,resolve})),
  saveSkillContent:(id,content)=>new Promise(resolve=>skillWrites.push({id,content,resolve})),
  quickAddInitialStatus:'to_clarify',selectedProjectId:'p',projects:[{id:'p',name:'Project',issueTracker:'local'}],tasks:[],
  setIsQuickAddOpen(value){state.isQuickAddOpen=value;render()},fetchProjectMacros:async()=>[],
  createTask:body=>new Promise(resolve=>creations.push({body,resolve})),setSelectedTask(){},runSkill:async()=>{},
  getTaskComments:id=>new Promise(resolve=>reads.push({id,resolve})),
  postTaskComment:(id,body)=>new Promise(resolve=>posts.push({id,body,resolve}))};
window.addEventListener('keydown',event=>{if(event.key==='Escape'&&state.isQuickAddOpen)state.setIsQuickAddOpen(false)});
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(mode==='quick'?<QuickAddModal/>:mode==='skills'?<SkillsView/>:<TaskComments task={task}/>);render();`
const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{ name: 'draft-fixture', enforce: 'pre',
    transform(code, id) { if (id.endsWith('/context/AppContext.tsx')) return 'export const useApp = () => window.state' },
    configureServer(s) { s.middlewares.use(async (req, res, next) => {
      if (req.url !== '/fixture') return next()
      res.setHeader('Content-Type', 'text/html')
      res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/draft-fixture.tsx"></script>'))
    }) },
    resolveId(id) { if (id === '/draft-fixture.tsx') return fixture },
    load(id) { if (id === fixture) return harness },
  }],
})
await server.listen()
let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })
  page.setDefaultTimeout(8000)
  const errors = []
  page.on('pageerror', e => errors.push(e.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  const title = page.getByRole('dialog').locator('input[type="text"]').first()
  await title.fill('Keep this draft')
  await page.locator('textarea').fill('Keep the description')
  await page.evaluate(() => { state.projects = [...state.projects]; render() })
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  assert.equal(await title.inputValue(), 'Keep this draft')
  assert.equal(await page.locator('textarea').inputValue(), 'Keep the description')
  await page.locator('button[type="submit"]').click()
  await page.waitForFunction(() => creations.length === 1)
  assert.equal(await title.isDisabled(), true)
  await page.keyboard.press('Escape')
  assert.equal(await page.getByRole('dialog').count(), 1)
  await page.evaluate(() => creations[0].resolve(null))
  await page.waitForFunction(() => !document.querySelector('fieldset').disabled)
  assert.equal(await title.inputValue(), 'Keep this draft')
  await page.getByRole('button', { name: 'Annuler', exact: true }).last().click()
  await page.evaluate(() => { state.isQuickAddOpen = true; render() })
  await page.waitForFunction(() => document.querySelector('input[type="text"]')?.value === '')

  await page.evaluate(() => { mode='comments';render() })
  await page.waitForFunction(() => reads.length === 1)
  await page.locator('textarea').fill('Draft for A')
  await page.evaluate(() => { task={id:'b',key:'B',source:'local'};render() })
  await page.waitForFunction(() => reads.length === 2)
  assert.equal(await page.locator('textarea').inputValue(), '')
  await page.evaluate(() => reads[1].resolve([{id:'b1',body:'Comment for B'}]))
  await page.getByText('Comment for B', { exact: true }).waitFor()
  await page.evaluate(() => reads[0].resolve([{id:'a1',body:'Comment for A'}]))
  await page.locator('textarea').fill('First comment')
  await page.locator('textarea').press('Control+Enter')
  await page.waitForFunction(() => posts.length === 1)
  await page.locator('textarea').fill('Next draft')
  await page.evaluate(() => posts[0].resolve([{id:'b2',body:'First comment'}]))
  await page.getByText('First comment', { exact: true }).waitFor()
  assert.equal(await page.locator('textarea').inputValue(), 'Next draft')
  assert.equal(await page.getByText('Comment for A', { exact: true }).count(), 0)
  const refresh = page.getByRole('button', { name: /Actualiser/ })
  await refresh.click()
  await page.waitForFunction(() => reads.length === 3)
  await page.evaluate(() => reads[2].resolve(null))
  await page.getByRole('alert').waitFor()
  assert.equal(await page.getByText('First comment', { exact: true }).count(), 1)
  await refresh.click()
  await page.waitForFunction(() => reads.length === 4)
  await page.locator('textarea').press('Control+Enter')
  await page.waitForFunction(() => posts.length === 2)
  await page.evaluate(() => posts[1].resolve([{id:'b3',body:'Newest comment'}]))
  await page.getByText('Newest comment', { exact: true }).waitFor()
  await page.evaluate(() => reads[3].resolve([{id:'stale',body:'Stale read'}]))
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  assert.equal(await page.getByText('Newest comment', { exact: true }).count(), 1)
  assert.equal(await page.getByText('Stale read', { exact: true }).count(), 0)
  await page.evaluate(() => { mode='skills';render() })
  await page.waitForFunction(() => skillReads.length === 1)
  const entries = [{id:'clarify',name:'Clarify',content:'Project P clarify',command:'/clarify',installed:true}, {id:'specify',name:'Specify',content:'Project P specify',command:'/specify',installed:true}]
  await page.evaluate(entries => skillReads[0](entries), entries)
  await page.waitForFunction(() => document.querySelector('textarea')?.value === 'Project P clarify')
  await page.locator('textarea').fill('Unsaved clarify')
  await page.getByRole('button', { name: /Specify/ }).click()
  await page.locator('textarea').fill('Unsaved specify')
  await page.getByRole('button', { name: /Clarify/ }).click()
  assert.equal(await page.locator('textarea').inputValue(), 'Unsaved clarify')
  await page.locator('select').selectOption('autonomous')
  await page.waitForFunction(() => skillWrites.length === 1)
  assert.equal(await page.locator('textarea').isDisabled(), true)
  await page.evaluate(entry => skillWrites[0].resolve({...entry,mode:'autonomous'}), entries[0])
  await page.waitForFunction(() => !document.querySelector('textarea').disabled)
  assert.equal(await page.locator('textarea').inputValue(), 'Unsaved clarify')
  await page.evaluate(() => { state.currentProject={id:'q'};render() })
  await page.waitForFunction(() => skillReads.length === 2)
  assert.equal(await page.locator('textarea').count(), 0, 'old project cannot be edited while the new one loads')
  await page.evaluate(entry => skillReads[1]([{...entry,content:'Project Q clarify'}]), entries[0])
  await page.waitForFunction(() => document.querySelector('textarea')?.value === 'Project Q clarify')
  await page.evaluate(() => { state.currentProject={id:'r'};render() })
  await page.waitForFunction(() => skillReads.length === 3)
  await page.evaluate(() => { state.currentProject={id:'s'};render() })
  await page.waitForFunction(() => skillReads.length === 4)
  await page.evaluate(entry => { skillReads[3]([{...entry,content:'Project S clarify'}]);skillReads[2]([{...entry,content:'Project R clarify'}]) }, entries[0])
  await page.waitForFunction(() => document.querySelector('textarea')?.value === 'Project S clarify')
  assert.deepEqual(errors, [])
  console.log('PASS: creation draft refresh/pending/retry, ticket isolation, typing during posting, read failure/retry, stale read after posting, project skill isolation and draft preservation')
} finally { await browser?.close(); await server.close() }
