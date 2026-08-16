import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { BrainCircuit, CheckSquare2, ChevronDown, Clock3, KeyRound, LayoutDashboard, LogOut, Menu, Network, Plus, Search, Settings2, UserRound, X } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useAuth } from '../lib/auth'

const nav = [
  { to: '/', label: '시간의 흐름', icon: LayoutDashboard, end: true },
  { to: '/graph', label: 'Decision Network', icon: Network },
  { to: '/search', label: '기억 검색', icon: Search },
  { to: '/reviews', label: '검토함', icon: CheckSquare2 },
  { to: '/insights', label: '판단 패턴', icon: BrainCircuit },
]

export function Shell({ children }: { children: ReactNode }) {
  const { user, version, logout } = useAuth()
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()
  const admin = user?.permissions.includes('admin.access')
  return <div className="app-shell">
    <header className="mobile-header"><button className="icon-button" aria-label="메뉴 열기" onClick={() => setMobileOpen(true)}><Menu /></button><Link to="/" className="brand"><img src="/logo.svg" alt="Trace" className="brand-logo-img" /><span>TRACE</span></Link><Link to="/decisions/new" className="icon-button accent" aria-label="새 결정"><Plus /></Link></header>
    {mobileOpen && <button className="nav-scrim" aria-label="메뉴 닫기" onClick={() => setMobileOpen(false)} />}
    <aside className={`sidebar ${mobileOpen ? 'open' : ''}`}>
      <div className="sidebar-top"><Link to="/" className="brand" onClick={() => setMobileOpen(false)}><img src="/logo.svg" alt="Trace" className="brand-logo-img" /><span>TRACE</span></Link><button className="icon-button close-nav" onClick={() => setMobileOpen(false)}><X /></button></div>
      <Link to="/decisions/new" className="button primary new-decision" onClick={() => setMobileOpen(false)}><Plus size={18} />새로운 판단 남기기</Link>
      <nav className="main-nav" aria-label="주 메뉴">{nav.map(item => <NavLink key={item.to} to={item.to} end={item.end} onClick={() => setMobileOpen(false)} className={({ isActive }) => isActive ? 'active' : ''}><item.icon size={19} /><span>{item.label}</span>{item.to === '/reviews' && <span className="nav-dot" />}</NavLink>)}</nav>
      <div className="sidebar-time"><Clock3 size={16} /><div><span>NOW</span><strong>{new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric' }).format(new Date())}</strong></div></div>
      <div className="sidebar-spacer" />
      {admin && <NavLink to="/admin/settings" className={location.pathname.startsWith('/admin') ? 'admin-link active' : 'admin-link'} onClick={() => setMobileOpen(false)}><Settings2 size={18} /><span>서비스 관리</span></NavLink>}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild><button className="profile-trigger"><span className="avatar">{user?.displayName?.slice(0, 1) || 'T'}</span><span className="profile-copy"><strong>{user?.displayName}</strong><small>{user?.email}</small></span><ChevronDown size={16} /></button></DropdownMenu.Trigger>
        <DropdownMenu.Portal><DropdownMenu.Content className="profile-menu" sideOffset={8} align="start">
          <div className="profile-menu-head"><span className="avatar large">{user?.displayName?.slice(0, 1)}</span><div><strong>{user?.displayName}</strong><small>Trace {version?.version}</small></div></div>
          <DropdownMenu.Separator />
          <DropdownMenu.Item asChild><Link to="/personal"><UserRound size={17} />개인화 설정</Link></DropdownMenu.Item>
          <DropdownMenu.Item asChild><Link to="/personal?section=keys"><KeyRound size={17} />내 키 관리</Link></DropdownMenu.Item>
          <DropdownMenu.Separator />
          <DropdownMenu.Item className="danger" onSelect={() => void logout()}><LogOut size={17} />로그아웃</DropdownMenu.Item>
        </DropdownMenu.Content></DropdownMenu.Portal>
      </DropdownMenu.Root>
    </aside>
    <main className="main-content">{children}</main>
  </div>
}
