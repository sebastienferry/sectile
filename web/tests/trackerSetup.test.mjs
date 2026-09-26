import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'
import {
  PERSONAL_TRACKERS,
  PROJECT_TRACKERS,
  trackerHas,
  prefillFromCredential,
  TRACKERS,
  canCheck,
  credentialState,
  initialTracker,
  needsCredentialsFor,
  saveBlockedReason,
  scopesFor,
  sealingConsequence,
  SEALING_INVITATION,
  sealingInvitation,
  storedFor,
  trackerFields,
  getTrackers,
} from '../src/lib/trackers.ts'

test('the three trackers are offered, Jira asking for an account e-mail', () => {
  assert.deepEqual(TRACKERS.map(t => t.id), ['jira', 'github', 'gitlab'])
  assert.equal(trackerFields('jira').wantsEmail, true)
  assert.equal(trackerFields('github').wantsEmail, false)
  assert.equal(trackerFields('gitlab').wantsEmail, false)
  // Jira takes a project key, which is what the sync queries on.
  assert.equal(trackerFields('jira').projectPlaceholder, 'PE')
})

test('the screen opens on the tracker the project already uses', () => {
  assert.equal(initialTracker('github'), 'github')
  assert.equal(initialTracker('jira'), 'jira')
  assert.equal(initialTracker('gitlab'), 'gitlab')
  assert.equal(initialTracker('local'), 'jira')
  assert.equal(initialTracker(undefined), 'jira')
})

test('each tracker is prefilled from its own stored values', () => {
  const settings = {
    jiraUrl: 'https://acme.atlassian.net',
    jiraProject: 'PE',
    jiraApiTokenSet: true,
    githubApiUrl: 'https://api.github.com',
    githubRepo: 'acme/app',
    githubTokenFromEnv: true,
    gitlabUrl: 'https://gitlab.com/api/v4',
    gitlabProject: 'group/app',
  }
  assert.deepEqual(storedFor(settings, 'jira'), {
    siteUrl: 'https://acme.atlassian.net',
    project: 'PE',
    tokenIsSet: true,
    tokenFromEnv: false,
  })
  assert.deepEqual(storedFor(settings, 'github'), {
    siteUrl: 'https://api.github.com',
    project: 'acme/app',
    tokenIsSet: false,
    tokenFromEnv: true,
  })
  assert.equal(storedFor(settings, 'gitlab').project, 'group/app')
  // An empty configuration shows empty fields rather than undefined.
  assert.deepEqual(storedFor({}, 'jira'), { siteUrl: '', project: '', tokenIsSet: false, tokenFromEnv: false })
})

test('a token may stay empty: the check revalidates the stored one', () => {
  // An Atlassian account belongs to a site, so the personal card carries both.
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }), true)
  assert.equal(canCheck('jira', { email: 'ada@example.com' }), false)
  assert.equal(canCheck('jira', { siteUrl: 'acme.atlassian.net' }), false)
  // GitHub and GitLab have a public instance the server falls back to, so an
  // empty site must not block their check either.
  assert.equal(canCheck('github', { siteUrl: '' }), true)
  assert.equal(canCheck('gitlab', {}), true)
})

