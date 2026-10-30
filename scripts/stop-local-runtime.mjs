import { readFile } from 'node:fs/promises';
import { isAbsolute, join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

class LocalUpdateError extends Error {}

async function optionalJson(path) {
  try { return JSON.parse(await readFile(path, 'utf8')); }
  catch (error) { if (error.code === 'ENOENT') return undefined; throw error; }
}

function loopbackOrigin(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port ||
      url.username || url.password || url.pathname !== '/' || url.search || url.hash) {
    throw new LocalUpdateError('Adresse du compagnon invalide. Aucun arrêt demandé.');
  }
  return url.origin;
}

/** Stop only the authenticated managed Local runtime, never a Fleet agent. */
export async function stopLocalRuntime(root, { timeoutMs = 30_000, pollMs = 100 } = {}) {
  if (!isAbsolute(root)) throw new LocalUpdateError('Indiquez le chemin absolu du dossier local-runtime.');
  const mode = await optionalJson(join(root, 'runtime-mode.json'));
  if (mode !== undefined && (!mode || mode.protocol !== 1 || mode.mode !== 'local')) {
    throw new LocalUpdateError('Cette commande est réservée au mode Local. Aucun arrêt demandé.');
  }
  const selection = await optionalJson(join(root, 'runtime-selection.json'));
  if (selection !== undefined && (!selection || selection.protocol !== 1 || selection.kind !== 'companion' ||
      selection.generation !== 'generations/companion-v1')) {
    throw new LocalUpdateError('Sélection du service incompatible. Aucun arrêt demandé.');
  }
  const active = selection ? join(root, selection.generation) : root;
  const installation = await optionalJson(join(active, 'installation.json'));
  const connection = await optionalJson(join(active, 'agent-connection.json'));
  if (!installation || !Number.isInteger(installation.port) || installation.port < 1 ||
      installation.port > 65535 || typeof installation.key !== 'string' || installation.key.length < 32 ||
      !connection || typeof connection.token !== 'string' || !connection.token) {
    throw new LocalUpdateError('Installation locale ou connexion absente. Aucun arrêt demandé.');
  }
  const origin = `http://127.0.0.1:${installation.port}`;
  const agent = loopbackOrigin(connection.url);
  const request = (base, path, token, method = 'GET') => fetch(base + path, {
    method, headers: { Authorization: `Bearer ${token}` },
    redirect: 'error', signal: AbortSignal.timeout(3000),
  });
  const proofResponse = await request(origin, '/api/local-runtime', installation.key);
  if (proofResponse.status !== 200) throw new LocalUpdateError('Compagnon local non authentifié. Aucun arrêt demandé.');
  const proof = await proofResponse.json();
  if (proof.protocol !== 1 || proof.companion !== true || proof.ready !== true) {
    throw new LocalUpdateError('Le service connecté n’est pas le compagnon Local. Aucun arrêt demandé.');
  }
  const statusResponse = await request(agent, '/desktop/status', connection.token);
  if (statusResponse.status !== 200 || (await statusResponse.json()).server !== origin) {
    throw new LocalUpdateError('L’agent ne correspond pas au compagnon Local. Aucun arrêt demandé.');
  }
  const response = await request(agent, '/desktop/shutdown', connection.token, 'POST');
  if (response.status === 409) {
    throw new LocalUpdateError('Arrêt refusé : terminez les exécutions et préparations actives, puis réessayez.');
  }
  if (response.status !== 204) throw new LocalUpdateError('Arrêt non confirmé par le compagnon.');

  // Only connection refusal proves the transport closed. An HTTP error or a
  // request timeout can be a live service and must not authorize replacement.
  async function closed(base, path, token) {
    try { const response = await request(base, path, token); await response.body?.cancel(); return false; }
    catch (error) { return error.cause?.code === 'ECONNREFUSED'; }
  }
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const stopped = await Promise.all([
      closed(origin, '/api/local-runtime', installation.key),
      closed(agent, '/desktop/status', connection.token),
    ]);
    if (stopped.every(Boolean)) return;
    await delay(pollMs);
  }
  throw new LocalUpdateError('Arrêt incomplet ou service relancé. Fermez toutes les fenêtres Tauri avant de réessayer.');
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    if (process.argv.length !== 4 || process.argv[2] !== '--data-dir') {
      throw new LocalUpdateError('Usage : npm run stop:local -- --data-dir /chemin/absolu/local-runtime');
    }
    await stopLocalRuntime(process.argv[3]);
    console.log('Arrêt confirmé des connexions du compagnon Local. Vous pouvez remplacer l’application puis la rouvrir.');
  } catch (error) {
    // Do not print request objects or nested causes: they may contain credentials.
    console.error(error instanceof LocalUpdateError ? error.message : 'Connexion ou configuration locale non confirmée. Aucun remplacement autorisé.');
    process.exitCode = 1;
  }
}
