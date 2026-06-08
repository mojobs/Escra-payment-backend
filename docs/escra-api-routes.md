# ESCRA API Routes Reference
Version: 2026-06-05
Audience: Mobile app developer
Base URL: `/api/v1`

## General rules
- All fiat money uses integer minor units.
- Example: `100000` means `NGN 1000.00`.
- Protected routes require `Authorization: Bearer <access_token>`.
- Protected `POST` routes also require `Idempotency-Key: <uuid>`.
- Admin routes require `X-Admin-Key: <admin_key>`.
- Common error shape:
- `error`
- `message`

## Public routes

### Health check
- `GET /health`
- auth: no
- purpose: backend health check

### Register
- `POST /api/v1/auth/register`
- auth: no
- body:
- `phone`
- `password`
- `first_name`
- `last_name`
- `pin`
- `email` optional
- `role` optional (`buyer` or `seller`)
- returns:
- `user`
- `access_token`
- `refresh_token`

### Login
- `POST /api/v1/auth/login`
- auth: no
- body:
- `phone`
- `password`
- returns:
- `user`
- `access_token`
- `refresh_token`

### Public escrow order preview
- `GET /api/v1/public/escrows/orders/:reference`
- auth: no
- purpose: preview a seller-created escrow order before buyer login
- returns:
- `reference`
- `seller_name`
- `title`
- `description`
- `amount_kobo`
- `currency`
- `sales_channel`
- `delivery_mode`
- `status`
- `buyer_confirmation_ttl_hours`
- `cross_border`
- `settlement_currency`
- `fx_locked_rate`
- `fx_quote_reference`
- `created_at`

### Kora webhook
- `POST /api/v1/webhooks/kora`
- auth: no
- purpose: receives Kora webhook events
- note: backend verifies `x-korapay-signature`

### Quidax webhook
- `POST /api/v1/webhooks/quidax`
- auth: no
- purpose: receives Quidax webhook events
- note: backend verifies `quidax-signature`

## Authenticated user routes

### Get profile
- `GET /api/v1/users/profile`
- auth: yes
- returns:
- `id`
- `first_name`
- `last_name`
- `phone`
- `email`
- `role`
- `status`
- `kyc_status`
- `metrics`
- `merchant_details`

### Update merchant business details
- `PUT /api/v1/users/profile/business`
- auth: yes
- purpose: stores seller business particulars for profile and trust screens
- body:
- `business_name` optional
- `business_type` optional
- `rc_number` optional
- `website` optional
- `address` optional
- `city` optional
- `country` optional
- `support_phone` optional
- returns:
- `success`
- `message`

### Get wallet
- `GET /api/v1/wallets/`
- auth: yes
- returns: current user wallet object

### Get wallet balance
- `GET /api/v1/wallets/balance`
- auth: yes
- returns:
- `balance_kobo`
- `balance_formatted`
- `currency`

### Start Kora wallet checkout
- `POST /api/v1/wallets/checkout/kora`
- auth: yes
- idempotency: yes
- purpose: generates a Kora checkout URL for direct wallet funding
- body:
- `amount_kobo`
- `redirect_url` optional
- `narration` optional
- returns:
- `success`
- `checkout_url`
- `reference`
- note: wallet balance is credited only after the Kora webhook confirms payment success

### Get limits
- `GET /api/v1/limits/`
- auth: yes
- returns: current daily and monthly transaction limits and usage

## Internal transfer routes

### Transfer to another ESCRA user wallet
- `POST /api/v1/transactions/transfer`
- auth: yes
- idempotency: yes
- body:
- `wallet_id`
- `amount_kobo`
- `description` optional
- `pin`
- returns:
- `success`
- `transaction`
- `new_balance_kobo`

### Transaction history
- `GET /api/v1/transactions/history`
- auth: yes
- returns: object containing `success`, `transactions`, and `count`

### Transaction by reference
- `GET /api/v1/transactions/:reference`
- auth: yes
- returns: one transaction scoped to the authenticated user

## Escrow routes

