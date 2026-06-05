# ESCRA Mobile Developer Handoff
Version: 2026-06-05
Audience: Mobile app developer
Product: ESCRA social commerce escrow
Owner: Backend and product team

## Executive summary
ESCRA is an escrow-powered trust layer for social commerce in Africa. The first version is focused on Nigerian SMEs and their buyers who currently trade over WhatsApp, Instagram, Telegram, and similar channels where fake payment proof and fake merchants are common.

The app is not just a wallet. The product promise is:

1. When ESCRA says an order is funded, the seller should trust that the money is real.
2. When ESCRA holds the order in escrow, the buyer should trust that the seller cannot cash out too early.
3. Both parties should be able to see the exact order state from creation through delivery, dispute, release, refund, or cancellation.

The backend now supports:

- registration and login
- JWT auth
- wallet creation and balances
- internal wallet transfer
- escrow order creation
- public order preview for shared links
- authenticated buyer checkout initialization through Kora
- webhook-driven escrow funding from Kora checkout success
- seller shipping and delivery updates
- buyer confirmation by delivery code or PIN
- disputes
- admin dispute resolution to release or refund
- seller bank payout initiation through Kora
- provider transaction tracking
- webhook recording and idempotent processing

The backend does not yet support:

- guest checkout without authentication
- file upload handling for proof or evidence
- push notifications
- original-rail refunds back to card or bank instead of ESCRA wallet credit
- merchant KYC flows in-app
- internal crypto balances
- full cross-border FX quote and lock orchestration

## Product definition
ESCRA is a social-commerce escrow platform.

Kora is the fiat payment rail:

- buyer checkout collection
- bank metadata lookup
- bank account resolution
- seller withdrawal and payout
- payment and payout webhook callbacks

Quidax is still a future-facing rail in the mobile story:

- provider plumbing exists in the backend
- crypto withdrawal plumbing exists
- the consumer-facing crypto wallet product is not ready yet

For the mobile app right now, the main product is fiat escrow.

## Core product workflow
The intended end-to-end flow is:

1. Seller creates an escrow order.
2. Seller shares the order reference or public order link.
3. Buyer opens the public order preview.
4. Buyer signs in or registers.
5. Buyer starts Kora checkout for that order.
6. Kora completes payment.
7. Kora webhook hits ESCRA.
8. ESCRA marks the order as FUNDED and moves the value into the escrow ledger.
9. Seller marks the order as SHIPPED.
10. Seller marks the order as DELIVERED.
11. Buyer confirms delivery with either:
- a six-digit delivery code, for wallet-funded flow
- or their transaction PIN, which is important for Kora-funded flow
12. ESCRA releases funds from escrow wallet to seller wallet.
13. Seller can withdraw to bank through Kora payout.

Dispute path:

1. Buyer opens a dispute before release.
2. Order status becomes DISPUTED.
3. Admin resolves the dispute:
- RELEASE
- or REFUND

Refund path:

- refund moves value from escrow wallet back to buyer ESCRA wallet
- refund does not yet reverse to the original external payment rail

## Important architecture note for mobile
Do not assume the mobile app talks directly to Kora or Quidax.

The mobile app should only talk to the ESCRA backend.

The flow is:

1. Mobile app calls ESCRA API.
2. ESCRA API calls Kora or Quidax where needed.
3. Provider webhooks return to ESCRA.
4. ESCRA updates internal state.
5. Mobile app polls or refreshes order state.

## User roles
Seller:

- creates escrow orders
- views own escrow list
- marks orders shipped
- marks orders delivered
- cancels an unfunded order
- refunds a funded order
- withdraws released balance to bank

Buyer:

- previews an order
- signs in or registers
- starts Kora checkout
- views own escrow list
- confirms delivery
- opens dispute before release

Admin:

- resolves disputes through a protected admin endpoint
- not yet represented as a separate mobile user role

System:

