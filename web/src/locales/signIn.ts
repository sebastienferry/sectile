/**
 * Strings of the sign-in screen and the account flows (#533): sign-in and its
 * notices, API keys and workstations, local agent and MCP setup, tracker
 * credentials and user management feedback.
 *
 * French is the reference; English is typed after it, so a key missing in
 * English fails the build.
 */
const fr = {
  screen: {
    title: 'Se connecter à Sectile',
    oidcNotice: 'Ce déploiement connecte les personnes via son fournisseur d\'identité.',
    localNotice: 'Ce déploiement utilise la connexion locale temporaire.',
    continue: 'Continuer',
    continueWithProvider: 'Continuer avec le fournisseur d\'identité',
    email: 'Adresse e-mail',
    passphrase: 'Phrase de scellement',
    optional: '(facultatif)',
    passphrasePlaceholder: 'Seulement si vous avez scellé vos jetons de tracker',
    submit: 'Se connecter',
    submitting: 'Connexion en cours',
    localWarning: 'Aucun mot de passe de connexion n\'est demandé : ce mode identifie les personnes sans les authentifier et est prévu pour un réseau de confiance, en attendant qu\'un fournisseur d\'identité soit branché. Le premier compte créé devient l\'admin. La phrase ci-dessus sert uniquement à sceller vos propres jetons de tracker ; la laisser vide vous connecte avec ces jetons verrouillés.',
    signInFailed: 'Connexion impossible.',
    /** Shown when the server refuses without saying why, with `{status}`. */
    signInFailedStatus: 'Connexion impossible (HTTP {status}).',
    unlockUnreadable: 'Les jetons scellés n\'ont pas pu être lus ; déverrouillez-les depuis votre profil.',
    /** Names the trackers whose tokens stay locked, with `{trackers}`. */
    unlockRefused: 'Phrase de scellement refusée pour {trackers} : ces jetons restent verrouillés, déverrouillez-les depuis votre profil.',
  },
  language: {
    /** Accessible name of the French/English switch of the sign-in screen. */
    switchLabel: 'Langue de l\'interface',
    /** Each language is named in itself, so either reader recognises theirs. */
    fr: 'Français',
    en: 'English',
  },
  status: {
    roles: {
      admin: 'Admin',
      member: 'Membre',
    },
    modes: {
      oidc: 'Fournisseur d\'identité',
      local: 'Connexion locale par e-mail (temporaire)',
    },
  },
  apiKeys: {
    expiryUnknown: 'expiration inconnue',
    /** Pairing code validity, with `{time}`. */
    validUntil: 'Valable jusqu\'à {time}',
    loadFailed: 'Impossible de charger les machines.',
    copyUnavailable: 'Copie indisponible.',
    copyUnavailableManual: 'Copie indisponible. Copiez manuellement.',
    pairingFailed: 'Impossible d\'émettre un code d\'appairage.',
    pairingCodeTitle: 'Code d\'appairage temporaire',
    codeCopied: 'Code d\'appairage copié.',
    renewed: {
      one: 'Accès de {label} prolongé de {count} jour.',
      other: 'Accès de {label} prolongé de {count} jours.',
    },
    noLongerExpires: 'L\'accès de {label} n\'expire plus.',
    renewFailed: 'Impossible de prolonger cette machine.',
    revoked: 'Accès de {label} révoqué.',
    revokeFailed: 'Impossible de révoquer cette machine.',
    copy: 'Copier',
    copied: 'Copié',
    machines: {
      one: '{count} machine',
      other: '{count} machines',
    },
    unnamedWorkstation: 'Machine sans nom',
    renewAria: 'Prolonger {label}',
    revokeAria: 'Révoquer {label}',
    createFailed: 'Impossible de créer une clé API.',
    /** Name used when a key was created without a label. */
    clientFallback: 'Client',
    noExpiry: 'Sans expiration',
    expired: 'Expirée le {date}',
    expiresSoon: {
      one: 'Expire dans {count} jour, prolongez-la',
      other: 'Expire dans {count} jours, prolongez-la',
    },
    expiresOn: {
      one: 'Expire le {date} ({count} jour)',
      other: 'Expire le {date} ({count} jours)',
    },
  },
  agent: {
    copyServerUrl: 'Copier l\'URL du serveur',
    onWorkstationLabel: 'Sur la machine de travail :',
  },
  profile: {
    density: {
      compact: 'Compact',
      standard: 'Standard',
      comfortable: 'Confortable',
    },
    close: 'Fermer',
  },
  users: {
    title: 'Utilisateurs',
    intro: 'Les admins gèrent les comptes : qui existe, quel rôle chacun détient et si son compte s\'ouvre encore. Les membres travaillent sur le tableau partagé, ouvrent et configurent les projets, et n\'agissent que sur leur propre agent et leurs exécutions.',
    rolesFromProvider: 'Les rôles viennent du fournisseur d\'identité : un changement ici dure jusqu\'à la prochaine connexion de la personne.',
    loadFailed: 'Impossible de charger les utilisateurs.',
    columnUser: 'Utilisateur',
    columnLastSignIn: 'Dernière connexion',
    columnRole: 'Rôle',
    columnAccount: 'Compte',
    you: '(vous)',
    blocked: 'Bloqué',
    never: 'jamais',
    roleOf: 'Rôle de {name}',
    roleChanged: '{name} est maintenant {role}.',
    roleFailed: 'Impossible de changer le rôle.',
    confirmBlock: 'Bloquer {name} ? Ses sessions se terminent immédiatement et ses clés de machine cessent de fonctionner. Rien de ce qui lui appartient n\'est supprimé.',
    blockedDone: '{name} est bloqué.',
    unblockedDone: '{name} peut à nouveau se connecter.',
    accountFailed: 'Impossible de modifier le compte.',
    confirmDelete: 'Supprimer {name} ? Le compte, ses sessions et ses clés de machine sont supprimés définitivement. Les tâches, commentaires et exécutions qui lui appartiennent restent sur le tableau, sans propriétaire.',
    deletedDone: '{name} est supprimé.',
    deleteFailed: 'Impossible de supprimer le compte.',
    block: 'Bloquer',
    unblock: 'Débloquer',
    blockAria: 'Bloquer {name}',
    unblockAria: 'Débloquer {name}',
    blockTitle: 'Fermer ce compte sans rien supprimer',
    unblockTitle: 'Autoriser ce compte à se connecter de nouveau',
    delete: 'Supprimer',
    deleteAria: 'Supprimer {name}',
    deleteTitle: 'Supprimer définitivement le compte et ses identifiants',
    /** A refusal without a server reason: the translated message and `{status}`. */
    httpFailure: '{message} (HTTP {status})',
  },
}

