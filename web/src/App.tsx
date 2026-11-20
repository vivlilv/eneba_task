import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { BrowserRouter, Link, Route, Routes } from 'react-router-dom'
import { clearGuestCart, readGuestCart, request } from './api'
import type { Cart, User } from './api'
import { AppContext, useApp } from './appContext'
import { NotFoundPage, SiteSearch } from './components'
import {
  AuthPage,
  CartPage,
  CheckoutPage,
  HomePage,
  OrderPage,
  OrdersPage,
  ProductPage,
  StorePage,
} from './pages'

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
          <Link className="brand" to="/" aria-label="Keylane home">
            <span className="brand-mark" aria-hidden="true">K</span><span>keylane</span>
          </Link>
          <SiteSearch />
          <nav className="main-nav" aria-label="Main navigation">
            <Link to="/store">Browse</Link>
            {authReady && user && <Link to="/orders">Orders</Link>}
            <Link className="cart-link" to="/cart" aria-label={`Cart, ${cartCount} items`}>Cart <span className="cart-count">{cartCount}</span></Link>
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
          <Route path="/store" element={<StorePage />} />
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