- owns escrow and provider inflow wallets
- records ledger transactions
- stores provider transactions
- processes webhooks idempotently

## Authentication and session rules
Register:

- route: POST /api/v1/auth/register
- input:
- phone
- password
- first_name
- last_name
- pin
- optional email

Login:

- route: POST /api/v1/auth/login
- input:
- phone
- password

Profile:

- route: GET /api/v1/users/profile

Mobile auth guidance:

- email should be collected at onboarding because Kora checkout and payout flows depend on it
- password is for login
- PIN is for money movement and delivery confirmation fallback
- store access and refresh tokens in secure storage
- do not store the PIN

## Money and API conventions
All fiat money uses integer minor units.

Examples:

- 100000 means NGN 1000.00
- 150000 means NGN 1500.00

Rules:

- always send amount_kobo for fiat money
- never send floats
- protected POST requests need Authorization Bearer token and Idempotency-Key
- reuse the same Idempotency-Key only when retrying the exact same request
- timestamps are UTC

## Route map
Base URL:

- /api/v1

Public routes:

- GET /health
- POST /api/v1/auth/register
- POST /api/v1/auth/login
- POST /api/v1/webhooks/kora
- POST /api/v1/webhooks/quidax
- GET /api/v1/public/escrows/orders/:reference

Protected general routes:

- GET /api/v1/users/profile
- GET /api/v1/wallets/
- GET /api/v1/wallets/balance
- GET /api/v1/transactions/history
- GET /api/v1/transactions/:reference
- POST /api/v1/transactions/transfer
- GET /api/v1/limits/

Protected escrow routes:

- POST /api/v1/escrows/orders
- GET /api/v1/escrows/orders
- GET /api/v1/escrows/orders/:reference
- POST /api/v1/escrows/orders/:reference/checkout/kora
- POST /api/v1/escrows/orders/:reference/fund
- POST /api/v1/escrows/orders/:reference/ship
- POST /api/v1/escrows/orders/:reference/mark-delivered
- POST /api/v1/escrows/orders/:reference/confirm-delivery
- POST /api/v1/escrows/orders/:reference/release
- POST /api/v1/escrows/orders/:reference/dispute
- POST /api/v1/escrows/orders/:reference/cancel
- POST /api/v1/escrows/orders/:reference/refund

Protected provider routes:

- GET /api/v1/providers/kora/banks
- GET /api/v1/providers/kora/banks/resolve
- POST /api/v1/providers/kora/payouts/bank
- POST /api/v1/providers/quidax/withdrawals

Admin route:

- POST /api/v1/admin/escrows/orders/:reference/resolve-dispute

The admin route requires:

- X-Admin-Key

It is for operations tooling, not standard buyer or seller UI.

## Escrow statuses
CREATED:

- seller created order
- no buyer funding completed yet
- seller can cancel
- public order preview is available

FUNDED:

- buyer payment is confirmed
- money is in escrow wallet
- seller can ship, deliver, or refund

SHIPPED:

- seller marked order as shipped
- tracking reference may exist

DELIVERY_PENDING_CONFIRMATION:

- seller marked order as delivered
- buyer can confirm or dispute
- auto-release countdown is active

DISPUTED:

- buyer raised dispute before release
- seller cannot get funds until admin resolves it

RELEASED:

- funds moved from escrow wallet to seller ESCRA wallet

REFUNDED:

- funds moved from escrow wallet back to buyer ESCRA wallet

CANCELLED:

- seller cancelled before funding

## Core objects the mobile app will see
User:

- id
- first_name
- last_name
- phone
- optional email
- status

Wallet:

- id
- balance_kobo
- currency
- owner_type
- status

Transaction:

- reference
- amount_kobo
- type
- status
- description
- created_at

Escrow order:

