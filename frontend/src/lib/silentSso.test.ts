import { describe, expect, it } from 'vitest'
import { clearSilentSsoState, markSignedOut, safeReturnTo, shouldAttemptSilentSso, silentSsoAllowedOn, silentSsoStartUrl, type SilentSsoEnvironment } from './silentSso'

class MemoryStorage {
  private items = new Map<string, string>()
  getItem(key: string) { return this.items.get(key) ?? null }
  setItem(key: string, value: string) { this.items.set(key, value) }
  removeItem(key: string) { this.items.delete(key) }
}

class BrokenStorage {
  getItem(): string | null { throw new DOMException('blocked', 'SecurityError') }
  setItem(): void { throw new DOMException('blocked', 'SecurityError') }
  removeItem(): void { throw new DOMException('blocked', 'SecurityError') }
}

function env(overrides: Partial<SilentSsoEnvironment> = {}): SilentSsoEnvironment {
  return { pathname: '/', search: '', storage: new MemoryStorage(), ...overrides }
}

describe('shouldAttemptSilentSso', () => {
  it('does nothing when auto_login is off', () => {
    expect(shouldAttemptSilentSso(false, env())).toBe(false)
  })

  it('tries once on an ordinary page', () => {
    expect(shouldAttemptSilentSso(true, env({ pathname: '/decisions/42' }))).toBe(true)
  })

  it('does not retry after the callback marked a refusal in the URL', () => {
    expect(shouldAttemptSilentSso(true, env({ search: '?sso=none' }))).toBe(false)
    expect(shouldAttemptSilentSso(true, env({ search: '?sso=error' }))).toBe(false)
  })

  it('does not try again in the same tab session once attempted', () => {
    const storage = new MemoryStorage()
    storage.setItem('trace.sso.silentAttempted', 'true')
    expect(shouldAttemptSilentSso(true, env({ storage }))).toBe(false)
  })

  it('stays quiet after a deliberate sign-out until a session exists again', () => {
    const e = env()
    markSignedOut(e)
    expect(shouldAttemptSilentSso(true, e)).toBe(false)
    clearSilentSsoState(e)
    expect(shouldAttemptSilentSso(true, e)).toBe(true)
  })

  it('fails closed when storage cannot be read', () => {
    expect(shouldAttemptSilentSso(true, env({ storage: new BrokenStorage() }))).toBe(false)
    expect(shouldAttemptSilentSso(true, env({ storage: undefined }))).toBe(false)
  })

  it('never starts from the login, callback, API, MCP or health paths', () => {
    for (const pathname of ['/login', '/api/v1/auth/oidc/callback', '/api/v1/me', '/mcp', '/healthz']) {
      expect(silentSsoAllowedOn(pathname), pathname).toBe(false)
      expect(shouldAttemptSilentSso(true, env({ pathname }))).toBe(false)
    }
    expect(silentSsoAllowedOn('/')).toBe(true)
    expect(silentSsoAllowedOn('/insights')).toBe(true)
  })
})

describe('return target', () => {
  it('keeps deep links so the visitor lands back where they started', () => {
    expect(silentSsoStartUrl('/api/v1/auth/oidc/start', '/decisions/42?tab=evidence')).toBe('/api/v1/auth/oidc/start?prompt=none&returnTo=%2Fdecisions%2F42%3Ftab%3Devidence')
  })

  it('only accepts same-origin paths', () => {
    expect(safeReturnTo('/decisions/42')).toBe('/decisions/42')
    expect(safeReturnTo('//evil.example/')).toBe('/')
    expect(safeReturnTo('/\\evil.example')).toBe('/')
    expect(safeReturnTo('https://evil.example/')).toBe('/')
    expect(safeReturnTo('')).toBe('/')
  })
})
