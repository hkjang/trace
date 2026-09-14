import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from './api'
import { beginSilentSso, clearSilentSsoState, markSignedOut, shouldAttemptSilentSso, silentSsoRuledOut } from './silentSso'
import type { PublicConfig, User, VersionInfo } from '../types'

type AuthState = { user: User | null; version: VersionInfo | null; loading: boolean; refresh: () => Promise<void>; logout: () => Promise<void> }
const AuthContext = createContext<AuthState | null>(null)

// Tries a prompt=none sign-in when the administrator turned auto_login on.
// Returns true when the browser is being sent away, so the caller keeps the
// boot screen up instead of flashing the login page first.
async function maybeBeginSilentSso(): Promise<boolean> {
  if (silentSsoRuledOut()) return false
  let config: PublicConfig
  try { config = await api<PublicConfig>('/api/v1/public/config') } catch { return false }
  if (!shouldAttemptSilentSso(config.oidc.enabled && config.oidc.autoLogin)) return false
  beginSilentSso(config.oidc.loginUrl, window.location.pathname + window.location.search)
  return true
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [version, setVersion] = useState<VersionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const refresh = async () => {
    try {
      const value = await api<{ user: User; version: VersionInfo }>('/api/v1/me')
      setUser(value.user)
      setVersion(value.version)
      // A session exists again, so a later sign-out may be followed by a fresh silent attempt.
      clearSilentSsoState()
    } catch {
      setUser(null)
      if (await maybeBeginSilentSso()) return
    }
    setLoading(false)
  }
  useEffect(() => { void refresh() }, [])
  const logout = async () => { await api('/api/v1/auth/logout', { method: 'POST' }); markSignedOut(); setUser(null) }
  const state = useMemo(() => ({ user, version, loading, refresh, logout }), [user, version, loading])
  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
