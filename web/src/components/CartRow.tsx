import { Link } from 'react-router-dom'
import type { CartItem } from '../api'
import { formatPrice } from '../api'
import { productArtwork } from '../lib/productArtwork'

export function CartRow({ item, onRemove }: { item: CartItem; onRemove?: () => void }) {
  return (
    <div className="cart-row">
      <div className={`mini-art art-${item.productTitle.toLowerCase().includes('racer') ? 'racing' : 'adventure'}`}><img src={productArtwork(item.productSlug)} alt="" /></div>
      <div className="cart-copy"><Link to={`/products/${item.productSlug}`}><strong>{item.productTitle}</strong></Link><span>{item.platform} · {item.region} · {item.sellerName}</span>
        {item.priceChanged && <span className="price-change">Price changed: was {formatPrice(item.priceSeenCents, item.currency)}</span>}
        {item.availableKeys === 0 && <span className="price-change">Out of stock</span>}
      </div>
      <strong className="cart-price">{formatPrice(item.priceCents, item.currency)}</strong>
      {onRemove && <button className="icon-button" type="button" onClick={onRemove} aria-label={`Remove ${item.productTitle}`}>×</button>}
    </div>
  )
}
