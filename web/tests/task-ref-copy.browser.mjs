// Copy buttons of the issue detail's reference badge, on the real component.
// Run: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node tests/task-ref-copy.browser.mjs
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { TaskDetailModal } from '/src/components/TaskDetailModal.tsx';
import { translations } from '/src/locales/translations.ts';
import '/src/index.css';
window.calls=[];
const spy=name=>(...args)=>{window.calls.push([name,...args]);};
window.fixtures={
  github:{id:'gh-431',key:'#431',title:'Copy the key',status:'to_clarify',priority:'medium',source:'github',projectId:'project',parentKey:'M-7',parentType:'Macro',labels:[],createdAt:'2026-09-25T08:00:00Z'},
  jira:{id:'jira-123',key:'SFE-123',title:'Jira ticket',status:'to_clarify',priority:'medium',source:'jira',projectId:'project',labels:[],createdAt:'2026-09-25T08:00:00Z'},
  local:{id:'local-9',key:'LOC-9',title:'Local ticket',status:'to_clarify',priority:'medium',source:'local',projectId:'project',labels:[],createdAt:'2026-09-25T08:00:00Z'},
};
window.ctx={selectedTask:fixtures.github,projects:[{id:'project',name:'Sectile',issueTracker:'github',githubRepo:'owner/repo'}],tasks:Object.values(fixtures),activities:[],skills:[],settings:{detailMode:'panel'},t:translations.fr,isPinned:()=>false,isSkillRunning:false,membersForTeam:async()=>[],fetchProjectMacros:async()=>[],searchAssignableUsers:async()=>[],searchTrackerTeams:async()=>[],...Object.fromEntries(['setSelectedTask','updateTask','deleteTask','openCloneModal','migrateTasks','runSkill','updateSettings','addToast','setTaskTeam','setTaskSprint','setTaskMacro','createMacro','togglePin','syncSingleTask'].map(name=>[name,spy(name)]))};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<TaskDetailModal/>);
window.render();`
const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{ name: 'task-ref-copy-fixture', enforce: 'pre',
    transform(code, id) {
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp = () => window.ctx')
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/fixture.tsx' || id === root + '/fixture.tsx') return root + '/fixture.tsx' },
    load(id) { if (id === root + '/fixture.tsx') return harness },
  }],
})
await server.listen()
let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } })
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin })
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  let popups = 0
  context.on('page', () => { popups++ })
  await page.goto(`${origin}/fixture`)

  const copyButtons = page.getByRole('button', { name: /^Copier (?!la )/ })
  const button = key => page.getByRole('button', { name: `Copier ${key}`, exact: true })
  // lucide-react names each icon in its class: lucide-copy, lucide-check.
  const icon = async key => (await button(key).locator('svg').getAttribute('class')).match(/lucide-(copy|check)\b/)?.[1]
  const clipboard = () => page.evaluate(() => navigator.clipboard.readText())
  const toasts = () => page.evaluate(() => calls.filter(c => c[0] === 'addToast').map(c => c[1]))
  const reset = () => page.evaluate(() => { calls.length = 0 })
  const show = async (mode, fixture) => {
    await page.evaluate(([mode, fixture]) => { ctx.settings.detailMode = mode; ctx.selectedTask = fixtures[fixture]; render() }, [mode, fixture])
    await button(await page.evaluate(f => fixtures[f].key, fixture)).waitFor()
  }

  for (const mode of ['panel', 'modal']) {
    // GitHub ticket with a parent: one button after each key, links unchanged.
    await show(mode, 'github')
    assert.deepEqual(await copyButtons.evaluateAll(els => els.map(e => e.getAttribute('aria-label'))), ['Copier M-7', 'Copier #431'], mode + ': parent then task button')
    assert.equal(await button('#431').getAttribute('title'), 'Copier #431', mode + ': tooltip names the key')
    const taskLink = page.locator('a[href="https://github.com/owner/repo/issues/431"]')
    assert.equal(await taskLink.getAttribute('target'), '_blank', mode + ': task key still opens the tracker')
    assert.equal(await taskLink.locator('button').count(), 0, mode + ': the button is outside the link')

    // Copying the task key.
    await reset()
    await button('#431').click()
    await page.waitForFunction(() => calls.some(c => c[0] === 'addToast'))
    assert.equal(await clipboard(), '#431', mode + ': clipboard holds the task key alone')
    assert.equal(await icon('#431'), 'check', mode + ': clicked button shows the check mark')
    assert.equal(await icon('M-7'), 'copy', mode + ': the other button is unaffected')
    let sent = await toasts()
    assert.equal(sent.length, 1)
    assert.equal(sent[0].type, 'success')
    assert.equal(sent[0].title, 'Identifiant copié')
    assert.equal(sent[0].description, '#431 a été copié dans le presse-papiers.')

    // Copying the parent key.
    await reset()
    await button('M-7').click()
    await page.waitForFunction(() => calls.some(c => c[0] === 'addToast'))
    assert.equal(await clipboard(), 'M-7', mode + ': clipboard holds the parent key alone')
    assert.equal(await icon('M-7'), 'check')
    assert.match((await toasts())[0].description, /^M-7 /)

    // A repeated click restarts the delay from that click.
    await page.waitForTimeout(1500)
    await button('M-7').click()
    await page.waitForTimeout(1000)
    assert.equal(await icon('M-7'), 'check', mode + ': a second click restarts the delay')
    await page.waitForTimeout(1300)
    assert.equal(await icon('M-7'), 'copy', mode + ': the copy icon is back after 2 seconds')
    assert.equal(await icon('#431'), 'copy')

    // Jira ticket without a parent: a single button.
    await show(mode, 'jira')
    assert.equal(await copyButtons.count(), 1, mode + ': one button without a parent')
    await button('SFE-123').click()
    await page.waitForFunction(() => calls.some(c => c[0] === 'addToast' && c[1].description.startsWith('SFE-123')))
    assert.equal(await clipboard(), 'SFE-123')

    // Local ticket without a tracker URL: still copies.
    await show(mode, 'local')
    assert.equal(await page.locator('a', { hasText: 'LOC-9' }).count(), 0, mode + ': local key has no link')
    await button('LOC-9').click()
    await page.waitForFunction(() => calls.some(c => c[0] === 'addToast' && c[1].description.startsWith('LOC-9')))
    assert.equal(await clipboard(), 'LOC-9')

    // A refused write: an error toast, no check mark, no success toast.
    await show(mode, 'github')
    await page.waitForTimeout(2100)
    await reset()
    await page.evaluate(() => {
      window.realWriteText = navigator.clipboard.writeText.bind(navigator.clipboard)
      navigator.clipboard.writeText = () => Promise.reject(new DOMException('denied', 'NotAllowedError'))
    })
    await button('#431').click()
    await page.waitForFunction(() => calls.some(c => c[0] === 'addToast'))
    sent = await toasts()
    assert.deepEqual(sent.map(t => t.type), ['error'], mode + ': only an error toast')
    assert.equal(sent[0].title, 'Copie impossible')
    assert.equal(sent[0].description, "Le presse-papiers n'est pas accessible depuis ce navigateur.")
    assert.equal(await icon('#431'), 'copy', mode + ': no check mark on failure')
    await page.evaluate(() => { navigator.clipboard.writeText = window.realWriteText })
  }

  // Clicking the key itself opens the tracker and copies nothing.
  await show('panel', 'github')
  await page.evaluate(() => navigator.clipboard.writeText('untouched'))
  await reset()
  const opened = context.waitForEvent('page')
  await page.locator('a[href="https://github.com/owner/repo/issues/431"]').click()
  await (await opened).close()
  assert.equal(popups, 1, 'only the link click opened a tab')
  assert.equal(await clipboard(), 'untouched', 'the link click copied nothing')
  assert.deepEqual(await toasts(), [])

  assert.deepEqual(errors, [])
  console.log('PASS: task and parent key copy buttons in panel and modal layouts; exact clipboard text, check mark and its delay, success and failure toasts, links unchanged.')
} finally {
  await browser?.close()
  await server.close()
}
