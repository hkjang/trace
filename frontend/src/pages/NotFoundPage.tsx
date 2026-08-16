import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router-dom'

export default function NotFoundPage() { return <div className="not-found"><span className="mono">404 · LOST IN TIME</span><h1>이 시간축에는<br />아무 흔적이 없습니다.</h1><p>주소가 바뀌었거나 존재하지 않는 화면입니다.</p><Link className="button primary" to="/"><ArrowLeft size={17} />현재로 돌아가기</Link></div> }
