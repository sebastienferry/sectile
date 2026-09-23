import { GitMerge, GitPullRequest, GitPullRequestClosed, TriangleAlert } from 'lucide-react'
import type { PullRequestLink, Task } from '../types'
import { currentPullRequestLink, pullRequestStateLabel } from '../lib/pullRequests'

export function PullRequestStateIcon({ task, link, size = 12 }: {
  task?: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>
  link?: PullRequestLink
  size?: number
}) {
  const state = (link ?? (task ? currentPullRequestLink(task) : undefined))?.state
  const Icon = state === 'merged' ? GitMerge : state === 'closed' ? GitPullRequestClosed : state === 'conflicting' ? TriangleAlert : GitPullRequest
  const color = state === 'merged' ? 'text-purple-400' : state === 'closed' ? 'text-red-400' : state === 'conflicting' ? 'text-amber-400' : state === 'open' ? 'text-green-400' : 'text-slate-400'
  return <span title={pullRequestStateLabel(state)}><Icon size={size} className={color} role="img" aria-label={pullRequestStateLabel(state)} /></span>
}
