import { ArrowRight, Fingerprint, LockKeyhole } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import { useAuth } from '../lib/auth'
import type { PublicConfig, User } from '../types'

export default function LoginPage() {
  const [config, setConfig] = useState<PublicConfig | null>(null)
  const [identity, setIdentity] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const { user, refresh } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const destination = (location.state as { from?: string } | null)?.from || '/'
  useEffect(() => { api<PublicConfig>('/api/v1/public/config').then(setConfig).catch(() => setError('서비스 설정을 불러오지 못했습니다.')) }, [])
  useEffect(() => { if (user) navigate(destination, { replace: true }) }, [user])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setError(''); setBusy(true)
    try { await api<User>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ identity, password }) }); await refresh(); navigate(destination, { replace: true }) }
    catch (err) { setError(err instanceof Error ? err.message : '로그인하지 못했습니다.') }
    finally { setBusy(false) }
  }
  return <main className="login-page">
    <section className="login-story" aria-hidden="true">
      <div className="login-brand"><img src="/logo.svg" alt="Trace" className="brand-logo-img large" /><span>TRACE</span></div>
      <div className="time-art">
        <span className="time-label past">PAST</span><span className="time-label now">NOW</span><span className="time-label future">FUTURE</span>
        <span className="time-line solid" /><span className="time-line dotted" /><span className="time-node evidence">E</span><span className="time-node decision">D</span><span className="time-node outcome">O</span>
      </div>
      <div className="login-manifesto"><p className="eyebrow">DECISION MEMORY</p><h1>그때의 판단 속으로<br />다시 들어갑니다.</h1><p>결과가 아니라, 당시 알고 있던 정보와 판단의 품질을 기억하세요.</p></div>
    </section>
    <section className="login-panel">
      <div className="login-form-wrap">
        <p className="eyebrow">WELCOME BACK</p><h2>{config?.branding.serviceName || 'Trace'}에 로그인</h2><p className="muted">{config?.branding.tagline || 'Remember why you decided.'}</p>
        {error && <div className="error-notice">{error}</div>}
        <form onSubmit={submit} className="login-form">
          <label><span>아이디 또는 이메일</span><input autoFocus autoComplete="username" value={identity} onChange={e => setIdentity(e.target.value)} placeholder="admin@example.com" required /></label>
          <label><span>비밀번호</span><input type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} placeholder="••••••••••••" required /></label>
          <button className="button primary wide" disabled={busy}><LockKeyhole size={18} />{busy ? '확인하는 중…' : '로그인'}<ArrowRight size={18} /></button>
        </form>
        {config?.oidc.enabled && <><div className="or"><span>또는</span></div><a className="button secondary wide" href={`${config.oidc.loginUrl}?returnTo=${encodeURIComponent(destination)}`}><Fingerprint size={18} />Keycloak SSO로 계속</a></>}
      </div>
      <footer className="login-version"><span>TRACE</span><span>v{config?.version.version || '…'}</span><span>{config?.version.commit !== 'unknown' ? config?.version.commit.slice(0, 8) : 'offline ready'}</span></footer>
    </section>
  </main>
}
