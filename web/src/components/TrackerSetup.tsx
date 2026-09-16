import React, { useState } from 'react'
import { Check, Key, Globe, Mail, Loader2, ShieldCheck, X, AlertCircle, FolderGit2 } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { TrackerCredentials } from '../types'

/** Ce que chaque tracker demande. Un seul écran, trois jeux de champs. */
type TrackerKind = TrackerCredentials['tracker']

const TRACKERS: {
  id: TrackerKind
  label: string
  siteLabel: string
  sitePlaceholder: string
  /** Jira s'authentifie avec un e-mail ; GitHub et GitLab non. */
  wantsEmail: boolean
  projectLabel?: string
  projectPlaceholder?: string
  tokenHint: string
}[] = [
  {
    id: 'jira',
    label: 'Jira',
    siteLabel: 'Site Jira',
    sitePlaceholder: 'mon-org.atlassian.net',
    wantsEmail: true,
    tokenHint: "À créer sur id.atlassian.com, section jetons d'API.",
  },
  {
    id: 'github',
    label: 'GitHub',
    siteLabel: "URL de l'API GitHub",
    sitePlaceholder: 'https://api.github.com',
    wantsEmail: false,
    projectLabel: 'Dépôt par défaut',
    projectPlaceholder: 'organisation/depot',
    tokenHint: 'Jeton personnel (PAT) avec la portée repo.',
  },
  {
    id: 'gitlab',
    label: 'GitLab',
    siteLabel: "URL de l'API GitLab",
    sitePlaceholder: 'https://gitlab.com/api/v4',
    wantsEmail: false,
    projectLabel: 'Projet par défaut',
    projectPlaceholder: 'groupe/projet',
    tokenHint: 'Jeton personnel avec la portée api.',
  },
]

/**
 * Premier démarrage : ce qu'il faut savoir avant que quoi que ce soit fonctionne.
 *
 * Sans instance et sans jeton, la synchronisation ne ramène rien, les équipes
 * restent vides et aucune écriture ne part. Jusqu'ici on l'apprenait en
 * synchronisant pour rien. Cet écran demande les valeurs du tracker choisi, les
 * vérifie auprès de l'instance, et dit à quel compte elles donnent accès avant
 * d'enregistrer quoi que ce soit.
 *
 * Les champs suivent le tracker : demander un site Jira et un e-mail Atlassian
 * pour configurer GitHub n'a jamais eu de sens.
 */
