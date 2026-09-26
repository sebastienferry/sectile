import test from 'node:test'
import assert from 'node:assert/strict'
import { serverCredentialLabel, serverCredentialVariables } from '../src/lib/serverCredentials.ts'

test('serverCredentialLabel tells the four situations apart', () => {
  assert.equal(serverCredentialLabel({ source: 'database' }), 'stored')
  assert.equal(serverCredentialLabel({ source: 'database', unreadable: true }), 'storedUnreadable')
  assert.equal(serverCredentialLabel({ source: 'environment' }), 'environment')
  assert.equal(serverCredentialLabel({ source: 'none' }), 'none')
  // An unreadable flag on anything but a stored credential means nothing.
  assert.equal(serverCredentialLabel({ source: 'environment', unreadable: true }), 'environment')
})

test('serverCredentialVariables names each provider its own variables only', () => {
  assert.deepEqual(serverCredentialVariables('github'), ['SECTILE_GITHUB_TOKEN'])
  assert.deepEqual(serverCredentialVariables('gitlab'), ['SECTILE_GITLAB_TOKEN'])
  assert.deepEqual(serverCredentialVariables('jira'), ['SECTILE_JIRA_EMAIL', 'SECTILE_JIRA_TOKEN'])
})
