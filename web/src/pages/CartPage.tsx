import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { formatPrice, readGuestCart, request, writeGuestCart } from '../api'
import type { Cart } from '../api'
import { useApp } from '../appContext'
import { CartRow } from '../components/CartRow'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { errorMessage } from '../lib/errorMessage'
import { fetchCart } from '../lib/cart'

export function CartPage() {
  const { user, authReady, refreshCartCount } = useApp()
  const [cart, setCart] = useState<Cart>({ items: [], totalCents: 0, hasPriceChanges: false })
  const [loadedForUserId, setLoadedForUserId] = useState<number | null>()
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  const loading = !authReady || loadedForUserId !== (user?.id ?? null) || refreshing

  useEffect(() => {
    if (!authReady) return

    let active = true
    fetchCart(user)
      .then((nextCart) => {
        if (!active) return
        setCart(nextCart)
        setError('')
        setLoadedForUserId(user?.id ?? null)
      })
      .catch((reason) => {
        if (!active) return
        setError(errorMessage(reason))
        setLoadedForUserId(user?.id ?? null)
      })
    return () => { active = false }
  }, [authReady, user])

  async function remove(offerID: string) {
    setRefreshing(true)
    setError('')
    try {
      if (user) {
        await request<Cart>(`/api/cart/items/${encodeURIComponent(offerID)}`, { method: 'DELETE' })
      } else {
        writeGuestCart(readGuestCart().filter((item) => item.offerId !== offerID))
      }
      await refreshCartCount()
      setCart(await fetchCart(user))
      setLoadedForUserId(user?.id ?? null)
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setRefreshing(false)
    }
  }

  if (loading || !authReady) return <Loading label="Loading your cart…" />

  return (
    <section className="content-section page-section">
      <div className="section-heading"><div><p className="eyebrow">YOUR SELECTION</p><h1>Your cart</h1></div><span className="offer-count">{cart.items.length} items</span></div>
      {error && <ErrorPanel message={error} />}
      {cart.items.length === 0 ? <EmptyState title="Your cart is empty" body="Pick a product from the homepage and compare its seller offers." action={<Link className="button button-primary" to="/">Explore picks</Link>} /> : (
        <div className="cart-layout">
          <div className="cart-items">
            {cart.items.map((item) => <CartRow key={item.offerId} item={item} onRemove={() => remove(item.offerId)} />)}
          </div>
          <aside className="summary-card">
            <p className="eyebrow">ORDER SUMMARY</p>
            <div className="summary-line"><span>Subtotal</span><strong>{formatPrice(cart.totalCents)}</strong></div>
            <div className="summary-line"><span>Digital delivery</span><strong>Free</strong></div>
            <div className="summary-total"><span>Total</span><strong>{formatPrice(cart.totalCents)}</strong></div>
            {cart.hasPriceChanges && <div className="warning-box">One or more seller prices changed. Review and confirm the updated total at checkout.</div>}
            {cart.items.some((item) => item.availableKeys === 0) && <div className="warning-box">An offer is out of stock. It cannot be purchased until you remove it or choose another seller.</div>}
            <Link className="button button-primary button-wide" to={user ? '/checkout' : '/auth?returnTo=%2Fcheckout'}>Continue to checkout</Link>
            {!user && <p className="small-muted">You can keep a guest cart. Sign in is required at checkout.</p>}
          </aside>
        </div>
      )}
    </section>
  )
}
