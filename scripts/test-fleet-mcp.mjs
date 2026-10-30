// Opt-in real-provider MCP isolation checks against disposable Fleet state.
// Run with SECTILE_TEST_REAL_CODEX=1 and/or SECTILE_TEST_REAL_CLAUDE=1.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, readFile, rm } from 'node:fs/promises';
import { createServer } from 'node:http';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const providers = [
  ...(process.env.SECTILE_TEST_REAL_CODEX === '1' ? ['codex'] : []),
  ...(process.env.SECTILE_TEST_REAL_CLAUDE === '1' ? ['claude'] : []),
];
assert.ok(providers.length, 'Select a signed-in provider with SECTILE_TEST_REAL_CODEX=1 or SECTILE_TEST_REAL_CLAUDE=1');
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const binary = name => process.env[`SECTILE_TEST_${name.toUpperCase()}_BINARY`] ?? join(root, 'src-tauri/binaries', `sectile-${name}${process.platform === 'win32' ? '.exe' : ''}`);
const children = [];
const home = process.env.HOME;
assert.ok(home, 'The signed-in CLI home directory is required');
const baseEnv = Object.fromEntries(['PATH', 'SYSTEMROOT', 'WINDIR', 'TMP', 'TEMP', 'USER', 'CODEX_HOME'].filter(key => process.env[key]).map(key => [key, process.env[key]]));
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, timeoutMs = 120000) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    try { return await check(); } catch (error) { last = error; await delay(250); }
  }
  throw last ?? new Error('Timed out waiting for the Fleet fixture');
}
async function fetchJSON(origin, route, method = 'GET', body, headers = {}) {
  const response = await fetch(origin + route, {
    method,
    headers: { 'Content-Type': 'application/json', ...headers },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(15000),
  });
  assert.ok(response.ok, `${method} ${route}: HTTP ${response.status}`);
  return response.status === 204 ? null : response.json();
}
const requestedPort = Number(process.env.SECTILE_TEST_FLEET_PORT ?? 0);
assert.ok(Number.isInteger(requestedPort) && requestedPort >= 0 && requestedPort <= 65535, 'SECTILE_TEST_FLEET_PORT must be an integer between 0 and 65535');
const reservation = createServer();
await new Promise((resolve, reject) => {
  reservation.once('error', reject);
  reservation.listen(requestedPort, '127.0.0.1', resolve);
});
const port = reservation.address().port;
await new Promise(resolve => reservation.close(resolve));
const versions = {};
for (const name of ['server', 'agent']) {
  const result = spawnSync(binary(name), ['--version'], { encoding: 'utf8', windowsHide: true });
  assert.equal(result.status, 0, `The packaged ${name} must report its version`);
  versions[name] = result.stdout.trim();
}
console.log(JSON.stringify({ versions, port, providers }));
const data = await mkdtemp(join(tmpdir(), 'sectile-fleet-mcp-'));
function start(name, args, extra) {
  const child = spawn(binary(name), args, { cwd: data, env: { ...baseEnv, ...extra }, stdio: 'ignore', windowsHide: true });
  children.push(child);
  return child;
}
async function exit(child, signal = 'SIGTERM') {
  if (child.exitCode !== null || child.signalCode !== null) return;
  child.kill(signal);
  await Promise.race([new Promise(resolve => child.once('exit', resolve)), delay(3000)]);
  if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
}
async function sharedMCPFingerprint() {
  const values = [];
  for (const path of [join(home, '.claude.json'), join(home, '.codex/config.toml')]) {
    try {
      const bytes = await readFile(path);
      if (path.endsWith('.json')) {
        const config = JSON.parse(bytes.toString('utf8'));
        values.push(JSON.stringify({ servers: config.mcpServers, projects: Object.fromEntries(Object.entries(config.projects ?? {}).filter(([, value]) => value.mcpServers).map(([key, value]) => [key, value.mcpServers])) }));
      } else {
        // Codex may update unrelated preferences while a session runs. Compare
        // parsed MCP tables, including every nested transport/key field.
        const parsed = spawnSync('python3', ['-c', 'import sys,json,tomllib; print(json.dumps(tomllib.loads(sys.stdin.read()).get("mcp_servers", {}), sort_keys=True))'], { input: bytes, encoding: 'utf8', windowsHide: true });
        assert.equal(parsed.status, 0, 'Python 3.11+ is required to compare Codex MCP configuration');
        values.push(parsed.stdout);
      }
    } catch (error) { if (error.code !== 'ENOENT') throw error; values.push('absent'); }
  }
  return values.map(value => createHash('sha256').update(value).digest('hex'));
}
const originalMCP = await sharedMCPFingerprint();
let agentProcess;
let connection;
try {
  await mkdir(join(data, 'server'));
  await mkdir(join(data, 'agent'));
  const workspace = join(data, 'workspace');
  await mkdir(workspace);
  assert.equal(spawnSync('git', ['init', workspace], { stdio: 'ignore', windowsHide: true }).status, 0);
  const origin = `http://127.0.0.1:${port}`;
  start('server', [], {
    HOME: data, USERPROFILE: data, PORT: String(port), SECTILE_SERVER_HOST: '127.0.0.1',
    SECTILE_SERVER_DATA_DIR: join(data, 'server'), DB_PATH: join(data, 'server/tasks.db'),
  });
  await until(() => fetchJSON(origin, '/api/health'), 30000);
  assert.equal((await fetch(origin + '/api/local-runtime')).status, 404, 'The Fleet fixture must not enable Local runtime discovery');
  const signIn = await fetch(origin + '/auth/local', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: 'fleet-mcp@example.invalid' }) });
  assert.equal(signIn.status, 200);
  const cookie = signIn.headers.get('set-cookie').split(';')[0];
  const code = await fetchJSON(origin, '/api/pairing-codes', 'POST', {}, { Cookie: cookie });
  const pair = await fetchJSON(origin, '/api/v1/agent/pair', 'POST', { code: code.code, label: 'Disposable Fleet MCP isolation fixture' });
  const apiHeaders = { Authorization: `Bearer ${pair.token}` };
  const project = await fetchJSON(origin, '/api/projects', 'POST', { name: 'Fleet MCP isolation fixture', trackers: [], folderOnly: true }, apiHeaders);
  const discovery = join(data, 'agent-connection.json');
  async function startAgent() {
    await rm(discovery, { force: true });
    agentProcess = start('agent', ['--url', origin, '--project', 'all', '--desktop-info', discovery], {
      HOME: home, USERPROFILE: process.env.USERPROFILE ?? home,
      TOKEN: pair.token, SECTILE_PAIRED_DEVICE_ID: pair.deviceId,
      SECTILE_DESKTOP_DATA_DIR: join(data, 'agent'), SECTILE_DESKTOP_MCP_URL: origin,
    });
    connection = await until(async () => {
      const found = JSON.parse(await readFile(discovery, 'utf8'));
      const status = await fetchJSON(found.url, '/desktop/status', 'GET', undefined, { Authorization: `Bearer ${found.token}` });
      assert.equal(status.server, origin);
      assert.equal(status.connected, true);
      return found;
    }, 30000);
  }
  async function agent(route, method = 'GET', body) {
    return fetchJSON(connection.url, route, method, body, { Authorization: `Bearer ${connection.token}` });
  }
  await startAgent();
  await agent('/desktop/projects', 'POST', { projectId: project.id, path: workspace, folderOnly: true });
  for (const provider of providers) {
    const chat = await agent('/desktop/consoles', 'POST', { projectId: project.id, provider, view: 'conversation' });
    const route = `/desktop/conversation?id=${chat.id}`;
    await agent(route, 'POST', {
      message: `Call only get_project_context on the sectile_companion MCP server with projectId ${project.id}. Do not use shell, files, other MCP servers or other tools. Read the project name from the actual tool result, then reply FLEET_MCP_OK followed by that name.`,
      model: provider === 'codex' ? 'gpt-6-astra' : 'sonnet',
      mode: provider === 'codex' ? 'workspace-write' : 'acceptEdits',
    });
    await until(async () => {
      const view = await agent(route);
      for (const approval of view.approvals ?? []) {
        const claudeRead = approval.tool === 'mcp__sectile_companion__get_project_context';
        const codexRead = approval.tool === 'MCP · sectile_companion' && approval.description?.includes('get_project_context');
        assert.ok(claudeRead || codexRead, `Unexpected approval type: ${approval.tool}`);
        assert.ok(JSON.stringify(approval.input).includes(project.id), 'MCP consent must target only the disposable project');
        await agent(route, 'POST', { approval: { id: approval.id, decision: 'allow' } });
      }
      assert.equal(view.busy, false);
      const events = (view.events ?? []).map(JSON.parse);
      const tools = events.filter(event => event.kind === 'tool' && JSON.stringify(event).includes('get_project_context'));
      assert.ok(tools.length > 0, 'The provider must actually call the scoped MCP tool');
      assert.ok(events.some(event => event.kind === 'tool_result' && !event.error && tools.some(tool => tool.toolId === event.toolId) && event.text?.includes(project.name)), 'The selected backend must return the actual fixture project');
      assert.ok(events.some(event => event.kind === 'assistant' && event.text?.includes('FLEET_MCP_OK') && event.text?.includes(project.name)), 'The provider must use the real result in its answer');
    });
    await agent(`/desktop/stop?id=${chat.id}`, 'POST');
    assert.deepEqual(await sharedMCPFingerprint(), originalMCP, 'Provider MCP registrations changed');
    console.log(`${provider}: real scoped Fleet MCP read and response passed; shared MCP registrations unchanged.`);
  }
} finally {
  for (const child of children.reverse()) await exit(child);
  await rm(data, { recursive: true, force: true });
}