### Create escrow order
- `POST /api/v1/escrows/orders`
- auth: yes
- idempotency: yes
- seller action
- body:
- `title`
- `description` optional
- `amount_kobo`
- `currency`
- `sales_channel` optional
- `delivery_mode` optional
- `buyer_confirmation_ttl_hours` optional
- `cross_border`
- `settlement_currency` optional
- `fx_locked_rate` optional
- `fx_quote_reference` optional
- `metadata` optional
- returns: full escrow order object

### List my escrow orders
- `GET /api/v1/escrows/orders`
- auth: yes
- returns:
- `success`
- `orders`
- `count`

### Get escrow order by reference
- `GET /api/v1/escrows/orders/:reference`
- auth: yes
- returns: full escrow order object

### Start Kora checkout for escrow
- `POST /api/v1/escrows/orders/:reference/checkout/kora`
- auth: yes
- idempotency: yes
- buyer action
- body:
- `redirect_url` optional
- `notification_url` optional
- `channels` optional
- `default_channel` optional
- `merchant_bears_cost`
- returns:
- `order`
- `provider`
- `provider_reference`
- `checkout_url`
- `checkout_reference`
- `checkout_status`
- `redirect_url`
- `notification_url`
- `merchant_bears_cost`
- `default_channel`
- `channels`
- `provider_transaction`

### Fund escrow from internal ESCRA wallet
- `POST /api/v1/escrows/orders/:reference/fund`
- auth: yes
- idempotency: yes
- buyer action
- body:
- `pin`
- returns:
- `order`
- `delivery_code`

### Mark order shipped
- `POST /api/v1/escrows/orders/:reference/ship`
- auth: yes
- idempotency: yes
- seller action
- body:
- `tracking_reference` optional
- `delivery_proof_url` optional
- `note` optional
- returns: full escrow order object

### Mark order delivered
- `POST /api/v1/escrows/orders/:reference/mark-delivered`
- auth: yes
- idempotency: yes
- seller action
- body:
- `delivery_proof_url` optional
- `note` optional
- returns: full escrow order object

### Confirm delivery
- `POST /api/v1/escrows/orders/:reference/confirm-delivery`
- auth: yes
- idempotency: yes
- buyer action
- body:
- `delivery_code` optional
- `pin` optional
- note:
- send either `delivery_code` or `pin`
- returns: full escrow order object

### Auto release after deadline
- `POST /api/v1/escrows/orders/:reference/release`
- auth: yes
- idempotency: yes
- actor: buyer or seller
- body: empty object allowed
- purpose: releases escrow after confirmation deadline if eligible
- returns: full escrow order object

### Open dispute
- `POST /api/v1/escrows/orders/:reference/dispute`
- auth: yes
- idempotency: yes
- buyer action
- body:
- `reason`
- `details` optional
- `evidence_url` optional
- returns: full escrow order object

### Cancel unfunded order
- `POST /api/v1/escrows/orders/:reference/cancel`
- auth: yes
- idempotency: yes
- seller action
- body:
- `reason` optional
- returns: full escrow order object

### Refund escrow to buyer wallet
- `POST /api/v1/escrows/orders/:reference/refund`
- auth: yes
- idempotency: yes
- seller action
- body:
- `reason` optional
- note:
- refund goes back to the buyer ESCRA wallet
- returns: full escrow order object

## Kora provider routes

### Verify Kora KYC
- `POST /api/v1/providers/kora/kyc/verify`
- auth: yes
- idempotency: yes
- body:
- `id_number`
- `id_type`
- allowed `id_type` values:
- `BVN`
- `NIN`
- purpose:
- verifies the user with Kora and marks them as KYC verified before escrow, pay-in, and payout actions

### List supported banks
- `GET /api/v1/providers/kora/banks?countryCode=NG`
- auth: yes
- note:
- uses the Kora public key
- returns:
- `success`
- `banks`

### Resolve bank account
- `GET /api/v1/providers/kora/banks/resolve?bank_code=044&account_number=0123456789&currency=NG`
- auth: yes
- returns:
- `bank_name`
- `bank_code`
- `account_number`
- `account_name`

