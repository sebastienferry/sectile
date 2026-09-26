/**
 * The browser side of sign-in: who the server says we are, where to send a
 * person who must sign in, and how to come back afterwards. Pure helpers live
 * here so the redirect rules are tested without a DOM.
 */

/** How people sign in on this deployment, as `/api/me` reports it. */
export type SignInMode = 'oidc' | 'local'

export type Role = 'admin' | 'member'

/** What the server says it stores about a personal tracker credential. */
export interface SealedCredential {
  tracker: string
  sealed: boolean
  unlocked: boolean
}

export interface CurrentUser {
  userId: string
  signedIn: boolean
  identityProvider: boolean
  mode: SignInMode
  role: Role | ''
  email?: string
  displayName?: string
  sharedServerToken?: boolean
}

/** The interface route that shows the sign-in screen. */
export const SIGN_IN_PATH = '/signin'

/**
 * A return target is only ever a path inside the interface: anything else
 * would turn sign-in into a bounce to whatever a link named. Mirrors the
 * server's rule so the two never disagree.
 */
export function safeRedirectPath(target: string | null | undefined): string {
  const value = (target ?? '').trim()
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/'
  // Coming back to the sign-in screen itself would loop.
  if (value === SIGN_IN_PATH || value.startsWith(SIGN_IN_PATH + '?')) return '/'
  return value
}

/** Where to send someone who must sign in, remembering where they were. */
export function signInPath(returnTo: string): string {
  const target = safeRedirectPath(returnTo)
  return target === '/' ? SIGN_IN_PATH : SIGN_IN_PATH + '?redirect=' + encodeURIComponent(target)
}

/** The return target carried by a sign-in URL, made safe. */
export function redirectFromSearch(search: string): string {
  return safeRedirectPath(new URLSearchParams(search).get('redirect'))
}

/**
 * Whether a response to an interface call means "sign in": a 401 on the API,
 * while not already on the sign-in screen. Machine surfaces and static files
 * never trigger it, and a second 401 on the sign-in screen must not loop.
 */
export function shouldRedirectToSignIn(url: string, status: number, currentPath: string): boolean {
  if (status !== 401) return false
  if (currentPath === SIGN_IN_PATH) return false
  let path: string
  try {
    path = new URL(url, 'http://sectile.local').pathname
  } catch {
    return false
  }
  return path.startsWith('/api/') && !path.startsWith('/api/v1/')
}

/**
 * Whether the interface must show the sign-in screen for this user. Signing in
 * is mandatory (ADR 0015): a signed-out visitor sees the screen and nothing
 * else, whatever the deployment's mode.
 */
export function needsSignIn(user: CurrentUser | null): boolean {
  if (!user) return false
  return !user.signedIn
}

/** The caller's catalog words for each sign-in mode. */
export type SignInModeLabels = Record<SignInMode, string>

/** The caller's catalog words for each role. */
export type RoleLabels = Record<Role, string>

export function describeSignInMode(mode: SignInMode, labels: SignInModeLabels): string {
  switch (mode) {
    case 'oidc': return labels.oidc
    default: return labels.local
  }
}

export function describeRole(role: Role | '', labels: RoleLabels): string {
  switch (role) {
    case 'admin': return labels.admin
    case 'member': return labels.member
    default: return ''
  }
}

/**
 * Why sealed tokens stay locked after sign-in, for the screen to word in its
 * language: they could not be listed, or the passphrase was refused for the
 * named trackers.
 */
export type UnlockRefusal =
  | { code: 'unreadable' }
  | { code: 'refused'; trackers: string[] }

/**
 * Unlocks the sealed tracker tokens of the person who just signed in, one call
 * per sealed tracker since the route takes one at a time. It reports, it never
 * blocks: the session is already open. `null` means there is nothing to say.
 */
export async function unlockSealedCredentials(passphrase: string): Promise<UnlockRefusal | null> {
  const phrase = passphrase.trim()
  if (!phrase) return null
  let sealed: SealedCredential[] = []
  try {
    const res = await fetch('/api/me/tracker-credentials')
    if (!res.ok) return null
    const body = await res.json().catch(() => ({}))
    const list: SealedCredential[] = Array.isArray(body.credentials) ? body.credentials : []
    sealed = list.filter(credential => credential.sealed && !credential.unlocked)
  } catch {
    return { code: 'unreadable' }
  }
  if (sealed.length === 0) return null
  const refused: string[] = []
  for (const credential of sealed) {
    try {
      const res = await fetch('/api/me/tracker-credentials/unlock', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tracker: credential.tracker, passphrase: phrase }),
      })
      if (!res.ok) refused.push(credential.tracker)
    } catch {
      refused.push(credential.tracker)
    }
  }
  if (refused.length === 0) return null
  return { code: 'refused', trackers: refused }
}

/**
 * Installs the one place the interface learns about an expired or missing
 * session: a 401 from the API sends the browser to the sign-in screen, once.
 * Every existing fetch call keeps its shape; none has to know.
 */
export function installUnauthorizedRedirect(target: typeof window = window): void {
  const original = target.fetch.bind(target)
  let redirecting = false
  target.fetch = async (input, init) => {
    const response = await original(input, init)
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    if (!redirecting && shouldRedirectToSignIn(url, response.status, target.location.pathname)) {
      redirecting = true
      target.location.assign(signInPath(target.location.pathname + target.location.search))
    }
    return response
  }
}
