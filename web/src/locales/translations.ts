export interface TranslationSchema {
  app: {
    title: string
    tagline: string
  }
  nav: {
    allTasks: string
    backlog: string
    toClarify: string
    specified: string
    inProgress: string
    toValidate: string
    done: string
    views: string
    board: string
    list: string
    roadmap: string
    timeline: string
    roadmapTooltip: string
    timelineTooltip: string
    activities: string
    sync: string
    filters: string
    myTasks: string
    urgentHigh: string
    labels: string
    sources: string
    allSources: string
    localSource: string
    parents: string
    clearParentFilter: string
    /** Bouton qui demande le brief du jour à l'agent du projet. */
    dailyBrief: string
    dailyBriefRunning: string
    settings: string
    reseedDemo: string
    toggleSidebar: string
    syncGithub: string
  }
  syncView: {
    title: string
    subtitle: string
    githubCard: {
      title: string
      desc: string
      repoLabel: string
      pathLabel: string
      btnSync: string
      statusConnected: string
    }
    globalCard: {
      title: string
      desc: string
      btnSync: string
    }
    options: {
      title: string
      desc: string
      defaultTracker: string
      githubRepo: string
      repoPath: string
      save: string
      saved: string
    }
    history: {
      title: string
      desc: string
      viewInActivities: string
      noHistory: string
    }
  }
  header: {
    searchPlaceholder: string
    quickAdd: string
    commandPalette: string
    boardView: string
    listView: string
    activitiesView: string
    filterStatus: string
    filterPriority: string
    activeFilter: string
    activeFilters: string
    syncing: string
    syncNow: string
    hideDone: string
    showDone: string
  }
  board: {
    emptyColumn: string
    addTask: string
    dragHint: string
  }
  list: {
    columns: {
      key: string
      title: string
      status: string
      priority: string
      labels: string
      assignee: string
      dueDate: string
      source: string
      actions: string
    }
    empty: string
  }
  status: {
    to_clarify: string
    clarified: string
    to_implement: string
    to_test: string
    to_close: string
    finished: string
    backlog: string
    specified: string
    in_progress: string
    to_validate: string
    done: string
  }
  priority: {
    urgent: string
    high: string
    medium: string
    low: string
  }
  skills: {
    title: string
    subtitle: string
    pipeline: string
    runSkill: string
    running: string
    nextStep: string
    autoPilot: string
    clarify: string
    specify: string
    implement: string
    createPr: string
    history: string
    noHistory: string
    branch: string
    pullRequest: string
    viewPr: string
    output: string
    stepsTitle: string
    skillSuccess: string
  }
  taskModal: {
    createTitle: string
    editTitle: string
    titlePlaceholder: string
    descPlaceholder: string
    status: string
    priority: string
    labels: string
    addLabel: string
    assignee: string
    dueDate: string
    cancel: string
    save: string
    create: string
    delete: string
    deleteConfirm: string
    created: string
    updated: string
    switchToModal: string
    switchToPanel: string
  }
  convert: {
    bannerTitle: string
    bannerDesc: string
    btnGithub: string
    converting: string
    success: string
  }
  quickAdd: {
    title: string
    placeholder: string
    hint: string
    status: string
    priority: string
    tracker: string
  }
  commandPalette: {
    searchPlaceholder: string
    general: string
    createTask: string
    switchBoard: string
    switchList: string
    toggleTheme: string
    changeLanguage: string
    openProfile: string
    reseed: string
    syncGithub: string
    tasksSection: string
    skillsSection: string
    noResults: string
    hintNavigate: string
    hintSelect: string
    hintClose: string
  }
  profileModal: {
    title: string
    subtitle: string
    tabs: {
      account: string
      appearance: string
      trackers: string
      aiEngine: string
      sdd: string
      workstations: string
      aiConfig: string
      tracker: string
    }
    userSection: string
    name: string
    email: string
    appearanceSection: string
    accentColor: string
    theme: string
    themes: {
      dark: string
      light: string
      system: string
    }
    language: string
    languages: {
      fr: string
      en: string
    }
    density: string
    densities: {
      compact: string
      standard: string
      comfortable: string
    }
    densityDesc?: {
      compact: string
      standard: string
      comfortable: string
    }
    defaultView: string
    defaultViews?: {
      board: string
      list: string
    }
    uiScale?: string
    uiScales?: {
      s90: string
      s100: string
      s112: string
      s125: string
    }
    detailMode: string
    detailModes: {
      modal: string
      panel: string
    }
    detailModeDesc?: {
      modal: string
      panel: string
    }
    accents: {
      indigo: string
      violet: string
      emerald: string
      amber: string
      rose: string
      cyan: string
      blue: string
      orange: string
      'neon-cyan': string
      'neon-purple': string
      'neon-green': string
      'neon-amber': string
    }
    sectilePreferences?: string
    ai: {
      title: string
      engine: string
      defaultEngine: string
      proposedModelsFor?: string
      proposedModelsGroup?: string
      noModelsConfigured?: string
      addModelPlaceholder?: string
      addModelButton?: string
      addModelAria?: string
      removeModelAria?: string
      engineIgnoresModelNotice?: string
      modelTemplatePlaceholderNotice?: string
      providerIgnoresModel?: string
      invalidModelIdentifier?: string
      defaultModel: string
      defaultModelPlaceholder?: string
      customModelOption?: string
      quickSelect?: string
      cliParametersTitle?: string
      cmdInteractive?: string
      cmdAutonomous?: string
      fastPresets?: string
      engineDesc: string
      cmdTemplate: string
      repoPath: string
      repoPathDesc: string
      promptsTitle: string
      promptClarify: string
      promptSpecify: string
      promptImplement: string
      promptAdjust: string
      promptHandoff: string
      promptCreatePr?: string
      cliStatusTitle: string
      availableVariables?: string
      executedCommand?: string
      modeInteractive?: string
      modeAutonomous?: string
      mcpConfigWithoutAgent?: string
      mcpConnectDirectlyDesc?: string
      mcpConnectDirectlyWorkflow?: string
      mcpKeyRequired?: string
      mcpKeyRequiredDesc?: string
      mcpKeyPlaceholder?: string
      mcpGenerateKey?: string
      mcpCopy?: string
      mcpCopied?: string
      mcpQuickCliCommand?: string
      mcpOrInConfig?: string
      customProviderLabel?: string
      customProviderSub?: string
    }
    sdd?: {
      title: string
      subtitle: string
      contractBadge: string
      defaultFramework: string
      speckitTitle: string
      speckitSubtitle: string
      speckitDesc: string
      speckitCommands: string
      openspecTitle: string
      openspecSubtitle: string
      openspecDesc: string
      openspecCommands: string
      lifecycleTitle: string
      customizePrompts: string
      steps: {
        clarify: string
        specify: string
        code: string
        adjust: string
        handoff: string
      }
      promptsSectionTitle: string
      promptsSectionDesc: string
      prompts: {
        clarify: {
          label: string
          hint: string
          placeholder: string
        }
        specify: {
          label: string
          hint: string
          placeholder: string
        }
        implement: {
          label: string
          hint: string
          placeholder: string
        }
        adjust: {
          label: string
          hint: string
          placeholder: string
        }
        handoff: {
          label: string
          hint: string
          placeholder: string
        }
      }
    }
    workstations?: {
      desc: string
      title?: string
      badge?: string
      pairBtn?: string
      workingBtn?: string
      singleUseNotice?: string
      onWorkstationTitle?: string
      terminalStep?: string
      desktopStep?: string
      pairedListTitle?: string
      noWorkstations?: string
      lastSeen?: string
      renewBtn?: string
      renewNever?: string
      revokeBtn?: string
      sharedTokenWarning?: string
      // Direct MCP
      directMcpTitle?: string
      directMcpBadge?: string
      directMcpDesc?: string
      labelInput?: string
      labelPlaceholder?: string
      expiresInput?: string
      ttl90?: string
      ttl30?: string
      ttl365?: string
      ttl0?: string
      createKeyBtn?: string
      keyCreatedSuccess?: string
      copyKeyBtn?: string
      copiedKey?: string
      desktopConfigsTitle?: string
      tabClaude?: string
      tabAgy?: string
      tabCodex?: string
      tabCursor?: string
      copyConfigBtn?: string
      copiedConfig?: string
      // Local Execution
      localExecTitle?: string
      desktopAppTitle?: string
      desktopAppBadge?: string
      desktopAppDesc?: string
      desktopStep1?: string
      desktopStep2?: string
      desktopStep3?: string
      headlessCliTitle?: string
      headlessCliBadge?: string
      headlessCliDesc?: string
      serverUrlLabel?: string
      urlInvalidAlert?: string
      copyCommandBtn?: string
      copiedCommand?: string
      cliPrereqNotice?: string
    }
    tracker: {
      title: string
      selectTracker: string
      githubRepo: string
      syncGithubBtn: string
    }
    save: string
    reseedBtn: string
  }
  account: {
    signOut: string
    displayName: string
    displayNamePlaceholder: string
    save: string
    notSignedIn: string
    signIn: string
    saved: string
    couldNotSignOut: string
  }
  trackerCredentials: {
    description: string
    masterPassphraseTitle: string
    statusLocked: string
    statusActive: string
    statusNotConfigured: string
    passphraseDescription: string
    lock: string
    locking: string
    unlockPrompt: string
    unlockPlaceholder: string
    unlockAll: string
    unlocking: string
    operationalBanner: string
    changePassphrase: string
    cancelChangePassphrase: string
    changePassphrasePrompt: string
    newPassphrasePlaceholder: string
    apply: string
    applying: string
    definePassphrasePrompt: string
    definePassphrasePlaceholder: string
    sealMyTokens: string
    toastUpdatedTitle: string
    toastUpdatedDesc: string
    toastRemovedTitle: string
    toastRemovedDesc: string
    toastErrorTitle: string
    toastErrorDefault: string
    states: {
      none: string
      unsealed: string
      unlocked: string
      locked: string
    }
    sealingInvitation: string
    sealingConsequences: {
      sealed: string
      unsealed: string
    }
    trackers: {
      jira: {
        siteLabel: string
        sitePlaceholder: string
        projectLabel: string
        projectPlaceholder: string
        tokenHint: string
      }
      github: {
        siteLabel: string
        sitePlaceholder: string
        projectLabel: string
        projectPlaceholder: string
        tokenHint: string
      }
      gitlab: {
        siteLabel: string
        sitePlaceholder: string
        projectLabel: string
        projectPlaceholder: string
        tokenHint: string
      }
    }
    saveBlockedReasons: {
      wantsEmail: string
      default: string
      needCheck: string
    }
    form: {
      accountEmail: string
      accountEmailPlaceholder: string
      personalAccessToken: string
      tokenPlaceholderSet: string
      tokenPlaceholderEnv: string
      tokenPlaceholderEmpty: string
      siteIsPersonalNotice: string
      lockedNoticeSuffix: string
      sealedUnlockedNotice: string
      willBeSealedNotice: string
      connectedAs: string
      forgetAccess: string
      forgetConfirm: string
      verify: string
      save: string
      saveTitleReady: string
      saveTitleNotChecked: string
      configuredTitle: string
      saveErrorTitle: string
      checkErrorDefault: string
    }
    setup: {
      title: string
      description: string
      closeTitle: string
      trackerLabel: string
      later: string
    }
  }
  activities: {
    title: string
    subtitle: string
    stats: {
      total: string
      running: string
      waiting: string
      queued: string
      completed: string
      failed: string
      canceled: string
    }
    filters: {
      all: string
      running: string
      waiting: string
      queued: string
      completed: string
      failed: string
      allSkills: string
      searchPlaceholder: string
    }
    table: {
      skill: string
      task: string
      status: string
      duration: string
      created: string
      actions: string
    }
    detail: {
      title: string
      prompt: string
      steps: string
      output: string
      error: string
      rawLogs: string
      renderedMarkdown: string
      copyOutput: string
      copied: string
      openTask: string
      openPr: string
      retry: string
      cancel: string
      delete: string
    }
    empty: {
      title: string
      desc: string
      noFilterMatch: string
    }
    actions: {
      refresh: string
      clearCompleted: string
      clearConfirm: string
    }
  }
  compactCard: {
    pin: string
    unpin: string
    advance: string
    advanceAuto: string
    advanceInteractive: string
    advanceWithModel: string
    currentModel: string
    advanceAutonomous: string
    filterParent: string
    clearParent: string
    openPr: string
  }
  statusBar: {
    branch: string
    clean: string
    modified: string
    untracked: string
    noRepo: string
    notGit: string
    refresh: string
    aiProvider: string
    activeJobs: string
    ready: string
    runningSkill: string
    cwd: string
    remote: string
    latestCommit: string
    copyBranch: string
    branchCopied: string
    viewDiff: string
  }
  mcp: {
    panelTitle: string
    client: string
    clients: string
    noClient: string
    run: string
    runs: string
    noRuns: string
    connected: string
    unavailable: string
    ownership: string
  }
  toasts: {
    taskCreated: string
    taskUpdated: string
    taskMoved: string
    taskDeleted: string
    settingsSaved: string
    demoReseeded: string
    skillStarted: string
    skillCompleted: string
    skillQueued: string
    activityRetried: string
    activityCanceled: string
    activityDeleted: string
    activitiesCleared: string
    syncSuccess: string
    error: string
  }
}

