import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { BrowserRouter, Link, Route, Routes, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  APIError,
  clearGuestCart,
  formatPrice,
  readGuestCart,
  request,
  writeGuestCart,
} from './api'
import type { Cart, CartItem, HomepageBlock, OfferPreview, Order, Product, User } from './api'

type AppContextValue = {
  user: User | null
  authReady: boolean
  cartCount: number
  onSignedIn: (user: User) => Promise<void>
  onSignedOut: () => Promise<void>
  refreshCartCount: () => Promise<void>
}

const AppContext = createContext<AppContextValue | null>(null)

function useApp() {
  const value = useContext(AppContext)
  if (!value) throw new Error('App context missing')
  return value
}

export default function App() {
  const [user, setUser] = useState<User | null>(null)
  const [authReady, setAuthReady] = useState(false)
  const [cartCount, setCartCount] = useState(0)
  const userRef = useRef<User | null>(null)

  useEffect(() => { userRef.current = user }, [user])

  const refreshCartCount = useCallback(async (requestedUser?: User | null) => {
    const currentUser = requestedUser === undefined ? userRef.current : requestedUser
    if (!currentUser) {
      setCartCount(readGuestCart().length)
      return
    }
    try {
      const cart = await request<Cart>('/api/cart')
      setCartCount(cart.items.length)
    } catch {
      setCartCount(0)
    }
  }, [])

  const onSignedIn = useCallback(async (nextUser: User) => {
    userRef.current = nextUser
    setUser(nextUser)
    const guestItems = readGuestCart()
    try {
      if (guestItems.length > 0) {
        await request<Cart>('/api/cart/merge', {
          method: 'POST',
          body: JSON.stringify({ items: guestItems }),
        })
        clearGuestCart()
      }
      await refreshCartCount(nextUser)
    } catch (error) {
      await refreshCartCount(nextUser)
      throw error
    }
  }, [refreshCartCount])

  const onSignedOut = useCallback(async () => {
    await request<void>('/api/auth/logout', { method: 'POST' })
    userRef.current = null
    setUser(null)
    await refreshCartCount(null)
  }, [refreshCartCount])

  useEffect(() => {
    let active = true
    request<{ user: User }>('/api/me')
      .then(async ({ user: currentUser }) => {
        if (!active) return
        try {
          await onSignedIn(currentUser)
        } catch {
          // Keep the valid session even if an old guest-cart item cannot be merged.
          userRef.current = currentUser
          setUser(currentUser)
        }
      })
      .catch(() => {
        if (active) {
          userRef.current = null
          setUser(null)
          setCartCount(readGuestCart().length)
        }
      })
      .finally(() => {
        if (active) setAuthReady(true)
      })
    return () => { active = false }
  }, [onSignedIn])

  useEffect(() => {
    const syncGuestCart = () => {
      if (!user) setCartCount(readGuestCart().length)
    }
    window.addEventListener('storage', syncGuestCart)
    return () => window.removeEventListener('storage', syncGuestCart)
  }, [user])

  const context = useMemo(() => ({ user, authReady, cartCount, onSignedIn, onSignedOut, refreshCartCount }), [user, authReady, cartCount, onSignedIn, onSignedOut, refreshCartCount])

  return (
    <BrowserRouter>
      <AppContext.Provider value={context}>
        <SiteLayout />
      </AppContext.Provider>
    </BrowserRouter>
  )
}

