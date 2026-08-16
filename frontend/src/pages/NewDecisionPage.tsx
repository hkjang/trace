import { ArrowLeft, ArrowRight, BrainCircuit, Check, Plus, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, toLocalDateTime } from '../lib/api'
import { ErrorNotice } from '../components/UI'
import type { Decision, SemanticSearchResult, Team } from '../types'

const steps = [
  { key: 'decision', eyebrow: '01 · DECISION', title: '무엇을 결정했나요?', hint: '결론부터 짧고 분명하게 남겨보세요.' },
  { key: 'why', eyebrow: '02 · WHY', title: '왜 이 선택이 옳다고 느꼈나요?', hint: '당시의 논리와 숨은 전제를 미래의 나에게 설명합니다.' },
  { key: 'evidence', eyebrow: '03 · EVIDENCE', title: '어떤 정보를 근거로 했나요?', hint: '중요한 것은 문서 날짜가 아니라 내가 이 정보를 알게 된 날짜입니다.' },
  { key: 'expectation', eyebrow: '04 · EXPECTATION', title: '무엇이 일어날 것으로 예상하나요?', hint: '성공 기준과 결정이 틀렸음을 알려줄 조건도 함께 남기세요.' },
  { key: 'confidence', eyebrow: '05 · CONFIDENCE', title: '얼마나 확신하나요?', hint: '정답이 아니라 당시의 믿음을 기록하는 숫자입니다.' },
]

function confidenceLabel(value: number) { return value <= 20 ? 'Very uncertain' : value <= 40 ? 'Uncertain' : value <= 60 ? 'Leaning' : value <= 80 ? 'Confident' : 'Highly confident' }

