import assert from 'node:assert/strict'
import { test } from 'node:test'
import { translations } from '../src/locales/translations.ts'

test('profile tabs are fully localized in French and English', () => {
  const expectedTabs = ['account', 'appearance', 'trackers', 'aiEngine', 'sdd', 'workstations']

  for (const tab of expectedTabs) {
    const frLabel = translations.fr.profileModal.tabs[tab]
    const enLabel = translations.en.profileModal.tabs[tab]

    assert.ok(frLabel, `Missing French translation for tab: ${tab}`)
    assert.ok(enLabel, `Missing English translation for tab: ${tab}`)
    assert.notEqual(frLabel.trim(), '')
    assert.notEqual(enLabel.trim(), '')
  }

  assert.equal(translations.fr.profileModal.tabs.account, 'Compte')
  assert.equal(translations.en.profileModal.tabs.account, 'Account')

  assert.equal(translations.fr.profileModal.tabs.appearance, 'Apparence')
  assert.equal(translations.en.profileModal.tabs.appearance, 'Appearance')

  assert.equal(translations.fr.profileModal.tabs.trackers, 'Identifiants Trackers')
  assert.equal(translations.en.profileModal.tabs.trackers, 'Tracker Credentials')

  assert.equal(translations.fr.profileModal.tabs.aiEngine, 'Paramètres de l\'agent')
  assert.equal(translations.en.profileModal.tabs.aiEngine, 'Agent settings')

  assert.equal(translations.fr.profileModal.tabs.sdd, 'Compétences & SDD')
  assert.equal(translations.en.profileModal.tabs.sdd, 'Skills & SDD')

  assert.equal(translations.fr.profileModal.tabs.workstations, 'Workstations & Agent')
  assert.equal(translations.en.profileModal.tabs.workstations, 'Workstations & Agent')
})

test('appearance settings (density, default views, uiScale, detailModes) are fully localized', () => {
  const fr = translations.fr.profileModal
  const en = translations.en.profileModal

  // Density descriptions
  assert.ok(fr.densityDesc?.compact)
  assert.ok(en.densityDesc?.compact)
  assert.match(fr.densityDesc.compact, /padding réduit/)
  assert.match(en.densityDesc.compact, /reduced padding/)

  assert.ok(fr.densityDesc?.standard)
  assert.ok(en.densityDesc?.standard)
  assert.match(fr.densityDesc.standard, /équilibre optimal/)
  assert.match(en.densityDesc.standard, /optimal balance/)

  assert.ok(fr.densityDesc?.comfortable)
  assert.ok(en.densityDesc?.comfortable)
  assert.match(fr.densityDesc.comfortable, /grands espacements/)
  assert.match(en.densityDesc.comfortable, /spacious padding/)

  // Default Views
  assert.equal(fr.defaultViews?.board, 'Tableau (Kanban)')
  assert.equal(en.defaultViews?.board, 'Board (Kanban)')
  assert.equal(fr.defaultViews?.list, 'Liste détaillée')
  assert.equal(en.defaultViews?.list, 'Detailed List')

  // UI Scale
  assert.equal(fr.uiScale, "Échelle de l'interface")
  assert.equal(en.uiScale, 'Interface Scale')
  assert.equal(fr.uiScales?.s90, '90% (Compact)')
  assert.equal(en.uiScales?.s90, '90% (Compact)')
  assert.equal(fr.uiScales?.s100, '100% (Défaut)')
  assert.equal(en.uiScales?.s100, '100% (Default)')
  assert.equal(fr.uiScales?.s112, '112% (Agrandie)')
  assert.equal(en.uiScales?.s112, '112% (Enlarged)')
  assert.equal(fr.uiScales?.s125, '125% (Large)')
  assert.equal(en.uiScales?.s125, '125% (Large)')

  // Detail Mode Descriptions
  assert.equal(fr.detailModeDesc?.panel, 'Glissement latéral à droite')
  assert.equal(en.detailModeDesc?.panel, 'Right-side sliding panel')
  assert.equal(fr.detailModeDesc?.modal, 'Boîte de dialogue au centre')
  assert.equal(en.detailModeDesc?.modal, 'Centered modal dialog')

  // Preferences footer
  assert.equal(fr.sectilePreferences, 'Préférences Sectile')
  assert.equal(en.sectilePreferences, 'Sectile Preferences')
})

