export type User = { id: number; email: string }

export type ProductCard = {
  id: string
  slug: string
  title: string
  edition: string
  platform: string
  region: string
  category: string
  lowestPriceCents: number
  currency: string
  available: boolean
  offerCount: number
}

export type HomepageBlock = {
  id: string
  title: string
  kind: string
  eyebrow: string
  body: string
  ctaLabel: string
  ctaUrl: string
  products: ProductCard[]
}

export type Offer = {
  id: string
  sellerName: string
  sellerRating: number
  priceCents: number
  currency: string
  availableKeys: number
  available: boolean
}

export type Product = ProductCard & {
  description: string
  offers: Offer[]
}

export type OfferPreview = {
  id: string
  productSlug: string
  productTitle: string
  platform: string
  region: string
  sellerName: string
  priceCents: number
  currency: string
  availableKeys: number
  available: boolean
}

export type CartItem = {
  offerId: string
  productSlug: string
  productTitle: string
  platform: string
  region: string
  sellerName: string
  priceCents: number
  priceSeenCents: number
  currency: string
  availableKeys: number
  priceChanged: boolean
}

export type Cart = {
  items: CartItem[]
  totalCents: number
  hasPriceChanges: boolean
}

export type OrderItem = {
  id: number
  productSlug: string
  productTitle: string
  offerId: string
  sellerName: string
  unitPriceCents: number
  currency: string
  digitalKey?: string
}

export type Order = {
  id: string
  status: 'pending_payment' | 'paid'
  currency: string
  totalCents: number
  createdAt: string
  items: OrderItem[]
}

export type PaymentResult = {
  paymentStatus: 'succeeded' | 'declined'
  order: Order
}

export type APIErrorPayload = {
  error: string
  code?: string
  details?: unknown
}

export class APIError extends Error {
  readonly status: number
  readonly code?: string
  readonly details?: unknown

  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message)
    this.name = 'APIError'
    this.status = status
    this.code = code
    this.details = details
  }
}

export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')

  const response = await fetch(path, {
    ...init,
    headers,
    credentials: 'same-origin',
  })
  if (response.status === 204) return undefined as T

  const payload = (await response.json().catch(() => ({}))) as APIErrorPayload
  if (!response.ok) {
    throw new APIError(payload.error || `Request failed (${response.status})`, response.status, payload.code, payload.details)
  }
  return payload as T
}

export type GuestCartItem = { offerId: string; priceSeenCents: number }

const GUEST_CART_KEY = 'eneba-demo-guest-cart'

export function readGuestCart(): GuestCartItem[] {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(GUEST_CART_KEY) ?? '[]')
    if (!Array.isArray(value)) return []
    return value.filter((item): item is GuestCartItem =>
      typeof item?.offerId === 'string' && typeof item?.priceSeenCents === 'number',
    )
  } catch {
    return []
  }
}

export function writeGuestCart(items: GuestCartItem[]) {
  localStorage.setItem(GUEST_CART_KEY, JSON.stringify(items))
}

export function clearGuestCart() {
  localStorage.removeItem(GUEST_CART_KEY)
}

export function formatPrice(cents: number, currency = 'EUR') {
  return new Intl.NumberFormat('en-IE', { style: 'currency', currency }).format(cents / 100)
}
