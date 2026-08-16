import { Navigate, Outlet, Route, Routes, useLocation } from 'react-router-dom'
import { lazy, Suspense, type ReactNode } from 'react'
import { useAuth } from './lib/auth'
import { Shell } from './components/Shell'
import LoginPage from './pages/LoginPage'

const HomePage = lazy(() => import('./pages/HomePage'))
const NewDecisionPage = lazy(() => import('./pages/NewDecisionPage'))
const DecisionPage = lazy(() => import('./pages/DecisionPage'))
const ApprovalsPage = lazy(() => import('./pages/ApprovalsPage'))
const InsightsPage = lazy(() => import('./pages/InsightsPage'))
const PersonalPage = lazy(() => import('./pages/PersonalPage'))
const AdminPage = lazy(() => import('./pages/AdminPage'))
const NotFoundPage = lazy(() => import('./pages/NotFoundPage'))

function Deferred({ children }: { children: ReactNode }) { return <Suspense fallback={<div className="loading-state">시간의 층을 여는 중…</div>}>{children}</Suspense> }

function Protected() {
  const { user, loading } = useAuth()
  const location = useLocation()
  if (loading) return <div className="boot-screen"><span className="trace-mark">T</span><span>시간의 층을 불러오는 중…</span></div>
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Shell><Outlet /></Shell>
}

export default function App() {
  return <Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route element={<Protected />}>
      <Route path="/" element={<Deferred><HomePage /></Deferred>} />
      <Route path="/decisions/new" element={<Deferred><NewDecisionPage /></Deferred>} />
      <Route path="/decisions/:id" element={<Deferred><DecisionPage /></Deferred>} />
      <Route path="/reviews" element={<Deferred><ApprovalsPage /></Deferred>} />
      <Route path="/insights" element={<Deferred><InsightsPage /></Deferred>} />
      <Route path="/personal" element={<Deferred><PersonalPage /></Deferred>} />
      <Route path="/admin/*" element={<Deferred><AdminPage /></Deferred>} />
      <Route path="*" element={<Deferred><NotFoundPage /></Deferred>} />
    </Route>
  </Routes>
}