test('spec-driven design (SDD) and skill prompts are fully localized in French and English', () => {
  const frSdd = translations.fr.profileModal.sdd
  const enSdd = translations.en.profileModal.sdd

  assert.ok(frSdd)
  assert.ok(enSdd)

  // Header & Badges
  assert.equal(frSdd.title, 'Framework Spec-Driven Design (SDD)')
  assert.equal(enSdd.title, 'Spec-Driven Design (SDD) Framework')
  assert.equal(frSdd.contractBadge, 'Contract-First')
  assert.equal(enSdd.contractBadge, 'Contract-First')
  assert.match(frSdd.subtitle, /Le Spec-Driven Design garantit/)
  assert.match(enSdd.subtitle, /Spec-Driven Design ensures/)

  // Frameworks
  assert.equal(frSdd.speckitTitle, 'GitHub Spec Kit')
  assert.equal(enSdd.speckitTitle, 'GitHub Spec Kit')
  assert.equal(frSdd.openspecTitle, 'OpenSpec')
  assert.equal(enSdd.openspecTitle, 'OpenSpec')

  // Lifecycle Steps
  assert.equal(frSdd.steps.clarify, '1. Clarifier')
  assert.equal(enSdd.steps.clarify, '1. Clarify')
  assert.equal(frSdd.steps.specify, '2. Spécifier')
  assert.equal(enSdd.steps.specify, '2. Specify')
  assert.equal(frSdd.steps.code, '3. Coder')
  assert.equal(enSdd.steps.code, '3. Code')
  assert.equal(frSdd.steps.adjust, '4. Ajuster')
  assert.equal(enSdd.steps.adjust, '4. Adjust')
  assert.equal(frSdd.steps.handoff, '5. Clôturer')
  assert.equal(enSdd.steps.handoff, '5. Handoff')

  // Prompts Customization
  const skills = ['clarify', 'specify', 'implement', 'adjust', 'handoff']
  for (const skill of skills) {
    const frP = frSdd.prompts[skill]
    const enP = enSdd.prompts[skill]
    assert.ok(frP, `Missing French SDD prompt: ${skill}`)
    assert.ok(enP, `Missing English SDD prompt: ${skill}`)
    assert.ok(frP.label && frP.label.length > 0)
    assert.ok(enP.label && enP.label.length > 0)
    assert.ok(frP.hint && frP.hint.length > 0)
    assert.ok(enP.hint && enP.hint.length > 0)
    assert.ok(frP.placeholder && frP.placeholder.length > 0)
    assert.ok(enP.placeholder && enP.placeholder.length > 0)
  }

  // Workstations (Section 1)
  assert.match(translations.fr.profileModal.workstations?.desc || '', /Gérez vos machines de développement/)
  assert.match(translations.en.profileModal.workstations?.desc || '', /Manage your paired development workstations/)
  assert.equal(translations.fr.profileModal.workstations?.title, 'Machines de travail')
  assert.equal(translations.en.profileModal.workstations?.title, 'Workstations')
  assert.equal(translations.fr.profileModal.workstations?.pairBtn, 'Appairer une machine')
  assert.equal(translations.en.profileModal.workstations?.pairBtn, 'Pair a workstation')

  // Direct MCP Integration (Section 2)
  assert.equal(translations.fr.profileModal.workstations?.directMcpTitle, 'Intégration MCP Directe')
  assert.equal(translations.en.profileModal.workstations?.directMcpTitle, 'Direct MCP Integration')
  assert.equal(translations.fr.profileModal.workstations?.directMcpBadge, 'Apps IA Desktop')
  assert.equal(translations.en.profileModal.workstations?.directMcpBadge, 'AI Desktop Apps')
  assert.match(translations.fr.profileModal.workstations?.directMcpDesc || '', /Connectez des applications d'IA desktop/)
  assert.match(translations.en.profileModal.workstations?.directMcpDesc || '', /Connect AI desktop applications/)

  // Local Execution (Section 3: Desktop App & Headless CLI Agent)
  assert.equal(translations.fr.profileModal.workstations?.desktopAppTitle, 'Application Sectile Desktop')
  assert.equal(translations.en.profileModal.workstations?.desktopAppTitle, 'Sectile Desktop App')
  assert.equal(translations.fr.profileModal.workstations?.headlessCliTitle, 'Agent CLI Headless')
  assert.equal(translations.en.profileModal.workstations?.headlessCliTitle, 'Headless CLI Agent')

  // AI Custom Provider & MCP Key
  assert.equal(translations.fr.profileModal.ai.customProviderLabel, 'CLI Personnalisé')
  assert.equal(translations.en.profileModal.ai.customProviderLabel, 'Custom CLI')
  assert.equal(translations.fr.profileModal.ai.mcpKeyPlaceholder, '<VOTRE_CLE_WORKSTATION>')
  assert.equal(translations.en.profileModal.ai.mcpKeyPlaceholder, '<YOUR_WORKSTATION_KEY>')
})
