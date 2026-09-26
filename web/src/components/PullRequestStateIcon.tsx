import { GitMerge, GitPullRequest, GitPullRequestClosed, TriangleAlert } from 'lucide-react'
import { useApp } from '../context/AppContext'
import type { PullRequestLink, Task } from '../types'
import { currentPullRequestLink } from '../lib/pullRequests'

export function PullRequestStateIcon({ task, link, size = 12 }: {
  task?: Pick<Task, 'prLinks' | 'prUrl' | 'branchName'>
  link?: PullRequestLink
  size?: number
}) {
  const { t } = useApp()
  const state = (link ?? (task ? currentPullRequestLink(task) : undefined))?.state
  const Icon = state === 'merged' ? GitMerge : state === 'closed' ? GitPullRequestClosed : state === 'conflicting' ? TriangleAlert : GitPullRequest
  const color = state === 'merged' ? 'text-purple-400' : state === 'closed' ? 'text-red-400' : state === 'conflicting' ? 'text-amber-400' : state === 'open' ? 'text-green-400' : 'text-slate-400'
  const states = t.taskDetail.pr.states
  const label = state === 'open' || state === 'conflicting' || state === 'merged' || state === 'closed' ? states[state] : states.unknown
  return <span title={label}><Icon size={size} className={color} role="img" aria-label={label} /></span>
}
