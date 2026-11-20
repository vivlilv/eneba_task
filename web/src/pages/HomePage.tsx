import { useEffect, useState } from 'react'
import type { HomepageBlock } from '../api'
import { request } from '../api'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { HeroBanner } from '../components/HeroBanner'
import { ProductCard } from '../components/ProductCard'
import { errorMessage } from '../lib/errorMessage'

export function HomePage() {
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
