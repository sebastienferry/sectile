/**
 * The browser side of sign-in: who the server says we are, where to send a
 * person who must sign in, and how to come back afterwards. Pure helpers live
 * here so the redirect rules are tested without a DOM.
 */

/** How people sign in on this deployment, as `/api/me` reports it. */
export type SignInMode = 'oidc' | 'local' | 'implicit'

export type Role = 'admin' | 'member'

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

/** Whether the interface must show the sign-in screen for this user. */
export function needsSignIn(user: CurrentUser | null): boolean {
  if (!user) return false
  return !user.signedIn && user.mode !== 'implicit'
}

export function describeSignInMode(mode: SignInMode): string {
  switch (mode) {
    case 'oidc': return 'Identity provider'
    case 'local': return 'Local e-mail sign-in (temporary)'
    default: return 'Single user, no sign-in'
  }
}

export function describeRole(role: Role | ''): string {
  switch (role) {
    case 'admin': return 'Admin'
    case 'member': return 'Member'
    default: return ''
  }
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
