// Isolated browser regression (#534): representative components rendered with
// the English catalog must expose English accessible names, while user and
// tracker text stays as written. useApp is mocked in every component.
// Run with Playwright available: node tests/i18n-shell.browser.mjs
// PLAYWRIGHT_MODULE can point to an existing installation; Chrome is used with a fresh profile.
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
import assert from 'node:assert/strict'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixtureId = root + '/i18n-shell-fixture.tsx'

// The card is a French-authored task on a tracker with a French column name:
// neither may be translated. Everything Sectile says around it must be English.
const harness = `
import React from 'react';
import {createRoot} from 'react-dom/client';
import {TaskCard} from '/src/components/TaskCard.tsx';
import {ListView} from '/src/components/ListView.tsx';
import {translations} from '/src/locales/translations.ts';
import '/src/index.css';
window.translations=translations;
const noop=()=>{};
const project={id:'p',epicColors:true};
window.task={id:'fixture',key:'#39',title:'Corriger la synchronisation du tableau',projectId:'p',status:'to_clarify',trackerStatus:'À faire',priority:'high',source:'github',externalUrl:'https://example.test/39',parentKey:'#1',prUrl:'https://example.test/pr/1',description:'Description rédigée par une personne',labels:['new'],issueType:'Story',assignee:'Alice',branchName:'feature/39'};
window.ctx={settings:{density:'compact',language:'en'},projects:[project],currentProject:project,activities:[],activeTasks:new Set(),tasks:[window.task],
  t:translations.en,skillLabel:x=>x,skillCommand:(id,command)=>command,fetchActivities:async()=>{},isPinned:()=>false,parentFilter:null,
  boardSort:{field:'priority',asc:false},setBoardSort:noop,boardGrouping:'workflow',setBoardGrouping:noop,hideDone:false,startBatchPickup:async()=>false,
  advanceTask:async()=>{},setSelectedTask:noop,togglePin:noop,setParentFilter:noop,setChatTask:noop,runSkill:noop,setIsTerminalPanelOpen:noop,addToast:noop};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<div style={{display:'flex',gap:20}}><div style={{width:280,margin:20}}><TaskCard task={{...window.task}} compact/></div><div style={{flex:1}}><ListView/></div></div>);
window.render();`

const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{
    name: 'i18n-shell-fixture', enforce: 'pre',
    transform(code, id) {
      if (id.endsWith('/TaskFilters.tsx')) return 'export const TaskFilters=()=>null'
      if (id.includes('/src/components/')) return code.replace(/import \{ useApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useApp=()=>window.ctx')
    },
    configureServer(s) {
      s.middlewares.use(async (req, res, next) => {
        if (req.url !== '/fixture') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end(await s.transformIndexHtml('/fixture', '<div id="root"></div><script type="module" src="/i18n-shell-fixture.tsx"></script>'))
      })
    },
    resolveId(id) { if (id === '/i18n-shell-fixture.tsx') return fixtureId },
    load(id) { if (id === fixtureId) return harness },
  }],
})
await server.listen()

let browser
try {
  browser = await chromium.launch({ headless: true, channel: 'chrome' })
  const page = await browser.newPage({ viewport: { width: 1400, height: 900 } })
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)

  // The English value at the path where the catalog holds a French value, so
  // the test follows the catalog instead of repeating its wording.
  const english = french => page.evaluate(value => {
    const find = (fr, en) => {
      for (const key of Object.keys(fr)) {
        if (fr[key] === value) return en[key]
        if (fr[key] && typeof fr[key] === 'object') {
          const found = find(fr[key], en[key])
          if (found !== undefined) return found
        }
      }
      return undefined
    }
    return find(window.translations.fr, window.translations.en)
  }, french)

  const card = page.locator('[draggable]').first()
  await card.getByRole('button', { name: 'Actions', exact: true }).click()
  for (const french of ['Épingler', 'Avancer une étape', 'Supprimer la tâche']) {
    const name = await english(french)
    assert.ok(name && name !== french, `no English value for "${french}"`)
    assert.equal(await page.getByRole('button', { name, exact: true }).count() > 0, true, `English action "${name}" is missing`)
    assert.equal(await page.getByRole('button', { name: french, exact: true }).count(), 0, `French action "${french}" is still shown`)
  }
  await page.keyboard.press('Escape')

  // Accessible names, not only visible text: no control of the rendered
  // surfaces may carry one of the French names the audit found.
  const names = await page.evaluate(() => [...document.querySelectorAll('button, a, input, [role="checkbox"]')]
    .map(el => [el.getAttribute('aria-label'), el.getAttribute('title'), el.getAttribute('placeholder')].filter(Boolean).join(' | ')))
  for (const french of ['Sélectionner pour un lot', 'Ouvrir la fiche', 'Désépingler', 'Épingler']) {
    assert.equal(names.some(name => name.includes(french)), false, `"${french}" is still an accessible name`)
  }

  // User and tracker text is shown as written.
  assert.ok(await page.getByText('Corriger la synchronisation du tableau').first().isVisible(), 'the French task title must be kept')

  // Counts agree with their number in English.
  const count = await english('{count} tâche dans le backlog')
  assert.equal(count, '{count} task in the backlog')
  assert.ok(await page.getByText('1 task in the backlog').first().isVisible(), 'the backlog count must use the English singular')

  assert.deepEqual(errors, [])
  console.log('PASS: English card actions and accessible names, French task title kept, English singular count.')
} finally {
  await browser?.close()
  await server.close()
}
