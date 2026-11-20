import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { formatPrice, request } from '../api'
import type { Order } from '../api'
import { useApp } from '../appContext'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { errorMessage } from '../lib/errorMessage'

export function OrdersPage() {
  const { user, authReady } = useApp()
  const navigate = useNavigate()
  const [orders, setOrders] = useState<Order[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!authReady) return
    if (!user) {
      navigate('/auth?returnTo=%2Forders', { replace: true })
      return
    }
    request<{ orders: Order[] }>('/api/orders')
      .then((response) => setOrders(response.orders))
      .catch((reason) => setError(errorMessage(reason)))
      .finally(() => setLoading(false))
  }, [authReady, user, navigate])

  if (loading || !authReady) return <Loading label="Loading your orders…" />
  return (
    <section className="content-section page-section">
      <div className="section-heading"><div><p className="eyebrow">YOUR PURCHASES</p><h1>My orders</h1></div></div>
      {error && <ErrorPanel message={error} />}
      {orders.length === 0 ? <EmptyState title="No purchases yet" body="Your completed orders and keys will appear here." action={<Link className="button button-primary" to="/">Explore picks</Link>} /> : (
        <div className="order-list">
          {orders.map((order) => <Link className="order-list-card" key={order.id} to={`/orders/${order.id}`}><div><span className="eyebrow">ORDER</span><strong>{order.id.slice(0, 8).toUpperCase()}</strong></div><div><span className="muted">{order.items.length} items</span><strong>{formatPrice(order.totalCents, order.currency)}</strong></div><span className="order-status">PAID</span></Link>)}
        </div>
      )}
    </section>
  )
}
