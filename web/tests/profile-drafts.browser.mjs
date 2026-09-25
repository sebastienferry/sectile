// Run with PLAYWRIGHT_MODULE pointing to playwright/index.mjs; Chrome is required.
// Real profile/account components, with an isolated context and mocked account HTTP.
import assert from 'node:assert/strict'
import { browserRoot } from './browserRoot.mjs'
import { createServer } from 'vite'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const fixture = `
import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { ProfileModal } from '/src/components/ProfileModal.tsx';
import { translations } from '/src/locales/translations.ts';
window.profileContext = React.createContext(null);
function Fixture() {
  const [settings, setSettings] = useState({theme:'dark', language:'en', density:'standard', defaultView:'board', detailMode:'panel', uiScale:100, aiProvider:'claude', userName:'Alice', userEmail:'alice@example.test', promptClarify:'Saved prompt'});
  const [isProfileOpen, setIsProfileOpen] = useState(true);
  return <window.profileContext.Provider value={{settings, isProfileOpen, setIsProfileOpen, t:translations.en,
    reloadSettings: async () => { const user = await fetch('/api/me').then(r => r.json()); setSettings(s => ({...s, userName:user.displayName})); },
    updateSettings: async patch => setSettings(s => ({...s, ...patch})),
  }}>
    <output data-testid="name">{settings.userName}</output>
    <button onClick={() => setIsProfileOpen(false)}>Close fixture</button>
    <button onClick={() => setIsProfileOpen(true)}>Open fixture</button>
    <ProfileModal />
  </window.profileContext.Provider>;
}
createRoot(document.getElementById('root')).render(<React.StrictMode><Fixture /></React.StrictMode>);
`
const server = await createServer({root, resolve: { preserveSymlinks }, configFile:root+'/vite.config.ts', server:{host:'127.0.0.1',port:0}, plugins:[{
  name:'profile-drafts-fixture', enforce:'pre',
  transform(_code,id) {
    if (id.endsWith('/context/AppContext.tsx')) return `import React from 'react'; export const useApp = () => React.useContext(window.profileContext); export const useOptionalApp = useApp; export const UI_SCALE_OPTIONS = [90,100,112,125];`
  },
  configureServer(s) {
    s.middlewares.use(async(req,res,next) => {
      if(req.url !== '/fixture') return next()
      res.setHeader('Content-Type','text/html')
      res.end(await s.transformIndexHtml('/fixture','<div id="root"></div><script type="module" src="/fixture.tsx"></script>'))
    })
  },
  resolveId(id) { if(id === '/fixture.tsx' || id === root+'/fixture.tsx') return root+'/fixture.tsx' },
  load(id) { if(id === root+'/fixture.tsx') return fixture },
}]})
let browser
try {
  await server.listen()
  browser = await chromium.launch({channel:'chrome',headless:true})
  const page = await browser.newPage()
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  let displayName = 'Alice'
  await page.route('**/api/me', async route => {
    if(route.request().method() === 'PATCH') displayName = route.request().postDataJSON().displayName
    await route.fulfill({json:{signedIn:true,userId:'alice',displayName,email:'alice@example.test',role:'member'}})
  })
  await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/fixture`)
  const tab = name => page.locator('nav').getByRole('button',{name,exact:true})
  await tab('Skills & SDD').click()
  const prompt = page.locator('textarea').first()
  await prompt.fill('Unsaved prompt')
  await tab('Appearance').click()
  await page.getByRole('button',{name:'Light',exact:true}).click()
  await tab('Account').click()
  await page.locator('#account-display-name').fill('Alice renamed')
  await page.locator('section[aria-labelledby="account-title"]').getByRole('button',{name:'Save',exact:true}).click()
  await page.waitForFunction(() => document.querySelector('[data-testid="name"]').textContent === 'Alice renamed')
  await tab('Skills & SDD').click()
  assert.equal(await prompt.inputValue(),'Unsaved prompt','renaming must preserve the prompt draft')
  await tab('Appearance').click()
  assert.match(await page.getByRole('button',{name:'Light',exact:true}).getAttribute('class'),/accent-text/,'renaming must preserve the appearance draft')
  await page.getByRole('button',{name:'Close fixture',exact:true}).click()
  await page.getByRole('button',{name:'Open fixture',exact:true}).click()
  await tab('Skills & SDD').click()
  assert.equal(await prompt.inputValue(),'Saved prompt','reopening must discard unsaved drafts')
  await prompt.fill('New saved prompt')
  await page.getByRole('button',{name:'Save Configuration',exact:true}).click()
  await page.getByRole('button',{name:'Open fixture',exact:true}).click()
  await tab('Skills & SDD').click()
  assert.equal(await prompt.inputValue(),'New saved prompt','reopening must load the latest saved settings')
  assert.deepEqual(errors,[])
  console.log('PASS: rename preserves drafts; reopen discards drafts and loads saved settings.')
} finally {
  await browser?.close()
  await server.close()
}
