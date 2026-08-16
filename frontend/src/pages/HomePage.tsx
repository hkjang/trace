import { ArrowUpRight, CalendarClock, CircleDotDashed, HeartPulse, Plus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, formatDate } from '../lib/api'
import type { Dashboard, Decision } from '../types'
import { Empty, ErrorNotice, Loading, PageHeader } from '../components/UI'

export default function HomePage() {
  const [data, setData] = useState<Dashboard | null>(null)
  const [focus, setFocus] = useState<Decision | null>(null)
  const [error, setError] = useState('')
  useEffect(() => { api<Dashboard>('/api/v1/dashboard').then(value => { setData(value); setFocus(value.recent[0] || null) }).catch(err => setError(err.message)) }, [])
  if (error) return <div className="page"><ErrorNotice message={error} /></div>
  if (!data) return <Loading label="판단의 시간축을 불러오는 중…" />
  return <div className="page home-page">
    <PageHeader eyebrow="YOUR DECISION MEMORY" title="시간의 흐름"><span>{formatDate(new Date().toISOString())} · 지금 추적 중인 판단을 중심으로 보여줍니다.</span></PageHeader>
    {data.recent.length === 0 ? <Empty title="당신의 결정은 생각보다 빨리 사라집니다." action={<Link className="button primary" to="/decisions/new"><Plus size={18} />기억할 첫 판단 남기기</Link>}><p>근거와 기대를 지금 남겨, 미래의 내가 당시의 판단을 정직하게 다시 볼 수 있게 하세요.</p></Empty> : <>
      <div className="status-ribbon" aria-label="판단 현황"><span><strong>{data.activeCount}</strong>진행 중</span><span><strong>{data.waitingCount}</strong>승인 대기</span><span><strong>{data.reviewDue}</strong>회고 필요</span><span><strong>{data.closedCount}</strong>완료</span></div>
      {!!data.reviewInbox?.length && <section className="home-review-inbox"><header><div><p className="eyebrow">TODAY</p><h2><HeartPulse size={19} />{data.reviewInbox.length}개의 판단이 주의를 기다립니다</h2></div><Link to="/reviews" className="text-link">Review Inbox <ArrowUpRight size={15} /></Link></header><div>{data.reviewInbox.map(item => <Link key={item.decisionId} to={`/decisions/${item.decisionId}`}><span className={`health-light ${item.health.toLowerCase()}`} /><strong>{item.title}</strong><small>{item.reasons[0]}</small><b>{item.priority}</b></Link>)}</div></section>}
      <section className="focus-stage">
        <div className="stage-date"><span>{new Date(focus!.decidedAt).getFullYear()}</span><strong>{new Intl.DateTimeFormat('en', { month: 'short', day: '2-digit' }).format(new Date(focus!.decidedAt)).toUpperCase()}</strong></div>
        <div className="focus-threads" aria-hidden="true"><span className="thread left" /><span className="thread right dotted" /></div>
        <article className="focus-card">
          <div className="focus-halo" /><span className="object-kind">{focus!.category.toUpperCase()} DECISION</span><h2>{focus!.title}</h2><p>{focus!.decision}</p>
          <div className="confidence-ring" style={{ '--confidence': `${focus!.confidence * 3.6}deg` } as React.CSSProperties}><div><strong>{focus!.confidence}</strong><span>%</span></div></div>
          <div className="focus-meta"><span>CONFIDENCE</span><span>{focus!.status === 'active' ? 'ACTIVE' : focus!.status.toUpperCase()}</span></div>
          <Link className="focus-link" to={`/decisions/${focus!.id}`}>판단 속으로 들어가기 <ArrowUpRight size={18} /></Link>
        </article>
        <div className="stage-side past-object"><span>PAST EVIDENCE</span><strong>당시의 근거</strong><small>{focus!.reason ? 'Reason recorded' : 'No reason yet'}</small></div>
        <div className="stage-side future-object"><span>EXPECTED</span><strong>예상된 결과</strong><small>{focus!.reviewAt ? formatDate(focus!.reviewAt) : 'Open future'}</small></div>
      </section>
      <section className="home-timeline"><div className="timeline-head"><div><p className="eyebrow">RECENT TRACES</p><h2>최근 판단</h2></div><Link to="/decisions/new" className="text-link"><Plus size={16} />새 판단</Link></div>
        <div className="decision-strip">{data.recent.map((item, index) => <button key={item.id} onClick={() => setFocus(item)} className={focus?.id === item.id ? 'decision-tick selected' : 'decision-tick'}><span className="tick-line" /><span className="tick-marker">D{String(index + 1).padStart(2, '0')}</span><strong>{item.title}</strong><small>{formatDate(item.decidedAt)}</small>{item.reviewAt && new Date(item.reviewAt) <= new Date() && <CalendarClock size={15} />}</button>)}</div>
        <div className="timeline-axis"><span>PAST</span><div><i /><i /><i /><i /><i /></div><span>NOW <CircleDotDashed size={14} /></span></div>
      </section>
    </>}
  </div>
}