function SiteLayout() {
  const { user, authReady, cartCount, onSignedOut } = useApp()
  const [menuError, setMenuError] = useState('')

  async function signOut() {
    setMenuError('')
    try {
      await onSignedOut()
    } catch {
      setMenuError('Could not sign out. Try again.')
    }
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="topbar-inner">
          <Link className="brand" to="/" aria-label="Eneba home">
            <span className="brand-mark">e</span><span>eneba</span>
          </Link>
          <div className="header-promise">Digital games. Better deals.</div>
          <nav className="main-nav" aria-label="Main navigation">
            {authReady && user && <Link to="/orders">My orders</Link>}
            <Link className="cart-link" to="/cart">Cart <span className="cart-count">{cartCount}</span></Link>
            {!authReady ? <span className="nav-placeholder" /> : user ? (
              <button className="nav-button" type="button" onClick={signOut}>Sign out</button>
            ) : <Link className="nav-button" to="/auth">Sign in</Link>}
          </nav>
        </div>
        {menuError && <div className="topbar-error">{menuError}</div>}
      </header>

      <main className="page-content">
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/products/:slug" element={<ProductPage />} />
          <Route path="/cart" element={<CartPage />} />
          <Route path="/auth" element={<AuthPage />} />
          <Route path="/checkout" element={<CheckoutPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/orders/:orderID" element={<OrderPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </main>
      <footer className="site-footer">
        <span>Demo marketplace · no real payment is processed</span>
        <span>All demo keys are synthetic and cannot be redeemed</span>
      </footer>
    </div>
  )
}

function HomePage() {
  const [blocks, setBlocks] = useState<HomepageBlock[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    request<{ blocks: HomepageBlock[] }>('/api/homepage')
      .then((response) => setBlocks(response.blocks))
      .catch((reason) => setError(errorMessage(reason)))
      .finally(() => setLoading(false))
  }, [])

  return (
    <>
      {loading && <Loading label="Loading today's picks…" />}
      {error && <ErrorPanel message={error} />}
      {!loading && !error && blocks.length === 0 && <EmptyState title="No picks yet" body="Homepage content will appear here when it is ready." />}
      {blocks.filter((block) => block.kind === 'hero').map((block) => <HeroBanner key={block.id} block={block} />)}
      {blocks.filter((block) => block.kind !== 'hero').map((block) => (
        <section className="content-section" id={block.id} key={block.id}>
          <div className="section-heading"><div><p className="eyebrow">{block.eyebrow}</p><h2>{block.title}</h2><p className="block-description">{block.body}</p></div><span className="section-arrow" aria-hidden="true">↗</span></div>
          <div className="product-grid">
            {block.products.map((product) => <ProductCard key={product.id} product={product} />)}
          </div>
        </section>
      ))}
    </>
  )
}

function HeroBanner({ block }: { block: HomepageBlock }) {
  return (
    <section className="hero-banner">
      <div className="hero-copy">
        <p className="eyebrow">{block.eyebrow}</p>
        <h1>{block.title}</h1>
        <p className="hero-description">{block.body}</p>
        {block.ctaLabel && block.ctaUrl && <a className="button button-primary" href={block.ctaUrl}>{block.ctaLabel}</a>}
      </div>
      <div className="hero-art" aria-hidden="true"><div className="orb orb-one" /><div className="orb orb-two" /></div>
    </section>
  )
}

function ProductCard({ product }: { product: HomepageBlock['products'][number] }) {
  return (
    <Link className="product-card" to={`/products/${product.slug}`}>
      <div className={`product-art art-${product.category.toLowerCase()}`}>
        <span className="art-kicker">{product.category}</span>
        <span className="art-title">{product.title.split(' ').map((word) => word[0]).join('')}</span>
        <span className="platform-pill">{product.platform} · {product.region}</span>
      </div>
      <div className="product-card-body">
        <div className="product-card-title">{product.title}</div>
        <div className="product-card-edition">{product.edition}</div>
        <div className="product-card-meta"><span>{product.offerCount} seller offers</span><span className="price">From {formatPrice(product.lowestPriceCents, product.currency)}</span></div>
      </div>
    </Link>
  )
}

