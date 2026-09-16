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
    digest: string
    /** Bouton qui demande le brief du jour à l'agent du projet. */
    dailyBrief: string
    dailyBriefRunning: string
    settings: string
    reseedDemo: string
    toggleSidebar: string
    syncLinear: string
    syncGithub: string
  }
  syncView: {
    title: string
    subtitle: string
    linearCard: {
      title: string
      desc: string
      teamLabel: string
      btnSync: string
      statusConnected: string
    }
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
      linearTeam: string
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
    btnLinear: string
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
    syncLinear: string
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
      appearance: string
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
    defaultView: string
    detailMode: string
    detailModes: {
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
    ai: {
      title: string
      engine: string
      engineDesc: string
      cmdTemplate: string
      repoPath: string
      repoPathDesc: string
      promptsTitle: string
      promptClarify: string
      promptSpecify: string
      promptImplement: string
      promptCreatePr: string
      cliStatusTitle: string
    }
    tracker: {
      title: string
      selectTracker: string
      linearTeam: string
      githubRepo: string
      syncLinearBtn: string
      syncGithubBtn: string
    }
    save: string
    reseedBtn: string
  }
  activities: {
    title: string
    subtitle: string
    stats: {
      total: string
      running: string
      queued: string
      completed: string
      failed: string
      canceled: string
    }
    filters: {
      all: string
      running: string
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
      digest: 'Digest quotidien',
      dailyBrief: 'Brief du jour',
      dailyBriefRunning: 'Brief en cours…',
      settings: 'Profil & Préférences',
      reseedDemo: 'Réinitialiser démo',
      toggleSidebar: 'Replier / Déplier le menu',
      syncLinear: 'Synchroniser Linear',
      syncGithub: 'Synchroniser GitHub',
    },
    syncView: {
      title: 'Centre de Synchronisation',
      subtitle: 'Gérez l\'intégration bidirectionnelle avec Linear et GitHub. Les synchronisations s\'exécutent en arrière-plan sous forme de tâches dans vos Activités.',
      linearCard: {
        title: 'Synchronisation Linear',
        desc: 'Importe et met à jour les tickets de votre équipe Linear avec leurs statuts, priorités et labels.',
        teamLabel: 'Équipe Linear',
        btnSync: 'Lancer la synchro Linear',
        statusConnected: 'Linear CLI Connecté',
      },
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
        desc: 'Exécute une synchronisation complète en file d\'attente sur tous vos trackers configurés (Linear + GitHub).',
        btnSync: 'Tout synchroniser',
      },
      options: {
        title: 'Options & Préférences de synchronisation',
        desc: 'Personnalisez les identifiants d\'équipe, dépôts par défaut et chemins locaux utilisés par les workers d\'arrière-plan.',
        defaultTracker: 'Tracker distant principal',
        linearTeam: 'Clé d\'équipe Linear (Team Key)',
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
      btnLinear: 'Exporter vers Linear',
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
      syncLinear: 'Synchroniser avec Linear CLI',
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
      subtitle: 'Personnalisez le moteur IA (agy, vibe, claude), Linear/GitHub CLI et l\'apparence',
      tabs: {
        appearance: 'Apparence & Profil',
        aiConfig: 'Moteur IA & Prompts',
        tracker: 'Linear & GitHub CLI',
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
      defaultView: 'Vue par défaut',
      detailMode: 'Affichage des détails de tâche',
      detailModes: {
        modal: 'Modale centrée',
        panel: 'Panneau latéral droit (Right Panel)',
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
      ai: {
        title: 'Moteur IA & Exécution Shell',
        engine: 'Agent IA sélectionné',
        engineDesc: 'Choisissez l\'outil CLI qui exécutera les skills en arrière-plan',
        cmdTemplate: 'Template de commande Shell CLI',
        repoPath: 'Répertoire du projet cible (CWD)',
        repoPathDesc: 'Emplacement du repo dans lequel l\'agent exécutera les commandes',
        promptsTitle: 'Personnalisation des Prompts par Skill',
        promptClarify: 'Prompt /clarify-issue (Cadrage & questions)',
        promptSpecify: 'Prompt /specify-issue (Spécification Spec Kit / OpenSpec)',
        promptImplement: 'Prompt /code-issue (Implémentation & tests)',
        promptCreatePr: 'Legacy adjustment prompt (reconciliation required)',
        cliStatusTitle: 'Statut des CLI Locales',
      },
      tracker: {
        title: 'Intégration Issue Tracker',
        selectTracker: 'Tracker Actif',
        linearTeam: 'Clé d\'équipe Linear (Team Key)',
        githubRepo: 'Repository GitHub (owner/repo)',
        syncLinearBtn: 'Importer & Synchroniser Linear',
        syncGithubBtn: 'Importer & Synchroniser GitHub',
      },
      save: 'Enregistrer la configuration',
      reseedBtn: 'Réinitialiser le jeu de test démo',
    },
    activities: {
      title: 'Activités & File d\'attente',
      subtitle: 'Suivi en temps réel des exécutions de skills agentiques, logs CLI et artefacts générés.',
      stats: {
        total: 'Total exécutions',
        running: 'En cours',
        queued: 'En attente',
        completed: 'Terminées',
        failed: 'Échouées',
        canceled: 'Annulées',
      },
      filters: {
        all: 'Toutes les activités',
        running: 'En cours',
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
      advanceAuto: 'Avancer automatiquement',
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
      digest: 'Daily digest',
      dailyBrief: 'Daily brief',
      dailyBriefRunning: 'Brief running…',
      settings: 'Profile & Preferences',
      reseedDemo: 'Reset Demo Data',
      toggleSidebar: 'Toggle Sidebar',
      syncLinear: 'Sync Linear',
      syncGithub: 'Sync GitHub',
    },
    syncView: {
      title: 'Synchronization Hub',
      subtitle: 'Manage bidirectional integration with Linear and GitHub. Sync executions run in the background as tasks in your Activities queue.',
      linearCard: {
        title: 'Linear Synchronization',
        desc: 'Import and update issues from your Linear team with their workflow states, priorities, and labels.',
        teamLabel: 'Linear Team',
        btnSync: 'Run Linear Sync',
        statusConnected: 'Linear CLI Connected',
      },
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
        desc: 'Run a full synchronization queue job across all configured remote trackers (Linear + GitHub).',
        btnSync: 'Sync All Remote',
      },
      options: {
        title: 'Sync Settings & Preferences',
        desc: 'Configure team keys, default remote repositories, and workspace paths used by background sync workers.',
        defaultTracker: 'Primary Remote Tracker',
        linearTeam: 'Linear Team Key',
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
      btnLinear: 'Export to Linear',
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
      syncLinear: 'Sync with Linear CLI',
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
      subtitle: 'Configure AI engine (agy, vibe, claude), Linear/GitHub CLI and appearance',
      tabs: {
        appearance: 'Appearance & Profile',
        aiConfig: 'AI Engine & Prompts',
        tracker: 'Linear & GitHub CLI',
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
      defaultView: 'Default View',
      detailMode: 'Story Detail View Style',
      detailModes: {
        modal: 'Centered Modal',
        panel: 'Right Sliding Panel (Drawer)',
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
      ai: {
        title: 'AI Engine & Shell Execution',
        engine: 'Selected AI Agent',
        engineDesc: 'Choose the CLI tool that will execute skills in the background',
        cmdTemplate: 'Shell CLI Command Template',
        repoPath: 'Target Project Directory (CWD)',
        repoPathDesc: 'Workspace directory where the agent will run commands',
        promptsTitle: 'Custom Skill Prompts',
        promptClarify: 'Prompt /clarify-issue',
        promptSpecify: 'Prompt /specify-issue',
        promptImplement: 'Prompt /code-issue',
        promptCreatePr: 'Legacy adjustment prompt',
        cliStatusTitle: 'Local CLI Tools Status',
      },
      tracker: {
        title: 'Issue Tracker Integration',
        selectTracker: 'Active Tracker',
        linearTeam: 'Linear Team Key',
        githubRepo: 'GitHub Repository (owner/repo)',
        syncLinearBtn: 'Import & Sync Linear',
        syncGithubBtn: 'Import & Sync GitHub',
      },
      save: 'Save Configuration',
      reseedBtn: 'Reset Demo Dataset',
    },
    activities: {
      title: 'Activities & Execution Queue',
      subtitle: 'Real-time tracking of agentic skill executions, CLI logs, and generated artifacts.',
      stats: {
        total: 'Total Runs',
        running: 'Running',
        queued: 'Queued',
        completed: 'Completed',
        failed: 'Failed',
        canceled: 'Canceled',
      },
      filters: {
        all: 'All Activities',
        running: 'Running',
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
      advanceAuto: 'Advance automatically',
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
