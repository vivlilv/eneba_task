import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { formatPrice, readGuestCart, request, writeGuestCart } from '../api'
import type { Cart, Product } from '../api'
import { useApp } from '../appContext'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { productArtwork } from '../lib/productArtwork'
import { errorMessage } from '../lib/errorMessage'

export function ProductPage() {
  const { slug = '' } = useParams()
  const { user, refreshCartCount } = useApp()
  const [result, setResult] = useState<{ slug: string; product: Product | null; error: string } | null>(null)
  const [notice, setNotice] = useState('')
  const [adding, setAdding] = useState('')
  const loading = result?.slug !== slug
  const product = result?.slug === slug ? result.product : null
  const error = result?.slug === slug ? result.error : ''
  useEffect(() => {
    let active = true
    request<Product>(`/api/products/${encodeURIComponent(slug)}`)
      .then((product) => {
        if (active) setResult({ slug, product, error: '' })
      })
      .catch((reason) => {
        if (active) setResult({ slug, product: null, error: errorMessage(reason) })
      })
    return () => { active = false }
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
        <div className={`product-detail-art art-${product.category.toLowerCase()}`}><img src={productArtwork(product.slug)} alt="" /><span className="art-kicker">{product.category}</span><span className="platform-pill">{product.platform} · {product.region}</span></div>
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
