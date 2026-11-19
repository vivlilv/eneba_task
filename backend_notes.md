# Backend v1 cleanup log

Goal: keep the marketplace backend runnable while improving readability and pinning behavior with tests. Work one flow at a time; do not mix behavior changes into readability refactors.

## Explicit assignment shortcuts

- **Payment is simulated:** the current UI submits `outcome: "success"`; the API also supports `"declined"`. No payment provider, card details, or real charge is involved. The backend still records attempts and only fulfills an Order after success.
- **Inventory is synthetic:** seeded Digital keys are fictional and not redeemable.
- **Discovery is curated:** this version has homepage and Product detail discovery, not browse-all search/filter/sort or seller management. This keeps effort on the end-to-end purchase path, as the assignment permits a narrow, well-finished scope.

## Workflow for each substantial change

1. Trace the route, handler, helpers, SQL, and observable success/error behavior.
2. Add characterization tests against the existing implementation and run them first.
3. Refactor only the selected flow, preserving status codes, response shape, transaction boundaries, ordering, and side effects.
4. Rerun focused tests, then `go test ./...`, `go vet ./...`, formatting, and diff checks where available.
5. Record the scope, important design decisions, proof run, and remaining test gaps here.

Database-backed handler tests use PostgreSQL-specific SQL and create uniquely named scratch databases. They require a reachable PostgreSQL server and a role allowed to create databases; the tests skip if the server is unreachable. CI should provide `TEST_DATABASE_URL` so these tests actually execute rather than skip.

## Completed slices

### Homepage and Product reads

- `GET /api/homepage`: split the handler into `loadHomepageBlocks` and `attachBlockProducts`; tests pin ordering, product/offer aggregates, empty arrays, and database-error behavior.
- `GET /api/products/{slug}`: split the handler into `loadProductSummary` and `loadOffersForProduct`; tests pin active Offer filtering/order, price and key-availability summaries, zero-Offer defaults, 404, and database-error behavior.
- Decision: keep the two SQL stages separate because each has a distinct responsibility and error response. The handler now reads as orchestration; query helpers own row scanning and transformation.

### Payment key allocation (`POST /api/orders/{orderID}/pay`)

- Added characterization coverage for successful key assignment and idempotent retry, declined Payment leaving the Order unpaid and keys unassigned, and out-of-stock rollback without a recorded Payment attempt.
- Extracted `loadOrderItemsForPayment`, `findAvailableKeyForOffer`, and `assignDigitalKeys` from `handlePay`; `writePaymentResponse` now centralizes the repeated post-commit Order response.
- Decision: keep transaction ownership and HTTP error mapping in `handlePay`; helpers name the allocation steps and response loading. Preserve row locks, `SKIP LOCKED`, SQL conditions, error messages, and commit ordering.
- Verification: focused checkout/payment/Order-read tests passed before and after the `orders.go` refactor; `go test ./... -count=1` and `go vet ./...` passed.
- Remaining gap: no test yet for an Order with multiple items/offers where allocation runs out partway through, or injected query/scan failures inside the helper steps.

### Offer preview and Cart interactions

- `GET /api/offers/{offerID}` now has tests for active Offer data, free-key count, inactive/missing 404s, and database failure. Decision: keep `handleOfferPreview` as one compact query-and-map handler; another loader would add indirection without clarifying its single-query flow.
- Cart tests cover add/read/price-change/confirm/delete, stale/inactive/out-of-stock Offers, authentication, and merge deduplication/rollback.
- Current guest-merge rule: deduplicate normalized Offer IDs (keep the first seen price), then transactionally insert only existing active Offers. Merge does not validate stock or reprice; `loadCart` displays current stock/price changes and checkout revalidates before creating an Order.
- Extracted `loadCartOfferState` from `handleAddCartItem` so the inventory/price lookup and response-validation flow are distinct.
- Verification: focused Cart/Offer tests passed before and after the Cart extraction.

### Checkout (`POST /api/checkout`)

- Tests cover pending Order creation without key assignment, Order-item snapshots, idempotent retry, price-change confirmation, empty Cart, out-of-stock Cart, and required idempotency key.
- Extracted `summarizeCheckoutLines` and added a unit test for total calculation, price changes, and unavailable Offers. Persistence is split into `insertPendingOrder` and `insertOrderItems`.
- Decision: keep transaction ownership and response mapping in the handler; isolate pure summary logic and the two persistence responsibilities without changing their transaction boundary.
- Verification: focused Checkout tests passed before and after refactoring.

### Authentication and Order reads

- Authentication tests cover registration, normalized email, session creation, `/api/me`, logout invalidation, duplicate accounts, invalid registration, and wrong-password login.
- Refactored `auth.go` to share `decodeCredentials` between registration and login, keeping JSON decoding/email normalization together while each handler retains its distinct validation and response behavior.
- Focused auth tests passed before and after the refactor.
- Order-read tests cover pending Orders without keys, paid-only listing, key visibility after success, and access scoped to the owning Buyer account. `loadPaidOrderIDs` now owns the paid-ID query and row scan; `handleListOrders` orchestrates loading those Orders through shared `loadOrder`.

## Remaining priorities

1. Uncovered edge cases in Cart merging and multi-item payment allocation, if time allows.
2. Confirm authentication expiration/revocation and additional invalid-input cases only if they affect the demo flow.
3. Lower-risk health, startup, and migration checks only if time remains.

The end-to-end buyer lane now has tests across Product/Offer selection, Cart, checkout, Payment, and Order readback. Tests still primarily exercise handlers against PostgreSQL; lower-level unit tests are present for checkout-summary logic and shared helpers. Avoid testing every incidental helper or refactoring compact code without a readability benefit.

The highest-risk work is transaction and identity behavior. Add tests before restructuring those paths; avoid a backend-wide rewrite without a behavior boundary and a focused verification gate.
