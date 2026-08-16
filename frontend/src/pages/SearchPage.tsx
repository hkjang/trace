import { ArrowUpRight, BrainCircuit, Search, Sparkles } from 'lucide-react'
import { type FormEvent, useState } from 'react'
import { Link } from 'react-router-dom'
import { Empty, ErrorNotice, PageHeader } from '../components/UI'
import { api, formatDate } from '../lib/api'
import type { SemanticSearchResult } from '../types'

const examples = ['내가 기술 선택에서 반대 근거를 놓친 경우', '높은 확신이었지만 결과가 나빴던 판단', 'PostgreSQL을 선택한 이유와 비슷한 결정']

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [result, setResult] = useState<SemanticSearchResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const search = async (event?: FormEvent) => { event?.preventDefault(); if (!query.trim()) return; setBusy(true); setError(''); try { setResult(await api<SemanticSearchResult>('/api/v1/search/semantic', { method: 'POST', body: JSON.stringify({ query, category, limit: 12 }) })) } catch (err) { setError(err instanceof Error ? err.message : '기억을 검색하지 못했습니다.') } finally { setBusy(false) } }
  return <div className="page search-page"><PageHeader eyebrow="SEARCH & MEMORY" title="과거의 판단을 질문하세요"><span>제목이 아니라 이유·근거·회고의 의미를 따라 유사한 기억을 찾습니다.</span></PageHeader>{error && <ErrorNotice message={error} />}
    <form className="memory-search" onSubmit={search}><Search size={23} /><input value={query} onChange={event => setQuery(event.target.value)} placeholder="예: 내가 AI 투자에서 너무 낙관적으로 판단했던 경우" autoFocus /><select value={category} onChange={event => setCategory(event.target.value)} aria-label="카테고리"><option value="">모든 영역</option><option value="technology">기술</option><option value="investment">투자</option><option value="project">프로젝트</option><option value="career">커리어</option></select><button className="button primary" disabled={busy}>{busy ? '기억을 찾는 중…' : '검색'}</button></form>
    {!result && <section className="search-examples"><p className="eyebrow">TRY A MEMORY QUESTION</p>{examples.map(example => <button key={example} onClick={() => setQuery(example)}><Sparkles size={15} />{example}</button>)}</section>}
    {result && <><div className="search-result-head"><div><BrainCircuit size={17} /><strong>{result.items.length}개의 Decision Memory</strong></div><span>{result.fallback ? 'OFFLINE LOCAL VECTOR' : result.model}</span></div>{result.items.length === 0 ? <Empty title="연결되는 판단을 찾지 못했습니다."><p>표현을 조금 넓히거나 카테고리 필터를 해제해 보세요.</p></Empty> : <div className="memory-results">{result.items.map(item => <article key={item.decision.id}><div className="memory-score"><strong>{Math.round(item.contextScore * 100)}</strong><span>CONTEXT</span></div><div><span className="object-kind">{item.decision.category} · {formatDate(item.decision.decidedAt)}</span><h2>{item.decision.title}</h2><p>{item.matchedExcerpt}</p><div className="memory-reasons">{item.reasons.map(reason => <span key={reason}>{reason}</span>)}</div></div><Link to={`/decisions/${item.decision.id}`} aria-label={`${item.decision.title} 열기`}><ArrowUpRight /></Link></article>)}</div>}</>}
  </div>
}
