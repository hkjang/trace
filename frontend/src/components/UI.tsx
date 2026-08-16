import { AlertCircle, LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'

export function Loading({ label = '불러오는 중…' }: { label?: string }) { return <div className="loading-state"><LoaderCircle className="spin" /><span>{label}</span></div> }
export function Empty({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) { return <div className="empty-state"><span className="empty-orbit" /><p className="eyebrow">EMPTY TRACE</p><h2>{title}</h2><div>{children}</div>{action}</div> }
export function ErrorNotice({ message }: { message: string }) { return <div className="error-notice" role="alert"><AlertCircle size={18} />{message}</div> }
export function PageHeader({ eyebrow, title, children, action }: { eyebrow: string; title: string; children?: ReactNode; action?: ReactNode }) { return <header className="page-header"><div><p className="eyebrow">{eyebrow}</p><h1>{title}</h1>{children && <div className="page-subtitle">{children}</div>}</div>{action}</header> }
