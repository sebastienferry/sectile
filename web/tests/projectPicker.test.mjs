import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  PICKER_RESULT_LIMIT,
  descriptionExcerpt,
  highlightParts,
  matchProject,
  orderProjects,
  overviewProjects,
  pickerModel,
  projectLocation,
  projectRepositories,
  repositoryLabel,
  trackerLabel,
  projectTrackerKinds,
} from '../src/lib/projectPicker.ts'

const project = (id, fields = {}) => ({
  id,
  name: id,
  slug: id.toLowerCase(),
  description: '',
  icon: '',
  color: 'indigo',
  githubRepo: '',
  issueTracker: 'github',
  isDefault: false,
  createdAt: '',
  updatedAt: '',
  ...fields,
})

const opened = (...ids) => ids.map(id => ({ id, openedAt: '2026-09-28T09:00:00.000Z' }))
const names = rows => rows.map(row => (row.project ? row.project.id : row.id))

test('an empty search lists 3 recents, skipping favorites and unknown projects', () => {
  const projects = [
    project('A'), project('B', { bookmarked: true }), project('C'), project('D'), project('E'),
    project('F1', { bookmarked: true }),
  ]
  const model = pickerModel(projects, opened('A', 'gone', 'B', 'C', 'D', 'E'), '', 'en')
  assert.deepEqual(names(model.recent), ['A', 'C', 'D'])
  assert.deepEqual(names(model.favorites), ['B', 'F1'])
  assert.deepEqual(model.others, [])
  assert.equal(model.hiddenFavorites, 0)
  assert.equal(model.hiddenMatches, 0)
})

test('an empty search shows 6 favorites A–Z and counts the others', () => {
  const projects = ['Zeta', 'alpha', 'Beta', 'Gamma', 'Delta', 'Epsilon', 'Eta', 'Theta', 'Iota']
    .map(name => project(name, { bookmarked: true }))
  const model = pickerModel(projects, [], '', 'en')
  assert.deepEqual(names(model.favorites), ['alpha', 'Beta', 'Delta', 'Epsilon', 'Eta', 'Gamma'])
  assert.equal(model.hiddenFavorites, 3)
  assert.deepEqual(model.recent, [])
})

test('with no favorite and no history, the menu lists no project', () => {
  const model = pickerModel([project('A'), project('B')], [], '', 'en')
  assert.deepEqual(model.recent, [])
  assert.deepEqual(model.favorites, [])
  assert.deepEqual(model.others, [])
})

test('a search lists favorites first, then others by history, capped at 6 overall', () => {
  const favorites = Array.from({ length: 8 }, (_, i) => project(`Fav${i}`, { bookmarked: true, description: 'billing' }))
  const others = [project('Old', { description: 'billing' }), project('New', { description: 'billing' })]
  const model = pickerModel([...others, ...favorites], opened('New'), 'billing', 'en')
  assert.equal(model.favorites.length + model.others.length, PICKER_RESULT_LIMIT)
  assert.deepEqual(names(model.favorites), ['Fav0', 'Fav1', 'Fav2', 'Fav3', 'Fav4', 'Fav5'])
  assert.deepEqual(model.others, [])
  assert.equal(model.hiddenMatches, 4)
  assert.deepEqual(model.recent, [])
})

test('other matches follow the history, then A–Z', () => {
  const projects = [project('Charlie'), project('Alpha'), project('Bravo'), project('Delta', { bookmarked: true })]
  const model = pickerModel(projects, opened('Bravo'), 'a', 'en')
  assert.deepEqual(names(model.favorites), ['Delta'])
  assert.deepEqual(names(model.others), ['Bravo', 'Alpha', 'Charlie'])
  assert.equal(model.hiddenMatches, 0)
})