- reference
- seller_id
- optional buyer_id
- buyer_name
- buyer_email
- buyer_phone
- amount_kobo
- currency
- title
- description
- sales_channel
- delivery_mode
- status
- tracking_reference
- delivery_proof_url
- buyer_confirmation_ttl_hours
- cross_border
- settlement_currency
- fx_locked_rate
- fx_quote_reference
- funded_at
- shipped_at
- delivered_at
- confirmation_deadline
- released_at
- disputed_at
- cancelled_at
- refunded_at
- created_at
- updated_at
- events
- disputes

Escrow event:

- action
- note
- created_at

Escrow dispute:

- id
- reason
- details
- evidence_url
- status
- created_at
- resolved_at

Provider transaction:

- provider
- reference
- external_reference
- type
- status
- amount_kobo or amount text
- currency
- created_at

## Main mobile flows
### Seller creates order
Route:

- POST /api/v1/escrows/orders

Input:

- title
- description
- amount_kobo
- currency
- sales_channel
- delivery_mode
- buyer_confirmation_ttl_hours
- cross_border
- settlement_currency
- fx_locked_rate
- fx_quote_reference
- metadata

Recommended UI:

- create order form
- success screen with order reference and share action
- share public order link using the public order route

### Buyer views public order
Route:

- GET /api/v1/public/escrows/orders/:reference

Purpose:

- lets buyer preview order before authentication or checkout

Recommended UI:

- seller name
- title
- description
- amount
- currency
- delivery mode
- current status
- call to action to continue by signing in

Important limitation:

- preview is public
- checkout is not guest checkout yet
- buyer still needs an ESCRA account and token to start Kora checkout

### Buyer starts Kora checkout
Route:

- POST /api/v1/escrows/orders/:reference/checkout/kora

Input:

- redirect_url
- optional notification_url
- optional channels
- optional default_channel
- merchant_bears_cost

Response includes:

- order
- provider
- provider_reference
- checkout_url
- checkout_reference
- checkout_status
- provider_transaction

Recommended mobile behavior:

1. buyer taps Pay Securely
2. app calls this endpoint
3. app opens checkout_url in in-app browser or external browser
4. after payment, app returns to order detail and refreshes status

Important note:

- the order becomes truly FUNDED only after Kora webhook confirmation
- do not mark it paid just because checkout initialization succeeded

### Wallet funding fallback
Route:

- POST /api/v1/escrows/orders/:reference/fund

Input:

- pin

Purpose:

- older internal-wallet funding path
- useful for testing and backup flows

Response:

- order
- delivery_code

Important note:

- this returns a six-digit delivery code
- Kora checkout path does not return a delivery code

### Seller marks shipped
Route:

- POST /api/v1/escrows/orders/:reference/ship

Input:

- tracking_reference
- delivery_proof_url
- note

### Seller marks delivered
Route:

- POST /api/v1/escrows/orders/:reference/mark-delivered

Input:

- delivery_proof_url
- note

This starts the buyer confirmation window.

### Buyer confirms delivery
Route:

- POST /api/v1/escrows/orders/:reference/confirm-delivery

Input:

- delivery_code
- or pin

Recommended mobile behavior:

- if order was internally funded and delivery code exists, allow code entry
- always allow PIN confirmation fallback

### Buyer opens dispute
Route:

- POST /api/v1/escrows/orders/:reference/dispute

Input:

- reason
- details
- evidence_url

Recommended mobile behavior:

- buyer picks a reason
- buyer adds details
- buyer optionally pastes proof URL for now
- status becomes DISPUTED

### Seller refunds
Route:

- POST /api/v1/escrows/orders/:reference/refund

Input:

- reason

Important note:

- this credits the buyer ESCRA wallet
- it does not yet return money to the original card or bank rail

### Auto release
Route:

- POST /api/v1/escrows/orders/:reference/release

Purpose:

- can be triggered after confirmation_deadline has passed and no dispute exists

### Seller withdraws released balance
Route:

- POST /api/v1/providers/kora/payouts/bank

Input:

- bank_code
- account_number
- optional account_name
- optional customer_email
- amount_kobo
- currency
- narration
- pin

