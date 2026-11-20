import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { formatPrice, request } from '../api'
import type { Order } from '../api'
import { useApp } from '../appContext'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { errorMessage } from '../lib/errorMessage'

export function OrderPage() {
  const { orderID = '' } = useParams()
  const { user, authReady } = useApp()
  const navigate = useNavigate()
  const [result, setResult] = useState<{ orderID: string; order: Order | null; error: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const order = result?.orderID === orderID ? result.order : null
  const error = result?.orderID === orderID ? result.error : ''
  const loading = !authReady || result?.orderID !== orderID

  useEffect(() => {
    if (!authReady) return
    if (!user) {
      navigate(`/auth?returnTo=${encodeURIComponent(`/orders/${orderID}`)}`, { replace: true })
      return
    }

    let active = true
    request<Order>(`/api/orders/${encodeURIComponent(orderID)}`)
      .then((order) => {
        if (active) setResult({ orderID, order, error: '' })
      })
      .catch((reason) => {
        if (active) setResult({ orderID, order: null, error: errorMessage(reason) })
      })
    return () => { active = false }
  }, [authReady, user, navigate, orderID])

  async function retryPayment() {
    if (!order) return
    setBusy(true)
    setResult((current) => current?.orderID === orderID
      ? { ...current, error: '' }
      : current)
    try {
      const result = await request<{ order: Order }>(`/api/orders/${order.id}/pay`, {
        method: 'POST',
        headers: { 'Idempotency-Key': crypto.randomUUID() },
        body: JSON.stringify({ outcome: 'success' }),
      })
      setResult({ orderID, order: result.order, error: '' })
    } catch (reason) {
      setResult((current) => current?.orderID === orderID
        ? { ...current, error: errorMessage(reason) }
        : current)
    } finally {
      setBusy(false)
    }
  }

  if (loading || !authReady) return <Loading label="Loading your Order…" />
  if (error && !order) return <ErrorPanel message={error} />
  if (!order) return <EmptyState title="Order not found" body="This Order may not belong to your account." action={<Link className="button button-secondary" to="/orders">Back to My Orders</Link>} />

  return (
    <section className="content-section page-section">
      <div className="success-banner"><span className="success-icon">{order.status === 'paid' ? '✓' : '…'}</span><div><p className="eyebrow">{order.status === 'paid' ? 'PURCHASE COMPLETE' : 'PAYMENT PENDING'}</p><h1>{order.status === 'paid' ? 'Your keys are ready' : 'Finish your payment'}</h1><p>{order.status === 'paid' ? 'Your digital items are attached to this Order.' : 'No keys are assigned until payment succeeds.'}</p></div></div>
      {error && <ErrorPanel message={error} />}
      <div className="order-detail-header"><div><span className="eyebrow">ORDER ID</span><strong>{order.id}</strong></div><span className={`order-status ${order.status === 'paid' ? 'paid' : 'pending'}`}>{order.status.replace('_', ' ')}</span></div>
      <div className="order-detail-list">
        {order.items.map((item) => (
          <div className="order-item-card" key={item.id}>
            <div><strong>{item.productTitle}</strong><span>{item.sellerName} · {formatPrice(item.unitPriceCents, item.currency)}</span></div>
            {item.digitalKey ? <code className="digital-key">{item.digitalKey}</code> : <span className="small-muted">Key appears after successful payment.</span>}
          </div>
        ))}
      </div>
      <div className="order-total"><span>Total</span><strong>{formatPrice(order.totalCents, order.currency)}</strong></div>
      {order.status !== 'paid' && <button className="button button-primary" type="button" disabled={busy} onClick={retryPayment}>{busy ? 'Processing…' : 'Retry simulated payment'}</button>}
      <div className="after-action"><Link className="button button-secondary" to="/orders">My orders</Link><Link className="back-link" to="/">Continue browsing</Link></div>
    </section>
  )
}
