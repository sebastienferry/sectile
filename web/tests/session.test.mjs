import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  safeRedirectPath, signInPath, redirectFromSearch, shouldRedirectToSignIn, needsSignIn,
  describeSignInMode, describeRole, SIGN_IN_PATH, unlockSealedCredentials,
} from '../src/lib/session.ts'

test('a return target stays inside the interface', () => {
  assert.equal(safeRedirectPath('/board'), '/board')
  assert.equal(safeRedirectPath('/tasks?id=1'), '/tasks?id=1')
  assert.equal(safeRedirectPath(''), '/')
  assert.equal(safeRedirectPath(null), '/')
  assert.equal(safeRedirectPath('//evil.example/x'), '/')
  assert.equal(safeRedirectPath('https://evil.example'), '/')
  assert.equal(safeRedirectPath('javascript:alert(1)'), '/')
  // Returning to the sign-in screen would loop.
  assert.equal(safeRedirectPath(SIGN_IN_PATH), '/')
  assert.equal(safeRedirectPath(SIGN_IN_PATH + '?redirect=%2Fboard'), '/')
})

test('the sign-in path remembers where the person was', () => {
  assert.equal(signInPath('/board?view=list'), '/signin?redirect=%2Fboard%3Fview%3Dlist')
  assert.equal(signInPath('/'), '/signin')
  assert.equal(signInPath('https://evil.example'), '/signin')
  assert.equal(redirectFromSearch('?redirect=%2Fboard'), '/board')
  assert.equal(redirectFromSearch('?redirect=//evil.example'), '/')
  assert.equal(redirectFromSearch(''), '/')
})

test('only a 401 on the interface API sends to sign-in, and never from the sign-in screen', () => {
  assert.equal(shouldRedirectToSignIn('/api/tasks', 401, '/'), true)
  assert.equal(shouldRedirectToSignIn('http://host/api/settings', 401, '/board'), true)
  assert.equal(shouldRedirectToSignIn('/api/tasks', 403, '/'), false)
  assert.equal(shouldRedirectToSignIn('/api/tasks', 200, '/'), false)
  assert.equal(shouldRedirectToSignIn('/api/tasks', 401, SIGN_IN_PATH), false)
  assert.equal(shouldRedirectToSignIn('/api/v1/agent/config', 401, '/'), false)
  assert.equal(shouldRedirectToSignIn('/assets/app.js', 401, '/'), false)
  assert.equal(shouldRedirectToSignIn('not a url at all ::', 401, '/'), false)
})

test('the sign-in screen is needed for anyone not signed in, with no escape hatch', () => {
  assert.equal(needsSignIn(null), false)
  assert.equal(needsSignIn({ userId: '', signedIn: false, identityProvider: false, mode: 'local', role: '' }), true)
  assert.equal(needsSignIn({ userId: '', signedIn: false, identityProvider: true, mode: 'oidc', role: '' }), true)
  assert.equal(needsSignIn({ userId: 'usr_1', signedIn: true, identityProvider: false, mode: 'local', role: 'member' }), false)
})

test('modes and roles have a label', () => {
  assert.equal(describeSignInMode('oidc'), 'Identity provider')
  assert.match(describeSignInMode('local'), /temporary/)
  assert.equal(describeRole('admin'), 'Admin')
  assert.equal(describeRole('member'), 'Member')
  assert.equal(describeRole(''), '')
})

test('a supplied sealing passphrase unlocks each sealed tracker, and a refusal only reports', async () => {
  const calls = []
  const originalFetch = globalThis.fetch
  globalThis.fetch = async (url, init) => {
    calls.push({ url, body: init?.body })
    if (url === '/api/me/tracker-credentials') {
      return { ok: true, json: async () => ({ credentials: [
        { tracker: 'jira', sealed: true, unlocked: false },
        { tracker: 'gitlab', sealed: true, unlocked: false },
        { tracker: 'github', sealed: false, unlocked: false },
      ] }) }
    }
    return { ok: JSON.parse(init.body).tracker === 'jira', json: async () => ({}) }
  }
  try {
    // No passphrase asks nothing: the session is open with the tokens locked.
    assert.equal(await unlockSealedCredentials('   '), '')
    assert.equal(calls.length, 0)

    const notice = await unlockSealedCredentials('open sesame')
    assert.equal(calls.length, 3)
    assert.deepEqual(calls.slice(1).map(call => JSON.parse(call.body).tracker), ['jira', 'gitlab'])
    assert.match(notice, /gitlab/)
    assert.doesNotMatch(notice, /jira/)
  } finally {
    globalThis.fetch = originalFetch
  }
})
