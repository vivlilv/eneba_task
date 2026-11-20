import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { ProductCard } from '../api'
import { request } from '../api'
import { EmptyState, ErrorPanel, Loading } from '../components/Feedback'
import { ProductCard as ProductCardView } from '../components/ProductCard'
import { StoreFilters } from '../components/StoreFilters'
import type { StoreFilterName } from '../components/StoreFilters'
import { errorMessage } from '../lib/errorMessage'
import { filterAndSortProducts } from '../lib/productSearch'

function parseEuroPrice(value: string) {
  if (value.trim() === '') return undefined
  const price = Number(value)
  return Number.isFinite(price) && price >= 0 ? Math.round(price * 100) : undefined
}

export function StorePage() {
  const [catalogState, setCatalogState] = useState<{
    query: string
    products: ProductCard[]
    loading: boolean
    error: string
  }>({ query: '', products: [], loading: true, error: '' })
  const [searchParams, setSearchParams] = useSearchParams()
  const query = searchParams.get('q') ?? ''
  const selectedPlatform = searchParams.get('platform') ?? 'all'
  const selectedRegion = searchParams.get('region') ?? 'all'
  const minimumPrice = searchParams.get('minPrice') ?? ''
  const maximumPrice = searchParams.get('maxPrice') ?? ''
  const requestedSort = searchParams.get('sort')
  const sortOrder = requestedSort === 'popular' ? 'featured' : requestedSort ?? 'featured'
  const minimumPriceCents = parseEuroPrice(minimumPrice)
  const maximumPriceCents = parseEuroPrice(maximumPrice)
  const hasInvalidPriceRange = minimumPriceCents !== undefined
    && maximumPriceCents !== undefined
    && minimumPriceCents > maximumPriceCents

  useEffect(() => {
    const controller = new AbortController()

    request<{ products: ProductCard[] }>(
      `/api/search?q=${encodeURIComponent(query)}&limit=100`,
      { signal: controller.signal },
    )
      .then(({ products }) => {
        if (!controller.signal.aborted) setCatalogState({ query, products, loading: false, error: '' })
      })
      .catch((reason) => {
        if (!controller.signal.aborted) {
          setCatalogState({ query, products: [], loading: false, error: errorMessage(reason) })
        }
      })

    return () => controller.abort()
  }, [query])

  const stateIsCurrent = catalogState.query === query
  const products = stateIsCurrent ? catalogState.products : []
  const loading = !stateIsCurrent || catalogState.loading
  const error = stateIsCurrent ? catalogState.error : ''
  const platforms = Array.from(new Set(products.map((product) => product.platform))).sort()
  const regions = Array.from(new Set(products.map((product) => product.region))).sort()
  const matchingProducts = hasInvalidPriceRange ? [] : filterAndSortProducts(products, {
    platform: selectedPlatform,
    region: selectedRegion,
    minimumPriceCents,
    maximumPriceCents,
    sortOrder,
  })

  function updateFilter(name: StoreFilterName, value: string) {
    const nextParams = new URLSearchParams(searchParams)
    if (value === 'all' || value === 'featured' || value === '') nextParams.delete(name)
    else nextParams.set(name, value)
    const isPriceInput = name === 'minPrice' || name === 'maxPrice'
    setSearchParams(nextParams, { replace: isPriceInput })
  }

  function clearFilters() {
    setSearchParams(query ? { q: query } : {})
  }

  return (
    <section className="store-page">
      <div className="store-heading">
        <div>
          <p className="eyebrow">THE DEMO SELECTION</p>
          <h1>Browse games</h1>
          <p className="muted">Search and browse the full demo catalog.</p>
        </div>
        <span className="result-count">
          {loading ? 'Loading…' : hasInvalidPriceRange ? 'Invalid price range' : `${matchingProducts.length} games`}
        </span>
      </div>

      <StoreFilters
        platforms={platforms}
        regions={regions}
        selectedPlatform={selectedPlatform}
        selectedRegion={selectedRegion}
        minimumPrice={minimumPrice}
        maximumPrice={maximumPrice}
        sortOrder={sortOrder}
        onChange={updateFilter}
      />

      {hasInvalidPriceRange && <p className="filter-error" role="alert">Minimum price cannot be greater than maximum price.</p>}
      {error ? <ErrorPanel message={error} /> : loading ? (
        <Loading label="Loading games…" />
      ) : matchingProducts.length > 0 ? (
        <div className="product-grid store-grid">
          {matchingProducts.map((product) => <ProductCardView key={product.id} product={product} />)}
        </div>
      ) : (
        <EmptyState
          title="No matching games"
          body="Try a different search or clear one of the filters."
          action={<button className="button button-secondary" type="button" onClick={clearFilters}>Clear filters</button>}
        />
      )}

      <p className="catalog-note">Product search and catalog results are supplied by the backend.</p>
    </section>
  )
}
