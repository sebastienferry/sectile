// Real issue detail and styles, isolated from tracker writes.
// Run: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node tests/issue-detail-layout.browser.mjs
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { fileURLToPath } from 'node:url'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const root = fileURLToPath(new URL('..', import.meta.url)).replace(/\\/g, '/').replace(/\/$/, '')
const harness = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { TaskDetailModal } from '/src/components/TaskDetailModal.tsx';
import { translations } from '/src/locales/translations.ts';
import '/src/index.css';
window.calls=[];
const spy=name=>(...args)=>{window.calls.push([name,...args]);};
window.task={id:'fixture',key:'#385',title:'Content-first issue details',description:'## Description\\nA readable story.\\n\\n## Technical context\\nKeep the existing editor.',status:'to_clarify',priority:'medium',source:'jira',projectId:'project',creator:'Author',assignee:'Alice',team:'Web team',labels:['new','layout'],sprint:'Sprint 1',createdAt:'2026-09-23T08:00:00Z',prLinks:[{url:'https://example.test/pull/1',branch:'feat/385'}]};
window.ctx={selectedTask:task,projects:[{id:'project',name:'Sectile',issueTracker:'jira'}],tasks:[task],activities:[],skills:[],settings:{detailMode:'panel'},t:translations.fr,isPinned:()=>false,isSkillRunning:false,membersForTeam:async()=>[],fetchProjectMacros:async()=>[],searchAssignableUsers:async()=>[],searchTrackerTeams:async()=>[],...Object.fromEntries(['setSelectedTask','updateTask','deleteTask','openCloneModal','migrateTasks','runSkill','updateSettings','addToast','setTaskTeam','setTaskSprint','setTaskMacro','createMacro','togglePin','syncSingleTask'].map(name=>[name,spy(name)]))};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<TaskDetailModal/>);
window.render();`
const server = await createServer({
  root, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  plugins: [{ name: 'issue-detail-fixture', enforce: 'pre',
    transform(code, id) {
      if (id.endsWith('/TaskDetailModal.tsx')) return code.replace("import { useApp } from '../context/AppContext'", 'const useApp = () => window.ctx')
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
  // Locate existing user-facing labels without adding production test attributes.
  const section = text => page.locator('label').filter({ hasText: text }).first().locator('..')
  const content = page.getByText('Description & Contexte Technique', { exact: true }).locator('../..')
  const metadata = page.getByText('Macro (Milestone)', { exact: true }).locator('../../..')
  const prs = page.getByText('Pull Requests', { exact: true }).locator('..')
  const titleInput = section('Titre de la Story').locator('input')
  await titleInput.waitFor()
  for (const mode of ['panel', 'modal']) {
    await page.evaluate(mode => { ctx.settings.detailMode = mode; render() }, mode)
    for (const width of [1440, 1024, 768, 390]) {
      await page.setViewportSize({ width, height: 1100 })
      await page.waitForTimeout(300)
      const [t, c, m, p] = await Promise.all([titleInput.boundingBox(), content.boundingBox(), metadata.boundingBox(), prs.boundingBox()])
      assert(c.y >= t.y + t.height, mode + ': content follows title')
      if (width >= 1024) {
        assert(Math.abs(c.y - m.y) < 2, mode + ': columns start together')
        assert(m.x >= c.x + c.width, mode + ': metadata is on the right')
        assert(t.width > c.width, mode + ': title spans columns')
      } else {
        assert(m.y >= c.y + c.height, mode + ': narrow view is content first')
      }
      assert(p.y >= Math.max(c.y + c.height, m.y + m.height), mode + ': PRs follow both columns')
      assert(c.x >= 0 && c.x + c.width <= width + 1, mode + ': content fits viewport ' + JSON.stringify({width,t,c,m,p}))
      assert(m.x >= 0 && m.x + m.width <= width + 1, mode + ': metadata fits viewport')
    }
  }
  await page.setViewportSize({ width: 1440, height: 1100 })
  assert.equal(await page.getByText('Équipe', { exact: true }).count(), 1)
  assert.equal(await metadata.getByText('Créé par', { exact: true }).count(), 1)
  await titleInput.fill('Edited title')
  await page.locator('textarea').fill('Updated description and technical context')
  await page.getByRole('checkbox', { name: 'Inclure les commentaires' }).check()
  await page.getByRole('button', { name: 'Reformuler la story', exact: true }).click()
  assert.deepEqual(await page.evaluate(() => calls.find(c => c[0] === 'runSkill')), ['runSkill', 'fixture', 'rewrite_story', '', { withComments: true }])
  await page.getByPlaceholder('https://github.com/owner/repo/pull/42').fill('https://example.test/pull/2')
  await page.getByRole('button', { name: 'Lier', exact: true }).click()
  assert.equal(await prs.locator('input[type="url"]').count(), 3)
  await page.getByRole('button', { name: 'Détacher cette pull request du ticket' }).first().click()
  await page.getByRole('button', { name: 'Enregistrer', exact: true }).click()
  const saved = await page.evaluate(() => calls.find(c => c[0] === 'updateTask'))
  assert.equal(saved[2].title, 'Edited title')
  assert.equal(saved[2].description, 'Updated description and technical context')
  assert.equal(saved[2].prLinks.length, 1)
  assert.equal(saved[2].prLinks[0].url, 'https://example.test/pull/2')
  await page.evaluate(() => { ctx.selectedTask = { ...task, source: 'github', creator: '' }; render() })
  assert.equal(await page.getByText('Équipe', { exact: true }).count(), 0)
  assert.equal(await metadata.getByText('Créé par', { exact: true }).count(), 0)
  if (process.env.DETAIL_SCREENSHOT) await page.screenshot({ path: process.env.DETAIL_SCREENSHOT })
  assert.deepEqual(errors, [])
  console.log('PASS: content-first panel/modal at four widths; full-width title and PRs; editor, rewrite, save, PR management and conditional metadata preserved.')
} finally {
  await browser?.close()
  await server.close()
}
