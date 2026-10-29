import { spawnSync } from 'node:child_process';
import { resolve, dirname, join } from 'node:path';
import { mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const source = resolve(process.env.SECTILE_SOURCE_DIR || join(root, '../sectile'));
const output = join(root, 'src-tauri/binaries');
mkdirSync(output, { recursive: true });
for (const service of ['server', 'agent']) {
  const name = `sectile-${service}${process.platform === 'win32' ? '.exe' : ''}`;
  const result = spawnSync('go', ['build', '-o', join(output, name), `./cmd/${service}`], { cwd: source, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
