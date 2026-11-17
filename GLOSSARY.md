# Eneba-Style Marketplace

A digital-goods marketplace where buyers choose a seller's offer for a specific product and receive a digital key after successful payment.

## Accounts and cart

**Buyer account**:
A signed-in identity that owns Orders and can access its order history and digital keys.
_Avoid_: Session (when referring to the person/account)

**Cart**:
A guest or signed-in buyer's selection of distinct Offers awaiting checkout.
_Avoid_: Product-only selection

## Catalog and fulfillment

**Product**:
A specific purchasable edition or variant of a digital item, including its platform and region; different platform or region variants are separate Products.
_Avoid_: Game title (when referring to a purchasable variant)

**Seller**:
A marketplace participant whose Offers sell Products to buyers.
_Avoid_: Buyer

**Offer**:
A Seller's listing for a specific Product at a stated EUR price, backed by that Offer's digital-key inventory; a Seller has at most one active Offer per Product.
_Avoid_: Product (when referring to the Seller's listing)

**Digital key**:
A unique redeemable code associated with one Offer and delivered to one buyer after successful payment.
_Avoid_: Stock count

**Cart item**:
An entry in a buyer's cart that records the exact Offer the buyer selected.
_Avoid_: Product-only cart entry

**Order**:
A buyer's purchase record containing one or more Order items and preserving the Offer details and buyer-confirmed prices at purchase time.
_Avoid_: Live Offer (when referring to a completed purchase)

**Order item**:
One selected Product and Offer within an Order, fulfilled by one digital key after successful payment.
_Avoid_: Cart item (when referring to a completed purchase)

**Payment**:
A simulated attempt to pay for an entire Order; a declined Payment leaves the Order unpaid, and only a successful Payment permits its items to be fulfilled.
_Avoid_: Real-money charge
