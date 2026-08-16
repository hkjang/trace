import { Component, type ErrorInfo, type ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'

export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Trace UI error', error, info) }
  render() {
    if (!this.state.error) return this.props.children
    return <main className="fatal-error"><AlertTriangle size={32} /><p className="eyebrow">TRACE RECOVERY</p><h1>화면을 표시하지 못했습니다.</h1><p>기록은 서버에 보존되어 있습니다. 화면을 다시 불러와 주세요.</p><button className="button primary" onClick={() => location.reload()}>다시 불러오기</button></main>
  }
}