function ProductPage() {
  const { slug = '' } = useParams()
  const { user, refreshCartCount } = useApp()
  const [product, setProduct] = useState<Product | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [adding, setAdding] = useState('')

  useEffect(() => {
    setLoading(true)
    request<Product>(`/api/products/${encodeURIComponent(slug)}`)
      .then(setProduct)
      .catch((reason) => setError(errorMessage(reason)))
      .finally(() => setLoading(false))
  }, [slug])

  async function addOffer(offerID: string, priceCents: number) {
    setAdding(offerID)
    setNotice('')
    try {
      if (user) {
        await request<Cart>('/api/cart/items', {
          method: 'POST',
          body: JSON.stringify({ offerId: offerID, priceSeenCents: priceCents }),
        })
      } else {
        const items = readGuestCart()
        if (!items.some((item) => item.offerId === offerID)) {
          writeGuestCart([...items, { offerId: offerID, priceSeenCents: priceCents }])
        }
      }
      await refreshCartCount()
      setNotice('Added to your cart.')
    } catch (reason) {
      setNotice(errorMessage(reason))
    } finally {
      setAdding('')
    }
  }

  if (loading) return <Loading label="Loading product…" />
  if (error || !product) return <ErrorPanel message={error || 'Product not found.'} />

  return (
    <>
      <div className="breadcrumbs"><Link to="/">Home</Link><span>/</span><span>{product.category}</span><span>/</span><span>{product.title}</span></div>
      <section className="product-detail-grid">
        <div className={`product-detail-art art-${product.category.toLowerCase()}`}><span className="art-kicker">{product.category}</span><span className="detail-art-title">{product.title}</span><span className="platform-pill">{product.platform} · {product.region}</span></div>
        <div className="product-detail-copy">
          <p className="eyebrow">{product.platform} · {product.region}</p>
          <h1>{product.title}</h1>
          <p className="muted">{product.edition}</p>
          <p className="product-description">{product.description}</p>
          <div className="trust-row"><span>✓ Instant digital delivery</span><span>✓ Secure checkout</span></div>
        </div>
      </section>

      <section className="content-section offers-section">
        <div className="section-heading"><div><p className="eyebrow">COMPARE SELLERS</p><h2>Available offers</h2></div><span className="offer-count">{product.offers.length} offers</span></div>
        {notice && <div className="notice" role="status">{notice}</div>}
        <div className="offer-list">
          {product.offers.map((offer) => (
            <div className="offer-row" key={offer.id}>
              <div className="seller-avatar">{offer.sellerName.slice(0, 1)}</div>
              <div className="seller-copy"><strong>{offer.sellerName}</strong><span>★ {offer.sellerRating.toFixed(2)} seller rating</span></div>
              <div className="offer-stock">{offer.available ? `${offer.availableKeys} available` : 'Out of stock'}</div>
              <div className="offer-price">{formatPrice(offer.priceCents, offer.currency)}</div>
              <button className="button button-primary button-small" disabled={!offer.available || adding === offer.id} onClick={() => addOffer(offer.id, offer.priceCents)}>
                {adding === offer.id ? 'Adding…' : offer.available ? 'Add to cart' : 'Unavailable'}
              </button>
            </div>
          ))}
          {product.offers.length === 0 && <EmptyState title="No offers available" body="Check back later for seller offers." />}
        </div>
        <div className="note-box">Choose a seller's offer. Each offer has its own price and available key stock.</div>
      </section>
      <div className="after-action"><Link className="button button-secondary" to="/cart">View cart</Link></div>
    </>
  )
}

