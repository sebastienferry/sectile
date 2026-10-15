// Development-only entry point. Packaged Electron keeps its real preload bridge.
// Read the paired local agent through the development server; its key stays there.
import { version as desktopVersion } from '../package.json'
const unavailable = async () => {
  throw new Error('The canvas connection is read-only. Use Sectile Desktop for this action.')
}
async function read(route, query = {}) {
  const response = await fetch('/__desktop/' + route + '?' + new URLSearchParams(query), {
    headers: { 'X-Sectile-Preview': '1' },
  })
  const value = await response.json()
  if (!response.ok) throw Object.assign(new Error(value.error || 'Agent request failed'), { status: response.status })
  return value
}
const preferences = {
  appearance: 'system', consoleView: 'terminal', conversationMode: 'acceptEdits',
}
const bridge = {
  connect: async () => { await read('status'); return true },
  start: async () => { await read('status'); return true },
  credentialState: async () => ({ state: 'present', server: (await read('status')).server }),
  settings: async () => ({ server: (await read('status')).server }),
  status: async () => {
    const status = await read('status')
    const native = new Set(['open-editor', 'project-terminal', 'folder-selection', 'run-folders', 'run-folders-terminals', 'conversation-controls', 'conversation-queue'])
    return { ...status, capabilities: status.capabilities?.filter(value => !native.has(value)) }
  },
  projects: () => read('projects'),
  project: id => read('project', { id }),
  runs: () => read('runs'),
  serverTasks: (projectId, q = '', launchable = false) => read('tasks', { projectId, q, launchable }),
  task: (projectId, taskId) => read('tasks/detail', { projectId, taskId }),
  taskPageAvailable: async () => true,
  workstationSettings: () => read('workstation'),
  engines: () => read('engines'),
  taskEngines: projectId => read('task-engines', { projectId }),
  repositories: projectId => read('repositories', { projectId }),
  folders: projectId => read('folders', { projectId }),
  conversation: (id, since) => read('conversation', { id, ...(Number.isSafeInteger(since) ? { since } : {}) }),
  runResult: async id => { try { return await read('run-result', { id }) } catch (e) { if (e.status === 404) return null; throw e } },
  gitDiff: async (id, folder) => ({ ...await read('git-diff', { id, ...(folder ? { folder } : {}) }), markdownDocuments: true, markdownImages: true }),
  codexSettings: () => read('codex-settings'),
  agySettings: () => read('agy-settings'),
  workstationSandbox: () => read('workstation/sandbox'),
  claimSetupPrompt: async () => false,
  syncConsoleView: async () => {},
  syncConversationMode: async () => {},
  detach: async () => {},
  resize: () => {},
  input: () => {},
  desktopUpdates: async () => null,
  version: async () => ({ desktop: desktopVersion, agent: await read('version').then(value => value.version).catch(() => null), outdated: false }),
  copyText: text => navigator.clipboard.writeText(text),
}
for (const key of Object.keys(preferences)) {
  bridge[key] = async () => preferences[key]
  bridge['set' + key[0].toUpperCase() + key.slice(1)] = async value => { preferences[key] = value }
}
window.localAgent ??= new Proxy(bridge, {
  get(target, key) {
    if (key in target) return target[key]
    if (typeof key === 'string' && key.startsWith('on')) return () => () => {}
    return unavailable
  },
})
await import('./main.js')
document.querySelector('#app-title').textContent = 'Sectile Desktop · Live, read-only'
