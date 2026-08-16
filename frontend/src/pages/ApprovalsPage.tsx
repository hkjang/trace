import { Check, Clock3, ExternalLink, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, formatDate } from '../lib/api'
import type { Approval, PublicConfig } from '../types'
import { Empty, ErrorNotice, Loading, PageHeader } from '../components/UI'

export default function ApprovalsPage() {
  const [items, setItems] = useState<Approval[] | null>(null)
  const [config, setConfig] = useState<PublicConfig | null>(null)
  const [error, setError] = useState('')
  const load = () => api<{ items: Approval[] }>('/api/v1/approvals').then(value => setItems(value.items)).catch(err => { if (err.status === 403) setItems([]); else setError(err.message) })
  useEffect(() => { api<PublicConfig>('/api/v1/public/config').then(setConfig); void load() }, [])
  const review = async (id: string, action: 'approved' | 'rejected') => { const note = action === 'approved' ? '판단 근거와 기대를 검토했습니다.' : '보완 후 다시 검토해 주세요.'; try { await api(`/api/v1/approvals/${id}/${action}`, { method: 'POST', body: JSON.stringify({ note }) }); await load() } catch (err) { setError(err instanceof Error ? err.message : '처리하지 못했습니다.') } }
  if (items === null) return <Loading />
  return <div className="page"><PageHeader eyebrow="TEAM REVIEW" title="검토함"><span>관리자가 승인 흐름을 켠 경우에만 나타나는 판단 검토 공간입니다.</span></PageHeader>{error && <ErrorNotice message={error} />}
    {!config?.workflow.approvalRequired ? <Empty title="검토 프로세스가 꺼져 있습니다."><p>서비스 관리자가 승인 흐름을 활성화하기 전까지 판단은 별도 승인 없이 바로 시간축에 남습니다.</p></Empty> : items.length === 0 ? <Empty title="기다리는 검토가 없습니다."><p>팀원이 검토를 요청하면 이곳에 판단의 근거와 기대가 나타납니다.</p></Empty> : <div className="approval-list">{items.map(item => <article className="approval-card" key={item.id}><div className="approval-time"><Clock3 size={17} /><span>{formatDate(item.requestedAt, true)}</span></div><div><span className="eyebrow">REQUESTED BY {item.requesterName}</span><h2>{item.decisionTitle}</h2><p>{item.requestNote || '검토 메모가 없습니다.'}</p><Link to={`/decisions/${item.decisionId}`} className="text-link">전체 판단 보기 <ExternalLink size={15} /></Link></div><div className="approval-actions"><button className="button secondary" onClick={() => void review(item.id, 'rejected')}><X size={17} />반려</button><button className="button positive" onClick={() => void review(item.id, 'approved')}><Check size={17} />승인</button></div></article>)}</div>}
  </div>
}
