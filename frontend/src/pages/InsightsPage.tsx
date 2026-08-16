import { BrainCircuit, Compass, Scale, Sparkles, Target } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'
import type { Analytics, Decision } from '../types'
import { Empty, Loading, PageHeader } from '../components/UI'

export default function InsightsPage() {
  const [items, setItems] = useState<Decision[] | null>(null)
  const [metrics,setMetrics]=useState<Analytics|null>(null)
  useEffect(() => { Promise.all([api<{ items: Decision[] }>('/api/v1/decisions?limit=100'),api<Analytics>('/api/v1/analytics/calibration')]).then(([list,analytics])=>{setItems(list.items);setMetrics(analytics)}) }, [])
  if (!items || !metrics) return <Loading label="개인 판단 패턴을 계산하는 중…" />
  return <div className="page insights-page"><PageHeader eyebrow="PERSONAL PATTERN" title="판단 패턴"><span>결과와 판단 품질을 분리해, 나의 사고 습관을 읽습니다.</span></PageHeader>
    {!items.length ? <Empty title="패턴을 만들 판단이 아직 없습니다."><p>몇 개의 판단과 회고가 쌓이면 확신 보정과 반복 편향을 볼 수 있습니다.</p></Empty> : <>
      <section className="pattern-hero"><div><Sparkles size={20} /><p className="eyebrow">TRACE OBSERVATION</p><h2>당신의 판단 데이터가<br />시간과 함께 선명해집니다.</h2><p>현재 {metrics.totalDecisions}개의 판단이 있습니다. 결과와 판단 품질을 함께 기록하면 Skill과 Luck을 더 정확히 분리할 수 있습니다.</p></div><div className="calibration-orbit"><span>{Math.round(metrics.averageConfidence)}%</span><small>평균 확신</small><i className="orbit one" /><i className="orbit two" /></div></section>
      <section className="insight-metrics"><article><Target /><span>AVERAGE CONFIDENCE</span><strong>{Math.round(metrics.averageConfidence)}</strong><p>기록 당시의 평균 확신</p></article><article><Compass /><span>EVIDENCE DEPTH</span><strong>{metrics.evidenceDepth.toFixed(1)}</strong><p>판단당 평균 근거 수</p></article><article><BrainCircuit /><span>REFLECTION RATE</span><strong>{Math.round(metrics.reflectionRate)}%</strong><p>회고를 남긴 판단 비율</p></article></section>
      <section className="decision-matrix"><header><div><p className="eyebrow">DECISION MATRIX</p><h2>Skill, Luck, Mistake</h2></div><Scale /></header><div className="matrix-grid"><span className="matrix-y good">판단 좋음</span><span className="matrix-y bad">판단 나쁨</span><span className="matrix-x good">결과 좋음</span><span className="matrix-x bad">결과 나쁨</span><article className="skill"><strong>{metrics.skill}</strong><span>SKILL</span></article><article className="bad-luck"><strong>{metrics.badLuck}</strong><span>BAD LUCK</span></article><article className="good-luck"><strong>{metrics.goodLuck}</strong><span>GOOD LUCK</span></article><article className="mistake"><strong>{metrics.mistake}</strong><span>MISTAKE</span></article></div></section>
    </>}
  </div>
}
