import assert from 'node:assert/strict'
import { test } from 'node:test'
import { currentPullRequestLink, pullRequestStateLabel, addPullRequestLink } from '../src/lib/pullRequests.ts'

test('the last PR determines the state independently of task workflow', () => {
 const old = {url:'https://github.com/a/b/pull/1',state:'merged'}
 const current = {url:'https://github.com/a/b/pull/2',state:'conflicting'}
 assert.equal(currentPullRequestLink({prLinks:[old,current],prUrl:old.url,status:'done'}),current)
 assert.equal(currentPullRequestLink({prUrl:old.url,status:'done'}).state,undefined)
 assert.equal(currentPullRequestLink({}),undefined)
})
test('every observed state has a distinct accessible label', () => {
 const labels = ['open','conflicting','merged','closed',undefined].map(pullRequestStateLabel)
 assert.equal(new Set(labels).size,5)
 assert.match(labels[1],/conflits/)
 assert.equal(pullRequestStateLabel('unexpected'),labels[4])
})
test('link edits preserve observed history without assigning state to a new PR', () => {
 const first={url:'https://github.com/a/b/pull/1',state:'merged'}
 const links=addPullRequestLink([first],'https://github.com/a/b/pull/2','feat/233')
 assert.equal(links[0],first)
 assert.equal(links[1].state,undefined)
 assert.equal(addPullRequestLink(links,first.url),links)
})
