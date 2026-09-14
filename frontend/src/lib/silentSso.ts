// Silent SSO: when Keycloak still has a session, sign the visitor in with
// prompt=none instead of showing the login screen.
//
// prompt=none never renders UI. It either answers with a code or bounces back
// with login_required, and retrying after that refusal would send the browser
// between the provider and this app forever. Three guards stop that:
//   1. an "attempted" flag in sessionStorage (one attempt per tab session),
//   2. a "signed out" flag written on logout, cleared once a session exists,
//   3. the ?sso=none marker the callback puts in the URL after a refusal.
// A storage that cannot be read counts as "already attempted" — private modes
// throw here, and reading that as "not yet" is exactly how loops start.

const ATTEMPTED_KEY = 'trace.sso.silentAttempted'
const SIGNED_OUT_KEY = 'trace.sso.signedOut'

// Paths that must never start a silent attempt: the login page and the
// callback are where loops come from, and API/MCP/health paths are not
// browser navigations at all.
const EXCLUDED_PREFIXES = ['/login', '/api/', '/mcp', '/healthz']

export type SilentSsoEnvironment = {
  pathname: string
  search: string
  storage: Pick<Storage, 'getItem' | 'setItem' | 'removeItem'> | undefined
}

function browserEnvironment(): SilentSsoEnvironment {
  let storage: SilentSsoEnvironment['storage']
  try { storage = window.sessionStorage } catch { storage = undefined }
  return { pathname: window.location.pathname, search: window.location.search, storage }
}

function readFlag(env: SilentSsoEnvironment, key: string): boolean {
  try {
    if (!env.storage) return true
    return env.storage.getItem(key) === 'true'
  } catch {
    // Fail closed: an unreadable storage is treated as "already attempted".
    return true
  }
}

function writeFlag(env: SilentSsoEnvironment, key: string, value: boolean) {
  try {
    if (!env.storage) return
    if (value) env.storage.setItem(key, 'true')
    else env.storage.removeItem(key)
  } catch {
    /* nothing to do; readFlag already fails closed */
  }
}

/** Only same-origin paths may be used as a return target. */
export function safeReturnTo(value: string): string {
  return value.startsWith('/') && !value.startsWith('//') && !value.startsWith('/\\') ? value : '/'
}

/** True when this path is a browser page where a silent attempt is allowed. */
export function silentSsoAllowedOn(pathname: string): boolean {
  return !EXCLUDED_PREFIXES.some(prefix => pathname === prefix.replace(/\/$/, '') || pathname.startsWith(prefix))
}

/**
 * Decides whether to try signing in without showing a login screen. It must
 * never say yes more than once per tab session.
 */
export function shouldAttemptSilentSso(autoLogin: boolean, env: SilentSsoEnvironment = browserEnvironment()): boolean {
  if (!autoLogin) return false
  if (!silentSsoAllowedOn(env.pathname)) return false
  const params = new URLSearchParams(env.search)
  // The callback appends this marker when the provider had no session, so a
  // refusal is remembered even if sessionStorage was cleared in between.
  if (params.get('sso') === 'none' || params.get('sso') === 'error') return false
  if (readFlag(env, SIGNED_OUT_KEY)) return false
  if (readFlag(env, ATTEMPTED_KEY)) return false
  return true
}

/** Cheap pre-check so the public config is not fetched when a retry is already ruled out. */
export function silentSsoRuledOut(env: SilentSsoEnvironment = browserEnvironment()): boolean {
  return !shouldAttemptSilentSso(true, env)
}

/** Builds the top-level navigation target for a silent attempt. */
export function silentSsoStartUrl(loginUrl: string, returnTo: string): string {
  return `${loginUrl}?prompt=none&returnTo=${encodeURIComponent(safeReturnTo(returnTo))}`
}

/** Records the attempt, then sends the browser (top-level, not an iframe) to the provider. */
export function beginSilentSso(loginUrl: string, returnTo: string, env: SilentSsoEnvironment = browserEnvironment()) {
  writeFlag(env, ATTEMPTED_KEY, true)
  window.location.assign(silentSsoStartUrl(loginUrl, returnTo))
}

/** Records that the user signed out on purpose, which suppresses auto-login. */
export function markSignedOut(env: SilentSsoEnvironment = browserEnvironment()) {
  writeFlag(env, SIGNED_OUT_KEY, true)
  writeFlag(env, ATTEMPTED_KEY, true)
}

/** Clears the suppression once a session exists again. */
export function clearSilentSsoState(env: SilentSsoEnvironment = browserEnvironment()) {
  writeFlag(env, SIGNED_OUT_KEY, false)
  writeFlag(env, ATTEMPTED_KEY, false)
}