test('a blocked save says why, rather than greying a button in silence', () => {
  assert.match(saveBlockedReason('jira', {}, false), /site/)
  assert.match(saveBlockedReason('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }, false), /Vérifiez les accès/)
  assert.match(saveBlockedReason('github', {}, false), /Vérifiez/)
  // Once the instance accepted them, nothing is in the way any more.
  assert.equal(saveBlockedReason('jira', { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' }, true), '')
})

test('a tracker that attributes its writes is personal only', () => {
  // The server token stays a configuration fallback, never a box in the screen.
  assert.deepEqual(scopesFor('jira'), ['personal'])
  assert.deepEqual(scopesFor('github'), ['personal'])
  assert.deepEqual(scopesFor('gitlab'), ['personal'])
  // The site belongs to the person, not to the server: an Atlassian account is
  // tied to its instance.
  assert.equal(trackerFields('jira').siteIsPersonal, true)
  assert.equal(trackerFields('github').siteIsPersonal, undefined)
})

test('every tracker with a server adapter can be set on a project', () => {
  // The two selectors (project card, sync view) share this list because they
  // drifted once: Jira left the project card and stayed in the sync view, so no
  // project could be put on the tracker the server knew how to drive.
  // GitLab joined them once its adapter was registered (#398).
  assert.deepEqual(PROJECT_TRACKERS.map(t => t.id), ['local', 'github', 'jira', 'gitlab'])
  assert.equal(PROJECT_TRACKERS.every(t => t.label.trim().length > 0), true)
})

test('the screen says what sealing asks of you, not how it works', () => {
  // What a reader can act on: they will have to unseal. A key and a cipher
  // teach them nothing, so neither appears.
  assert.match(SEALING_INVITATION, /agir sur les tâches/)
  assert.match(sealingConsequence(true), /phrase de scellement unique/)
  assert.notEqual(sealingConsequence(true), sealingConsequence(false))
  for (const text of [SEALING_INVITATION, sealingConsequence(true), sealingConsequence(false)]) {
    assert.doesNotMatch(text, /chiffr|clé du serveur|base de données/i)
  }
})

test('a personal credential reads as absent, stored, sealed or locked', () => {
  assert.match(credentialState(undefined), /ne partiront pas/)
  assert.match(credentialState({ tracker: 'jira', sealed: false, unlocked: true }), /sous votre compte/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: true }), /descellé/)
  assert.match(credentialState({ tracker: 'jira', sealed: true, unlocked: false }), /descellez/)
})

test('a stored credential puts its site and e-mail back in the form', () => {
  // Without the e-mail the check button stays disabled, so the save can never
  // unlock: the screen would ask again for what the person already gave.
  const mine = { tracker: 'jira', siteUrl: 'acme.atlassian.net', email: 'ada@example.com', sealed: true, unlocked: false }
  assert.deepEqual(prefillFromCredential(mine, {}), { siteUrl: 'acme.atlassian.net', email: 'ada@example.com' })
  assert.equal(canCheck('jira', prefillFromCredential(mine, {})), true)
  // What is being typed wins over what is stored.
  assert.deepEqual(prefillFromCredential(mine, { siteUrl: 'other.atlassian.net', email: 'bob@example.com' }), {
    siteUrl: 'other.atlassian.net',
    email: 'bob@example.com',
  })
  assert.deepEqual(prefillFromCredential(undefined, {}), { siteUrl: '', email: '' })
})

test('only a tracker the server can drive offers a personal credential', () => {
  // A personal GitLab token was once storable while no adapter could use it;
  // GitLab has one now (#398), so its personal credential is offered again.
  assert.deepEqual(PERSONAL_TRACKERS.map(t => t.id), ['jira', 'github', 'gitlab'])
  // And every tracker a project can be put on can hold a personal credential.
  for (const t of PROJECT_TRACKERS) {
    if (t.id === 'local') continue
    assert.equal(PERSONAL_TRACKERS.some(p => p.id === t.id), true, `${t.id} has an adapter but no personal credential`)
  }
})

test('a project only asks for credentials the person does not already have', () => {
  const noServer = {}
  const mine = [{ tracker: 'jira', siteUrl: 'acme.atlassian.net', email: 'ada@example.com', sealed: false, unlocked: true }]

  // Jira has no server-wide credential in the interface at all, so asking the
  // global settings answered "nothing configured" whatever the person stored:
  // the connection screen reopened on every Jira project created or edited.
  assert.equal(needsCredentialsFor('jira', noServer, mine), false)
  assert.equal(needsCredentialsFor('jira', noServer, []), true)

  // GitHub keeps its server token, and a personal one counts too.
  assert.equal(needsCredentialsFor('github', { githubTokenSet: true }, []), false)
  assert.equal(needsCredentialsFor('github', { githubTokenFromEnv: true }, []), false)
  assert.equal(needsCredentialsFor('github', noServer, [{ tracker: 'github', sealed: false, unlocked: true }]), false)
  assert.equal(needsCredentialsFor('github', noServer, []), true)

  // A local project needs nothing.
  assert.equal(needsCredentialsFor('local', noServer, []), false)
})

test('tracker credentials translations are complete and localized states format correctly', () => {
  assert.ok(translations.fr.trackerCredentials.masterPassphraseTitle)
  assert.ok(translations.en.trackerCredentials.masterPassphraseTitle)
  assert.notEqual(
    translations.fr.trackerCredentials.masterPassphraseTitle,
    translations.en.trackerCredentials.masterPassphraseTitle
  )

  const mine = { tracker: 'jira', sealed: true, unlocked: false }
  const defaultState = credentialState(mine)
  const frState = credentialState(mine, translations.fr)
  const enState = credentialState(mine, translations.en)

  assert.equal(defaultState, translations.fr.trackerCredentials.states.locked)
  assert.equal(frState, translations.fr.trackerCredentials.states.locked)
  assert.equal(enState, translations.en.trackerCredentials.states.locked)
  assert.match(enState, /Sealed and locked/)

  assert.equal(sealingConsequence(true, translations.fr), translations.fr.trackerCredentials.sealingConsequences.sealed)
  assert.equal(sealingConsequence(true, translations.en), translations.en.trackerCredentials.sealingConsequences.sealed)
  assert.match(sealingConsequence(true, translations.en), /Sealed: only you can open it/)
  assert.match(sealingConsequence(false, translations.en), /Unsealed: your actions will be performed/)

  assert.equal(sealingInvitation(translations.en), translations.en.trackerCredentials.sealingInvitation)
  assert.match(sealingInvitation(translations.en), /master sealing passphrase/)

  // Localized tracker fields
  assert.equal(trackerFields('jira', translations.fr).tokenHint, "À créer sur id.atlassian.com, section jetons d'API. Il s'utilise avec votre e-mail Atlassian, jamais seul.")
  assert.equal(trackerFields('jira', translations.en).tokenHint, 'Create at id.atlassian.com, API tokens section. Used with your Atlassian email, never alone.')
  assert.equal(trackerFields('jira', translations.en).projectPlaceholder, 'e.g. MKTG')
  assert.equal(trackerFields('github', translations.en).siteLabel, 'GitHub API URL')
  assert.equal(trackerFields('github', translations.fr).siteLabel, "URL de l'API GitHub")
  assert.equal(trackerFields('gitlab', translations.en).siteLabel, 'GitLab API URL')
  assert.equal(trackerFields('gitlab', translations.fr).siteLabel, "URL de l'API GitLab")
  assert.equal(trackerFields('gitlab', translations.en).projectLabel, 'Default project')
  assert.equal(trackerFields('gitlab', translations.fr).projectLabel, 'Projet par défaut')

  const enTrackers = getTrackers(translations.en)
  const frTrackers = getTrackers(translations.fr)
  assert.equal(enTrackers.length, 3)
  assert.equal(frTrackers.length, 3)
  assert.equal(enTrackers.find(t => t.id === 'github')?.siteLabel, 'GitHub API URL')
  assert.equal(frTrackers.find(t => t.id === 'github')?.siteLabel, "URL de l'API GitHub")

  // Form and setup translations
  assert.ok(translations.fr.trackerCredentials.form.accountEmail)
  assert.ok(translations.en.trackerCredentials.form.accountEmail)
  assert.equal(translations.fr.trackerCredentials.form.accountEmail, 'E-mail du compte')
  assert.equal(translations.en.trackerCredentials.form.accountEmail, 'Account email')
  assert.equal(translations.fr.trackerCredentials.setup.title, 'Connecter votre tracker')
  assert.equal(translations.en.trackerCredentials.setup.title, 'Connect your tracker')

  // Localized save blocked reason
  assert.match(saveBlockedReason('jira', {}, false, translations.fr), /Renseignez votre site/)
  assert.match(saveBlockedReason('jira', {}, false, translations.en), /Enter your site/)
  assert.match(saveBlockedReason('jira', { siteUrl: 'a', email: 'b' }, false, translations.en), /Verify credentials/)
})

test('profile AI and MCP configuration translations are complete in French and English', () => {
  const frAi = translations.fr.profileModal.ai
  const enAi = translations.en.profileModal.ai

  // Variables disponibles
  assert.equal(frAi.availableVariables, 'Variables disponibles :')
  assert.equal(enAi.availableVariables, 'Available variables:')

  // Commande exécutée
  assert.equal(frAi.executedCommand, 'Commande exécutée')
  assert.equal(enAi.executedCommand, 'Executed command')

  // Modes
  assert.equal(frAi.modeInteractive, 'Interactif')
  assert.equal(enAi.modeInteractive, 'Interactive')
  assert.equal(frAi.modeAutonomous, 'Autonome')
  assert.equal(enAi.modeAutonomous, 'Autonomous')

  // MCP sans agent local
  assert.equal(frAi.mcpConfigWithoutAgent, 'Configuration MCP sans agent local')
  assert.equal(enAi.mcpConfigWithoutAgent, 'MCP configuration without local agent')

  // Pour connecter directement...
  assert.match(frAi.mcpConnectDirectlyDesc || '', /Pour connecter directement votre CLI ou IDE/)
  assert.match(enAi.mcpConnectDirectlyDesc || '', /To connect your CLI or IDE directly/)
})


test('the ticket panel writes an assignee, a sprint and a team where the tracker has them', () => {
  for (const source of ['jira', 'gitlab']) {
    for (const feature of ['assigneeLookup', 'sprint', 'team']) {
      assert.equal(trackerHas(source, feature), true, `${source} ${feature}`)
    }
  }
  for (const source of ['github', 'local', '', undefined]) {
    assert.equal(trackerHas(source, 'team'), false, `${source} has no team`)
    assert.equal(trackerHas(source, 'sprint'), false, `${source} has no sprint`)
  }
})
