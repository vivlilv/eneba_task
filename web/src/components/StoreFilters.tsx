export type StoreFilterName = 'platform' | 'region' | 'minPrice' | 'maxPrice' | 'sort'

type StoreFiltersProps = {
  platforms: string[]
  regions: string[]
  selectedPlatform: string
  selectedRegion: string
  minimumPrice: string
  maximumPrice: string
  sortOrder: string
  onChange: (name: StoreFilterName, value: string) => void
}

export function StoreFilters({ platforms, regions, selectedPlatform, selectedRegion, minimumPrice, maximumPrice, sortOrder, onChange }: StoreFiltersProps) {
  return (
    <div className="store-toolbar" role="group" aria-label="Filter and sort games">
      <label htmlFor="platform-filter">
        Platform
        <select id="platform-filter" value={selectedPlatform} onChange={(event) => onChange('platform', event.target.value)}>
          <option value="all">All platforms</option>
          {platforms.map((platform) => <option key={platform} value={platform}>{platform}</option>)}
        </select>
      </label>
      <label htmlFor="region-filter">
        Region
        <select id="region-filter" value={selectedRegion} onChange={(event) => onChange('region', event.target.value)}>
          <option value="all">All regions</option>
          {regions.map((region) => <option key={region} value={region}>{region}</option>)}
        </select>
      </label>
      <label htmlFor="minimum-price-filter">
        Min price (€)
        <input
          id="minimum-price-filter"
          type="number"
          min="0"
          step="0.01"
          inputMode="decimal"
          value={minimumPrice}
          onChange={(event) => onChange('minPrice', event.target.value)}
        />
      </label>
      <label htmlFor="maximum-price-filter">
        Max price (€)
        <input
          id="maximum-price-filter"
          type="number"
          min="0"
          step="0.01"
          inputMode="decimal"
          value={maximumPrice}
          onChange={(event) => onChange('maxPrice', event.target.value)}
        />
      </label>
      <label className="sort-control" htmlFor="sort-filter">
        Sort by
        <select id="sort-filter" value={sortOrder} onChange={(event) => onChange('sort', event.target.value)}>
          <option value="featured">Featured</option>
          <option value="price-low">Price: low to high</option>
          <option value="price-high">Price: high to low</option>
          <option value="name">Name</option>
        </select>
      </label>
    </div>
  )
}
