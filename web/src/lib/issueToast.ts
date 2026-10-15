import type { Task, ToastLink, ToastMessage } from '../types/index.ts'

export function issueToastLink(
  task: Task,
  open: (task: Task) => void,
  labels: { internal: string; external: string },
): ToastLink | undefined {
  if (task.id) {
    return {
      label: `${labels.internal} ${task.key}`,
      onOpen: () => open(task),
      externalUrl: task.externalUrl || undefined,
    }
  }
  if (task.externalUrl) {
    return { label: `${labels.external} ${task.key}`, externalUrl: task.externalUrl }
  }
  return undefined
}

/** What a handler holds of a ticket: the returned `Task`, or its id and key. */
export type TicketRef = Task | { id?: string; key?: string }

/** A returned ticket, as opposed to a bare reference: only a `Task` has a title. */
export function isTask(ref: TicketRef): ref is Task {
  return 'title' in ref
}

/** A toast's ticket links, and how many subject tickets it lists without one (#841). */
export interface TicketLinks {
  links: ToastLink[]
  more: number
}

/**
 * The link of a ticket known only by id and key, which reads the ticket when
 * clicked rather than building one from what the caller holds. Both are
 * needed: the label names the key, the read takes the id.
 */
export function issueRefLink(
  ref: { id?: string; key?: string },
  openById: (id: string, key: string) => void,
  labels: { internal: string },
): ToastLink | undefined {
  const { id, key } = ref
  if (!id || !key) return undefined
  return { label: `${labels.internal} ${key}`, onOpen: () => openById(id, key) }
}

/**
 * The links of a toast about several tickets: the first `max` that have one,
 * in order. Every ticket past them, and every ticket without a link, is
 * counted in `more`, except when there is only one.
 */
export function cappedLinks(links: (ToastLink | undefined)[], max = 5): TicketLinks {
  // One ticket through a multi-ticket path reads as a single-ticket toast.
  if (links.length === 1) return { links: links[0] ? [links[0]] : [], more: 0 }
  const kept: ToastLink[] = []
  let more = 0
  for (const link of links) {
    if (link && kept.length < max) kept.push(link)
    else more++
  }
  return { links: kept, more }
}

/**
 * The toast with its ticket links added after the link it already carries,
 * such as the credential offer. A single link counts as one ticket: an unknown
 * ticket then adds nothing, not a "+1 more".
 */
export function withTicketLinks(
  toast: Omit<ToastMessage, 'id'>,
  tickets: ToastLink | TicketLinks | undefined,
): Omit<ToastMessage, 'id'> {
  const ticket: TicketLinks = !tickets ? { links: [], more: 0 } : 'more' in tickets ? tickets : { links: [tickets], more: 0 }
  if (ticket.links.length === 0 && ticket.more === 0) return toast
  const { link, ...rest } = toast
  const result: Omit<ToastMessage, 'id'> = { ...rest, links: [...(link ? [link] : toast.links ?? []), ...ticket.links] }
  if (ticket.more > 0) result.moreLinks = ticket.more
  return result
}
