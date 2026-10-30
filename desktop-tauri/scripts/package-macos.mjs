import { spawnSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

if (process.platform !== 'darwin') {
  throw new Error('The macOS package command must run on macOS.');
}

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: root,
    env: process.env,
    stdio: 'inherit',
    windowsHide: true,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

run(process.execPath, [
  resolve(root, 'node_modules/@tauri-apps/cli/tauri.js'),
  'build',
  '--bundles',
  'app,dmg',
]);
run('python3', [resolve(root, 'scripts/verify-macos-package.py')]);
