import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import type { ProductCard } from '../api'
import { formatPrice, request } from '../api'

export function SiteSearch() {
  const location = useLocation()
  const navigate = useNavigate()
  const urlQuery = new URLSearchParams(location.search).get('q') ?? ''
  const [draft, setDraft] = useState({ search: location.search, value: urlQuery })
  const query = draft.search === location.search ? draft.value : urlQuery
  const [suggestionState, setSuggestionState] = useState<{ query: string; products: ProductCard[] }>({ query: '', products: [] })
  const [focused, setFocused] = useState(false)

  useEffect(() => {
    const searchQuery = query.trim()
    if (!focused || searchQuery.length < 2) return

    const controller = new AbortController()
    const timeout = window.setTimeout(() => {
      request<{ products: ProductCard[] }>(
        `/api/search?q=${encodeURIComponent(searchQuery)}&limit=5`,
        { signal: controller.signal },
      )
        .then(({ products }) => {
          if (!controller.signal.aborted) setSuggestionState({ query: searchQuery, products })
        })
        .catch(() => {
          if (!controller.signal.aborted) setSuggestionState({ query: searchQuery, products: [] })
        })
    }, 200)

    return () => {
      window.clearTimeout(timeout)
      controller.abort()
    }
  }, [focused, query])

  function updateQuery(value: string) {
    setDraft({ search: location.search, value })
  }

  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const trimmedQuery = query.trim()
    setFocused(false)
    navigate(trimmedQuery ? `/store?q=${encodeURIComponent(trimmedQuery)}` : '/store')
  }

  function chooseSuggestion(product: ProductCard) {
    updateQuery(product.title)
    setFocused(false)
    navigate(`/products/${product.slug}`)
  }

  const normalizedQuery = query.trim()
  const showSuggestions = focused && normalizedQuery.length >= 2
  const suggestionsAreCurrent = suggestionState.query === normalizedQuery
  const searching = showSuggestions && !suggestionsAreCurrent
  const suggestions = suggestionsAreCurrent ? suggestionState.products : []

  return (
    <form className="site-search" role="search" onSubmit={search} onFocus={() => setFocused(true)}>
      <label className="visually-hidden" htmlFor="site-search-input">Search demo games</label>
      <input
        id="site-search-input"
        value={query}
        onChange={(event) => updateQuery(event.target.value)}
        onBlur={() => window.setTimeout(() => setFocused(false), 120)}
        placeholder="Search demo games"
        autoComplete="off"
        aria-autocomplete="list"
        aria-expanded={showSuggestions}
        aria-controls="search-suggestions"
      />
      <button type="submit" aria-label="Search"><svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="10.8" cy="10.8" r="6.3" /><path d="m15.5 15.5 4.2 4.2" /></svg></button>
      {showSuggestions && (
        <div className="search-suggestions" id="search-suggestions" role="listbox" aria-label="Suggested products">
          {searching ? <div className="search-suggestion-status" role="status">Searching…</div> : suggestions.length > 0 ? (
            suggestions.map((product) => (
              <button key={product.slug} type="button" role="option" onClick={() => chooseSuggestion(product)}>
                <strong>{product.title}</strong>
                <span>{product.platform} · From {formatPrice(product.lowestPriceCents, product.currency)}</span>
              </button>
            ))
          ) : <div className="search-suggestion-status">No matching games</div>}
        </div>
      )}
    </form>
  )
}
