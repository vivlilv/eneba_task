import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { APIError, formatPrice, request } from '../api'
import type { Cart, Order } from '../api'
import { useApp } from '../appContext'
import { CartRow } from '../components/CartRow'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { errorMessage } from '../lib/errorMessage'

export function CheckoutPage() {
  const { user, authReady, refreshCartCount } = useApp()
  const navigate = useNavigate()
  const [cart, setCart] = useState<Cart | null>(null)
  const [loadedForUserId, setLoadedForUserId] = useState<number | null>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const loading = !authReady || (user !== null && loadedForUserId !== user.id)

  useEffect(() => {
    if (!authReady) return
    if (!user) {
      navigate('/auth?returnTo=%2Fcheckout', { replace: true })
      return
    }

    let active = true
    request<Cart>('/api/cart')
      .then((nextCart) => {
        if (!active) return
        setCart(nextCart)
        setError('')
        setLoadedForUserId(user.id)
      })
      .catch((reason) => {
        if (!active) return
        setError(errorMessage(reason))
        setLoadedForUserId(user.id)
      })
    return () => { active = false }
  }, [authReady, user, navigate])

  async function acceptPrices() {
    setBusy(true)
    setError('')
    try {
      setCart(await request<Cart>('/api/cart/confirm-prices', { method: 'POST', body: '{}' }))
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  async function pay() {
    if (!cart || cart.items.length === 0) return
    setBusy(true)
    setError('')
    try {
      let checkoutKey = sessionStorage.getItem('eneba-checkout-key')
      if (!checkoutKey) {
        checkoutKey = crypto.randomUUID()
        sessionStorage.setItem('eneba-checkout-key', checkoutKey)
      }
      const created = await request<Order>('/api/checkout', {
        method: 'POST',
        headers: { 'Idempotency-Key': checkoutKey },
        body: JSON.stringify({ confirmPriceChanges: false }),
      })
      const paid = await request<{ paymentStatus: string; order: Order }>(`/api/orders/${created.id}/pay`, {
        method: 'POST',
        headers: { 'Idempotency-Key': crypto.randomUUID() },
        body: JSON.stringify({ outcome: 'success' }),
      })
      sessionStorage.removeItem('eneba-checkout-key')
      await refreshCartCount()
      navigate(`/orders/${paid.order.id}`, { replace: true })
    } catch (reason) {
      setError(errorMessage(reason))
      if (reason instanceof APIError && reason.code === 'out_of_stock') {
        sessionStorage.removeItem('eneba-checkout-key')
      }
      if (reason instanceof APIError && (reason.code === 'price_changed' || reason.code === 'out_of_stock')) {
        try {
          setCart(await request<Cart>('/api/cart'))
        } catch (refreshError) {
          setError(errorMessage(refreshError))
        }
      }
    } finally {
      setBusy(false)
    }
  }

  if (loading || !authReady) return <Loading label="Preparing checkout…" />
  if (!user) return null
  if (error && !cart) return <ErrorPanel message={error} />
  if (!cart || cart.items.length === 0) return <EmptyState title="Nothing to check out" body="Your cart is empty." action={<Link className="button button-primary" to="/">Back to picks</Link>} />

  return (
    <section className="content-section page-section">
      <div className="section-heading"><div><p className="eyebrow">SECURE CHECKOUT</p><h1>Review your order</h1></div><span className="checkout-step">1 / 2 · Review</span></div>
      {error && <ErrorPanel message={error} />}
      <div className="checkout-layout">
        <div className="checkout-lines">
          {cart.items.map((item) => <CartRow key={item.offerId} item={item} />)}
          {cart.hasPriceChanges && <div className="warning-box"><strong>Some prices changed.</strong> Review the updated total, then confirm it before paying.</div>}
          {cart.items.some((item) => item.availableKeys === 0) && <div className="warning-box">An item is out of stock. Remove it from the cart before continuing.</div>}
          {cart.hasPriceChanges && <button className="button button-secondary" type="button" disabled={busy} onClick={acceptPrices}>Accept updated prices</button>}
        </div>
        <aside className="summary-card">
          <p className="eyebrow">PAYMENT</p>
          <div className="summary-line"><span>Items</span><strong>{cart.items.length}</strong></div>
          <div className="summary-total"><span>Total</span><strong>{formatPrice(cart.totalCents)}</strong></div>
          <p className="small-muted">This is a simulation. No real payment will be taken.</p>
          <button className="button button-primary button-wide" disabled={busy || cart.hasPriceChanges || cart.items.some((item) => item.availableKeys === 0)} onClick={pay}>{busy ? 'Processing…' : `Simulate payment · ${formatPrice(cart.totalCents)}`}</button>
        </aside>
      </div>
    </section>
  )
}
