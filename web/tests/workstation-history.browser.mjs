// Real issue detail and styles, isolated from tracker writes.
// Run: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node tests/workstation-history.browser.mjs
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
window.translations=translations;
window.calls=[];
const spy=name=>(...args)=>{window.calls.push([name,...args]);};
window.task={id:'fixture',key:'#815',title:'Recorded workstation history',description:'## Description\\nA readable story.\\n\\n## Technical context\\nKeep the existing editor.',status:'to_clarify',priority:'medium',source:'jira',projectId:'project',creator:'Author',assignee:'Alice',team:'Web team',labels:['new','layout'],sprint:'Sprint 1',createdAt:'2026-09-23T08:00:00Z',prLinks:[{url:'https://example.test/pull/1',branch:'feat/385'}]};
window.ctx={selectedTask:task,projects:[{id:'project',name:'Sectile',issueTracker:'jira'}],tasks:[task],activities:[{id:'latest',taskId:'fixture',skillName:'implement',status:'completed',deviceId:'offline-laptop',summary:'Finished',createdAt:'2026-10-11T12:00:00Z'},{id:'past',taskId:'fixture',skillName:'clarify',status:'failed',deviceId:'older-workstation',summary:'Failed',createdAt:'2026-10-10T12:00:00Z'},{id:'legacy',taskId:'fixture',skillName:'specify',status:'completed',summary:'Legacy summary (guessed-workstation)',createdAt:'2026-10-09T12:00:00Z'}],skills:[],settings:{detailMode:'panel'},t:translations.fr,isPinned:()=>false,skillCommand:(_id,command)=>command,isSkillRunning:false,membersForTeam:async()=>[],fetchProjectMacros:async()=>[],searchAssignableUsers:async()=>[],searchTrackerTeams:async()=>[],...Object.fromEntries(['setSelectedTask','updateTask','deleteTask','openCloneModal','migrateTasks','runSkill','updateSettings','addToast','setTaskTeam','setTaskSprint','setTaskMacro','createMacro','togglePin','syncSingleTask'].map(name=>[name,spy(name)]))};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<TaskDetailModal/>);
window.render();`
const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{ name: 'workstation-history-fixture', enforce: 'pre',
    transform(code, id) {
      // Every component the detail renders reads the mocked context, PrioritySelect included.
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
  await page.route("**/api/**", route => route.fulfill({contentType:"application/json",body:JSON.stringify({agents:[],availableWorkstations:[],comments:[],provider:"claude",model:"sonnet"})}))
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  const url = `http://127.0.0.1:${server.httpServer.address().port}/fixture`
  await page.goto(url)
  for (const reload of [false, true]) {
    if (reload) await page.reload()
    await page.getByRole('button', { name: 'Skills & IA', exact: true }).click()
    await page.getByText('Poste : offline-laptop', { exact: true }).waitFor()
    await page.getByText('Poste : older-workstation', { exact: true }).waitFor()
    assert.equal(await page.getByText('Poste : guessed-workstation', { exact: true }).count(), 0)
  }
  await page.evaluate(() => { ctx.activities = JSON.parse(JSON.stringify(ctx.activities)); ctx.settings.language = 'en'; ctx.t = translations.en; render() })
  await page.getByText('Workstation: offline-laptop', { exact: true }).waitFor()
  await page.getByText('Workstation: older-workstation', { exact: true }).waitFor()
  assert.deepEqual(errors, [])
  console.log('PASS: recorded workstation history survives completion, disconnection and reload; legacy identity remains unknown')
} finally {
  await browser?.close()
  await server.close()
}