export default function NewDecisionPage() {
  const [step, setStep] = useState(0)
  const [form, setForm] = useState({ title: '', decision: '', category: 'technology', teamId: '', reason: '', assumptions: '', evidenceTitle: '', evidenceContent: '', evidenceReliability: 60, evidenceStance: 'support', knownAt: toLocalDateTime(), expectation: '', successCriteria: '', invalidationConditions: '', confidence: 72, decidedAt: toLocalDateTime(), reviewAt: '', alternatives: [] as string[] })
  const [teams, setTeams] = useState<Team[]>([])
  const [alternative, setAlternative] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [memoryBusy, setMemoryBusy] = useState(false)
  const [similar, setSimilar] = useState<SemanticSearchResult | null>(null)
  const navigate = useNavigate()
  const current = steps[step]
  useEffect(() => { api<{ items: Team[] }>('/api/v1/teams').then(value => setTeams(value.items)).catch(() => undefined) }, [])
  useEffect(() => {
    if (step !== 4 || !form.title.trim() || !form.decision.trim()) return
    let active = true
    setMemoryBusy(true)
    api<SemanticSearchResult>('/api/v1/search/semantic', {
      method: 'POST',
      body: JSON.stringify({ query: [form.title, form.decision, form.reason, form.assumptions].filter(Boolean).join('\n'), category: form.category, limit: 3 }),
    }).then(value => { if (active) setSimilar(value) }).catch(() => { if (active) setSimilar(null) }).finally(() => { if (active) setMemoryBusy(false) })
    return () => { active = false }
  }, [step, form.title, form.decision, form.reason, form.assumptions, form.category])
  const valid = useMemo(() => [form.title.trim() && form.decision.trim(), form.reason.trim(), form.evidenceTitle.trim(), form.expectation.trim(), true][step], [form, step])
  const update = (key: string, value: unknown) => setForm(prev => ({ ...prev, [key]: value }))
  const addAlternative = () => { if (alternative.trim()) { update('alternatives', [...form.alternatives, alternative.trim()]); setAlternative('') } }
  const submit = async () => {
    setBusy(true); setError('')
    try {
      const item = await api<Decision>('/api/v1/decisions', { method: 'POST', body: JSON.stringify({ title: form.title, decision: form.decision, category: form.category, teamId: form.teamId || null, reason: form.reason, assumptions: form.assumptions, invalidationConditions: form.invalidationConditions, confidence: form.confidence, status: 'active', decidedAt: new Date(form.decidedAt).toISOString(), reviewAt: form.reviewAt ? new Date(form.reviewAt).toISOString() : null, alternatives: form.alternatives.map(title => ({ title, description: '' })), evidence: [{ title: form.evidenceTitle, type: 'note', source: '', content: form.evidenceContent, snapshot: '', reliability: form.evidenceReliability, stance: form.evidenceStance, publishedAt: null, knownAt: new Date(form.knownAt).toISOString() }], expectation: { expectation: form.expectation, successCriteria: form.successCriteria, expectedAt: form.reviewAt ? new Date(form.reviewAt).toISOString() : null, probability: form.confidence } }) })
      navigate(`/decisions/${item.id}`)
    } catch (err) { setError(err instanceof Error ? err.message : '판단을 저장하지 못했습니다.') }
    finally { setBusy(false) }
  }
  return <div className="thought-page">
    <header className="thought-header"><button className="text-link" onClick={() => step ? setStep(step - 1) : navigate(-1)}><ArrowLeft size={17} />{step ? '이전 질문' : '나가기'}</button><div className="step-dots">{steps.map((_, index) => <span key={index} className={index === step ? 'active' : index < step ? 'done' : ''}>{index < step && <Check size={11} />}</span>)}</div><span className="mono">{step + 1} / {steps.length}</span></header>
    <main className="thought-stage">
      <div className="question-block"><p className="eyebrow">{current.eyebrow}</p><h1>{current.title}</h1><p>{current.hint}</p></div>
      {error && <ErrorNotice message={error} />}
      <div className="answer-block">
        {step === 0 && <><input className="thought-title" value={form.title} onChange={e => update('title', e.target.value)} placeholder="판단의 제목" autoFocus /><textarea className="thought-input" value={form.decision} onChange={e => update('decision', e.target.value)} placeholder="PostgreSQL을 메인 데이터베이스로 사용한다" rows={3} /><div className="inline-fields"><label>분류<select value={form.category} onChange={e => update('category', e.target.value)}><option value="technology">기술</option><option value="investment">투자</option><option value="project">프로젝트</option><option value="career">커리어</option><option value="personal">개인</option><option value="other">기타</option></select></label><label>결정 시점<input type="datetime-local" value={form.decidedAt} onChange={e => update('decidedAt', e.target.value)} /></label></div>{teams.length > 0 && <label>팀 (승인 흐름을 사용할 때 팀장을 결정합니다)<select value={form.teamId} onChange={e => update('teamId', e.target.value)}><option value="">개인 판단</option>{teams.map(team => <option value={team.id} key={team.id}>{team.name} · 팀장 {team.managerName || '미지정'}</option>)}</select></label>}</>}
        {step === 1 && <><textarea className="thought-input" value={form.reason} onChange={e => update('reason', e.target.value)} placeholder="개발 생산성, 운영 경험, JSON과 Vector 확장성을 한곳에서 가져갈 수 있다…" rows={5} autoFocus /><details className="progressive"><summary>숨은 전제와 대안도 남기기</summary><label>이 판단이 의존하는 전제<textarea value={form.assumptions} onChange={e => update('assumptions', e.target.value)} rows={3} /></label><div className="alternative-input"><input value={alternative} onChange={e => setAlternative(e.target.value)} placeholder="선택하지 않은 대안" onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); addAlternative() } }} /><button className="icon-button" onClick={addAlternative}><Plus /></button></div><div className="chip-row">{form.alternatives.map((item, index) => <span className="chip" key={item}>{item}<button onClick={() => update('alternatives', form.alternatives.filter((_, i) => i !== index))}><X size={13} /></button></span>)}</div></details></>}
        {step === 2 && <><input className="thought-title" value={form.evidenceTitle} onChange={e => update('evidenceTitle', e.target.value)} placeholder="근거의 제목" autoFocus /><textarea className="thought-input" value={form.evidenceContent} onChange={e => update('evidenceContent', e.target.value)} placeholder="이 근거에서 중요했던 내용" rows={4} /><div className="inline-fields"><label>이 근거의 신뢰도 <strong>{form.evidenceReliability}%</strong><input type="range" min="0" max="100" value={form.evidenceReliability} onChange={e => update('evidenceReliability', Number(e.target.value))} /></label><label>판단에 대한 방향<select value={form.evidenceStance} onChange={e => update('evidenceStance', e.target.value)}><option value="support">지지</option><option value="neutral">중립</option><option value="against">반대</option></select></label></div><label className="known-at">YOU KNEW THIS <input type="datetime-local" value={form.knownAt} onChange={e => update('knownAt', e.target.value)} /></label></>}
        {step === 3 && <><textarea className="thought-input" value={form.expectation} onChange={e => update('expectation', e.target.value)} placeholder="개발 생산성이 30% 개선될 것이다" rows={3} autoFocus /><label>성공했다고 판단할 기준<textarea value={form.successCriteria} onChange={e => update('successCriteria', e.target.value)} rows={2} /></label><label className="invalidation">이 결정이 틀렸다는 것을 알려줄 정보<textarea value={form.invalidationConditions} onChange={e => update('invalidationConditions', e.target.value)} placeholder="운영 비용이 기준을 초과하거나 요구 성능을 충족하지 못할 때" rows={3} /></label><label>검토할 시점<input type="datetime-local" value={form.reviewAt} onChange={e => update('reviewAt', e.target.value)} /></label></>}
        {step === 4 && <><div className="confidence-control"><div className="confidence-number"><strong>{form.confidence}</strong><span>%</span></div><input aria-label="확신 수준" type="range" min="0" max="100" value={form.confidence} onChange={e => update('confidence', Number(e.target.value))} style={{ '--value': `${form.confidence}%` } as React.CSSProperties} /><div className="range-labels"><span>UNCERTAIN</span><strong>{confidenceLabel(form.confidence)}</strong><span>CERTAIN</span></div><blockquote>“미래의 나는 당신이 얼마나 확신했는지 기억할 수 있습니다.”</blockquote></div><aside className="capture-memory"><header><BrainCircuit size={16} /><div><strong>과거의 나와 비교하기</strong><small>{memoryBusy ? '유사한 판단을 찾는 중…' : similar?.fallback ? '오프라인 메모리 검색' : similar?.model}</small></div></header>{!memoryBusy && !similar?.items.length && <p>같은 분류에서 비교할 만한 과거 판단이 아직 없습니다.</p>}{similar?.items.map(item => <article key={item.decision.id}><span>{Math.round(item.contextScore * 100)}%</span><div><strong>{item.decision.title}</strong><small>{item.matchedExcerpt}</small></div></article>)}</aside></>}
      </div>
      <button className="button primary thought-next" disabled={!valid || busy} onClick={() => step < steps.length - 1 ? setStep(step + 1) : void submit()}>{step < steps.length - 1 ? <>다음 질문 <ArrowRight size={18} /></> : <>{busy ? '시간축에 남기는 중…' : '이 판단을 시간축에 남기기'} <Check size={18} /></>}</button>
    </main>
  </div>
}
