import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from './api'
import type { User, VersionInfo } from '../types'

type AuthState = { user: User | null; version: VersionInfo | null; loading: boolean; refresh: () => Promise<void>; logout: () => Promise<void> }
const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [version, setVersion] = useState<VersionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const refresh = async () => {
    try {
      const value = await api<{ user: User; version: VersionInfo }>('/api/v1/me')
      setUser(value.user)
      setVersion(value.version)
    } catch {
      setUser(null)
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { void refresh() }, [])
  const logout = async () => { await api('/api/v1/auth/logout', { method: 'POST' }); setUser(null) }
  const state = useMemo(() => ({ user, version, loading, refresh, logout }), [user, version, loading])
  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
