import { Background, Controls, MarkerType, ReactFlow, type Edge, type Node, type NodeMouseHandler } from '@xyflow/react'
import { CalendarClock, Filter, Focus, Network, RotateCcw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ErrorNotice, Loading, PageHeader } from '../components/UI'
import { api, toLocalDateTime } from '../lib/api'
import type { DecisionGraph, GraphNode } from '../types'

const relationLabel: Record<string, string> = { DEPENDS_ON: '의존', CAUSED_BY: '원인', FOLLOW_UP: '후속', REPLACES: '대체', SUPPORTS: '강화', CONFLICTS_WITH: '상충', RELATED_TO: '관련' }

function positionNodes(items: GraphNode[], focusId?: string): Node[] {
  const focus = items.find(item => item.id === focusId)
  const ordered = focus ? [focus, ...items.filter(item => item.id !== focus.id)] : items
  return ordered.map((item, index) => {
    if (focus && index === 0) return { id: item.id, position: { x: 430, y: 280 }, data: { item }, className: `network-node focus health-${item.health.toLowerCase()}` }
    const count = Math.max(1, ordered.length - (focus ? 1 : 0)); const angle = (Math.PI * 2 * (index - (focus ? 1 : 0))) / count - Math.PI / 2
    const radius = item.depth > 1 ? 370 : 245
    return { id: item.id, position: { x: 430 + Math.cos(angle) * radius, y: 280 + Math.sin(angle) * radius }, data: { item }, className: `network-node depth-${item.depth} health-${item.health.toLowerCase()}` }
  })
}

function nodeLabel(item: GraphNode, zoom: number, focused: boolean) {
  if (zoom < .55) return <span className="semantic-dot" title={item.title} />
  return <div className="semantic-node"><small>{item.category.toUpperCase()} · {item.health.replace('_', ' ')}</small><strong>{item.title}</strong>{(zoom > .85 || focused) && <span>{item.confidence}% confidence{item.outcome !== undefined ? ` · outcome ${item.outcome > 0 ? '+' : ''}${item.outcome}` : ''}</span>}</div>
}

export default function GraphPage() {
  const navigate = useNavigate()
  const [graph, setGraph] = useState<DecisionGraph | null>(null)
  const [focusId, setFocusId] = useState<string>()
  const [depth, setDepth] = useState(1)
  const [category, setCategory] = useState('')
  const [at, setAt] = useState('')
  const [zoom, setZoom] = useState(.8)
  const [error, setError] = useState('')
  const load = useCallback(async (nextFocus = focusId) => {
    setError('')
    const query = new URLSearchParams({ depth: String(depth) }); if (category) query.set('category', category); if (at) query.set('at', new Date(at).toISOString())
    try { const value = await api<DecisionGraph>(nextFocus ? `/api/v1/decisions/${nextFocus}/graph?${query}` : `/api/v1/graph?${query}`); setGraph(value) } catch (err) { setError(err instanceof Error ? err.message : '그래프를 불러오지 못했습니다.') }
  }, [focusId, depth, category, at])
  useEffect(() => { void load() }, [depth, category, at])
  const nodes = useMemo(() => positionNodes(graph?.nodes || [], focusId).map(node => ({ ...node, data: { ...node.data, label: nodeLabel((node.data as { item: GraphNode }).item, zoom, node.id === focusId) } })), [graph, focusId, zoom])
  const edges = useMemo<Edge[]>(() => (graph?.edges || []).map(link => ({ id: link.id, source: link.sourceDecisionId, target: link.targetDecisionId, label: relationLabel[link.relationType] || link.relationType, className: `network-edge relation-${link.relationType.toLowerCase()}`, markerEnd: { type: MarkerType.ArrowClosed } })), [graph])
  const focusNode: NodeMouseHandler = (_, node) => { setFocusId(node.id); void load(node.id) }
  if (!graph) return <Loading label="Decision Network를 구성하는 중…" />
  return <div className="page graph-page"><PageHeader eyebrow="DECISION NETWORK" title="판단의 연결"><span>한 판단이 다음 판단에 어떤 흔적을 남겼는지 1–2 hop으로 탐색합니다.</span></PageHeader>{error && <ErrorNotice message={error} />}
    <section className="graph-toolbar"><label><Filter size={16} />Category<select value={category} onChange={event => setCategory(event.target.value)}><option value="">전체 영역</option><option value="technology">기술</option><option value="investment">투자</option><option value="project">프로젝트</option><option value="career">커리어</option><option value="personal">개인</option></select></label><label><Network size={16} />Depth<select value={depth} onChange={event => setDepth(Number(event.target.value))}><option value={1}>1-hop · Focus</option><option value={2}>2-hop · Expand</option></select></label><label><CalendarClock size={16} />Graph at<input type="datetime-local" value={at} max={toLocalDateTime()} onChange={event => setAt(event.target.value)} /></label><button className="button ghost" onClick={() => { setFocusId(undefined); setCategory(''); setAt(''); void load(undefined) }}><RotateCcw size={16} />전체로</button></section>
    <section className="network-canvas"><ReactFlow nodes={nodes} edges={edges} fitView minZoom={.25} maxZoom={1.6} onNodeClick={focusNode} onNodeDoubleClick={(_, node) => navigate(`/decisions/${node.id}`)} onMove={(_, viewport) => setZoom(viewport.zoom)} nodesDraggable={false} nodesConnectable={false} proOptions={{ hideAttribution: true }}><Background color="#303841" gap={32} size={1} /><Controls showInteractive={false} /></ReactFlow><div className="network-hint"><Focus size={15} />한 번 클릭해 Focus 이동 · 두 번 클릭해 Decision 열기 · 확대할수록 상세 표시</div></section>
  </div>
}
