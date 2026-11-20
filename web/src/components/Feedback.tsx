import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return <EmptyState title="Page not found" body="That page isn't part of this demo." action={<Link className="button button-primary" to="/">Go home</Link>} />
}

export function Loading({ label }: { label: string }) {
  return <div className="loading-state"><span className="spinner" />{label}</div>
}

export function ErrorPanel({ message }: { message: string }) {
  return <div className="error-panel" role="alert">{message}</div>
}

export function EmptyState({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-mark">✦</div><h2>{title}</h2><p>{body}</p>{action}</div>
}