export const translations: Record<'fr' | 'en', TranslationSchema> = {
  fr: {
    app: {
      title: 'Sectile',
      tagline: 'Gestionnaire de tâches agentique',
    },
    nav: {
      allTasks: 'Toutes les tâches',
      backlog: 'Backlog',
      toClarify: 'À clarifier',
      specified: 'Spécifié',
      inProgress: 'En cours',
      toValidate: 'À valider',
      done: 'Terminé',
      views: 'Vues',
      board: 'Board',
      list: 'Backlog',
      roadmap: 'Roadmap',
      timeline: 'Timeline',
      roadmapTooltip: 'Roadmap : NOW / NEXT / FUTURE',
      timelineTooltip: 'Timeline Sprints',
      activities: 'Activités',
      sync: 'Synchro',
      filters: 'Filtres rapides',
      myTasks: 'Mes tâches',
      urgentHigh: 'Priorité Haute',
      labels: 'Étiquettes',
      sources: 'Sources',
      allSources: 'Toutes les sources',
      localSource: 'Local',
      parents: 'Macros / Parents',
      clearParentFilter: 'Retirer le filtre parent',
      dailyBrief: 'Brief du jour',
      dailyBriefRunning: 'Brief en cours…',
      settings: 'Profil & Préférences',
      reseedDemo: 'Réinitialiser démo',
      toggleSidebar: 'Replier / Déplier le menu',
      syncGithub: 'Synchroniser GitHub',
    },
    syncView: {
      title: 'Centre de Synchronisation',
      subtitle: 'Gérez l\'intégration bidirectionnelle avec GitHub et Jira. Les synchronisations s\'exécutent en arrière-plan sous forme de tâches dans vos Activités.',
      githubCard: {
        title: 'Synchronisation GitHub',
        desc: 'Importe les issues distantes depuis votre dépôt GitHub configuré via GitHub CLI (gh).',
        repoLabel: 'Dépôt GitHub',
        pathLabel: 'Répertoire local',
        btnSync: 'Lancer la synchro GitHub',
        statusConnected: 'GitHub CLI Connecté',
      },
      globalCard: {
        title: 'Synchronisation Globale',
        desc: 'Exécute une synchronisation complète en file d\'attente sur tous vos trackers configurés (GitHub + Jira).',
        btnSync: 'Tout synchroniser',
      },
      options: {
        title: 'Options & Préférences de synchronisation',
        desc: 'Personnalisez les identifiants d\'équipe, dépôts par défaut et chemins locaux utilisés par les workers d\'arrière-plan.',
        defaultTracker: 'Tracker distant principal',
        githubRepo: 'Dépôt GitHub (owner/repo)',
        repoPath: 'Chemin du projet local (CWD)',
        save: 'Enregistrer les paramètres',
        saved: 'Paramètres enregistrés !',
      },
      history: {
        title: 'Dernières synchronisations',
        desc: 'Historique des jobs de synchronisation exécutés dans la file d\'attente.',
        viewInActivities: 'Inspecter les logs dans Activités',
        noHistory: 'Aucune synchronisation récente.',
      },
    },
    header: {
      searchPlaceholder: 'Rechercher des tâches... (Appuyez sur "/" pour cibler)',
      quickAdd: 'Ajouter une tâche',
      commandPalette: 'Palette de commandes',
      boardView: 'Tableau',
      listView: 'Liste',
      activitiesView: 'Activités',
      filterStatus: 'Statut',
      filterPriority: 'Priorité',
      activeFilter: 'filtre actif',
      activeFilters: 'filtres actifs',
      syncing: 'Synchronisation...',
      syncNow: 'Synchroniser',
      hideDone: 'Masquer les tâches terminées',
      showDone: 'Afficher les tâches terminées',
    },
    board: {
      emptyColumn: 'Aucune tâche dans cette colonne',
      addTask: 'Ajouter une tâche',
      dragHint: 'Glisser-déposer pour changer de statut ou réordonner',
    },
    list: {
      columns: {
        key: 'Clé',
        title: 'Titre',
        status: 'Statut',
        priority: 'Priorité',
        labels: 'Étiquettes',
        assignee: 'Assigné à',
        dueDate: 'Échéance',
        source: 'Source',
        actions: 'Actions',
      },
      empty: 'Aucune tâche ne correspond à votre recherche ou filtre.',
    },
    status: {
      to_clarify: 'À clarifier',
      clarified: 'Cadré',
      to_implement: 'À implémenter',
      to_test: 'À tester',
      to_close: 'En revue / PR',
      finished: 'Terminé',
      backlog: 'À clarifier',
      specified: 'À spécifier',
      in_progress: 'À implémenter',
      to_validate: 'À tester',
      done: 'Terminé',
    },
    priority: {
      urgent: 'Urgent',
      high: 'Haute',
      medium: 'Moyenne',
      low: 'Basse',
    },
    skills: {
      title: 'Agent Copilot & Skills',
      subtitle: 'Exécutez les skills du workflow pour faire avancer la tâche',
      pipeline: 'Pipeline de développement',
      runSkill: 'Lancer la skill',
      running: 'Exécution en cours via CLI...',
      nextStep: 'Étape suivante',
      autoPilot: 'Auto-Pilot (/pick-issue)',
      clarify: 'Clarifier (/clarify-issue)',
      specify: 'Spécifier (/specify-issue)',
      implement: 'Coder (/code-issue)',
      createPr: 'Adjust (/adjust-issue)',
      history: 'Historique des exécutions & Artefacts',
      noHistory: 'Aucune exécution de skill pour le moment sur cette tâche.',
      branch: 'Branche Git',
      pullRequest: 'Pull Request',
      viewPr: 'Voir la PR sur GitHub',
      output: 'Rapport & Artefact généré',
      stepsTitle: 'Étapes exécutées',
      skillSuccess: 'Skill exécutée avec succès !',
    },
    taskModal: {
      createTitle: 'Nouvelle Tâche',
      editTitle: 'Détails & Agent Copilot',
      titlePlaceholder: 'Titre de la tâche...',
      descPlaceholder: 'Description, contexte technique, critères d\'acceptation...',
      status: 'Statut',
      priority: 'Priorité',
      labels: 'Étiquettes (séparées par des virgules ou Entrée)',
      addLabel: 'Ajouter une étiquette...',
      assignee: 'Assigné à',
      dueDate: 'Date d\'échéance',
      cancel: 'Annuler',
      save: 'Enregistrer',
      create: 'Créer la tâche',
      delete: 'Supprimer la tâche',
      deleteConfirm: 'Êtes-vous sûr de vouloir supprimer cette tâche ? Cette action est irréversible.',
      created: 'Créée le',
      updated: 'Modifiée le',
      switchToModal: 'Passer en modale centrée',
      switchToPanel: 'Passer en panneau latéral droit',
    },
    convert: {
      bannerTitle: 'Tâche locale (non synchronisée)',
      bannerDesc: 'Cette tâche est enregistrée uniquement en local dans SQLite.',
      btnGithub: 'Exporter vers GitHub',
      converting: 'Exportation vers le tracker distant...',
      success: 'Issue créée sur le tracker distant !',
    },
    quickAdd: {
      title: 'Ajout rapide',
      placeholder: 'Titre de la tâche (ex: Créer endpoint GraphQL, Corriger auth...)',
      hint: 'Appuyez sur Entrée pour créer immédiatement',
      status: 'Statut initial',
      priority: 'Priorité',
      tracker: 'Destination / Tracker',
    },
    commandPalette: {
      searchPlaceholder: 'Tapez une commande, skill ou tâche...',
      general: 'Actions générales',
      createTask: 'Créer une nouvelle tâche',
      switchBoard: 'Passer en vue Board',
      switchList: 'Passer en vue Liste',
      toggleTheme: 'Basculer le thème (Sombre / Clair)',
      changeLanguage: 'Changer la langue (FR / EN)',
      openProfile: 'Ouvrir les Préférences & Profil',
      reseed: 'Réinitialiser la base de données de démo',
      syncGithub: 'Synchroniser avec GitHub CLI',
      tasksSection: 'Accès direct aux tâches',
      skillsSection: 'Skills Agentiques (/skills)',
      noResults: 'Aucun résultat trouvé pour',
      hintNavigate: 'pour naviguer',
      hintSelect: 'pour choisir',
      hintClose: 'pour fermer',
    },
    profileModal: {
      title: 'Configuration & Paramètres',
      subtitle: 'Compte, apparence et configuration des outils',
      tabs: {
        account: 'Compte',
        appearance: 'Apparence',
        trackers: 'Identifiants Trackers',
        aiEngine: 'Paramètres de l\'agent',
        sdd: 'Compétences & SDD',
        workstations: 'Workstations & Agent',
        aiConfig: 'Moteur IA & Prompts',
        tracker: 'GitHub & Jira',
      },
      userSection: 'Informations utilisateur',
      name: 'Nom d\'utilisateur',
      email: 'Adresse e-mail',
      appearanceSection: 'Apparence & Ergonomie',
      accentColor: 'Couleur d\'accent',
      theme: 'Thème général',
      themes: {
        dark: 'Sombre',
        light: 'Clair',
        system: 'Système',
      },
      language: 'Langue de l\'interface',
      languages: {
        fr: 'Français (FR)',
        en: 'English (EN)',
      },
      density: 'Taille d\'affichage / Densité',
      densities: {
        compact: 'Compacte (Plus d\'éléments à l\'écran)',
        standard: 'Standard (Équilibrée)',
        comfortable: 'Confortable (Espaces généreux)',
      },
      densityDesc: {
        compact: '13px font, padding réduit',
        standard: '14px font, équilibre optimal',
        comfortable: '15px font, grands espacements',
      },
      defaultView: 'Vue par défaut',
      defaultViews: {
        board: 'Tableau (Kanban)',
        list: 'Liste détaillée',
      },
      uiScale: 'Échelle de l\'interface',
      uiScales: {
        s90: '90% (Compact)',
        s100: '100% (Défaut)',
        s112: '112% (Agrandie)',
        s125: '125% (Large)',
      },
      detailMode: 'Affichage des détails de tâche',
      detailModes: {
        modal: 'Modale centrée',
        panel: 'Panneau latéral droit (Right Panel)',
      },
      detailModeDesc: {
        panel: 'Glissement latéral à droite',
        modal: 'Boîte de dialogue au centre',
      },
      accents: {
        indigo: 'Indigo Royal',
        violet: 'Violet Électrique',
        emerald: 'Émeraude Vif',
        amber: 'Ambre Chaud',
        rose: 'Rose Bonbon',
        cyan: 'Cyan Tech',
        blue: 'Bleu Océan',
        orange: 'Orange Sunset',
        'neon-cyan': '⚡ Cyber Cyan Néon',
        'neon-purple': '🔮 Synthwave Magenta',
        'neon-green': '🟢 Matrix Green Néon',
        'neon-amber': '✨ Laser Gold Néon',
      },
      sectilePreferences: 'Préférences Sectile',
      ai: {
        title: 'Moteur IA & Exécution Shell',
        engine: 'Moteur Agentic IA par défaut',
        defaultEngine: 'Moteur Agentic IA par défaut',
        proposedModelsFor: 'Modèles proposés pour',
        proposedModelsGroup: 'Modèles proposés',
        noModelsConfigured: 'Aucun modèle : rien ne sera proposé au lancement.',
        addModelPlaceholder: 'Ajouter un modèle...',
        addModelButton: 'Ajouter ce modèle',
        addModelAria: 'Ajouter un modèle pour {target}',
        removeModelAria: 'Retirer {model} de {target}',
        engineIgnoresModelNotice: 'Ce moteur ignore le modèle sauf si sa commande porte le marqueur {model}.',
        modelTemplatePlaceholderNotice: 'Le modèle est appliqué via le marqueur {model} dans la commande.',
        providerIgnoresModel: '{provider} n\'accepte pas de sélection de modèle : la valeur est ignorée.',
        invalidModelIdentifier: 'Identifiant invalide : lettres, chiffres et . _ - : @ / uniquement, sans espace.',
        defaultModel: 'Modèle par défaut',
        defaultModelPlaceholder: 'Défaut du CLI',
        customModelOption: 'Autre modèle (saisie libre)...',
        quickSelect: 'Sélection rapide :',
        cliParametersTitle: 'CLI & Commandes : {provider}',
        cmdInteractive: 'Commande interactive',
        cmdAutonomous: 'Commande autonome (headless)',
        fastPresets: 'Modèles de commande rapides :',
        engineDesc: 'Configurez le moteur d\'intelligence artificielle par défaut, les modèles et les commandes CLI d\'exécution des skills.',
        cmdTemplate: 'Template de commande Shell CLI',
        repoPath: 'Répertoire du projet cible (CWD)',
        repoPathDesc: 'Emplacement du repo dans lequel l\'agent exécutera les commandes',
        promptsTitle: 'Personnalisation des Prompts par Skill',
        promptClarify: 'Prompt /clarify-issue (Cadrage & questions)',
        promptSpecify: 'Prompt /specify-issue (Spécification Spec Kit / OpenSpec)',
        promptImplement: 'Prompt /code-issue (Implémentation & tests)',
        promptAdjust: 'Prompt /adjust-issue (Ajustement, revue & màj PR)',
        promptHandoff: 'Prompt /handoff-issue (Clôture & handoff)',
        promptCreatePr: 'Prompt legacy de création de PR (obsolète)',
        cliStatusTitle: 'Statut des CLI Locales',
        availableVariables: 'Variables disponibles :',
        executedCommand: 'Commande exécutée',
        modeInteractive: 'Interactif',
        modeAutonomous: 'Autonome',
        mcpConfigWithoutAgent: 'Configuration MCP sans agent local',
        mcpConnectDirectlyDesc: "Pour connecter directement votre CLI ou IDE au serveur MCP Sectile sans passer par l'agent local (",
        mcpConnectDirectlyWorkflow: "). Le moteur accède directement aux outils de gestion des tâches et de suivi du workflow.",
        mcpKeyRequired: 'Clé requise :',
        mcpKeyRequiredDesc: "Remplacez {placeholder} par une clé d'API.",
        mcpKeyPlaceholder: '<VOTRE_CLE_WORKSTATION>',
        mcpGenerateKey: 'Générer une clé',
        mcpCopy: 'Copier',
        mcpCopied: 'Copié !',
        mcpQuickCliCommand: 'Commande rapide ({engine}) :',
        mcpOrInConfig: 'Ou dans',
        customProviderLabel: 'CLI Personnalisé',
        customProviderSub: 'Binaire ou script custom',
      },
      sdd: {
        title: 'Framework Spec-Driven Design (SDD)',
        subtitle: 'Le Spec-Driven Design garantit qu\'une spécification claire, structurée et vérifiable est rédigée et validée avant toute génération de code par les agents d\'IA.',
        contractBadge: 'Contract-First',
        defaultFramework: 'Framework par défaut du projet',
        speckitTitle: 'GitHub Spec Kit',
        speckitSubtitle: 'CLI specify',
        speckitDesc: 'Convention standard GitHub : structure modulaire dans .specify/ et specs/ (spec.md, plan.md, tasks.md).',
        speckitCommands: 'Commandes : /specify-issue, /code-issue',
        openspecTitle: 'OpenSpec',
        openspecSubtitle: 'CLI openspec',
        openspecDesc: 'Spécification formelle par deltas et exigences vérifiables. Les propositions de changements sont validées et revues avant l\'écriture de code.',
        openspecCommands: 'Commandes : openspec propose, validate',
        lifecycleTitle: 'Cycle de vie SDD dans Sectile',
        customizePrompts: 'Personnaliser les prompts',
        steps: {
          clarify: '1. Clarifier',
          specify: '2. Spécifier',
          code: '3. Coder',
          adjust: '4. Ajuster',
          handoff: '5. Clôturer',
        },
        promptsSectionTitle: 'Personnalisation des Prompts par Compétence',
        promptsSectionDesc: 'Personnalisez les invites (prompts) envoyées au CLI Agentic pour chaque étape du workflow SDD. Si laissé vide, les invites par défaut sont utilisées.',
        prompts: {
          clarify: {
            label: 'Prompt de Cadrage (/clarify-issue)',
            hint: 'Défaut : /clarify-issue {issueKey} tracked on {tracker} in {repo}',
            placeholder: '/clarify-issue {issueKey} tracked on {tracker} in {repo}',
          },
          specify: {
            label: 'Prompt de Spécification (/specify-issue)',
            hint: 'Spécification Spec Kit / OpenSpec',
            placeholder: 'Tu es le Product Owner pour {issueKey}. Rédige la spécification selon le framework SDD configuré...',
          },
          implement: {
            label: 'Prompt d\'Implémentation (/code-issue)',
            hint: 'Développement & Tests',
            placeholder: 'Tu es le développeur senior pour {issueKey}. Implémente le code dans {repoPath}...',
          },
          adjust: {
            label: 'Prompt d\'Ajustement & Revue (/adjust-issue)',
            hint: 'Revue de code & màj PR',
            placeholder: 'Tu es le reviewer senior pour {issueKey}. Revois les changements, applique les correctifs et mets à jour la PR...',
          },
          handoff: {
            label: 'Prompt de Clôture & Handoff (/handoff-issue)',
            hint: 'Documentation & Nettoyage local',
            placeholder: 'Tu es responsable de la clôture pour {issueKey}. Vérifie la fusion, rédige le rapport de handoff et nettoie le worktree...',
          },
        },
      },
      workstations: {
        desc: 'Gérez vos machines de développement appairées, les clés de signature des agents et la configuration de l\'agent local.',
        title: 'Machines de travail',
        badge: 'Appairage & Équipements',
        pairBtn: 'Appairer une machine',
        workingBtn: 'Opération en cours…',
        singleUseNotice: 'Usage unique. Non réutilisable une fois expiré.',
        onWorkstationTitle: 'Sur la machine de travail',
        terminalStep: 'Dans un terminal :',
        desktopStep: 'Ou dans Sectile Desktop :',
        pairedListTitle: 'Machines connectées',
        noWorkstations: 'Aucune machine n\'est actuellement appairée.',
        lastSeen: 'vu pour la dernière fois',
        renewBtn: 'Prolonger',
        renewNever: 'Permanent',
        revokeBtn: 'Révoquer',
        sharedTokenWarning: 'Ce serveur s\'exécute toujours avec SECTILE_SERVER_TOKEN, obsolète et retiré à la prochaine version. Appairez vos machines et supprimez la variable d\'environnement.',
        // Direct MCP
        directMcpTitle: 'Intégration MCP Directe',
        directMcpBadge: 'Apps IA Desktop',
        directMcpDesc: 'Connectez des applications d\'IA desktop (Cursor, Claude Desktop, Antigravity, etc.) directement au endpoint MCP (/mcp) de Sectile sans exécuter de démon local.',
        labelInput: 'Nom du client',
        labelPlaceholder: 'Ex: Claude Desktop sur MacBook Pro…',
        expiresInput: 'Expiration',
        ttl90: 'dans 90 jours',
        ttl30: 'dans 30 jours',
        ttl365: 'dans 1 an',
        ttl0: 'jamais',
        createKeyBtn: 'Créer une clé API',
        keyCreatedSuccess: 'Clé générée pour {label}. Copiez-la maintenant : elle ne sera plus jamais affichée.',
        copyKeyBtn: 'Copier la clé',
        copiedKey: 'Clé API copiée dans le presse-papier.',
        desktopConfigsTitle: 'Exemples de configuration Desktop',
        tabClaude: 'Claude Desktop',
        tabAgy: 'Antigravity',
        tabCodex: 'Codex / ChatGPT',
        tabCursor: 'Cursor',
        copyConfigBtn: 'Copier la config JSON',
        copiedConfig: 'Configuration copiée dans le presse-papier.',
        // Local Execution
        localExecTitle: 'Exécution Locale',
        desktopAppTitle: 'Application Sectile Desktop',
        desktopAppBadge: 'Compagnon GUI',
        desktopAppDesc: 'Application de bureau complète avec terminaux PTY natifs, streaming temps réel des journaux et gestion visuelle des espaces de travail.',
        desktopStep1: 'Lancez Sectile Desktop sur votre machine.',
        desktopStep2: 'Configurez l\'adresse du serveur sur :',
        desktopStep3: 'Saisissez le code d\'appairage temporaire généré ci-dessus.',
        headlessCliTitle: 'Agent CLI Headless',
        headlessCliBadge: 'Démon d\'arrière-plan',
        headlessCliDesc: 'Runner léger exécutant les tâches autonomes directement dans votre terminal ou votre infrastructure CI/CD.',
        serverUrlLabel: 'URL du serveur Sectile',
        urlInvalidAlert: 'Veuillez saisir une URL HTTP ou HTTPS valide sans identifiants ni paramètres.',
        copyCommandBtn: 'Copier la commande',
        copiedCommand: 'Commande copiée.',
        cliPrereqNotice: 'Nécessite le binaire sectile-agent dans votre variable PATH et une machine appairée une première fois.',
      },
      tracker: {
        title: 'Intégration Issue Tracker',
        selectTracker: 'Tracker Actif',
        githubRepo: 'Repository GitHub (owner/repo)',
        syncGithubBtn: 'Importer & Synchroniser GitHub',
      },
      save: 'Enregistrer la configuration',
      reseedBtn: 'Réinitialiser le jeu de test démo',
    },
    account: {
      signOut: 'Se déconnecter',
      displayName: 'Nom d\'affichage',
      displayNamePlaceholder: 'Votre nom sur ce tableau...',
      save: 'Enregistrer',
      notSignedIn: 'Vous n\'êtes pas connecté.',
      signIn: 'Se connecter',
      saved: 'Nom d\'affichage enregistré.',
      couldNotSignOut: 'Impossible de se déconnecter.',
    },
    trackerCredentials: {
      description: 'Vos accès personnels aux trackers. Les jetons saisis sont strictement individuels et protégés par votre compte.',
      masterPassphraseTitle: 'Phrase de scellement unique',
      statusLocked: 'Verrouillée',
      statusActive: 'Active',
      statusNotConfigured: 'Non configurée',
      passphraseDescription: "Une seule phrase protège l'ensemble de vos jetons de trackers. Elle est demandée pour déverrouiller vos accès à chaque session.",
      lock: 'Verrouiller',
      locking: 'Verrouillage...',
      unlockPrompt: 'Saisissez votre phrase de scellement unique pour déverrouiller tous vos jetons :',
      unlockPlaceholder: 'Phrase de scellement unique',
      unlockAll: 'Déverrouiller tous les jetons',
      unlocking: 'Déverrouillage...',
      operationalBanner: 'Vos accès aux trackers sont opérationnels pour cette session.',
      changePassphrase: 'Modifier la phrase',
      cancelChangePassphrase: 'Annuler',
      changePassphrasePrompt: 'Entrez une nouvelle phrase pour re-sceller tous vos jetons, ou laissez vide pour retirer le scellement :',
      newPassphrasePlaceholder: 'Nouvelle phrase de scellement (ou vide pour retirer)',
      apply: 'Appliquer',
      applying: 'Application...',
      definePassphrasePrompt: 'Définissez ici votre phrase de scellement unique. Tout jeton enregistré ci-dessous sera scellé avec celle-ci :',
      definePassphrasePlaceholder: 'Définir une phrase de scellement unique',
      sealMyTokens: 'Sceller mes jetons',
      toastUpdatedTitle: 'Phrase de scellement mise à jour',
      toastUpdatedDesc: 'Tous vos jetons sont désormais protégés par cette phrase unique.',
      toastRemovedTitle: 'Scellement retiré',
      toastRemovedDesc: 'Vos jetons sont désormais chiffrés par le serveur.',
      toastErrorTitle: 'Mise à jour impossible',
      toastErrorDefault: 'Erreur lors de la mise à jour des jetons',
      states: {
        none: 'Aucun jeton enregistré : vos actions sur les tâches ne partiront pas.',
        unsealed: 'Enregistré. Vos actions partent sous votre compte.',
        unlocked: 'Scellé, descellé pour cette session.',
        locked: 'Scellé et verrouillé : descellez-le pour agir sur les tâches.',
      },
      sealingInvitation: 'Par mesure de protection de votre identité, vous pouvez définir une phrase de scellement unique pour tous vos jetons. Vous devrez la saisir pour agir sur les tâches.',
      sealingConsequences: {
        sealed: 'Scellé : vous seul pouvez l’ouvrir avec votre phrase de scellement unique pour cette session.',
        unsealed: 'Non scellé : vos actions partent sans rien vous demander.',
      },
      trackers: {
        jira: {
          siteLabel: 'Site Jira',
          sitePlaceholder: 'mon-org.atlassian.net',
          projectLabel: 'Clé du projet par défaut',
          projectPlaceholder: 'PE',
          tokenHint: "À créer sur id.atlassian.com, section jetons d'API. Il s'utilise avec votre e-mail Atlassian, jamais seul.",
        },
        github: {
          siteLabel: "URL de l'API GitHub",
          sitePlaceholder: 'https://api.github.com',
          projectLabel: 'Dépôt par défaut',
          projectPlaceholder: 'organisation/depot',
          tokenHint: 'Personal Access Token avec la portée repo.',
        },
        gitlab: {
          siteLabel: "URL de l'API GitLab",
          sitePlaceholder: 'https://gitlab.com/api/v4',
          projectLabel: 'Projet par défaut',
          projectPlaceholder: 'groupe/projet',
          tokenHint: 'Personal Access Token avec la portée api.',
        },
      },
      saveBlockedReasons: {
        wantsEmail: "Renseignez votre site et l'e-mail de votre compte, puis vérifiez les accès.",
        default: 'Renseignez les accès, puis vérifiez-les.',
        needCheck: "Vérifiez les accès : l'enregistrement se débloque une fois que l'instance les a acceptés.",
      },
      form: {
        accountEmail: "E-mail du compte",
        accountEmailPlaceholder: "prenom.nom@exemple.com",
        personalAccessToken: "Personal Access Token",
        tokenPlaceholderSet: "Déjà configuré, laissez vide pour le garder",
        tokenPlaceholderEnv: "Fourni par l'environnement du serveur",
        tokenPlaceholderEmpty: "Collez le jeton",
        siteIsPersonalNotice: "Votre compte appartient à cette instance. Les projets que vous posez sur ce tracker la reprennent.",
        lockedNoticeSuffix: " — déverrouillez vos jetons dans la section ci-dessus.",
        sealedUnlockedNotice: "Jeton scellé avec votre phrase unique (déverrouillé).",
        willBeSealedNotice: "Ce jeton sera automatiquement scellé avec la phrase unique active définie plus haut.",
        connectedAs: "Connecté comme",
        forgetAccess: "Oublier mon accès",
        forgetConfirm: "Oublier votre accès {tracker} ? Vous devrez saisir votre jeton à nouveau.",
        verify: "Vérifier",
        save: "Enregistrer",
        saveTitleReady: "Enregistrer ces accès",
        saveTitleNotChecked: "Vérifiez d'abord les accès",
        configuredTitle: "{tracker} configuré",
        saveErrorTitle: "Enregistrement impossible",
        checkErrorDefault: "Vérification impossible",
      },
      setup: {
        title: "Connecter votre tracker",
        description: "Sans ces valeurs, la synchronisation ne ramène rien et aucune écriture ne part. Elles sont vérifiées auprès de l'instance avant d'être enregistrées.",
        closeTitle: "Configurer plus tard",
        trackerLabel: "Tracker",
        later: "Plus tard",
      },
    },
    activities: {
      title: 'Activités & File d\'attente',
      subtitle: 'Suivi en temps réel des exécutions de skills agentiques, logs CLI et artefacts générés.',
      stats: {
        total: 'Total exécutions',
        running: 'En cours',
        waiting: 'En attente de vous',
        queued: 'En attente',
        completed: 'Terminées',
        failed: 'Échouées',
        canceled: 'Annulées',
      },
      filters: {
        all: 'Toutes les activités',
        running: 'En cours',
        waiting: 'Attend une réponse',
        queued: 'En file d\'attente',
        completed: 'Terminées',
        failed: 'Échecs',
        allSkills: 'Toutes les skills',
        searchPlaceholder: 'Rechercher une activité, tâche, skill...',
      },
      table: {
        skill: 'Skill & Action',
        task: 'Tâche associée',
        status: 'Statut',
        duration: 'Durée',
        created: 'Date',
        actions: 'Actions',
      },
      detail: {
        title: 'Détails de l\'exécution',
        prompt: 'Prompt personnalisé',
        steps: 'Étapes d\'exécution',
        output: 'Rapport & Artefact généré',
        error: 'Message d\'erreur',
        rawLogs: 'Logs bruts (Console)',
        renderedMarkdown: 'Rendu Markdown',
        copyOutput: 'Copier les logs',
        copied: 'Copié !',
        openTask: 'Ouvrir la tâche',
        openPr: 'Voir la PR',
        retry: 'Relancer cette skill',
        cancel: 'Annuler l\'exécution',
        delete: 'Supprimer l\'activité',
      },
      empty: {
        title: 'Aucune activité pour le moment',
        desc: 'Les skills exécutées sur les tâches apparaîtront ici dans la file d\'attente avec leurs logs en direct.',
        noFilterMatch: 'Aucune activité ne correspond à vos critères de recherche.',
      },
      actions: {
        refresh: 'Rafraîchir',
        clearCompleted: 'Vider l\'historique terminé',
        clearConfirm: 'Êtes-vous sûr de vouloir vider toutes les activités terminées / échouées ?',
      },
    },
    compactCard: {
      pin: 'Épingler',
      unpin: 'Désépingler',
      advance: 'Avancer une étape',
      advanceAuto: 'Chaîne complète',
      advanceInteractive: 'Avancer en interactif',
      advanceWithModel: 'Modèle des lancements',
      currentModel: '(modèle configuré)',
      advanceAutonomous: 'Avancer en autonome',
      filterParent: 'Filtrer par parent',
      clearParent: 'Retirer le filtre parent',
      openPr: 'Ouvrir la PR / MR',
    },
    statusBar: {
      branch: 'Branche',
      clean: 'Arbre de travail propre',
      modified: 'modifié(s)',
      untracked: 'non suivi(s)',
      noRepo: 'Aucun dépôt Git',
      notGit: 'Pas un dépôt Git',
      refresh: 'Actualiser l\'état Git',
      aiProvider: 'Moteur IA',
      activeJobs: 'job(s) actif(s)',
      ready: 'Prêt',
      runningSkill: 'Skill en cours d\'exécution',
      cwd: 'CWD',
      remote: 'Dépôt distant',
      latestCommit: 'Dernier commit',
      copyBranch: 'Copier le nom de la branche',
      branchCopied: 'Nom de la branche copié !',
      viewDiff: 'Voir le Git Diff',
    },
    mcp: {
      panelTitle: 'Clients MCP connectés',
      client: 'client MCP',
      clients: 'clients MCP',
      noClient: 'Aucun client MCP',
      run: 'exécution',
      runs: 'exécutions',
      noRuns: 'aucune exécution',
      connected: 'connecté depuis',
      unavailable: 'Statut MCP indisponible',
      ownership: 'Les exécutions listées se ferment si leur client se déconnecte.',
    },
    toasts: {
      taskCreated: 'Tâche créée avec succès !',
      taskUpdated: 'Tâche mise à jour !',
      taskMoved: 'Statut de la tâche mis à jour',
      taskDeleted: 'Tâche supprimée',
      settingsSaved: 'Paramètres enregistrés avec succès',
      demoReseeded: 'Base de données réinitialisée avec succès !',
      skillStarted: 'Exécution du process IA via shell en cours...',
      skillCompleted: 'Skill exécutée avec succès !',
      skillQueued: 'Skill ajoutée à la file d\'exécution !',
      activityRetried: 'Exécution de la skill relancée !',
      activityCanceled: 'Exécution annulée avec succès',
      activityDeleted: 'Activité supprimée de l\'historique',
      activitiesCleared: 'Historique des activités nettoyé avec succès',
      syncSuccess: 'Synchronisation des tickets réussie !',
      error: 'Une erreur est survenue',
    },
  },
  en: {
    app: {
      title: 'Sectile',
      tagline: 'Agentic Task Workflow Manager',
    },
    nav: {
      allTasks: 'All Tasks',
      backlog: 'Backlog',
      toClarify: 'To Clarify',
      specified: 'Specified',
      inProgress: 'In Progress',
      toValidate: 'To Validate',
      done: 'Done',
      views: 'Views',
      board: 'Board',
      list: 'Backlog',
      roadmap: 'Roadmap',
      timeline: 'Timeline',
      roadmapTooltip: 'Roadmap: NOW / NEXT / FUTURE',
      timelineTooltip: 'Sprint Timeline',
      activities: 'Activities',
      sync: 'Sync',
      filters: 'Quick Filters',
      myTasks: 'My Tasks',
      urgentHigh: 'High Priority',
      labels: 'Labels',
      sources: 'Sources',
      allSources: 'All sources',
      localSource: 'Local',
      parents: 'Macros / Parents',
      clearParentFilter: 'Clear parent filter',
      dailyBrief: 'Daily brief',
      dailyBriefRunning: 'Brief running…',
      settings: 'Profile & Preferences',
      reseedDemo: 'Reset Demo Data',
      toggleSidebar: 'Toggle Sidebar',
      syncGithub: 'Sync GitHub',
    },
    syncView: {
      title: 'Synchronization Hub',
      subtitle: 'Manage bidirectional integration with GitHub and Jira. Sync executions run in the background as tasks in your Activities queue.',
      githubCard: {
        title: 'GitHub Synchronization',
        desc: 'Import remote issues from your configured GitHub repository using GitHub CLI (gh).',
        repoLabel: 'GitHub Repository',
        pathLabel: 'Local Directory',
        btnSync: 'Run GitHub Sync',
        statusConnected: 'GitHub CLI Connected',
      },
      globalCard: {
        title: 'Global Synchronization',
        desc: 'Run a full synchronization queue job across all configured remote trackers (GitHub + Jira).',
        btnSync: 'Sync All Remote',
      },
      options: {
        title: 'Sync Settings & Preferences',
        desc: 'Configure team keys, default remote repositories, and workspace paths used by background sync workers.',
        defaultTracker: 'Primary Remote Tracker',
        githubRepo: 'GitHub Repository (owner/repo)',
        repoPath: 'Local Project Path (CWD)',
        save: 'Save Sync Settings',
        saved: 'Settings saved!',
      },
      history: {
        title: 'Recent Synchronizations',
        desc: 'History of sync jobs dispatched and completed in the Activities queue.',
        viewInActivities: 'Inspect Logs in Activities',
        noHistory: 'No recent sync executions.',
      },
    },
    header: {
      searchPlaceholder: 'Search tasks... (Press "/" to focus)',
      quickAdd: 'Add Task',
      commandPalette: 'Command Palette',
      boardView: 'Board',
      listView: 'List',
      activitiesView: 'Activities',
      filterStatus: 'Status',
      filterPriority: 'Priority',
      activeFilter: 'active filter',
      activeFilters: 'active filters',
      syncing: 'Syncing...',
      syncNow: 'Sync Issues',
      hideDone: 'Hide completed tasks',
      showDone: 'Show completed tasks',
    },
    board: {
      emptyColumn: 'No tasks in this column',
      addTask: 'Add task',
      dragHint: 'Drag and drop to update status or reorder',
    },
    list: {
      columns: {
        key: 'Key',
        title: 'Title',
        status: 'Status',
        priority: 'Priority',
        labels: 'Labels',
        assignee: 'Assignee',
        dueDate: 'Due Date',
        source: 'Source',
        actions: 'Actions',
      },
      empty: 'No tasks match your search or filters.',
    },
    status: {
      to_clarify: 'To Clarify',
      clarified: 'Clarified',
      to_implement: 'To Implement',
      to_test: 'To Test',
      to_close: 'In Review / PR',
      finished: 'Finished',
      backlog: 'To Clarify',
      specified: 'To Specify',
      in_progress: 'To Implement',
      to_validate: 'To Test',
      done: 'Finished',
    },
    priority: {
      urgent: 'Urgent',
      high: 'High',
      medium: 'Medium',
      low: 'Low',
    },
    skills: {
      title: 'Agent Copilot & Skills',
      subtitle: 'Run workflow skills to advance this task automatically',
      pipeline: 'Development Pipeline',
      runSkill: 'Run Skill',
      running: 'Executing via CLI shell...',
      nextStep: 'Next Step',
      autoPilot: 'Auto-Pilot (/pick-issue)',
      clarify: 'Clarify (/clarify-issue)',
      specify: 'Specify (/specify-issue)',
      implement: 'Implement (/code-issue)',
      createPr: 'Adjust (/adjust-issue)',
      history: 'Execution History & Artifacts',
      noHistory: 'No skill runs recorded yet on this task.',
      branch: 'Git Branch',
      pullRequest: 'Pull Request',
      viewPr: 'View PR on GitHub',
      output: 'Report & Generated Artifact',
      stepsTitle: 'Executed Steps',
      skillSuccess: 'Skill executed successfully!',
    },
    taskModal: {
      createTitle: 'New Task',
      editTitle: 'Details & Agent Copilot',
      titlePlaceholder: 'Task title...',
      descPlaceholder: 'Description, technical context, acceptance criteria...',
      status: 'Status',
      priority: 'Priority',
      labels: 'Labels (separated by commas or Enter)',
      addLabel: 'Add label...',
      assignee: 'Assignee',
      dueDate: 'Due date',
      cancel: 'Cancel',
      save: 'Save Changes',
      create: 'Create Task',
      delete: 'Delete Task',
      deleteConfirm: 'Are you sure you want to delete this task? This action cannot be undone.',
      created: 'Created at',
      updated: 'Updated at',
      switchToModal: 'Switch to centered modal',
      switchToPanel: 'Switch to right sliding panel',
    },
    convert: {
      bannerTitle: 'Local task (unsynced)',
      bannerDesc: 'This task is stored only in local SQLite.',
      btnGithub: 'Export to GitHub',
      converting: 'Exporting to remote tracker...',
      success: 'Issue created on remote tracker!',
    },
    quickAdd: {
      title: 'Quick Add Task',
      placeholder: 'Task title (e.g. Create GraphQL endpoint, Fix auth...)',
      hint: 'Press Enter to create immediately',
      status: 'Initial status',
      priority: 'Priority',
      tracker: 'Destination / Tracker',
    },
    commandPalette: {
      searchPlaceholder: 'Type a command, skill or task...',
      general: 'General Actions',
      createTask: 'Create new task',
      switchBoard: 'Switch to Kanban Board',
      switchList: 'Switch to List View',
      toggleTheme: 'Toggle Theme (Dark / Light)',
      changeLanguage: 'Switch Language (FR / EN)',
      openProfile: 'Open Profile & Preferences',
      reseed: 'Reset Demo Database',
      syncGithub: 'Sync with GitHub CLI',
      tasksSection: 'Jump to Task',
      skillsSection: 'Agentic Skills (/skills)',
      noResults: 'No results found for',
      hintNavigate: 'to navigate',
      hintSelect: 'to select',
      hintClose: 'to close',
    },
    profileModal: {
      title: 'Configuration & Settings',
      subtitle: 'Account, appearance, and tool configuration',
      tabs: {
        account: 'Account',
        appearance: 'Appearance',
        trackers: 'Tracker Credentials',
        aiEngine: 'Agent settings',
        sdd: 'Skills & SDD',
        workstations: 'Workstations & Agent',
        aiConfig: 'AI Engine & Prompts',
        tracker: 'GitHub & Jira',
      },
      userSection: 'User Information',
      name: 'User Name',
      email: 'Email Address',
      appearanceSection: 'Appearance & Ergonomics',
      accentColor: 'Accent Color',
      theme: 'Theme Mode',
      themes: {
        dark: 'Dark',
        light: 'Light',
        system: 'System',
      },
      language: 'Interface Language',
      languages: {
        fr: 'Français (FR)',
        en: 'English (EN)',
      },
      density: 'Display Density / Scaling',
      densities: {
        compact: 'Compact (More content on screen)',
        standard: 'Standard (Balanced)',
        comfortable: 'Comfortable (Spacious)',
      },
      densityDesc: {
        compact: '13px font, reduced padding',
        standard: '14px font, optimal balance',
        comfortable: '15px font, spacious padding',
      },
      defaultView: 'Default View',
      defaultViews: {
        board: 'Board (Kanban)',
        list: 'Detailed List',
      },
      uiScale: 'Interface Scale',
      uiScales: {
        s90: '90% (Compact)',
        s100: '100% (Default)',
        s112: '112% (Enlarged)',
        s125: '125% (Large)',
      },
      detailMode: 'Story Detail View Style',
      detailModes: {
        modal: 'Centered Modal',
        panel: 'Right Sliding Panel (Drawer)',
      },
      detailModeDesc: {
        panel: 'Right-side sliding panel',
        modal: 'Centered modal dialog',
      },
      accents: {
        indigo: 'Royal Indigo',
        violet: 'Electric Violet',
        emerald: 'Vivid Emerald',
        amber: 'Warm Amber',
        rose: 'Candy Rose',
        cyan: 'Tech Cyan',
        blue: 'Ocean Blue',
        orange: 'Sunset Orange',
        'neon-cyan': '⚡ Cyber Neon Cyan',
        'neon-purple': '🔮 Synthwave Magenta',
        'neon-green': '🟢 Matrix Neon Green',
        'neon-amber': '✨ Laser Gold Neon',
      },
      sectilePreferences: 'Sectile Preferences',
      ai: {
        title: 'AI Engine & Shell Execution',
        engine: 'Default Agentic AI Engine',
        defaultEngine: 'Default Agentic AI Engine',
        proposedModelsFor: 'Proposed models for',
        proposedModelsGroup: 'Proposed models',
        noModelsConfigured: 'No models: nothing will be proposed at launch.',
        addModelPlaceholder: 'Add a model...',
        addModelButton: 'Add this model',
        addModelAria: 'Add a model for {target}',
        removeModelAria: 'Remove {model} from {target}',
        engineIgnoresModelNotice: 'This engine ignores the model unless its command contains the {model} marker.',
        modelTemplatePlaceholderNotice: 'The model is applied via the {model} placeholder in the command.',
        providerIgnoresModel: '{provider} does not accept a model selection: the value is ignored.',
        invalidModelIdentifier: 'Invalid identifier: letters, numbers and . _ - : @ / only, no spaces.',
        defaultModel: 'Default Model',
        defaultModelPlaceholder: 'CLI default',
        customModelOption: 'Other model (free text)...',
        quickSelect: 'Quick select:',
        cliParametersTitle: 'CLI & Commands: {provider}',
        cmdInteractive: 'Interactive command',
        cmdAutonomous: 'Autonomous command (headless)',
        fastPresets: 'Quick command presets:',
        engineDesc: 'Configure the default artificial intelligence engine, models, and CLI command execution for skills.',
        cmdTemplate: 'Shell CLI Command Template',
        repoPath: 'Target Project Directory (CWD)',
        repoPathDesc: 'Workspace directory where the agent will run commands',
        promptsTitle: 'Custom Skill Prompts',
        promptClarify: 'Prompt /clarify-issue',
        promptSpecify: 'Prompt /specify-issue',
        promptImplement: 'Prompt /code-issue',
        promptAdjust: 'Prompt /adjust-issue',
        promptHandoff: 'Prompt /handoff-issue',
        promptCreatePr: 'Legacy adjustment prompt',
        cliStatusTitle: 'Local CLI Tools Status',
        availableVariables: 'Available variables:',
        executedCommand: 'Executed command',
        modeInteractive: 'Interactive',
        modeAutonomous: 'Autonomous',
        mcpConfigWithoutAgent: 'MCP configuration without local agent',
        mcpConnectDirectlyDesc: 'To connect your CLI or IDE directly to the Sectile MCP server without going through the local agent (',
        mcpConnectDirectlyWorkflow: '). The engine accesses task management and workflow tracking tools directly.',
        mcpKeyRequired: 'Key required:',
        mcpKeyRequiredDesc: 'Replace {placeholder} with an API key.',
        mcpKeyPlaceholder: '<YOUR_WORKSTATION_KEY>',
        mcpGenerateKey: 'Generate a key',
        mcpCopy: 'Copy',
        mcpCopied: 'Copied!',
        mcpQuickCliCommand: 'Quick command ({engine}):',
        mcpOrInConfig: 'Or in',
        customProviderLabel: 'Custom CLI',
        customProviderSub: 'Custom binary or script',
      },
      sdd: {
        title: 'Spec-Driven Design (SDD) Framework',
        subtitle: 'Spec-Driven Design ensures a clear, structured, and verifiable specification is drafted and reviewed before any code generation by AI agents.',
        contractBadge: 'Contract-First',
        defaultFramework: 'Default project framework',
        speckitTitle: 'GitHub Spec Kit',
        speckitSubtitle: 'CLI specify',
        speckitDesc: 'Standard GitHub convention: modular structure in .specify/ and specs/ (spec.md, plan.md, tasks.md).',
        speckitCommands: 'Commands: /specify-issue, /code-issue',
        openspecTitle: 'OpenSpec',
        openspecSubtitle: 'CLI openspec',
        openspecDesc: 'Formal specification with deltas and verifiable requirements. Change proposals are reviewed and verified before writing code.',
        openspecCommands: 'Commands: openspec propose, validate',
        lifecycleTitle: 'SDD Lifecycle in Sectile',
        customizePrompts: 'Customize prompts',
        steps: {
          clarify: '1. Clarify',
          specify: '2. Specify',
          code: '3. Code',
          adjust: '4. Adjust',
          handoff: '5. Handoff',
        },
        promptsSectionTitle: 'Custom Prompts per Skill',
        promptsSectionDesc: 'Customize instructions sent to the Agentic CLI for each step of the SDD workflow. When left blank, default prompts are used.',
        prompts: {
          clarify: {
            label: 'Clarify Prompt (/clarify-issue)',
            hint: 'Default: /clarify-issue {issueKey} tracked on {tracker} in {repo}',
            placeholder: '/clarify-issue {issueKey} tracked on {tracker} in {repo}',
          },
          specify: {
            label: 'Specification Prompt (/specify-issue)',
            hint: 'Spec Kit / OpenSpec specification',
            placeholder: 'You are the Product Owner for {issueKey}. Write the specification following the configured SDD framework...',
          },
          implement: {
            label: 'Implementation Prompt (/code-issue)',
            hint: 'Development & tests',
            placeholder: 'You are the senior developer for {issueKey}. Implement the code in {repoPath}...',
          },
          adjust: {
            label: 'Adjustment & Review Prompt (/adjust-issue)',
            hint: 'Code review & PR update',
            placeholder: 'You are the senior reviewer for {issueKey}. Review the changes, address feedback and update the PR...',
          },
          handoff: {
            label: 'Closing & Handoff Prompt (/handoff-issue)',
            hint: 'Documentation & local cleanup',
            placeholder: 'You are responsible for closing {issueKey}. Verify the branch merge, write the handoff report and clean the worktree...',
          },
        },
      },
      workstations: {
        desc: 'Manage your paired development workstations, agent signing keys, and local agent configuration.',
        title: 'Workstations',
        badge: 'Pairing & Devices',
        pairBtn: 'Pair a workstation',
        workingBtn: 'Working…',
        singleUseNotice: 'Single use. Non-reusable once expired.',
        onWorkstationTitle: 'On the workstation',
        terminalStep: 'In a terminal:',
        desktopStep: 'Or in Sectile Desktop:',
        pairedListTitle: 'Connected Machines',
        noWorkstations: 'No workstation is paired yet.',
        lastSeen: 'last seen',
        renewBtn: 'Renew',
        renewNever: 'Never expires',
        revokeBtn: 'Revoke',
        sharedTokenWarning: 'This server still runs with SECTILE_SERVER_TOKEN, which is deprecated and will be removed in the next release. Pair your workstations and drop the variable.',
        // Direct MCP
        directMcpTitle: 'Direct MCP Integration',
        directMcpBadge: 'AI Desktop Apps',
        directMcpDesc: 'Connect AI desktop applications (Cursor, Claude Desktop, Antigravity, etc.) directly to Sectile\'s MCP endpoint (/mcp) via an API key without running a local agent daemon.',
        labelInput: 'Client Label',
        labelPlaceholder: 'e.g. Claude Desktop on MacBook Pro…',
        expiresInput: 'Expiration',
        ttl90: 'in 90 days',
        ttl30: 'in 30 days',
        ttl365: 'in 1 year',
        ttl0: 'never',
        createKeyBtn: 'Create an API Key',
        keyCreatedSuccess: 'API key generated for {label}. Copy it now: it will never be displayed again.',
        copyKeyBtn: 'Copy Key',
        copiedKey: 'API key copied to clipboard.',
        desktopConfigsTitle: 'Desktop App Configurations',
        tabClaude: 'Claude Desktop',
        tabAgy: 'Antigravity',
        tabCodex: 'Codex / ChatGPT',
        tabCursor: 'Cursor',
        copyConfigBtn: 'Copy JSON Config',
        copiedConfig: 'Configuration copied to clipboard.',
        // Local Execution
        localExecTitle: 'Local Execution',
        desktopAppTitle: 'Sectile Desktop App',
        desktopAppBadge: 'GUI Companion',
        desktopAppDesc: 'Full desktop application with native PTY terminals, live log streaming, and visual workspace management.',
        desktopStep1: 'Launch Sectile Desktop on your workstation.',
        desktopStep2: 'Set server address to:',
        desktopStep3: 'Enter the temporary pairing code generated above.',
        headlessCliTitle: 'Headless CLI Agent',
        headlessCliBadge: 'Background Daemon',
        headlessCliDesc: 'Lightweight background runner executing autonomous tasks directly in your terminal or CI/CD pipeline.',
        serverUrlLabel: 'Sectile Server URL',
        urlInvalidAlert: 'Enter a valid HTTP or HTTPS server URL without credentials or query parameters.',
        copyCommandBtn: 'Copy Command',
        copiedCommand: 'Command copied.',
        cliPrereqNotice: 'Requires the sectile-agent binary in your PATH and a workstation paired once.',
      },
      tracker: {
        title: 'Issue Tracker Integration',
        selectTracker: 'Active Tracker',
        githubRepo: 'GitHub Repository (owner/repo)',
        syncGithubBtn: 'Import & Sync GitHub',
      },
      save: 'Save Configuration',
      reseedBtn: 'Reset Demo Dataset',
    },
    account: {
      signOut: 'Sign out',
      displayName: 'Display name',
      displayNamePlaceholder: 'Your name on this board...',
      save: 'Save',
      notSignedIn: 'You are not signed in.',
      signIn: 'Sign in',
      saved: 'Display name saved.',
      couldNotSignOut: 'Could not sign out.',
    },
    trackerCredentials: {
      description: 'Your personal tracker credentials. Entered tokens are strictly individual and protected by your account.',
      masterPassphraseTitle: 'Master sealing passphrase',
      statusLocked: 'Locked',
      statusActive: 'Active',
      statusNotConfigured: 'Not configured',
      passphraseDescription: 'A single passphrase protects all your tracker tokens. It is requested to unlock your access for each session.',
      lock: 'Lock',
      locking: 'Locking...',
      unlockPrompt: 'Enter your master sealing passphrase to unlock all your tokens:',
      unlockPlaceholder: 'Master sealing passphrase',
      unlockAll: 'Unlock all tokens',
      unlocking: 'Unlocking...',
      operationalBanner: 'Your tracker credentials are operational for this session.',
      changePassphrase: 'Change passphrase',
      cancelChangePassphrase: 'Cancel',
      changePassphrasePrompt: 'Enter a new passphrase to re-seal all your tokens, or leave blank to remove sealing:',
      newPassphrasePlaceholder: 'New sealing passphrase (or leave blank to remove)',
      apply: 'Apply',
      applying: 'Applying...',
      definePassphrasePrompt: 'Set your master sealing passphrase here. Any token saved below will be sealed with it:',
      definePassphrasePlaceholder: 'Set a master sealing passphrase',
      sealMyTokens: 'Seal my tokens',
      toastUpdatedTitle: 'Sealing passphrase updated',
      toastUpdatedDesc: 'All your tokens are now protected by this unique passphrase.',
      toastRemovedTitle: 'Sealing removed',
      toastRemovedDesc: 'Your tokens are now encrypted by the server.',
      toastErrorTitle: 'Update failed',
      toastErrorDefault: 'Error updating tokens',
      states: {
        none: 'No token saved: your task actions will not be dispatched.',
        unsealed: 'Saved. Your actions are dispatched under your account.',
        unlocked: 'Sealed, unlocked for this session.',
        locked: 'Sealed and locked: unlock it to act on tasks.',
      },
      sealingInvitation: 'To protect your identity, you can define a master sealing passphrase for all your tokens. You will need to enter it to act on tasks.',
      sealingConsequences: {
        sealed: 'Sealed: only you can open it with your master sealing passphrase for this session.',
        unsealed: 'Unsealed: your actions will be performed without prompting.',
      },
      trackers: {
        jira: {
          siteLabel: 'Jira site',
          sitePlaceholder: 'my-org.atlassian.net',
          projectLabel: 'Default project key',
          projectPlaceholder: 'e.g. MKTG',
          tokenHint: 'Create at id.atlassian.com, API tokens section. Used with your Atlassian email, never alone.',
        },
        github: {
          siteLabel: 'GitHub API URL',
          sitePlaceholder: 'https://api.github.com',
          projectLabel: 'Default repository',
          projectPlaceholder: 'organization/repo',
          tokenHint: 'Personal Access Token with repo scope.',
        },
        gitlab: {
          siteLabel: 'GitLab API URL',
          sitePlaceholder: 'https://gitlab.com/api/v4',
          projectLabel: 'Default project',
          projectPlaceholder: 'group/project',
          tokenHint: 'Personal Access Token with api scope.',
        },
      },
      saveBlockedReasons: {
        wantsEmail: 'Enter your site and account email, then verify credentials.',
        default: 'Enter credentials, then verify them.',
        needCheck: 'Verify credentials: saving unlocks once accepted by the instance.',
      },
      form: {
        accountEmail: 'Account email',
        accountEmailPlaceholder: 'firstname.lastname@example.com',
        personalAccessToken: 'Personal Access Token',
        tokenPlaceholderSet: 'Already configured, leave blank to keep',
        tokenPlaceholderEnv: 'Provided by server environment',
        tokenPlaceholderEmpty: 'Paste token',
        siteIsPersonalNotice: 'Your account belongs to this instance. Projects configured with this tracker will inherit it.',
        lockedNoticeSuffix: ' — unlock your tokens in the section above.',
        sealedUnlockedNotice: 'Token sealed with your master passphrase (unlocked).',
        willBeSealedNotice: 'This token will automatically be sealed with the active master passphrase defined above.',
        connectedAs: 'Connected as',
        forgetAccess: 'Forget my access',
        forgetConfirm: 'Forget your {tracker} access? You will need to enter your token again.',
        verify: 'Verify',
        save: 'Save',
        saveTitleReady: 'Save these credentials',
        saveTitleNotChecked: 'Verify credentials first',
        configuredTitle: '{tracker} configured',
        saveErrorTitle: 'Failed to save',
        checkErrorDefault: 'Verification failed',
      },
      setup: {
        title: 'Connect your tracker',
        description: 'Without these values, synchronization fetches nothing and no writes are dispatched. They are verified with the instance before being saved.',
        closeTitle: 'Configure later',
        trackerLabel: 'Tracker',
        later: 'Later',
      },
    },
    activities: {
      title: 'Activities & Execution Queue',
      subtitle: 'Real-time tracking of agentic skill executions, CLI logs, and generated artifacts.',
      stats: {
        total: 'Total Runs',
        running: 'Running',
        waiting: 'Waiting for you',
        queued: 'Queued',
        completed: 'Completed',
        failed: 'Failed',
        canceled: 'Canceled',
      },
      filters: {
        all: 'All Activities',
        running: 'Running',
        waiting: 'Waiting for input',
        queued: 'In Queue',
        completed: 'Completed',
        failed: 'Failed',
        allSkills: 'All Skills',
        searchPlaceholder: 'Search activity, task, skill...',
      },
      table: {
        skill: 'Skill & Action',
        task: 'Associated Task',
        status: 'Status',
        duration: 'Duration',
        created: 'Date',
        actions: 'Actions',
      },
      detail: {
        title: 'Execution Details',
        prompt: 'Custom Prompt',
        steps: 'Execution Steps',
        output: 'Report & Generated Artifact',
        error: 'Error Message',
        rawLogs: 'Raw Console Logs',
        renderedMarkdown: 'Markdown View',
        copyOutput: 'Copy Logs',
        copied: 'Copied!',
        openTask: 'Open Task',
        openPr: 'View PR',
        retry: 'Re-run this skill',
        cancel: 'Cancel Execution',
        delete: 'Delete Activity',
      },
      empty: {
        title: 'No activity yet',
        desc: 'Executed skills on tasks will appear here in the queue with real-time logs and status.',
        noFilterMatch: 'No activities match your filter criteria.',
      },
      actions: {
        refresh: 'Refresh',
        clearCompleted: 'Clear Completed History',
        clearConfirm: 'Are you sure you want to delete all completed / failed activities?',
      },
    },
    compactCard: {
      pin: 'Pin',
      unpin: 'Unpin',
      advance: 'Advance one step',
      advanceAuto: 'Full chain',
      advanceInteractive: 'Advance interactively',
      advanceWithModel: 'Model used by launches',
      currentModel: '(configured model)',
      advanceAutonomous: 'Advance autonomously',
      filterParent: 'Filter by parent',
      clearParent: 'Clear parent filter',
      openPr: 'Open PR / MR',
    },
    statusBar: {
      branch: 'Branch',
      clean: 'Clean working tree',
      modified: 'modified',
      untracked: 'untracked',
      noRepo: 'No Git repository',
      notGit: 'Not a Git repository',
      refresh: 'Refresh Git Status',
      aiProvider: 'AI Engine',
      activeJobs: 'active job(s)',
      ready: 'Ready',
      runningSkill: 'Skill running',
      cwd: 'CWD',
      remote: 'Remote repository',
      latestCommit: 'Latest commit',
      copyBranch: 'Copy branch name',
      branchCopied: 'Branch name copied!',
      viewDiff: 'View Git Diff',
    },
    mcp: {
      panelTitle: 'Connected MCP clients',
      client: 'MCP client',
      clients: 'MCP clients',
      noClient: 'No MCP client',
      run: 'run',
      runs: 'runs',
      noRuns: 'no run',
      connected: 'connected for',
      unavailable: 'MCP status unavailable',
      ownership: 'The runs listed here close if their client disconnects.',
    },
    toasts: {
      taskCreated: 'Task created successfully!',
      taskUpdated: 'Task updated successfully!',
      taskMoved: 'Task status updated',
      taskDeleted: 'Task deleted',
      settingsSaved: 'Settings saved successfully',
      demoReseeded: 'Demo database reset successfully!',
      skillStarted: 'Executing AI process via shell...',
      skillCompleted: 'Skill executed successfully!',
      skillQueued: 'Skill added to execution queue!',
      activityRetried: 'Skill execution re-queued!',
      activityCanceled: 'Execution canceled successfully',
      activityDeleted: 'Activity removed from history',
      activitiesCleared: 'Activity history cleared successfully',
      syncSuccess: 'Issues synchronized successfully!',
      error: 'An error occurred',
    },
  },
}
