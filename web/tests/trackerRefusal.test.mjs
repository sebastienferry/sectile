import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  TRACKER_CREDENTIAL_MISSING,
  TrackerCredentialMissingError,
  missingCredentialFromActivity,
  missingCredentialFromBody,
} from '../src/lib/trackerRefusal.ts'

const refusal = { error: 'no personal GitHub token for this user', code: TRACKER_CREDENTIAL_MISSING, tracker: 'github' }

test('a 403 carrying the code names its provider', () => {
  assert.equal(missingCredentialFromBody(403, refusal), 'github')
  assert.equal(missingCredentialFromBody(403, { ...refusal, tracker: 'jira' }), 'jira')
  assert.equal(missingCredentialFromBody(403, { ...refusal, tracker: 'gitlab' }), 'gitlab')
})

test('any other failure is not a missing token', () => {
  // The refusal of a key tied to no user is a 403 too, without the code.
  assert.equal(missingCredentialFromBody(403, { error: 'tracker write refused: this key is not tied to a user' }), null)
  assert.equal(missingCredentialFromBody(500, refusal), null)
  assert.equal(missingCredentialFromBody(403, { ...refusal, tracker: 'bitbucket' }), null)
  assert.equal(missingCredentialFromBody(403, { ...refusal, tracker: undefined }), null)
  assert.equal(missingCredentialFromBody(403, null), null)
  assert.equal(missingCredentialFromBody(403, 'no personal GitHub token'), null)
})

test('a failed activity names the provider it was refused for', () => {
  assert.equal(missingCredentialFromActivity({ status: 'failed', credentialMissing: 'github' }), 'github')
  assert.equal(missingCredentialFromActivity({ status: 'failed', credentialMissing: '' }), null)
  assert.equal(missingCredentialFromActivity({ status: 'failed' }), null)
  assert.equal(missingCredentialFromActivity({ status: 'completed', credentialMissing: 'github' }), null)
  assert.equal(missingCredentialFromActivity({ status: 'failed', credentialMissing: 'svn' }), null)
})

test('the error keeps the provider and the message', () => {
  const err = new TrackerCredentialMissingError('refused', 'gitlab')
  assert.ok(err instanceof Error)
  assert.equal(err.tracker, 'gitlab')
  assert.equal(err.message, 'refused')
})
