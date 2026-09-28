// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright module.
// The real TaskCard, ListView and TaskDetailModal against a mocked context: every
// ticket of a running batch shows it, in the same words on the three surfaces
// (#522). No skill is executed.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/batch-members-fixture.tsx'
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {TaskCard} from '/src/components/TaskCard.tsx';
import {ListView} from '/src/components/ListView.tsx';
import {TaskDetailModal} from '/src/components/TaskDetailModal.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
const batch=(position,state)=>({runId:'run-10',leadTaskId:'t10',leadKey:'#10',position,size:3,state});
const task=(id,key,extra)=>({id,key,title:'Ticket '+key,projectId:'p',status:'to_clarify',priority:'medium',source:'local',labels:['new'],issueType:'Story',createdAt:'2026-09-26T08:00:00Z',...extra});
// The agent moved on to #11: the lead is done, #11 processing, #12 waiting.
window.allTasks=[
  task('t10','#10',{batch:batch(1,'done')}),
  task('t11','#11',{batch:batch(2,'processing')}),
  task('t12','#12',{batch:batch(3,'waiting')}),
  task('t13','#13'),
];
const project={id:'p',name:'Fixture'};
const spy=()=>()=>{};
window.view='cards'; window.compact=false;
window.ctx={tasks:allTasks,selectedTask:allTasks[1],boardSort:{field:'priority',asc:false},setBoardSort:()=>{},projects:[project],currentProject:project,
  // The batch run sits on the lead ticket, as the server records it.
  activities:[{id:'run-10',taskId:'t10',skillId:'remote_run',skillName:'pickup_issues',action:'Agent-owned remote execution',status:'running',summary:'',output:'',steps:[],createdAt:'2026-09-26T08:00:00Z',startedAt:'2026-09-26T08:00:00Z'}],
  activeTasks:new Set(),boardGrouping:'workflow',hideDone:false,parentFilter:null,settings:{density:'standard',detailMode:'panel'},skills:[],isSkillRunning:false,
  t:translations.fr,isPinned:()=>false,skillLabel:x=>x,skillCommand:(id,command)=>command,fetchActivities:async()=>{},advanceTask:async()=>{},
  membersForTeam:async()=>[],fetchProjectMacros:async()=>[],searchAssignableUsers:async()=>[],searchTrackerTeams:async()=>[],startBatchPickup:async()=>false,setBoardGrouping:()=>{},
  ...Object.fromEntries(['setSelectedTask','togglePin','setParentFilter','setChatTask','runSkill','setIsTerminalPanelOpen','addToast','updateTask','deleteTask','openCloneModal','migrateTasks','updateSettings','setTaskTeam','setTaskSprint','setTaskMacro','createMacro','syncSingleTask'].map(n=>[n,spy(n)]))};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(view==='cards'
  ? <div style={{width:320,margin:20,display:'grid',gap:12}}>{ctx.tasks.map(t=><div key={t.id} data-card={t.key}><TaskCard task={t} compact={compact}/></div>)}</div>
  : view==='list' ? <ListView/> : <TaskDetailModal/>);
render();`

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'batch-members-fixture', enforce: 'pre',
    transform(code, id) {
      if (id.endsWith('/TaskFilters.tsx')) return 'export const TaskFilters=()=>null'
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/batch-members-fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/batch-members-fixture.tsx' || id === fixtureId) return fixtureId },
    load(id) { if (id === fixtureId) return harness },
  }],
})
await server.listen()
let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)

  const card = key => page.locator(`[data-card="${key}"] [draggable]`)
  const badge = scope => scope.getByText('Lot #10', { exact: true })
  await badge(card('#10')).waitFor()

  // Every member names the batch and its place in it; the plain ticket does not.
  for (const [key, position] of [['#10', 1], ['#11', 2], ['#12', 3]]) {
    assert.equal(await badge(card(key)).count(), 1, `${key} shows its batch`)
    assert.equal(await badge(card(key)).getAttribute('title'), `Lot mené par #10 · ticket ${position} sur 3`)
  }
  assert.equal(await badge(card('#13')).count(), 0, 'a ticket in no batch shows no badge')

  if (process.env.BATCH_MEMBERS_CARDS_SCREENSHOT) await page.screenshot({ path: process.env.BATCH_MEMBERS_CARDS_SCREENSHOT })

  // Where each one stands: the label and the border.
  const border = key => card(key).evaluate(e => e.className.includes('border-indigo-500/60') ? 'indigo' : e.className.includes('border-amber-500/50') ? 'amber' : 'none')
  assert.equal(await card('#11').getByText('en cours', { exact: true }).count(), 1)
  assert.equal(await card('#12').getByText('en attente dans le lot', { exact: true }).count(), 1)
  assert.equal(await card('#10').getByText('en cours', { exact: true }).count(), 0, 'a done member shows its badge alone')
  assert.equal(await card('#10').getByText('en attente dans le lot', { exact: true }).count(), 0)
  assert.equal(await border('#11'), 'indigo')
  assert.equal(await border('#12'), 'amber')
  // The lead's border follows its member state, not the batch run it carries,
  // while the run badge that shows and stops that run stays.
  assert.equal(await border('#10'), 'none')
  assert.equal(await card('#10').getByTitle(/^Exécution distante en cours/).count(), 1, 'the lead keeps its run badge')
  assert.equal(await border('#13'), 'none')

  // The condensed card carries the same badge, label and border.
  await page.evaluate(() => { compact = true; render() })
  assert.equal(await card('#12').getByText('en attente dans le lot', { exact: true }).count(), 1)
  assert.equal(await badge(card('#11')).count(), 1)
  assert.equal(await border('#11'), 'indigo')
  await page.evaluate(() => { compact = false; render() })

  // Once the batch ends, nothing of it remains.
  await page.evaluate(() => { ctx.tasks = ctx.tasks.map(t => ({ ...t, batch: undefined })); ctx.activities = []; render() })
  assert.equal(await page.getByText('Lot #10', { exact: true }).count(), 0)
  assert.equal(await border('#11'), 'none')
  await page.evaluate(() => { ctx.tasks = allTasks; ctx.activities = []; render() })

  // The list row shows the same badge and label as the card.
  await page.evaluate(() => { view = 'list'; render() })
  const row = key => page.locator('tr').filter({ has: page.getByText(key, { exact: true }) })
  await badge(row('#11')).waitFor()
  assert.equal(await row('#11').getByText('en cours', { exact: true }).count(), 1)
  assert.equal(await row('#12').getByText('en attente dans le lot', { exact: true }).count(), 1)
  assert.equal(await badge(row('#10')).count(), 1)
  assert.equal(await badge(row('#13')).count(), 0)

  // So does the detail panel, read from the board's copy of the ticket: the
  // selection still carries the batch, the board no longer does.
  await page.evaluate(() => { view = 'detail'; render() })
  await badge(page).waitFor()
  assert.equal(await page.getByText('en cours', { exact: true }).count(), 1)
  await page.evaluate(() => { ctx.tasks = ctx.tasks.map(t => ({ ...t, batch: undefined })); render() })
  assert.equal(await badge(page).count(), 0, 'the panel follows the refreshed board, not a stale selection')

  if (process.env.BATCH_MEMBERS_SCREENSHOT) await page.screenshot({ path: process.env.BATCH_MEMBERS_SCREENSHOT })
  assert.deepEqual(errors, [])
  console.log('PASS: batch badge, tooltip, member labels and borders on full and condensed cards; lead border follows its state with its run badge kept; plain ticket unchanged; indicators vanish with the batch; list rows and detail panel agree')
} finally {
  await browser?.close()
  await server.close()
}
