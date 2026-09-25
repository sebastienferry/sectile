import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  codeRepositoryIdentity,
  declaredRepositories,
  droppedRepositoryPaths,
  duplicateRepository,
  repositoryIdentity,
} from '../src/lib/repositories.ts'

test('identity matches the server for every remote spelling', () => {
  const cases = {
    'git@github.com:sebastienferry/sectile.git': 'github.com/sebastienferry/sectile',
    'https://github.com/sebastienferry/sectile': 'github.com/sebastienferry/sectile',
    'https://GitHub.com/SebastienFerry/Sectile.git/': 'github.com/sebastienferry/sectile',
    'ssh://git@gitlab.com:22/smartadserver/private/arch/argocd-arch': 'gitlab.com/smartadserver/private/arch/argocd-arch',
    'git@gitlab.com:smartadserver/private/arch/argocd-arch.git': 'gitlab.com/smartadserver/private/arch/argocd-arch',
    'deploy@gitlab.example.org:group/app.git': 'gitlab.example.org/group/app',
    'https://user:secret@gitlab.example.org/group/app.git': 'gitlab.example.org/group/app',
    '  github.com/o/a  ': 'github.com/o/a',
    // The cases internal/models/repository_test.go pins for the server too.
    'https://github.com/o/r.git?x=1': 'github.com/o/r.git',
    'https://github.com/o/r#frag': 'github.com/o/r',
    'https://github.com/o/my%20repo': 'github.com/o/my repo',
    'ssh://git@[::1]:22/o/r': '::1/o/r',
  }
  for (const [remote, want] of Object.entries(cases)) {
    assert.equal(repositoryIdentity(remote), want, remote)
  }
})

test('a second spelling of a remote is a duplicate', () => {
  assert.equal(duplicateRepository('git@github.com:o/a.git', ['https://github.com/o/b', 'https://github.com/o/a']), 'https://github.com/o/a')
  assert.equal(duplicateRepository('', ['https://github.com/o/b', 'git@github.com:o/b.git']), 'git@github.com:o/b.git')
  assert.equal(duplicateRepository('git@github.com:o/a.git', ['https://github.com/o/b', '']), '')
})

test('declared repositories leave the code remote out', () => {
  const project = {
    gitRemoteUrl: 'git@github.com:o/a.git',
    repositories: [
      { url: 'git@github.com:o/a.git', identity: 'github.com/o/a' },
      { url: 'https://github.com/o/b', identity: 'github.com/o/b' },
    ],
  }
  assert.deepEqual(declaredRepositories(project), ['https://github.com/o/b'])
  assert.deepEqual(declaredRepositories({ gitRemoteUrl: '' }), [])
})

test('dropped paths are read off the migration report', () => {
  const report = JSON.stringify({
    converted: [{ path: '/src/a', url: 'git@github.com:o/a.git', taskIds: ['t1'] }],
    dropped: [{ path: '/src/gone', reason: 'no Git remote', taskIds: ['t2'] }],
    convertedAt: '2026-09-25T10:00:00Z',
    userId: 'u1',
  })
  assert.deepEqual(droppedRepositoryPaths(report), [{ path: '/src/gone', reason: 'no Git remote', taskIds: ['t2'] }])
  assert.deepEqual(droppedRepositoryPaths(''), [])
  assert.deepEqual(droppedRepositoryPaths('{not json'), [])
  assert.deepEqual(droppedRepositoryPaths(JSON.stringify({ converted: [] })), [])
})

test('the code remote of a project known by its GitHub repository is not declared', () => {
  const project = { gitRemoteUrl: '', githubRepo: 'o/a', repositories: [
    { url: 'https://github.com/o/a', identity: 'github.com/o/a' },
    { url: 'git@github.com:o/b.git', identity: 'github.com/o/b' },
  ] }
  assert.equal(codeRepositoryIdentity(project), 'github.com/o/a')
  assert.deepEqual(declaredRepositories(project), ['git@github.com:o/b.git'])
  assert.equal(codeRepositoryIdentity({ gitRemoteUrl: '', githubRepo: '' }), '')
})