Important note:

- releasing escrow to seller wallet does not automatically pay seller bank account
- bank payout is a separate step

## Suggested mobile screen map
Unauthenticated:

- splash
- onboarding
- login
- register
- public order preview

Authenticated common:

- home dashboard
- wallet
- transaction history
- profile

Seller screens:

- create escrow order
- seller escrow list
- seller escrow detail
- ship order
- mark delivered
- request refund
- withdraw to bank

Buyer screens:

- buyer escrow list
- buyer escrow detail
- secure checkout launcher
- confirm delivery
- open dispute

## Recommended order detail UI
Sections:

- order summary
- amount and currency
- seller or buyer counterparty
- current status badge
- activity timeline from events
- proof section
- dispute section
- next action button

Suggested action by status:

CREATED:

- seller sees Cancel
- buyer sees Pay Securely after login

FUNDED:

- seller sees Mark Shipped
- buyer sees Waiting for seller

SHIPPED:

- seller sees Mark Delivered
- buyer sees Waiting for delivery

DELIVERY_PENDING_CONFIRMATION:

- buyer sees Confirm Delivery and Raise Dispute
- seller sees countdown to auto release

DISPUTED:

- both sides see funds on hold

RELEASED:

- seller sees Withdraw to Bank if desired

REFUNDED:

- buyer sees wallet credited

## Response-shape caution
The backend is usable but not perfectly envelope-consistent.

Examples:

- some endpoints return a direct object
- some return success plus data arrays

The mobile API layer should normalize responses internally rather than assuming one global response wrapper.

## Headers and retry rules
Protected GET:

- Authorization: Bearer token

Protected POST:

- Authorization: Bearer token
- Idempotency-Key: uuid

Admin POST:

- X-Admin-Key

Retry guidance:

- retry network-failed POST requests with the same Idempotency-Key if the request body is identical
- use a fresh Idempotency-Key for a new action

## Kora-specific mobile notes
The Kora bank endpoints are backend-to-provider routes.

Use these for bank withdrawal UI:

- GET /api/v1/providers/kora/banks
- GET /api/v1/providers/kora/banks/resolve

The checkout route returns a Kora checkout_url.

Recommended mobile implementation:

- open the returned checkout_url
- on return, refresh the order detail
- do not assume immediate success until webhook updates order to FUNDED

## Known backend limitations the mobile app should design around
1. Guest checkout is not available yet.

2. File uploads are not available yet.
- delivery_proof_url and evidence_url are still URL fields, not upload sessions

3. Refunds go to buyer ESCRA wallet, not original external payment rail.

4. Seller bank payout is separate from escrow release.

5. Admin dispute tooling is backend-only right now.

6. Cross-border and crypto are not ready for first mobile release.

7. Kora environment setup must be valid in deployment:
- correct secret key
- working webhook URL
- valid PUBLIC_BASE_URL if the backend should derive notification URL automatically

## Suggested mobile implementation order
1. auth and secure session storage
2. wallet and profile
3. seller create-order flow
4. public order preview
5. buyer sign-in and Kora checkout launcher
6. order detail with timeline and status badges
7. seller ship and deliver actions
8. buyer confirm or dispute actions
9. seller withdrawal to bank

## Short implementation checklist
- build an API client that always supports Authorization and Idempotency-Key injection
- normalize mixed response envelopes
- treat escrow status as the source of truth for UI state
- refresh order detail after checkout return
- do not hardcode delivery code as the only confirmation path
- collect and preserve email for users
- design with public order preview plus authenticated checkout
- assume disputes freeze payout

## Contact assumptions for backend collaboration
If the mobile team needs backend clarification, the most likely areas of confusion will be:

- which actions are public versus authenticated
- when an order is truly FUNDED
- difference between RELEASED and bank-withdrawn
- why refunds return to ESCRA wallet for now
- how Idempotency-Key should be reused

This document reflects the backend behavior as of 2026-06-05.
