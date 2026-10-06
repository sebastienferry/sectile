// Command line of the MCP connection tab, on the real component (#667).
// Run: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node tests/mcp-engine-config.browser.mjs
import assert from 'node:assert/strict'
import { createServer } from 'vite'
import { browserRoot } from './browserRoot.mjs'
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright')
const { root, preserveSymlinks } = browserRoot(import.meta.url)
const harness = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import { MCPEngineConfig } from '/src/components/MCPEngineConfig.tsx';
import '/src/index.css';
window.ctx={settings:{language:'en'}};
window.props={selectedProvider:'claude'};
const app=createRoot(document.getElementById('root'));
window.render=()=>app.render(<MCPEngineConfig key={ctx.settings.language+props.selectedProvider} {...props}/>);
window.render();`
const server = await createServer({
  root, resolve: { preserveSymlinks }, configFile: root + '/vite.config.ts', server: { port: 0, host: '127.0.0.1' },
  define: { 'import.meta.env.VITE_MCP_SERVER_URL': JSON.stringify('https://sectile.example') },
  plugins: [{ name: 'mcp-engine-config-fixture', enforce: 'pre',
    transform(code, id) {
      // The API key form talks to the server; it is not what this test covers.
      if (id.endsWith('/src/components/MCPEngineConfig.tsx')) return code
        .replace(/import \{ useOptionalApp \} from ['"]\.\.\/context\/AppContext['"]/, 'const useOptionalApp = () => window.ctx')
        .replace(/import \{ MCPApiKeyForm \} from ['"]\.\/ApiKeys['"]/, 'const MCPApiKeyForm = () => null')
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
  const context = await browser.newContext({ viewport: { width: 1200, height: 1000 } })
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin })
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.goto(`${origin}/fixture`)

  const show = (language, provider) => page.evaluate(([language, provider]) => { ctx.settings.language = language; props.selectedProvider = provider; render() }, [language, provider])
  const card = title => page.locator('div', { has: page.getByRole('heading', { name: title, exact: true }) }).last()
  const clipboard = () => page.evaluate(() => navigator.clipboard.readText())
  const status = page.getByRole('status')
  // React renders after the click or the render() call returns: poll the text until it settles.
  const settled = async (locator, expected) => {
    let actual
    for (const end = Date.now() + 5000; Date.now() < end; await page.waitForTimeout(50)) {
      actual = await locator.innerText().catch(() => undefined)
      if (expected instanceof RegExp ? expected.test(actual ?? '') : actual === expected) return
    }
    if (expected instanceof RegExp) assert.match(actual ?? '', expected)
    else assert.equal(actual, expected)
  }

  // Claude, remote HTTP: the two-line block and its help.
  const command = card('Or register it from a terminal')
  await command.waitFor()
  await settled(command.locator('pre'), `claude mcp remove --scope user sectile
claude mcp add --transport http --scope user sectile https://sectile.example/mcp --header 'Authorization: Bearer <SECTILE_API_KEY>'`)
  await settled(command.locator('p'), /^Run both lines\. .*No MCP server named "sectile" in user scope/)

  // Its own Copy button copies the block exactly; the snippet's copies the snippet.
  await command.getByRole('button', { name: 'Copy command', exact: true }).click()
  await page.waitForFunction(() => document.querySelector('[role=status]')?.textContent === 'Command copied.')
  assert.equal(await clipboard(), await command.locator('pre').innerText())
  await page.getByRole('button', { name: 'Copy configuration HTTP', exact: true }).click()
  await page.waitForFunction(() => document.querySelector('[role=status]')?.textContent === 'Configuration copied.')
  assert.equal(JSON.parse(await clipboard()).mcpServers.sectile.url, 'https://sectile.example/mcp')

  // A refused write says so.
  await page.evaluate(() => { navigator.clipboard.writeText = () => Promise.reject(new DOMException('denied', 'NotAllowedError')) })
  await command.getByRole('button', { name: 'Copy command', exact: true }).click()
  await page.waitForFunction(() => document.querySelector('[role=status]')?.textContent.startsWith('Copy failed.'))
  await page.reload()

  // Switching mode regenerates the command.
  await page.getByRole('button', { name: 'STDIO', exact: true }).click()
  await settled(command.locator('pre'), `claude mcp remove --scope user sectile
claude mcp add --scope user sectile --env 'SECTILE_AGENT_TOKEN=<SECTILE_API_KEY>' -- sectile-agent mcp --url https://sectile.example`)
  await page.getByRole('button', { name: 'Local HTTP · proxy', exact: true }).click()
  await page.getByRole('textbox').fill('http://127.0.0.1:9999/')
  await settled(command.locator('pre'), `claude mcp remove --scope user sectile
claude mcp add --transport http --scope user sectile http://127.0.0.1:9999/mcp`)

  // Codex: the env-var help in remote HTTP, the replace help otherwise.
  await show('en', 'codex')
  await settled(command.locator('pre'), 'codex mcp add sectile --url https://sectile.example/mcp --bearer-token-env-var SECTILE_API_KEY')
  await settled(command.locator('p'), /^Export SECTILE_API_KEY .*not written to ~\/\.codex\/config\.toml\./)
  await page.getByRole('button', { name: 'STDIO', exact: true }).click()
  await settled(command.locator('p'), 'Codex replaces an existing sectile entry.')

  // Antigravity: no command card at all.
  await show('en', 'agy')
  await page.getByRole('button', { name: 'Copy configuration HTTP', exact: true }).waitFor()
  assert.equal(await page.getByRole('heading', { name: 'Or register it from a terminal' }).count(), 0)
  assert.equal(await page.getByRole('button', { name: 'Copy command' }).count(), 0)
  assert.equal(await page.locator('pre').count(), 1, 'only the snippet')

  // French.
  await show('fr', 'claude')
  const commande = card('Ou enregistrez-le depuis un terminal')
  await commande.waitFor()
  await settled(commande.locator('p'), /^Exécutez les deux lignes\./)
  await commande.getByRole('button', { name: 'Copier la commande', exact: true }).click()
  await page.waitForFunction(() => document.querySelector('[role=status]')?.textContent === 'Commande copiée.')
  await show('fr', 'codex')
  await settled(commande.locator('p'), /^Exportez SECTILE_API_KEY /)
  await page.getByRole('button', { name: 'STDIO', exact: true }).click()
  await settled(commande.locator('p'), 'Codex remplace une entrée sectile existante.')
  assert.equal(await status.count(), 1)

  assert.deepEqual(errors, [])
  console.log('PASS: command card per provider and mode, its own copy button and notice, none for Antigravity, English and French strings.')
} finally {
  await browser?.close()
  await server.close()
}
