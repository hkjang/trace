import { Background, Controls, MarkerType, ReactFlow, type Edge, type Node } from '@xyflow/react'
import { ArrowLeft, BrainCircuit, Clock3, Eye, GitBranch, Layers3, MessageSquareText, Pause, Play, Plus, RotateCcw, Send, Sparkles } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, formatDate } from '../lib/api'
import type { Decision, PublicConfig } from '../types'
import { ErrorNotice, Loading } from '../components/UI'
import { DecisionIntelligencePanel } from '../components/DecisionIntelligencePanel'

function graphData(decision: Decision): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = [{ id: 'decision', position: { x: 390, y: 190 }, data: { label: decision.title }, className: 'flow-decision' }]
  const edges: Edge[] = []
  const add = (id: string, label: string, x: number, y: number, className: string, dotted = false) => {
    nodes.push({ id, position: { x, y }, data: { label }, className })
    edges.push({ id: `e-${id}`, source: id.startsWith('ev') ? id : 'decision', target: id.startsWith('ev') ? 'decision' : id, className: dotted ? 'dotted-edge' : '', markerEnd: dotted ? undefined : { type: MarkerType.ArrowClosed }, animated: className === 'flow-outcome' })
  }
  ;(decision.evidence || []).slice(0, 2).forEach((item, i) => add(`ev-${i}`, item.title, 65, 90 + i * 185, 'flow-evidence'))
  ;(decision.alternatives || []).slice(0, 2).forEach((item, i) => add(`alt-${i}`, item.title, 710, 60 + i * 260, 'flow-alternative', true))
  if (decision.expectations?.[0]) add('expectation', decision.expectations[0].expectation, 680, 190, 'flow-expectation', true)
  if (decision.outcomes?.[0]) add('outcome', decision.outcomes[0].result, 390, 410, 'flow-outcome')
  return { nodes, edges }
}

