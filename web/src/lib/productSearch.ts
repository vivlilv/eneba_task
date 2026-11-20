import type { ProductCard } from '../api'

type StoreFilterOptions = {
  platform: string
  region: string
  minimumPriceCents?: number
  maximumPriceCents?: number
  sortOrder: string
}

export function filterAndSortProducts(products: ProductCard[], filters: StoreFilterOptions) {
  const matchingProducts = products.filter((product) => {
    const matchesPlatform = filters.platform === 'all' || product.platform === filters.platform
    const matchesRegion = filters.region === 'all' || product.region === filters.region
    const matchesMinimumPrice = filters.minimumPriceCents === undefined
      || product.lowestPriceCents >= filters.minimumPriceCents
    const matchesMaximumPrice = filters.maximumPriceCents === undefined
      || product.lowestPriceCents <= filters.maximumPriceCents
    return matchesPlatform && matchesRegion && matchesMinimumPrice && matchesMaximumPrice
  })

  switch (filters.sortOrder) {
    case 'price-low':
      return matchingProducts.sort((first, second) => first.lowestPriceCents - second.lowestPriceCents)
    case 'price-high':
      return matchingProducts.sort((first, second) => second.lowestPriceCents - first.lowestPriceCents)
    case 'name':
      return matchingProducts.sort((first, second) => first.title.localeCompare(second.title))
    default:
      return matchingProducts
  }
}