test('matching ignores case and accents, on every searchable field', () => {
  const p = project('Paiements', {
    slug: 'pay',
    description: 'Équipe facturation',
    repositories: [{ url: 'git@github.com:acme/Billing.git', identity: 'github.com/acme/billing' }],
    issueTracker: 'jira',
    jiraProject: 'PE',
  })
  assert.deepEqual(matchProject(p, 'PAIE'), { field: 'name', text: 'Paiements' })
  assert.deepEqual(matchProject(p, 'pay'), { field: 'slug', text: 'pay' })
  assert.deepEqual(matchProject(p, 'equipe'), { field: 'description', text: 'Équipe facturation' })
  assert.deepEqual(matchProject(p, 'acme/bil'), { field: 'repository', text: 'acme/Billing' })
  assert.deepEqual(matchProject(p, 'JIRA'), { field: 'tracker', text: 'Jira' })
  assert.deepEqual(matchProject(project('X', { jiraProject: 'QRS' }), 'qrs'), { field: 'tracker', text: 'QRS' })
  assert.equal(matchProject(p, 'nothing'), null)
  assert.deepEqual(matchProject(p, '  '), { field: 'name', text: 'Paiements' })
})

test('repositories come from every source, shortened and without duplicates', () => {
  assert.deepEqual(projectRepositories(project('A', { gitRemoteUrl: 'https://github.com/acme/web.git', githubRepo: 'acme/web' })), ['acme/web'])
  assert.deepEqual(projectRepositories(project('B', {
    repositories: [
      { url: 'git@gitlab.com:group/api.git', identity: '' },
      { url: 'ssh://git@host:2222/group/docs', identity: '' },
    ],
    gitlabProject: 'group/extra',
  })), ['group/api', 'group/docs', 'group/extra'])
  assert.deepEqual(projectRepositories(project('C')), [])
})

test('repositoryLabel keeps owner/name from any remote form', () => {
  assert.equal(repositoryLabel('git@github.com:sebastienferry/sectile.git'), 'sebastienferry/sectile')
  assert.equal(repositoryLabel('https://github.com/Acme/Web/'), 'Acme/Web')
  assert.equal(repositoryLabel('acme/web'), 'acme/web')
})

test('trackerLabel names every tracker', () => {
  assert.equal(trackerLabel('github'), 'GitHub')
  assert.equal(trackerLabel('gitlab'), 'GitLab')
  assert.equal(trackerLabel('jira'), 'Jira')
  assert.equal(trackerLabel('local'), 'Local')
  assert.equal(trackerLabel(undefined), 'Local')
})

test('orderProjects puts favorites A–Z first', () => {
  const projects = [project('b'), project('Z', { bookmarked: true }), project('a'), project('É', { bookmarked: true })]
  assert.deepEqual(names(orderProjects(projects, opened('b'), 'fr')), ['É', 'Z', 'b', 'a'])
})

test('highlightParts maps a folded match back to the original characters', () => {
  assert.deepEqual(highlightParts('Équipe paiement', 'equipe'), [
    { text: 'Équipe', match: true },
    { text: ' paiement', match: false },
  ])
  assert.deepEqual(highlightParts('Réunion Équipe', 'EQUI'), [
    { text: 'Réunion ', match: false },
    { text: 'Équi', match: true },
    { text: 'pe', match: false },
  ])
  // A decomposed accent stays attached to the matched letter.
  assert.deepEqual(highlightParts('café noir', 'cafe'), [
    { text: 'café', match: true },
    { text: ' noir', match: false },
  ])
  assert.deepEqual(highlightParts('Sectile', 'xyz'), [{ text: 'Sectile', match: false }])
  assert.deepEqual(highlightParts('Sectile', ''), [{ text: 'Sectile', match: false }])
})

test('descriptionExcerpt starts shortly before a distant match', () => {
  assert.equal(descriptionExcerpt('Short text', 'text'), 'Short text')
  const long = 'Orchestrates the agentic development workflow for every team, including billing'
  const excerpt = descriptionExcerpt(long, 'billing')
  assert.ok(excerpt.startsWith('…'), excerpt)
  assert.ok(excerpt.endsWith('including billing'), excerpt)
  assert.ok(excerpt.length < long.length)
  assert.equal(descriptionExcerpt(long, 'absent'), long)
})

