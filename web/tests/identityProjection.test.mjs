import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'

// The name in the chrome is now a projection of the signed-in account (#348), so
// the server always answers a real one and the components need no fallback. What
// this suite pins is the absence of the placeholders those fallbacks were — a
// maintainer's own name shipped as a default — and the fact that the settings
// payload no longer carries a field the server ignores. Both are read as text, so
// the suite runs without a build step, like the other ones here.
const read = (path) => readFile(new URL(path, import.meta.url), 'utf8')

const sidebar = await read('../src/components/Sidebar.tsx')
const statusBar = await read('../src/components/StatusBar.tsx')
const appContext = await read('../src/context/AppContext.tsx')
const profileModal = await read('../src/components/ProfileModal.tsx')
const signInStatus = await read('../src/components/SignInStatus.tsx')

test('no placeholder identity is shipped as a default', () => {
  for (const [name, source] of Object.entries({ sidebar, statusBar, appContext, profileModal, signInStatus })) {
    assert.ok(!source.includes('Sylvain Ferry'), `${name} still falls back to a maintainer's name`)
    assert.ok(!source.includes("'SF'"), `${name} still falls back to a maintainer's initials`)
    assert.ok(!source.includes('Paramètres & Profil'), `${name} still falls back to a hardcoded label`)
    assert.ok(!source.includes("'Developer'"), `${name} still carries the seeded 'Developer' name`)
  }
})

test('the chrome reads the projected name with no fallback of its own', () => {
  // The account button at the foot of the sidebar: initials, name, address. The
  // "My Tasks" control above it is a separate, out-of-scope matter (#348) and is
  // deliberately not read here.
  const accountButton = sidebar.slice(sidebar.indexOf('setIsProfileOpen(true)'))
  assert.ok(accountButton.includes('settings.userName.substring(0, 2).toUpperCase()'), 'the sidebar no longer derives initials')
  assert.ok(!/settings\.userName \|\|/.test(accountButton), 'the sidebar still substitutes a name of its own')
  assert.ok(!/settings\.userEmail \|\|/.test(accountButton), 'the sidebar still substitutes an address of its own')
  assert.ok(statusBar.includes('{settings.userName}'), 'the status bar no longer shows the projected name')
  assert.ok(!/settings\.userName \|\|/.test(statusBar), 'the status bar still substitutes a name of its own')
})

test('the pre-fetch default names nobody', () => {
  assert.match(appContext, /userName: '',/, "AppContext's default userName is not an empty string")
})

test('saving the profile no longer posts the projected name', () => {
  const start = profileModal.indexOf('const handleSave')
  assert.ok(start >= 0, 'handleSave was not found in ProfileModal')
  const handleSave = profileModal.slice(start, profileModal.indexOf('setIsProfileOpen(false)', start))
  assert.ok(!handleSave.includes('userName:'), 'handleSave still sends userName, which the server ignores')
})

test('a rename in the account tab re-reads the settings the chrome shows', () => {
  const start = signInStatus.indexOf('async function saveName')
  assert.ok(start >= 0, 'saveName was not found in SignInStatus')
  const saveName = signInStatus.slice(start, signInStatus.indexOf('async function signOut', start))
  assert.ok(saveName.includes('reloadSettings()'), 'a rename leaves the sidebar and the status bar on the old name')
  assert.match(appContext, /reloadSettings: fetchSettings,/, 'AppContext does not expose the settings re-read')
})
