/**
 * Strings of the workflow skill editor (#531): execution modes and their help,
 * skill list indicators, editor chrome and feedback.
 *
 * French is the reference and keeps the wording the interface already had;
 * English is typed after it, so a key missing in English fails the build.
 *
 * Skill names, commands, IDs and the skill Markdown are content, not chrome:
 * none of them is held here.
 */
const fr = {
  modes: {
    label: 'Mode',
    projectDefault: 'Défaut du projet',
    projectDefaultHelp: 'La skill ne fixe rien : le défaut du projet décide',
    interactive: 'Interactif',
    interactiveHelp: 'Ouvre un terminal que tu réponds, et tu confirmes la transition',
    autonomous: 'Autonome',
    autonomousHelp: 'Lance la CLI en headless, sans terminal ; le worker pose la transition',
    selectTitle: "Mode d'exécution de cette skill. Une surcharge au lancement le remplace pour ce lancement seulement.",
    autonomousRun: 'Exécution autonome (headless)',
    interactiveSession: 'Session interactive',
    executedCommand: 'Commande exécutée',
    providerDefault: 'Défaut du fournisseur',
  },
  list: {
    noProject: 'Sélectionne un projet : les skills sont éditées par projet, et régénérées dans le dépôt de ce projet.',
    title: 'Skills du workflow',
    pipeline: 'Clarify → Specify → Implement → Adjust → Handoff.',
    loading: 'Lecture des skills…',
    additionalSkills: 'Skills supplémentaires',
    additionalSkill: 'Skill supplémentaire',
    macroRefinement: 'Raffinage Macro',
    macroRealignment: 'Réalignement Macro',
    noSelection: 'Choisis une skill à gauche.',
  },
  indicators: {
    macro: 'MACRO',
    macroTitle: 'Skill de cadrage et raffinage Macro',
    custom: 'PERSO',
    customTitle: 'Contenu propre à ce projet',
    notInstalled: 'NON INSTALLÉE',
    notInstalledTitle: 'Aucun SKILL.md dans le dépôt',
    diverged: 'DIVERGENTE',
    divergedTitle: 'Le fichier du dépôt diffère',
  },
  editor: {
    reconciliationRequired:
      "La personnalisation héritée doit être réconciliée. Relis le contenu complet et enregistre-le sous Adjust, ou réinitialise-le au modèle par défaut. L'ajustement automatique est bloqué.",
    overrideOrigin: 'Source : {origin}. Autres entrées enregistrées : {entries}',
    noOtherEntries: 'aucune',
    preservedCustomization: 'Personnalisation conservée : {id}',
    reset: 'Réinitialiser',
    resetTitle: 'Revenir au modèle intégré de Sectile',
    save: 'Enregistrer',
    saveTitle: "Enregistrer les instructions de la skill pour l'agent local",
    upToDate: 'À jour',
    lines: { one: '{count} ligne', other: '{count} lignes' },
    updatedAt: 'modifiée le {date}',
    customContent: 'contenu propre à ce projet',
    builtInTemplate: 'modèle intégré de Sectile',
  },
  feedback: {
    modeDecidedByCommand:
      'Cette commande décide elle-même du mode. Définis une commande autonome, ou ajoute-lui un placeholder {mode:AUTONOMOUS|INTERACTIVE}.',
    noHeadlessMode:
      "{provider} n'a pas de mode headless attesté. Lance en interactif, ou écris un modèle portant {mode:AUTONOMOUS|INTERACTIVE}.",
    thisProvider: 'Ce fournisseur',
    unsupportedProvider: 'Fournisseur non pris en charge {provider} : configure un modèle de commande IA.',
    noProvider: '(aucun)',
  },
}

export type SkillsEditorStrings = typeof fr

const en: SkillsEditorStrings = {
  modes: {
    label: 'Mode',
    projectDefault: 'Project default',
    projectDefaultHelp: 'The skill sets nothing: the project default decides',
    interactive: 'Interactive',
    interactiveHelp: 'Opens a terminal you answer, and you confirm the transition',
    autonomous: 'Headless',
    autonomousHelp: 'Runs the CLI headless, without a terminal; the worker makes the transition',
    selectTitle: 'Execution mode of this skill. An override at launch replaces it for that launch only.',
    autonomousRun: 'Headless run',
    interactiveSession: 'Interactive session',
    executedCommand: 'Executed command',
    providerDefault: 'Provider default',
  },
  list: {
    noProject: "Select a project: skills are edited per project, and regenerated in that project's repository.",
    title: 'Workflow skills',
    pipeline: 'Clarify → Specify → Implement → Adjust → Handoff.',
    loading: 'Reading skills…',
    additionalSkills: 'Additional skills',
    additionalSkill: 'Additional skill',
    macroRefinement: 'Macro refinement',
    macroRealignment: 'Macro realignment',
    noSelection: 'Pick a skill on the left.',
  },
  indicators: {
    macro: 'MACRO',
    macroTitle: 'Macro scoping and refinement skill',
    custom: 'CUSTOM',
    customTitle: 'Content specific to this project',
    notInstalled: 'NOT INSTALLED',
    notInstalledTitle: 'No SKILL.md in the repository',
    diverged: 'DIVERGED',
    divergedTitle: 'The repository file differs',
  },
  editor: {
    reconciliationRequired:
      'Legacy customization requires reconciliation. Review the complete content and save under Adjust, or reset to the default. Automatic adjustment is blocked.',
    overrideOrigin: 'Source: {origin}. Other saved entries: {entries}',
    noOtherEntries: 'none',
    preservedCustomization: 'Preserved customization: {id}',
    reset: 'Reset',
    resetTitle: "Go back to Sectile's built-in template",
    save: 'Save',
    saveTitle: 'Save skill instructions for the local agent',
    upToDate: 'Up to date',
    lines: { one: '{count} line', other: '{count} lines' },
    updatedAt: 'updated {date}',
    customContent: 'content specific to this project',
    builtInTemplate: "Sectile's built-in template",
  },
  feedback: {
    modeDecidedByCommand:
      'This command decides the mode itself. Set an autonomous command, or add a {mode:AUTONOMOUS|INTERACTIVE} placeholder to this one.',
    noHeadlessMode:
      '{provider} has no attested headless mode. Run interactively, or write a template carrying {mode:AUTONOMOUS|INTERACTIVE}.',
    thisProvider: 'This provider',
    unsupportedProvider: 'Unsupported provider {provider}: configure an AI command template.',
    noProvider: '(none)',
  },
}

export const skillsEditor = { fr, en }
