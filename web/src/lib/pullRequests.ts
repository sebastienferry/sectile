import type { PullRequestLink, Task } from '../types'

/**
 * Les liens de pull request d'un ticket.
 *
 * Un ticket produit couramment plusieurs PR : une première fusionnée, puis une
 * suite poussée sur la même branche. Le serveur en tient l'ensemble ordonné, et
 * `prUrl` en est le dernier lien, la PR courante.
 *
 * Un ticket synchronisé avant cet ensemble, ou un objet de tracker qui ne porte
 * que `prUrl`, n'expose pas `prLinks`. Replier ce cas ici, plutôt qu'à chaque
 * appelant, évite que la fiche se croie modifiée dès son ouverture et réécrive
 * le ticket à sa fermeture.
 */
export function taskPullRequestLinks(task: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>): PullRequestLink[] {
  if (task.prLinks && task.prLinks.length > 0) return task.prLinks
  return task.prUrl ? [{ url: task.prUrl, branch: task.branchName }] : []
}

/** La PR courante du ticket : le dernier lien de l'ensemble. */
export function currentPullRequest(links: PullRequestLink[]): string | undefined {
  return links.length > 0 ? links[links.length - 1].url : undefined
}

/**
 * Ajoute un lien à l'ensemble. Une URL déjà présente laisse l'ensemble
 * inchangé, position comprise : rouvrir la fiche ne doit pas réordonner
 * l'historique d'un ticket.
 */
export function addPullRequestLink(links: PullRequestLink[], url: string, branch?: string): PullRequestLink[] {
  const trimmed = url.trim()
  if (!trimmed || links.some(l => l.url === trimmed)) return links
  return [...links, { url: trimmed, branch: branch?.trim() || undefined }]
}