export default function DecisionPage() {
  const { id } = useParams()
  const [original, setOriginal] = useState<Decision | null>(null)
  const [decision, setDecision] = useState<Decision | null>(null)
  const [config, setConfig] = useState<PublicConfig | null>(null)
  const [error, setError] = useState('')
  const [replayAt, setReplayAt] = useState<number | null>(null)
  const [aiText, setAIText] = useState('')
  const [aiBusy, setAIBusy] = useState(false)
  const [approvalBusy, setApprovalBusy] = useState(false)
  const [showEvidence, setShowEvidence] = useState(false)
  const [evidenceForm,setEvidenceForm]=useState({title:'',content:'',stance:'neutral',reliability:50})
  const [outcomeForm,setOutcomeForm]=useState({result:'',outcomeScore:0,decisionQuality:0})
  const [reflectionForm,setReflectionForm]=useState({reflection:'',learning:'',reasoningStillSound:true})
  const replayTimer = useRef<number | undefined>(undefined)
  const load = () => api<Decision>(`/api/v1/decisions/${id}`).then(value => { setOriginal(value); setDecision(value); setReplayAt(new Date().getTime()) }).catch(err => setError(err.message))
  useEffect(() => { void load(); api<PublicConfig>('/api/v1/public/config').then(setConfig).catch(() => undefined) }, [id])
  const minTime = original ? new Date(original.decidedAt).getTime() : Date.now()
  const maxTime = useMemo(() => original ? Math.max(Date.now(), ...(original.events || []).map(event => new Date(event.knownAt).getTime())) : Date.now(), [original])
  const isReplay = replayAt !== null && replayAt < maxTime - 60_000
  const changeReplay = (value: number) => {
    setReplayAt(value); window.clearTimeout(replayTimer.current)
    replayTimer.current = window.setTimeout(() => {
      if (value >= maxTime - 60_000) { setDecision(original); return }
      api<{ decision: Decision }>(`/api/v1/decisions/${id}/replay?at=${encodeURIComponent(new Date(value).toISOString())}`).then(result => setDecision(result.decision)).catch(err => setError(err.message))
    }, 180)
  }
  const runAI = async (mode: 'review' | 'counter' | 'counterfactual' | 'assumption' | 'replay') => {
    if (!decision || aiBusy) return
    setAIText(''); setAIBusy(true); setError('')
    try {
      const response = await fetch(`/api/v1/decisions/${id}/ai/stream`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Trace-Request': '1' }, body: JSON.stringify({ mode: isReplay ? 'replay' : mode, replayAt: isReplay ? new Date(replayAt!).toISOString() : null, prompt: '' }) })
      if (!response.ok || !response.body) { const value = await response.json().catch(() => null); throw new Error(value?.error?.message || 'AI 스트림을 시작하지 못했습니다.') }
      const reader = response.body.getReader(); const decoder = new TextDecoder(); let buffer = ''
      while (true) { const { value, done } = await reader.read(); if (done) break; buffer += decoder.decode(value, { stream: true }); const chunks = buffer.split('\n\n'); buffer = chunks.pop() || ''; for (const chunk of chunks) { const event = chunk.match(/^event: (.+)$/m)?.[1]; const data = chunk.match(/^data: (.+)$/m)?.[1]; if (event === 'delta' && data) { const parsed = JSON.parse(data) as { text: string }; setAIText(text => text + parsed.text) } } }
    } catch (err) { setError(err instanceof Error ? err.message : 'AI 분석 중 오류가 발생했습니다.') }
    finally { setAIBusy(false) }
  }
  const submitApproval = async () => { setApprovalBusy(true); try { await api(`/api/v1/decisions/${id}/approval`, { method: 'POST', body: JSON.stringify({ note: '검토를 요청합니다.' }) }); await load() } catch (err) { setError(err instanceof Error ? err.message : '검토를 요청하지 못했습니다.') } finally { setApprovalBusy(false) } }
  const addEvidence=async(event:FormEvent)=>{event.preventDefault();try{await api(`/api/v1/decisions/${id}/evidence`,{method:'POST',body:JSON.stringify({title:evidenceForm.title,type:'note',source:'',content:evidenceForm.content,snapshot:'',reliability:evidenceForm.reliability,stance:evidenceForm.stance,publishedAt:null,knownAt:new Date().toISOString()})});setEvidenceForm({title:'',content:'',stance:'neutral',reliability:50});setShowEvidence(false);await load()}catch(err){setError(err instanceof Error?err.message:'근거를 추가하지 못했습니다.')}}
  const addOutcome=async(event:FormEvent)=>{event.preventDefault();try{await api(`/api/v1/decisions/${id}/outcomes`,{method:'POST',body:JSON.stringify({...outcomeForm,outcomeAt:new Date().toISOString()})});await load()}catch(err){setError(err instanceof Error?err.message:'결과를 기록하지 못했습니다.')}}
  const addReflection=async(event:FormEvent)=>{event.preventDefault();try{await api(`/api/v1/decisions/${id}/reflections`,{method:'POST',body:JSON.stringify(reflectionForm)});await load()}catch(err){setError(err instanceof Error?err.message:'회고를 기록하지 못했습니다.')}}
  if (error && !decision) return <div className="page"><ErrorNotice message={error} /></div>
  if (!decision || !original) return <Loading label="판단의 흔적을 복원하는 중…" />
  const graph = graphData(decision)
  const hiddenCount = isReplay ? (original.evidence?.length || 0) - (decision.evidence?.length || 0) + (original.outcomes?.length || 0) - (decision.outcomes?.length || 0) + (original.reflections?.length || 0) - (decision.reflections?.length || 0) : 0
  const outcome = decision.outcomes?.at(-1)
  const initialConfidence = decision.confidenceHistory?.[0]?.confidence ?? decision.confidence
  return <div className={isReplay ? 'decision-page replay-mode' : 'decision-page'}>
    <header className="decision-topbar"><Link to="/" className="text-link"><ArrowLeft size={17} />시간의 흐름</Link><div className="mode-indicator"><span className={isReplay ? 'replay-light active' : 'replay-light'} />{isReplay ? `REPLAY · ${formatDate(new Date(replayAt!).toISOString(), true)}` : 'LIVE TRACE'}</div><div className="top-actions"><span className={`health-badge ${(decision.health || 'HEALTHY').toLowerCase()}`}>{decision.health || 'HEALTHY'}</span>{config?.workflow.approvalRequired && ['draft', 'rejected'].includes(decision.workflowState) && <button className="button amber" disabled={approvalBusy} onClick={() => void submitApproval()}><Send size={16} />팀장 검토 요청</button>}<span className={`state-label ${decision.workflowState}`}>{decision.workflowState === 'not_required' ? decision.status : decision.workflowState}</span></div></header>
    {error && <div className="floating-error"><ErrorNotice message={error} /></div>}
    <div className="decision-workspace">
      <section className="graph-canvas" aria-label="판단 관계 그래프"><ReactFlow nodes={graph.nodes} edges={graph.edges} fitView minZoom={0.55} maxZoom={1.5} proOptions={{ hideAttribution: true }} nodesDraggable={false} nodesConnectable={false}><Background color="#343a40" gap={28} size={1} /><Controls showInteractive={false} /></ReactFlow><div className="graph-legend"><span><i className="line solid" />사실</span><span><i className="line dotted" />예상</span><span><i className="purple-dot" />Trace 해석</span></div></section>
      <aside className="decision-detail custom-scroll"><div className="detail-head"><span className="object-kind">{decision.category.toUpperCase()} DECISION</span><h1>{decision.title}</h1><p>{decision.decision}</p><div className="detail-date mono">{formatDate(decision.decidedAt, true).toUpperCase()}</div></div>
        <div className="detail-confidence"><div><span>이 시점의 확신</span><strong>{decision.confidence}%</strong></div><div className="mini-bar"><i style={{ width: `${decision.confidence}%` }} /></div><small>{decision.confidence > 80 ? 'Highly confident' : decision.confidence > 60 ? 'Confident' : 'Leaning'}</small></div>
        <DetailSection title="왜 이렇게 판단했나" icon={<MessageSquareText size={17} />}><p>{decision.reason || '아직 이유가 기록되지 않았습니다.'}</p>{decision.assumptions && <div className="subtle-box"><span>ASSUMPTIONS</span>{decision.assumptions}</div>}</DetailSection>
        <DetailSection title="당시 알고 있던 근거" count={decision.evidence?.length} icon={<Eye size={17} />}>{decision.evidence?.map(item => <article className={`evidence-item ${item.stance}`} key={item.id}><div><strong>{item.title}</strong><span>{item.stance} · {item.reliability === undefined ? 'UNKNOWN' : item.reliability >= 75 ? 'HIGH' : item.reliability >= 40 ? 'MEDIUM' : 'LOW'}</span></div><p>{item.content}</p><small>YOU KNEW THIS · {formatDate(item.knownAt)}</small></article>)}{isReplay && hiddenCount > 0 && <div className="ghost-card"><Layers3 size={17} /><span>이후에 추가된 정보 {hiddenCount}개</span><small>내용은 Replay에서 숨겨집니다</small></div>}{!isReplay&&<>{showEvidence?<form className="inline-capture" onSubmit={addEvidence}><input value={evidenceForm.title} onChange={e=>setEvidenceForm({...evidenceForm,title:e.target.value})} placeholder="새 근거 제목" required/><textarea value={evidenceForm.content} onChange={e=>setEvidenceForm({...evidenceForm,content:e.target.value})} placeholder="무엇을 알게 되었나요?" rows={2}/><div className="form-row"><select value={evidenceForm.stance} onChange={e=>setEvidenceForm({...evidenceForm,stance:e.target.value})}><option value="support">지지</option><option value="neutral">중립</option><option value="against">반대</option></select><select value={evidenceForm.reliability} onChange={e=>setEvidenceForm({...evidenceForm,reliability:Number(e.target.value)})}><option value={90}>High · 공식/직접 데이터</option><option value={60}>Medium · 신뢰 가능한 2차 자료</option><option value={25}>Low · 의견/커뮤니티</option><option value={50}>Unknown</option></select></div><button className="button secondary">근거 추가</button></form>:<button className="text-link add-inline" onClick={()=>setShowEvidence(true)}><Plus size={15}/>지금 알게 된 근거 추가</button>}</>}</DetailSection>
        <DetailSection title="기대한 미래" icon={<GitBranch size={17} />}>{decision.expectations?.map(item => <div className="expectation-item" key={item.id}><p>{item.expectation}</p><small>{item.successCriteria}</small></div>)}{decision.invalidationConditions && <div className="invalidation-view"><span>INVALIDATION SIGNAL</span><p>{decision.invalidationConditions}</p></div>}</DetailSection>
        {!isReplay&&!outcome&&<form className="outcome-capture" onSubmit={addOutcome}><p className="eyebrow">REALITY</p><h2>실제로 무엇이 일어났나요?</h2><textarea value={outcomeForm.result} onChange={e=>setOutcomeForm({...outcomeForm,result:e.target.value})} rows={3} required/><label>예상보다 좋거나 나빴나요?<select value={outcomeForm.outcomeScore} onChange={e=>setOutcomeForm({...outcomeForm,outcomeScore:Number(e.target.value)})}><option value={-2}>-2 · Much worse</option><option value={-1}>-1 · Worse</option><option value={0}>0 · Neutral</option><option value={1}>+1 · Better</option><option value={2}>+2 · Much better</option></select></label><label>당시 판단 품질<select value={outcomeForm.decisionQuality} onChange={e=>setOutcomeForm({...outcomeForm,decisionQuality:Number(e.target.value)})}><option value={-2}>-2 · Poor</option><option value={-1}>-1 · Weak</option><option value={0}>0 · Fair</option><option value={1}>+1 · Sound</option><option value={2}>+2 · Strong</option></select></label><button className="button secondary">결과 남기기</button></form>}
        {outcome && <section className="then-now"><div><span>THEN</span><strong>{initialConfidence}%</strong><p>{decision.expectations?.[0]?.expectation}</p></div><div className="versus">VS</div><div><span>NOW</span><strong className={outcome.outcomeScore < 0 ? 'negative' : 'positive'}>{outcome.outcomeScore > 0 ? '+' : ''}{outcome.outcomeScore}</strong><p>{outcome.result}</p></div></section>}
        {!isReplay&&outcome&&!decision.reflections?.length&&<form className="outcome-capture reflection" onSubmit={addReflection}><p className="eyebrow">REFLECTION</p><h2>Then과 Now 사이에서 무엇을 배웠나요?</h2><textarea value={reflectionForm.reflection} onChange={e=>setReflectionForm({...reflectionForm,reflection:e.target.value})} rows={3} required/><textarea value={reflectionForm.learning} onChange={e=>setReflectionForm({...reflectionForm,learning:e.target.value})} placeholder="다음 판단에 적용할 교훈" rows={2}/><label>원래의 논리는 여전히 타당했나요?<select value={String(reflectionForm.reasoningStillSound)} onChange={e=>setReflectionForm({...reflectionForm,reasoningStillSound:e.target.value==='true'})}><option value="true">Yes</option><option value="false">No</option></select></label><button className="button secondary">회고 남기기</button></form>}
        <DecisionIntelligencePanel id={id!} decision={decision} original={original} isReplay={isReplay} replayAt={replayAt} onReload={async () => { await load() }} />
        <section className="ai-layer"><div className="ai-head"><div><Sparkles size={18} /><span>TRACE INSIGHT</span></div><BrainCircuit size={20} /></div>{aiText ? <div className="ai-output">{aiText}</div> : <p>내 판단을 먼저 읽은 뒤, Trace의 보라색 해석 레이어를 열어보세요.</p>}<div className="ai-actions"><button className="button purple" disabled={aiBusy} onClick={() => void runAI('review')}>{aiBusy ? <Pause size={16} /> : <Play size={16} />}{isReplay ? '당시 정보만으로 평가' : '판단 품질 분석'}</button><button className="button ghost" disabled={aiBusy} onClick={() => void runAI('counter')}>Devil's Advocate</button><button className="button ghost" disabled={aiBusy} onClick={() => void runAI('counterfactual')}>대안 시나리오</button><button className="button ghost" disabled={aiBusy} onClick={() => void runAI('assumption')}>전제 점검</button></div></section>
      </aside>
    </div>
    <section className="replay-dock"><div className="replay-title"><button className={isReplay ? 'icon-button active' : 'icon-button'} onClick={() => changeReplay(isReplay ? maxTime : minTime)}>{isReplay ? <RotateCcw /> : <Clock3 />}</button><div><strong>DECISION REPLAY</strong><span>{isReplay ? '이 날짜에 존재했던 정보만 표시 중' : '슬라이더를 움직여 과거의 판단으로 들어가세요'}</span></div></div><div className="replay-slider"><div className="marker-row">{(original.events || []).map(event => <i key={event.id} title={`${event.eventType} · ${formatDate(event.knownAt)}`} style={{ left: `${Math.max(0, Math.min(100, (new Date(event.knownAt).getTime() - minTime) / Math.max(1, maxTime - minTime) * 100))}%` }}><span>{event.eventType.includes('evidence') ? 'E' : event.eventType.includes('outcome') ? 'O' : 'D'}</span></i>)}</div><input type="range" min={minTime} max={maxTime} value={replayAt || maxTime} onChange={event => changeReplay(Number(event.target.value))} /><div className="slider-labels"><span>{formatDate(new Date(minTime).toISOString())}</span><strong>{formatDate(new Date(replayAt || maxTime).toISOString(), true)}</strong><span>NOW</span></div></div></section>
  </div>
}

function DetailSection({ title, icon, count, children }: { title: string; icon: React.ReactNode; count?: number; children: React.ReactNode }) { return <section className="detail-section"><h2>{icon}{title}{count !== undefined && <span>{count}</span>}</h2>{children}</section> }
