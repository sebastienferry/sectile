// The task detail pins a ticket to a repository its project does not declare
// (#737), typed by hand, and sends it on save.
// Run: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node tests/repository-pin.browser.mjs
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
window.task={id:'fixture',key:'#737',title:'Any repository',description:'',status:'to_implement',priority:'medium',source:'github',projectId:'project',labels:[],prLinks:[]};
const repositories=[{url:'git@github.com:o/a.git',identity:'github.com/o/a'},{url:'git@github.com:o/b.git',identity:'github.com/o/b'}];
window.ctx={selectedTask:task,projects:[{id:'project',name:'Sectile',issueTracker:'github',repositories}],tasks:[task],activities:[],skills:[],settings:{detailMode:'panel'},t:translations.fr,isPinned:()=>false,isSkillRunning:false,membersForTeam:async()=>[],fetchProjectMacros:async()=>[],searchAssignableUsers:async()=>[],searchTrackerTeams:async()=>[],...Object.fromEntries(['setSelectedTask','updateTask','deleteTask','openCloneModal','migrateTasks','runSkill','updateSettings','addToast','setTaskTeam','setTaskSprint','setTaskMacro','createMacro','togglePin','syncSingleTask'].map(name=>[name,spy(name)]))};
const app=createRoot(document.getElementById('root'));
app.render(<TaskDetailModal/>);`
const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{ name: 'repository-pin-fixture', enforce: 'pre',
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
  const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } })
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  const select = page.locator('select[title^="Dépôt dans lequel"]')
  await select.waitFor()
  assert.equal(await page.getByLabel('Autre dépôt…').count(), 0, 'no free entry before it is chosen')
  await select.selectOption({ label: 'Autre dépôt…' })
  const input = page.getByLabel('Autre dépôt…')
  await input.fill('git@gitlab.com:g/akamai-python.git')
  await page.getByRole('button', { name: 'Enregistrer', exact: true }).first().click()
  await page.waitForFunction(() => window.calls.some(call => call[0] === 'updateTask'))
  const update = await page.evaluate(() => window.calls.find(call => call[0] === 'updateTask')[2])
  assert.equal(update.repository, 'git@gitlab.com:g/akamai-python.git', 'the typed repository is sent')
  assert.deepEqual(errors, [])
  console.log('repository-pin: ok')
} finally {
  await browser?.close()
  await server.close()
}