export const TrackerSetup: React.FC<{ onClose: () => void }> = ({ onClose }) => {
  const { checkTrackerCredentials, saveTrackerCredentials, settings, addToast } = useApp()

  const initial: TrackerKind = settings.issueTracker === 'github' ? 'github' : 'jira'
  const [tracker, setTracker] = useState<TrackerKind>(initial)
  const kind = TRACKERS.find(t => t.id === tracker) ?? TRACKERS[0]

  const storedSite = (t: TrackerKind) =>
    t === 'jira' ? settings.jiraUrl || '' : t === 'github' ? settings.githubApiUrl || '' : settings.gitlabUrl || ''
  const storedProject = (t: TrackerKind) =>
    t === 'github' ? settings.githubRepo || '' : t === 'gitlab' ? settings.gitlabProject || '' : ''
  const tokenIsSet =
    tracker === 'jira'
      ? settings.jiraApiTokenSet
      : tracker === 'github'
        ? settings.githubTokenSet
        : settings.gitlabTokenSet
  const tokenFromEnv =
    tracker === 'jira'
      ? settings.jiraApiTokenFromEnv
      : tracker === 'github'
        ? settings.githubTokenFromEnv
        : settings.gitlabTokenFromEnv

  const [siteUrl, setSiteUrl] = useState(storedSite(initial))
  const [project, setProject] = useState(storedProject(initial))
  const [email, setEmail] = useState(settings.jiraEmail || '')
  const [token, setToken] = useState('')
  // Le fichier hors base n'existe que pour Jira : rien ne l'écrit encore pour
  // les autres, l'afficher promettrait ce que le serveur ne fait pas.
  const [storeInFile, setStoreInFile] = useState(true)

  const selectTracker = (next: TrackerKind) => {
    setTracker(next)
    setSiteUrl(storedSite(next))
    setProject(storedProject(next))
    setToken('')
    setCheck(null)
  }
  const [isChecking, setIsChecking] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [check, setCheck] = useState<{
    ok: boolean
    error?: string
    account?: string
    identity?: { displayName: string; email?: string; siteUrl: string }
    projects?: { id: string; name: string }[]
  } | null>(null)

  const credentials = (): TrackerCredentials => ({
    tracker,
    siteUrl,
    project,
    email,
    token,
    storeTokenInFile: tracker === 'jira' && storeInFile,
  })

  const runCheck = async () => {
    setIsChecking(true)
    setCheck(await checkTrackerCredentials(credentials()))
    setIsChecking(false)
  }

  const save = async () => {
    setIsSaving(true)
    const saved = await saveTrackerCredentials(credentials())
    setIsSaving(false)
    if (saved) {
      addToast({
        type: 'success',
        title: `${kind.label} configuré`,
        description:
          tracker === 'jira' && storeInFile
            ? 'Le jeton est écrit dans le fichier de configuration, hors de la base.'
            : 'Le jeton est enregistré dans la configuration utilisateur.',
      })
      onClose()
    }
  }

  const fieldClass =
    'w-full pl-8 pr-3 py-2 text-xs rounded-xl bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent-color)]'

  return (
    <div className="fixed top-0 left-0 h-[var(--app-h)] w-[var(--app-w)] z-60 flex items-center justify-center p-4 bg-black/70 backdrop-blur-xs">
      <div className="relative w-full max-w-lg rounded-2xl bg-[var(--bg-secondary)] border border-[var(--border-color)] shadow-2xl overflow-hidden flex flex-col max-h-[calc(var(--app-h)*0.9)]">
        <div className="flex items-start justify-between px-5 py-4 border-b border-[var(--border-color)]">
          <div className="min-w-0">
            <h2 className="text-sm font-bold text-[var(--text-primary)]">Connecter votre tracker</h2>
            <p className="text-[11px] text-[var(--text-secondary)] mt-0.5 leading-relaxed">
              Sans ces valeurs, la synchronisation ne ramène rien et aucune écriture ne part. Elles
              sont vérifiées auprès de l'instance avant d'être enregistrées.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-tertiary)] cursor-pointer shrink-0"
            title="Configurer plus tard"
          >
            <X size={16} />
          </button>
        </div>

        <div className="p-5 space-y-3 overflow-y-auto">
          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Tracker
            </label>
            <div className="flex items-center gap-1.5">
              {TRACKERS.map(t => (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => selectTracker(t.id)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-semibold border cursor-pointer ${
                    t.id === tracker
                      ? 'accent-bg text-white border-transparent'
                      : 'bg-[var(--bg-tertiary)] border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'
                  }`}
                >
                  {t.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              {kind.siteLabel}
            </label>
            <div className="relative">
              <input
                type="text"
                value={siteUrl}
                onChange={e => setSiteUrl(e.target.value)}
                placeholder={kind.sitePlaceholder}
                className={fieldClass}
              />
              <Globe size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
            </div>
          </div>

          {kind.wantsEmail && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                E-mail Atlassian
              </label>
              <div className="relative">
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  placeholder="prenom.nom@exemple.com"
                  className={fieldClass}
                />
                <Mail size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
              </div>
            </div>
          )}

          {kind.projectLabel && (
            <div>
              <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
                {kind.projectLabel}
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={project}
                  onChange={e => setProject(e.target.value)}
                  placeholder={kind.projectPlaceholder}
                  className={fieldClass}
                />
                <FolderGit2 size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
              </div>
            </div>
          )}

          <div>
            <label className="block text-[10px] font-bold uppercase tracking-wider text-[var(--text-muted)] mb-1">
              Jeton d'API
            </label>
            <div className="relative">
              <input
                type="password"
                value={token}
                onChange={e => setToken(e.target.value)}
                placeholder={
                  tokenIsSet
                    ? 'Déjà configuré, laissez vide pour le garder'
                    : tokenFromEnv
                      ? "Fourni par l'environnement du serveur, laissez vide pour le garder"
                      : 'Collez le jeton'
                }
                className={fieldClass}
              />
              <Key size={13} className="absolute left-2.5 top-2.5 text-[var(--accent-color)]" />
            </div>
            <span className="text-[9.5px] text-[var(--text-muted)] block mt-1">
              {kind.tokenHint}
              {tokenFromEnv && ' Un jeton vient déjà de l’environnement du serveur ; celui saisi ici prime.'}
            </span>
          </div>

          {tracker === 'jira' && (
          <label className="flex items-start gap-2 text-[10.5px] text-[var(--text-secondary)] cursor-pointer">
            <input
              type="checkbox"
              checked={storeInFile}
              onChange={e => setStoreInFile(e.target.checked)}
              className="mt-0.5 accent-[var(--accent-color)]"
            />
            <span>
              Écrire le jeton dans un fichier de configuration plutôt que dans la base. La base est
              un fichier qu'on copie et qu'on sauvegarde ; le fichier de configuration, lui, est
              celui qu'on sait ne pas transmettre.
            </span>
          </label>
          )}

          {check && (
            <div
              className="p-3 rounded-xl border text-[11px] leading-relaxed"
              style={{
                background: check.ok ? 'rgb(var(--status-ok-rgb) / 0.1)' : 'rgb(var(--status-danger-rgb) / 0.1)',
                borderColor: check.ok ? 'rgb(var(--status-ok-rgb) / 0.35)' : 'rgb(var(--status-danger-rgb) / 0.35)',
                color: check.ok ? 'var(--status-ok)' : 'var(--status-danger)',
              }}
            >
              {check.ok ? (
                <>
                  <div className="flex items-center gap-1.5 font-bold">
                    <ShieldCheck size={13} />
                    Connecté comme {check.identity?.displayName ?? check.account}
                  </div>
                  {check.projects && check.projects.length > 0 && (
                    <div className="mt-1 text-[var(--text-secondary)]">
                      {check.projects.length} projet(s) visibles, dont{' '}
                      <span className="font-mono">
                        {check.projects.slice(0, 5).map(p => p.id).join(', ')}
                      </span>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex items-start gap-1.5">
                  <AlertCircle size={13} className="shrink-0 mt-0.5" />
                  <span>{check.error}</span>
                </div>
              )}
            </div>
          )}
        </div>

        <div className="flex items-center justify-between gap-2 px-5 py-3.5 border-t border-[var(--border-color)] bg-[var(--bg-tertiary)]/40">
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-1.5 rounded-xl text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            Plus tard
          </button>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={runCheck}
              disabled={isChecking || (kind.wantsEmail && (!siteUrl.trim() || !email.trim()))}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold bg-[var(--bg-tertiary)] border border-[var(--border-color)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-40 cursor-pointer"
            >
              {isChecking ? <Loader2 size={13} className="animate-spin" /> : <ShieldCheck size={13} />}
              Vérifier
            </button>
            <button
              type="button"
              onClick={save}
              disabled={isSaving || !check?.ok}
              title={check?.ok ? 'Enregistrer ces accès' : "Vérifiez d'abord les accès"}
              className="flex items-center gap-1.5 px-4 py-1.5 rounded-xl text-xs font-bold text-white accent-bg shadow-xs hover:opacity-90 disabled:opacity-40 cursor-pointer"
            >
              {isSaving ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
              Enregistrer
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
