import { Activity, Check, Clock3, ExternalLink, HeartPulse, ShieldCheck, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, formatDate } from '../lib/api'
import type { Approval, PublicConfig, ReviewItem } from '../types'
import { Empty, ErrorNotice, Loading, PageHeader } from '../components/UI'

export default function ApprovalsPage() {
  const [reviews, setReviews] = useState<ReviewItem[] | null>(null)
  const [approvals, setApprovals] = useState<Approval[]>([])
  const [config, setConfig] = useState<PublicConfig | null>(null)
  const [notes, setNotes] = useState<Record<string, string>>({})
  const [error, setError] = useState('')
  const load = async () => {
    try {
      const [reviewValue, configValue] = await Promise.all([api<{ items: ReviewItem[] }>('/api/v1/reviews'), api<PublicConfig>('/api/v1/public/config')])
      setReviews(reviewValue.items); setConfig(configValue)
      if (configValue.workflow.approvalRequired) api<{ items: Approval[] }>('/api/v1/approvals').then(value => setApprovals(value.items)).catch(err => { if (err.status !== 403) setError(err.message) })
    } catch (err) { setError(err instanceof Error ? err.message : '검토함을 불러오지 못했습니다.') }
  }
  useEffect(() => { void load() }, [])
  const completeReview = async (item: ReviewItem) => { const note = notes[item.decisionId]?.trim(); if (!note) { setError('검토 메모를 남겨 주세요.'); return } const next = new Date(); next.setDate(next.getDate() + 30); try { await api(`/api/v1/decisions/${item.decisionId}/review`, { method: 'POST', body: JSON.stringify({ note, confidence: item.confidence, nextReviewAt: next.toISOString() }) }); setNotes(value => ({ ...value, [item.decisionId]: '' })); await load() } catch (err) { setError(err instanceof Error ? err.message : '검토를 완료하지 못했습니다.') } }
  const reviewApproval = async (id: string, action: 'approved' | 'rejected') => { const note = action === 'approved' ? '판단 근거와 기대를 검토했습니다.' : '보완 후 다시 검토해 주세요.'; try { await api(`/api/v1/approvals/${id}/${action}`, { method: 'POST', body: JSON.stringify({ note }) }); await load() } catch (err) { setError(err instanceof Error ? err.message : '처리하지 못했습니다.') } }
  if (reviews === null) return <Loading label="지금 다시 봐야 할 판단을 계산하는 중…" />
  return <div className="page reviews-page"><PageHeader eyebrow="REVIEW ENGINE" title="Review Inbox"><span>결과 기한·새 근거·전제 변화·고확신을 조합해 지금 볼 판단을 정렬합니다.</span></PageHeader>{error && <ErrorNotice message={error} />}
    <section className="review-summary"><div><HeartPulse /><strong>{reviews.length}</strong><span>attention needed</span></div><p>높은 점수는 실패를 뜻하지 않습니다. 지금 다시 볼 이유가 많다는 신호입니다.</p></section>
    {reviews.length === 0 ? <Empty title="지금 필요한 검토가 없습니다."><p>새 근거, 결과 예정일, 전제 변화가 생기면 이곳에 우선순위와 함께 나타납니다.</p></Empty> : <div className="review-queue">{reviews.map(item => <article key={item.decisionId} className={`review-priority health-${item.health.toLowerCase()}`}><div className="priority-score"><strong>{item.priority}</strong><span>PRIORITY</span></div><div className="review-copy"><span className="object-kind">{item.category} · {item.health.replace('_', ' ')}</span><h2>{item.title}</h2><div className="review-reasons">{item.reasons.map(reason => <span key={reason}><Activity size={13} />{reason}</span>)}</div><Link to={`/decisions/${item.decisionId}`} className="text-link">판단 전체 보기 <ExternalLink size={14} /></Link></div><div className="review-complete"><textarea value={notes[item.decisionId] || ''} onChange={event => setNotes(value => ({ ...value, [item.decisionId]: event.target.value }))} placeholder="이번 검토에서 확인한 변화" rows={2} /><button className="button secondary" onClick={() => void completeReview(item)}><Check size={16} />검토 완료 · 30일 후 다시</button></div></article>)}</div>}
    {config?.workflow.approvalRequired && <section className="manager-review"><header><div><p className="eyebrow">TEAM GOVERNANCE</p><h2>팀장 승인 대기</h2></div><ShieldCheck /></header>{approvals.length === 0 ? <p className="muted-copy">기다리는 승인 요청이 없습니다.</p> : <div className="approval-list">{approvals.map(item => <article className="approval-card" key={item.id}><div className="approval-time"><Clock3 size={17} /><span>{formatDate(item.requestedAt, true)}</span></div><div><span className="eyebrow">REQUESTED BY {item.requesterName}</span><h2>{item.decisionTitle}</h2><p>{item.requestNote || '검토 메모가 없습니다.'}</p><Link to={`/decisions/${item.decisionId}`} className="text-link">전체 판단 보기 <ExternalLink size={15} /></Link></div><div className="approval-actions"><button className="button secondary" onClick={() => void reviewApproval(item.id, 'rejected')}><X size={17} />반려</button><button className="button positive" onClick={() => void reviewApproval(item.id, 'approved')}><Check size={17} />승인</button></div></article>)}</div>}</section>}
  </div>
}