function CartPage() {
  const { user, authReady, refreshCartCount } = useApp()
  const [cart, setCart] = useState<Cart>({ items: [], totalCents: 0, hasPriceChanges: false })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    if (!authReady) return
    setLoading(true)
    setError('')
    try {
      if (user) {
        setCart(await request<Cart>('/api/cart'))
      } else {
        const guestItems = readGuestCart()
        const previews = await Promise.all(guestItems.map(async (item) => {
          const preview = await request<OfferPreview>(`/api/offers/${encodeURIComponent(item.offerId)}`)
          return {
            offerId: item.offerId,
            productSlug: preview.productSlug,
            productTitle: preview.productTitle,
            platform: preview.platform,
            region: preview.region,
            sellerName: preview.sellerName,
            priceCents: preview.priceCents,
            priceSeenCents: item.priceSeenCents,
            currency: preview.currency,
            availableKeys: preview.availableKeys,
            priceChanged: preview.priceCents !== item.priceSeenCents,
          } satisfies CartItem
        }))
        setCart({ items: previews, totalCents: previews.reduce((sum, item) => sum + item.priceCents, 0), hasPriceChanges: previews.some((item) => item.priceChanged) })
      }
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [authReady, user])

  useEffect(() => { void load() }, [load])

  async function remove(offerID: string) {
    try {
      if (user) {
        await request<Cart>(`/api/cart/items/${encodeURIComponent(offerID)}`, { method: 'DELETE' })
      } else {
        writeGuestCart(readGuestCart().filter((item) => item.offerId !== offerID))
      }
      await refreshCartCount()
      await load()
    } catch (reason) {
      setError(errorMessage(reason))
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

function CartRow({ item, onRemove }: { item: CartItem; onRemove?: () => void }) {
  return (
    <div className="cart-row">
      <div className={`mini-art art-${item.productTitle.toLowerCase().includes('racer') ? 'racing' : 'adventure'}`}>{item.productTitle.slice(0, 1)}</div>
      <div className="cart-copy"><Link to={`/products/${item.productSlug}`}><strong>{item.productTitle}</strong></Link><span>{item.platform} · {item.region} · {item.sellerName}</span>
        {item.priceChanged && <span className="price-change">Price changed: was {formatPrice(item.priceSeenCents, item.currency)}</span>}
        {item.availableKeys === 0 && <span className="price-change">Out of stock</span>}
      </div>
      <strong className="cart-price">{formatPrice(item.priceCents, item.currency)}</strong>
      {onRemove && <button className="icon-button" type="button" onClick={onRemove} aria-label={`Remove ${item.productTitle}`}>×</button>}
    </div>
  )
}

function AuthPage() {
  const { onSignedIn } = useApp()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const requestedReturn = searchParams.get('returnTo') || '/'
  const returnTo = requestedReturn.startsWith('/') && !requestedReturn.startsWith('//') ? requestedReturn : '/'

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const response = await request<{ user: User }>(`/api/auth/${mode === 'register' ? 'register' : 'login'}`, {
        method: 'POST',
        body: JSON.stringify({ email, password }),
      })
      await onSignedIn(response.user)
      navigate(returnTo, { replace: true })
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="auth-wrap">
      <div className="auth-card">
        <p className="eyebrow">YOUR ENEBA ACCOUNT</p>
        <h1>{mode === 'login' ? 'Welcome back' : 'Create your account'}</h1>
        <p className="muted">Sign in to continue to checkout and keep your orders in one place.</p>
        {error && <ErrorPanel message={error} />}
        <form onSubmit={submit} className="form-stack">
          <label>Email address<input type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} /></label>
          <label>Password<input type="password" autoComplete={mode === 'register' ? 'new-password' : 'current-password'} minLength={10} required value={password} onChange={(event) => setPassword(event.target.value)} /></label>
          {mode === 'register' && <p className="small-muted">Use at least 10 characters. This demo does not send verification email.</p>}
          <button className="button button-primary button-wide" disabled={busy}>{busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}</button>
        </form>
        <p className="auth-switch">{mode === 'login' ? 'New to Eneba?' : 'Already have an account?'}{' '}
          <button type="button" onClick={() => { setMode(mode === 'login' ? 'register' : 'login'); setError('') }}>{mode === 'login' ? 'Create an account' : 'Sign in'}</button>
        </p>
        <Link className="back-link" to={returnTo === '/checkout' ? '/cart' : '/'}>← Back</Link>
      </div>
    </section>
  )
}

function CheckoutPage() {
  const { user, authReady, refreshCartCount } = useApp()
  const navigate = useNavigate()
  const [cart, setCart] = useState<Cart | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    if (!authReady) return
    if (!user) {
      navigate('/auth?returnTo=%2Fcheckout', { replace: true })
      return
    }
    setLoading(true)
    try {
      setCart(await request<Cart>('/api/cart'))
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [authReady, user, navigate])

  useEffect(() => { void load() }, [load])

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
        await load()
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

function OrdersPage() {
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

function OrderPage() {
  const { orderID = '' } = useParams()
  const { user, authReady } = useApp()
  const navigate = useNavigate()
  const [order, setOrder] = useState<Order | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    if (!authReady) return
    if (!user) {
      navigate(`/auth?returnTo=${encodeURIComponent(`/orders/${orderID}`)}`, { replace: true })
      return
    }
    setLoading(true)
    try {
      setOrder(await request<Order>(`/api/orders/${encodeURIComponent(orderID)}`))
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [authReady, user, navigate, orderID])

  useEffect(() => { void load() }, [load])

  async function retryPayment() {
    if (!order) return
    setBusy(true)
    setError('')
    try {
      const result = await request<{ order: Order }>(`/api/orders/${order.id}/pay`, {
        method: 'POST',
        headers: { 'Idempotency-Key': crypto.randomUUID() },
        body: JSON.stringify({ outcome: 'success' }),
      })
      setOrder(result.order)
    } catch (reason) {
      setError(errorMessage(reason))
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

function NotFoundPage() {
  return <EmptyState title="Page not found" body="That page isn't part of this demo." action={<Link className="button button-primary" to="/">Go home</Link>} />
}

function Loading({ label }: { label: string }) {
  return <div className="loading-state"><span className="spinner" />{label}</div>
}

function ErrorPanel({ message }: { message: string }) {
  return <div className="error-panel" role="alert">{message}</div>
}

function EmptyState({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-mark">✦</div><h2>{title}</h2><p>{body}</p>{action}</div>
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : 'Something went wrong. Please try again.'
}