export type SignInStrings = typeof fr

const en: SignInStrings = {
  screen: {
    title: 'Sign in to Sectile',
    oidcNotice: 'This deployment signs people in through its identity provider.',
    localNotice: 'This deployment uses the temporary local sign-in.',
    continue: 'Continue',
    continueWithProvider: 'Continue with the identity provider',
    email: 'E-mail address',
    passphrase: 'Sealing passphrase',
    optional: '(optional)',
    passphrasePlaceholder: 'Only if you sealed your tracker tokens',
    submit: 'Sign in',
    submitting: 'Signing in',
    localWarning: 'No login password is asked: this mode identifies people without authenticating them and is meant for a trusted network until an identity provider is connected. The first account created becomes the admin. The passphrase above is only the one sealing your own tracker tokens; leaving it empty signs you in with those tokens locked.',
    signInFailed: 'Could not sign in.',
    signInFailedStatus: 'Could not sign in (HTTP {status}).',
    unlockUnreadable: 'Sealed tokens could not be read; unlock them from your profile.',
    unlockRefused: 'Sealing passphrase refused for {trackers}: those tokens stay locked, unlock them from your profile.',
  },
  language: {
    switchLabel: 'Interface language',
    fr: 'Français',
    en: 'English',
  },
  status: {
    roles: {
      admin: 'Admin',
      member: 'Member',
    },
    modes: {
      oidc: 'Identity provider',
      local: 'Local e-mail sign-in (temporary)',
    },
  },
  apiKeys: {
    expiryUnknown: 'expiry unknown',
    validUntil: 'Valid until {time}',
    loadFailed: 'Could not load workstations.',
    copyUnavailable: 'Copy unavailable.',
    copyUnavailableManual: 'Copy unavailable. Please copy manually.',
    pairingFailed: 'Could not issue a pairing code.',
    pairingCodeTitle: 'Temporary Pairing Code',
    codeCopied: 'Pairing code copied.',
    renewed: {
      one: '{label} renewed for {count} day.',
      other: '{label} renewed for {count} days.',
    },
    noLongerExpires: '{label} no longer expires.',
    renewFailed: 'Could not renew that workstation.',
    revoked: '{label} revoked.',
    revokeFailed: 'Could not revoke that workstation.',
    copy: 'Copy',
    copied: 'Copied',
    machines: {
      one: '{count} machine',
      other: '{count} machines',
    },
    unnamedWorkstation: 'Unnamed Workstation',
    renewAria: 'Renew {label}',
    revokeAria: 'Revoke {label}',
    createFailed: 'Could not create an API key.',
    clientFallback: 'Client',
    noExpiry: 'No expiry',
    expired: 'Expired {date}',
    expiresSoon: {
      one: 'Expires in {count} day, renew it',
      other: 'Expires in {count} days, renew it',
    },
    expiresOn: {
      one: 'Expires {date} ({count} day)',
      other: 'Expires {date} ({count} days)',
    },
  },
  agent: {
    copyServerUrl: 'Copy Server URL',
    onWorkstationLabel: 'On the workstation:',
  },
  profile: {
    density: {
      compact: 'Compact',
      standard: 'Standard',
      comfortable: 'Comfortable',
    },
    close: 'Close',
  },
  users: {
    title: 'Users',
    intro: 'Admins manage the accounts: who exists, what role they hold, and whether their account still opens. Members work on the shared board, open and configure projects, and act only on their own agent and executions.',
    rolesFromProvider: 'Roles come from the identity provider: a change here lasts until that person signs in again.',
    loadFailed: 'Could not load the users.',
    columnUser: 'User',
    columnLastSignIn: 'Last sign-in',
    columnRole: 'Role',
    columnAccount: 'Account',
    you: '(you)',
    blocked: 'Blocked',
    never: 'never',
    roleOf: 'Role of {name}',
    roleChanged: '{name} is now {role}.',
    roleFailed: 'Could not change the role.',
    confirmBlock: 'Block {name}? Their sessions end immediately and their workstation keys stop working. Nothing they own is deleted.',
    blockedDone: '{name} is blocked.',
    unblockedDone: '{name} can sign in again.',
    accountFailed: 'Could not change the account.',
    confirmDelete: 'Delete {name}? The account, its sessions and its workstation keys are removed for good. The tasks, comments and executions it owns stay on the board, with no owner.',
    deletedDone: '{name} is deleted.',
    deleteFailed: 'Could not delete the account.',
    block: 'Block',
    unblock: 'Unblock',
    blockAria: 'Block {name}',
    unblockAria: 'Unblock {name}',
    blockTitle: 'Close this account without deleting anything',
    unblockTitle: 'Let this account sign in again',
    delete: 'Delete',
    deleteAria: 'Delete {name}',
    deleteTitle: 'Remove the account and its credentials for good',
    httpFailure: '{message} (HTTP {status})',
  },
}

export const signIn = { fr, en }
