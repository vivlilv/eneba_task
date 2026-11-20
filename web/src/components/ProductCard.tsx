import { Link } from 'react-router-dom'
import type { HomepageBlock } from '../api'
import { formatPrice } from '../api'
import { productArtwork } from '../lib/productArtwork'

export function ProductCard({ product }: { product: HomepageBlock['products'][number] }) {
  return (
    <Link className="product-card" to={`/products/${product.slug}`}>
      <div className={`product-art art-${product.category.toLowerCase()}`}>
        <img src={productArtwork(product.slug)} alt="" />
        <span className="art-kicker">{product.category}</span>
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
