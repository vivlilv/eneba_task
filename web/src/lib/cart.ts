import { readGuestCart, request } from '../api'
import type { Cart, CartItem, OfferPreview, User } from '../api'

export async function fetchCart(user: User | null): Promise<Cart> {
  if (user) return request<Cart>('/api/cart')

  const guestItems = readGuestCart()
  const previews = await Promise.all(guestItems.map(async (item): Promise<CartItem> => {
    const preview = await request<OfferPreview>(`/api/offers/${encodeURIComponent(item.offerId)}`)
    return {
      offerId: item.offerId,
      productSlug: preview.productSlug,
      productTitle: preview.productTitle,
      platform: preview.platform,
      region: preview.region,
      sellerName: preview.sellerName,
      priceCents: preview.priceCents,
      priceSeenCents: item.priceSeenCents,
      currency: preview.currency,
      availableKeys: preview.availableKeys,
      priceChanged: preview.priceCents !== item.priceSeenCents,
    }
  }))

  return {
    items: previews,
    totalCents: previews.reduce((sum, item) => sum + item.priceCents, 0),
    hasPriceChanges: previews.some((item) => item.priceChanged),
  }
}