### Get Kora balances
- `GET /api/v1/providers/kora/balances`
- auth: yes
- purpose:
- reconciliation and provider-side fund tracking
- returns:
- `provider`
- `balances`

### Create Kora virtual account
- `POST /api/v1/providers/kora/virtual-accounts`
- auth: yes
- idempotency: yes
- body:
- `escrow_reference` optional
- `account_name` optional
- `bank_code` optional
- `currency` optional
- `id_number`
- `id_type`
- `permanent`
- purpose:
- creates a Kora virtual bank account for pay-ins
- if `escrow_reference` is supplied, successful webhook funding maps to that escrow order

### List my Kora virtual accounts
- `GET /api/v1/providers/kora/virtual-accounts`
- auth: yes
- returns:
- `success`
- `accounts`
- `count`

### Initiate bank payout
- `POST /api/v1/providers/kora/payouts/bank`
- auth: yes
- idempotency: yes
- body:
- `bank_code`
- `account_number`
- `account_name` optional
- `customer_email` optional
- `amount_kobo`
- `currency` optional
- `narration` optional
- `pin`
- returns: provider transaction response
- note:
- user must be KYC verified

## Quidax provider routes

### Initiate crypto withdrawal
- `POST /api/v1/providers/quidax/withdrawals`
- auth: yes
- idempotency: yes
- body:
- `currency`
- `amount`
- `fund_uid`
- `fund_uid2` optional
- `network` optional
- `transaction_note` optional
- `narration` optional
- `provider_user_id` optional
- `pin`
- returns: provider transaction response

## Admin routes

### Resolve escrow dispute
- `POST /api/v1/admin/escrows/orders/:reference/resolve-dispute`
- auth: admin header
- headers:
- `X-Admin-Key`
- body:
- `action`
- `resolution_note` optional
- allowed `action` values:
- `RELEASE`
- `REFUND`
- returns: full escrow order object

### Initiate Kora refund
- `POST /api/v1/admin/providers/kora/refunds`
- auth: admin header
- headers:
- `X-Admin-Key`
- body:
- `payment_reference`
- `amount_kobo`
- `currency` optional
- `reason` optional
- returns: provider transaction response

## Main response objects

### Escrow order object
- `id`
- `reference`
- `seller_id`
- `buyer_id` optional
- `buyer_name` optional
- `buyer_email` optional
- `buyer_phone` optional
- `amount_kobo`
- `currency`
- `title`
- `description`
- `sales_channel`
- `delivery_mode`
- `status`
- `tracking_reference`
- `delivery_proof_url`
- `buyer_confirmation_ttl_hours`
- `cross_border`
- `settlement_currency`
- `fx_locked_rate`
- `fx_quote_reference`
- `funded_at`
- `shipped_at`
- `delivered_at`
- `confirmation_deadline`
- `released_at`
- `disputed_at`
- `cancelled_at`
- `refunded_at`
- `created_at`
- `updated_at`
- `events`
- `disputes`

### Escrow event object
- `action`
- `note`
- `created_at`

### Escrow dispute object
- `id`
- `reason`
- `details`
- `evidence_url`
- `status`
- `created_at`
- `resolved_at`

### Provider transaction object
- `id`
- `provider`
- `reference`
- `external_reference`
- `type`
- `status`
- `amount_kobo`
- `amount`
- `currency`
- `created_at`

## Escrow status values
- `CREATED`
- `FUNDED`
- `SHIPPED`
- `DELIVERY_PENDING_CONFIRMATION`
- `DISPUTED`
- `RELEASED`
- `REFUNDED`
- `CANCELLED`

## Mobile implementation notes
- Kora checkout init success does not mean order is funded yet.
- The order is truly funded only after webhook confirmation updates it to `FUNDED`.
- Seller bank payout is separate from escrow release.
- Some endpoints return wrapped responses and some return direct objects, so normalize responses in the mobile API layer.
- For protected `POST` routes, always generate an idempotency key and reuse it only when retrying the exact same request.