test('overviewProjects combines the tracker and the text filters', () => {
  const projects = [
    project('Web', { issueTracker: 'github', bookmarked: true }),
    project('Api', { issueTracker: 'gitlab', description: 'web backend' }),
    project('Board', { issueTracker: 'jira' }),
    project('Notes', { issueTracker: 'local' }),
  ]
  assert.deepEqual(names(overviewProjects(projects, [], '', 'all', 'en')), ['Web', 'Api', 'Board', 'Notes'])
  assert.deepEqual(names(overviewProjects(projects, [], 'web', 'all', 'en')), ['Web', 'Api'])
  assert.deepEqual(names(overviewProjects(projects, [], 'web', 'gitlab', 'en')), ['Api'])
  assert.deepEqual(names(overviewProjects(projects, [], '', 'local', 'en')), ['Notes'])
  assert.deepEqual(names(overviewProjects(projects, [], 'zzz', 'all', 'en')), [])
})

test('projectLocation shows the field that matched, highlighted', () => {
  const jira = project('Billing', { issueTracker: 'jira', jiraProject: 'BIL', gitRemoteUrl: 'git@gitlab.com:finance/billing.git' })
  assert.deepEqual(projectLocation(jira, matchProject(jira, 'bil')), { tracker: 'Jira', key: '', location: 'finance/billing', matched: null }, 'a name match highlights nothing here')
  const keyOnly = project('Ledger', { issueTracker: 'jira', jiraProject: 'FIN' })
  assert.deepEqual(projectLocation(keyOnly, matchProject(keyOnly, 'fin')), { tracker: 'Jira', key: 'FIN', location: 'ledger', matched: 'key' })
  assert.deepEqual(projectLocation(keyOnly, matchProject(keyOnly, 'jira')), { tracker: 'Jira', key: '', location: 'ledger', matched: 'tracker' })
  const site = project('Website', { slug: 'www', githubRepo: 'marketing/site' })
  assert.deepEqual(projectLocation(site, matchProject(site, 'ww')), { tracker: 'GitHub', key: '', location: 'www', matched: 'location' }, 'a slug match shows the slug')
  assert.deepEqual(projectLocation(site, matchProject(site, 'market')), { tracker: 'GitHub', key: '', location: 'marketing/site', matched: 'location' })
  assert.deepEqual(projectLocation(site, null), { tracker: 'GitHub', key: '', location: 'marketing/site', matched: null })
})

test('a project of several trackers is found under each of them, and by each Jira key', () => {
  // A project selects trackers (#741): its legacy fields name its default
  // tracker only, the others come from the trackers' identities.
  const delivery = project('Delivery', {
    issueTracker: 'jira',
    jiraProject: 'GODE',
    trackers: [
      { trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' },
      { trackerId: 'be', identity: 'jira|acme.atlassian.net|BE' },
      { trackerId: 'app', identity: 'github|api.github.com|acme/app' },
    ],
  })
  const notes = project('Notes', { issueTracker: 'local', trackers: [{ trackerId: 'loc', identity: 'local||notes' }] })
  assert.deepEqual(projectTrackerKinds(delivery), ['jira', 'github'])
  assert.deepEqual(projectTrackerKinds(notes), ['local'])
  assert.deepEqual(projectTrackerKinds(project('Old', { issueTracker: 'gitlab' })), ['gitlab'])
  assert.deepEqual(names(overviewProjects([delivery, notes], [], '', 'github', 'en')), ['Delivery'])
  assert.deepEqual(names(overviewProjects([delivery, notes], [], '', 'jira', 'en')), ['Delivery'])
  assert.deepEqual(matchProject(delivery, 'be'), { field: 'tracker', text: 'BE' })
  assert.deepEqual(matchProject(delivery, 'github'), { field: 'tracker', text: 'GitHub' })
})
