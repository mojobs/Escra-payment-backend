# ESCRA API Reference

Last updated: June 8, 2026

ESCRA is an escrow-powered payment backend for SME commerce across WhatsApp, Instagram, websites, and other informal sales channels. The core goal is to protect buyers from fake merchants while giving genuine sellers a clear path to receive funds after delivery is confirmed.

This document is written for the mobile app developer.

## Base URL

Local development:

```text
http://localhost:8080
```

API prefix:

```text
/api/v1
```

Health check:

```text
GET /health
```

## Core Rules

1. KYC must happen before escrow and provider payment actions.
2. Protected routes require a bearer token from login.
3. Money values are sent as integer minor units.
4. For NGN, use kobo. Example: `100000` means `NGN 1,000.00`.
5. Most protected `POST` routes should include an idempotency key to prevent duplicate payments or duplicate state changes.
6. Kora webhooks are the source of truth for pay-in confirmation.
7. Funds should only be released to sellers after delivery confirmation, automatic expiry, or admin dispute resolution.

## Global Headers

Protected user routes:

```http
Authorization: Bearer <access_token>
Content-Type: application/json
```

Protected mutation routes:

```http
Authorization: Bearer <access_token>
Content-Type: application/json
Idempotency-Key: <uuid-v4>
```

Admin routes:

```http
X-Admin-Key: <admin_api_key>
Content-Type: application/json
```

Kora webhook:

```http
Content-Type: application/json
```

Kora may also send a signature header depending on the configured webhook verification method.

## Common Error Response

```json
{
  "error": "machine_readable_error_code",
  "message": "Human readable message"
}
```

## Recommended Mobile Flow

1. Register the user.
2. Log the user in and store the access token securely.
3. Complete KYC verification.
4. Seller creates an escrow order.
5. Buyer opens the public order preview or authenticated order details.
6. Buyer pays through Kora checkout or virtual bank account.
7. Backend receives Kora webhook and marks the escrow as funded.
8. Seller ships or delivers the product/service.
9. Buyer confirms delivery.
10. Backend releases funds to the seller.
11. If there is a problem, buyer opens a dispute.
12. Admin resolves dispute by releasing funds to seller or refunding buyer.

## Escrow Statuses

Common escrow lifecycle:

```text
CREATED
AWAITING_PAYMENT
FUNDED
SHIPPED
DELIVERED
RELEASED
DISPUTED
CANCELLED
REFUNDED
EXPIRED
```

The mobile app should display status labels in a buyer-friendly and seller-friendly way.

## Authentication

### Register

```http
POST /api/v1/auth/register
```

Auth: public

Request body:

```json
{
  "phone": "08012345678",
  "email": "buyer@example.com",
  "password": "StrongPassword123",
  "pin": "1234",
  "first_name": "Ada",
  "last_name": "Okafor",
  "role": "buyer"
}
```

Response:

```json
{
  "user": {
    "id": 1,
    "phone": "08012345678",
    "email": "buyer@example.com",
    "first_name": "Ada",
    "last_name": "Okafor",
    "role": "buyer",
    "status": "ACTIVE"
  },
  "access_token": "<jwt_token>"
}
```

### Login

```http
POST /api/v1/auth/login
```

Auth: public

Request body:

```json
{
  "phone": "08012345678",
  "password": "StrongPassword123"
}
```

Response:

```json
{
  "user": {
    "id": 1,
    "phone": "08012345678",
    "email": "buyer@example.com",
    "first_name": "Ada",
    "last_name": "Okafor",
    "role": "buyer",
    "status": "ACTIVE"
  },
  "access_token": "<jwt_token>"
}
```

## User

### Get Profile

```http
GET /api/v1/users/profile
```

Auth: bearer token required

Response:

```json
{
  "id": 1,
  "phone": "08012345678",
  "email": "buyer@example.com",
  "first_name": "Ada",
  "last_name": "Okafor",
  "role": "buyer",
  "status": "ACTIVE",
  "kyc_status": "VERIFIED",
  "metrics": {
    "completed_orders": 4,
    "trust_score": 100,
    "sales_volume_kobo": 0,
    "rating": 0
  },
  "merchant_details": {}
}
```

### Update Merchant Business Details

```http
PUT /api/v1/users/profile/business
```

Auth: bearer token required

Request body:

```json
{
  "business_name": "Horology House Ltd.",
  "business_type": "Private Limited Company",
  "rc_number": "RC 1782940",
  "website": "www.horologyhouse.com",
  "address": "12 Marina, Lagos Island",
  "city": "Lagos",
  "country": "Nigeria",
  "support_phone": "08012345678"
}
```

Response:

```json
{
  "success": true,
  "message": "Merchant profile details updated successfully"
}
```

### Upload KYC Document

```http
POST /api/v1/users/kyc/upload
```

Auth: bearer token required

Content type:

```http
multipart/form-data
```

Fields:

```text
document_type=CAC_CERTIFICATE
file=<PDF/JPG/JPEG/PNG binary file, max 10MB>
```

Allowed `document_type` values:

```text
CAC_CERTIFICATE
PASSPORT
```

Response:

```json
{
  "success": true,
  "document_url": "http://localhost:8080/uploads/kyc/docs/user_d4176c58_1780950000000000000_cac_certificate.pdf",
  "status": "UNDER_REVIEW"
}
```

After upload, the user's account `status` remains `ACTIVE`, while `kyc_status` becomes `UNDER_REVIEW`.

## KYC

### Verify Identity With Kora

```http
POST /api/v1/providers/kora/kyc/verify
```

Auth: bearer token required

Idempotency key: recommended

Use this before allowing the user to create escrow orders, generate virtual accounts, start checkout, request payouts, or use provider-backed money movement.

Request body:

```json
{
  "id_type": "BVN",
  "id_number": "22222222222"
}
```

Allowed `id_type` values:

```text
BVN
NIN
```

Response:

```json
{
  "status": "VERIFIED",
  "provider": "KORA",
  "message": "Identity verified successfully"
}
```

## Escrow Orders

### Create Escrow Order

```http
POST /api/v1/escrows/orders
```

Auth: bearer token required

Idempotency key: required

KYC: required

Usually called by the seller.

Request body:

```json
{
  "title": "iPhone 13 Pro",
  "description": "Clean UK-used iPhone 13 Pro, 256GB",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "sales_channel": "INSTAGRAM",
  "delivery_mode": "PHYSICAL",
  "buyer_confirmation_ttl_hours": 48,
  "cross_border": false,
  "settlement_currency": "NGN",
  "metadata": {
    "instagram_handle": "@seller_shop",
    "order_note": "Buyer will pick color before shipping"
  }
}
```

Allowed `sales_channel` values:

```text
WHATSAPP
INSTAGRAM
TELEGRAM
WEBSITE
OTHER
```

Allowed `delivery_mode` values:

```text
PHYSICAL
DIGITAL
SERVICE
```

Cross-border request body example:

```json
{
  "title": "Handmade leather bag",
  "description": "Cross-border order from Ghana to Nigeria",
  "amount_kobo": 12000000,
  "currency": "NGN",
  "sales_channel": "WHATSAPP",
  "delivery_mode": "PHYSICAL",
  "buyer_confirmation_ttl_hours": 72,
  "cross_border": true,
  "settlement_currency": "GHS",
  "fx_locked_rate": "0.0112",
  "fx_quote_reference": "quidax_quote_123"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "AWAITING_PAYMENT",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "seller_id": 2,
  "public_url": "http://localhost:8080/api/v1/public/escrows/orders/ESC-20260608-ABC123"
}
```

### List My Escrow Orders

```http
GET /api/v1/escrows/orders
```

Auth: bearer token required

Response:

```json
{
  "orders": [
    {
      "reference": "ESC-20260608-ABC123",
      "title": "iPhone 13 Pro",
      "status": "FUNDED",
      "amount_kobo": 45000000,
      "currency": "NGN"
    }
  ]
}
```

### Get Escrow Order

```http
GET /api/v1/escrows/orders/:reference
```

Auth: bearer token required

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "title": "iPhone 13 Pro",
  "description": "Clean UK-used iPhone 13 Pro, 256GB",
  "status": "FUNDED",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "seller_id": 2,
  "buyer_id": 1
}
```

### Public Escrow Preview

```http
GET /api/v1/public/escrows/orders/:reference
```

Auth: public

Use this for order preview screens opened from seller links.

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "title": "iPhone 13 Pro",
  "description": "Clean UK-used iPhone 13 Pro, 256GB",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "status": "AWAITING_PAYMENT",
  "seller": {
    "display_name": "Ada Stores"
  }
}
```

### Start Kora Checkout For Escrow

```http
POST /api/v1/escrows/orders/:reference/checkout/kora
```

Auth: bearer token required

Idempotency key: required

KYC: required

Usually called by the buyer.

Request body:

```json
{
  "redirect_url": "escra://payment-complete",
  "notification_url": "https://your-api.com/api/v1/webhooks/kora",
  "channels": ["bank_transfer", "card"],
  "default_channel": "bank_transfer",
  "merchant_bears_cost": true
}
```

Response:

```json
{
  "checkout_url": "https://checkout.korapay.com/pay/...",
  "payment_reference": "KORA-PAY-123",
  "escrow_reference": "ESC-20260608-ABC123"
}
```

### Create Virtual Account For Escrow Pay-In

```http
POST /api/v1/providers/kora/virtual-accounts
```

Auth: bearer token required

Idempotency key: required

KYC: required

Use this when the buyer wants to fund an escrow with a bank transfer.

Request body:

```json
{
  "escrow_reference": "ESC-20260608-ABC123",
  "account_name": "ESCRA Ada Okafor",
  "bank_code": "000",
  "currency": "NGN",
  "id_type": "BVN",
  "id_number": "22222222222",
  "permanent": false
}
```

Response:

```json
{
  "provider": "KORA",
  "account_number": "1234567890",
  "account_name": "ESCRA Ada Okafor",
  "bank_name": "Kora Bank",
  "reference": "VA-123456"
}
```

### List Virtual Accounts

```http
GET /api/v1/providers/kora/virtual-accounts
```

Auth: bearer token required

Response:

```json
{
  "virtual_accounts": [
    {
      "account_number": "1234567890",
      "account_name": "ESCRA Ada Okafor",
      "bank_name": "Kora Bank",
      "currency": "NGN"
    }
  ]
}
```

### Manually Mark Escrow As Funded

```http
POST /api/v1/escrows/orders/:reference/fund
```

Auth: bearer token required

Idempotency key: required

Request body:

```json
{
  "pin": "1234"
}
```

Use this only for local testing or manual internal flows. In production, Kora webhook confirmation should fund the escrow.

### Mark Escrow As Shipped

```http
POST /api/v1/escrows/orders/:reference/ship
```

Auth: bearer token required

Idempotency key: required

Usually called by the seller.

Request body:

```json
{
  "tracking_reference": "GIGM-123456",
  "delivery_proof_url": "https://example.com/proof.jpg",
  "note": "Package handed to courier"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "SHIPPED"
}
```

### Mark Escrow As Delivered

```http
POST /api/v1/escrows/orders/:reference/mark-delivered
```

Auth: bearer token required

Idempotency key: required

Usually called by the seller after delivery evidence is available.

Request body:

```json
{
  "delivery_proof_url": "https://example.com/delivery-photo.jpg",
  "note": "Buyer received the package"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "DELIVERED"
}
```

### Confirm Delivery

```http
POST /api/v1/escrows/orders/:reference/confirm-delivery
```

Auth: bearer token required

Idempotency key: required

Usually called by the buyer. The buyer confirms that the seller delivered correctly.

Request body:

```json
{
  "delivery_code": "493021",
  "pin": "1234"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "DELIVERED",
  "buyer_confirmed": true
}
```

### Release Escrow Funds

```http
POST /api/v1/escrows/orders/:reference/release
```

Auth: bearer token required

Idempotency key: required

Releases escrow funds to the seller if release criteria are met.

Request body:

```json
{
  "pin": "1234"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "RELEASED"
}
```

### Open Dispute

```http
POST /api/v1/escrows/orders/:reference/dispute
```

Auth: bearer token required

Idempotency key: required

Usually called by the buyer when delivery failed, item was fake, item was incomplete, or seller is suspected of fraud.

Request body:

```json
{
  "reason": "ITEM_NOT_RECEIVED",
  "details": "Seller marked the item as delivered but I have not received anything.",
  "evidence_url": "https://example.com/chat-screenshot.jpg"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "DISPUTED"
}
```

### Cancel Escrow

```http
POST /api/v1/escrows/orders/:reference/cancel
```

Auth: bearer token required

Idempotency key: required

Request body:

```json
{
  "reason": "Buyer changed their mind before payment"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "CANCELLED"
}
```

### Refund Escrow

```http
POST /api/v1/escrows/orders/:reference/refund
```

Auth: bearer token required

Idempotency key: required

Request body:

```json
{
  "reason": "Seller could not fulfil order"
}
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "REFUNDED"
}
```

## Wallets

### Get Wallet Balance

```http
GET /api/v1/wallets/balance
```

Auth: bearer token required

Response:

```json
{
  "wallet_id": 1,
  "currency": "NGN",
  "balance_kobo": 0,
  "available_balance_kobo": 0,
  "ledger_balance_kobo": 0
}
```

### Start Kora Wallet Checkout

```http
POST /api/v1/wallets/checkout/kora
```

Auth: bearer token required

Idempotency key: required

Use this when a user wants to fund their ESCRA wallet directly through Kora checkout.

Request body:

```json
{
  "amount_kobo": 500000,
  "redirect_url": "https://escra.app/wallet/callback",
  "narration": "Direct wallet deposit"
}
```

Response:

```json
{
  "success": true,
  "checkout_url": "https://checkout.korapay.com/pay/...",
  "reference": "DEP-KORA-123456"
}
```

The wallet balance is credited only after `POST /api/v1/webhooks/kora` receives a successful Kora payment event for the returned reference.

### List Wallets

```http
GET /api/v1/wallets
```

Auth: bearer token required

Response:

```json
{
  "wallets": [
    {
      "id": 1,
      "currency": "NGN",
      "balance_kobo": 0
    }
  ]
}
```

## Transactions

### Transfer Between ESCRA Wallets

```http
POST /api/v1/transactions/transfer
```

Auth: bearer token required

Idempotency key: required

Request body:

```json
{
  "wallet_id": 1,
  "amount_kobo": 500000,
  "description": "Test transfer",
  "pin": "1234"
}
```

Response:

```json
{
  "reference": "TRX-123456",
  "status": "SUCCESS",
  "amount_kobo": 500000
}
```

### Transaction History

```http
GET /api/v1/transactions/history
```

Auth: bearer token required

Response:

```json
{
  "success": true,
  "transactions": [
    {
      "reference": "TRX-123456",
      "amount_kobo": 500000,
      "currency": "NGN",
      "status": "SUCCESS",
      "description": "Test transfer"
    }
  ],
  "count": 1
}
```

### Get Transaction

```http
GET /api/v1/transactions/:reference
```

Auth: bearer token required

Response:

```json
{
  "reference": "TRX-123456",
  "amount_kobo": 500000,
  "currency": "NGN",
  "status": "SUCCESS"
}
```

## Limits

### Get User Limits

```http
GET /api/v1/limits
```

Auth: bearer token required

Response:

```json
{
  "daily_limit_kobo": 50000000,
  "single_transaction_limit_kobo": 10000000,
  "used_today_kobo": 0,
  "remaining_today_kobo": 50000000
}
```

## Kora Provider

### List Kora Banks

```http
GET /api/v1/providers/kora/banks?countryCode=NG
```

Auth: bearer token required

Query parameters:

```text
countryCode=NG
```

Response:

```json
{
  "banks": [
    {
      "name": "Access Bank",
      "code": "044"
    }
  ]
}
```

### Resolve Bank Account

```http
GET /api/v1/providers/kora/banks/resolve?bankCode=044&accountNumber=0123456789
```

Auth: bearer token required

Query parameters:

```text
bankCode=044
accountNumber=0123456789
```

Response:

```json
{
  "account_number": "0123456789",
  "account_name": "ADA OKAFOR",
  "bank_code": "044"
}
```

### Get Kora Balances

```http
GET /api/v1/providers/kora/balances
```

Auth: bearer token required

Use this for reconciliation and admin/provider fund tracking.

Response:

```json
{
  "balances": [
    {
      "currency": "NGN",
      "available_balance": 1000000,
      "ledger_balance": 1000000
    }
  ]
}
```

### Bank Payout

```http
POST /api/v1/providers/kora/payouts/bank
```

Auth: bearer token required

Idempotency key: required

KYC: required

Used to release funds to a seller's bank account.

Request body:

```json
{
  "bank_code": "044",
  "account_number": "0123456789",
  "account_name": "ADA OKAFOR",
  "customer_email": "seller@example.com",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "narration": "ESCRA escrow release",
  "pin": "1234"
}
```

Response:

```json
{
  "provider": "KORA",
  "reference": "PAYOUT-123456",
  "status": "PROCESSING"
}
```

### Create Kora Refund

```http
POST /api/v1/admin/providers/kora/refunds
```

Auth: admin key required

Idempotency key: required

Request body:

```json
{
  "payment_reference": "KORA-PAY-123",
  "amount_kobo": 45000000,
  "currency": "NGN",
  "reason": "Dispute resolved in buyer's favour"
}
```

Response:

```json
{
  "provider": "KORA",
  "refund_reference": "REFUND-123456",
  "status": "PROCESSING"
}
```

## Quidax Provider

### Create Crypto Withdrawal

```http
POST /api/v1/providers/quidax/withdrawals
```

Auth: bearer token required

Idempotency key: required

Request body:

```json
{
  "currency": "usdt",
  "amount": "10.00",
  "fund_uid": "wallet_or_address_uid",
  "fund_uid2": "optional_secondary_uid",
  "network": "trc20",
  "transaction_note": "ESCRA crypto transfer",
  "narration": "Withdrawal to user",
  "provider_user_id": "quidax_user_id",
  "pin": "1234"
}
```

Response:

```json
{
  "provider": "QUIDAX",
  "reference": "QDX-WITHDRAWAL-123456",
  "status": "PROCESSING"
}
```

## Webhooks

### Kora Webhook

```http
POST /api/v1/webhooks/kora
```

Auth: Kora webhook signature or secret verification

Kora should call this endpoint when payment, virtual account, transfer, refund, or payout events happen.

Request body shape depends on Kora's event payload.

Example:

```json
{
  "event": "charge.success",
  "data": {
    "reference": "KORA-PAY-123",
    "amount": 450000,
    "currency": "NGN",
    "status": "success",
    "metadata": {
      "escrow_reference": "ESC-20260608-ABC123"
    }
  }
}
```

Expected backend behaviour:

1. Verify webhook authenticity.
2. Store the raw webhook event.
3. Ignore duplicate webhook events safely.
4. Match the provider reference to an escrow payment.
5. Mark escrow as funded after successful pay-in confirmation.
6. Update provider transaction status for payouts and refunds.

Response:

```json
{
  "received": true
}
```

### Quidax Webhook

```http
POST /api/v1/webhooks/quidax
```

Auth: Quidax webhook signature or secret verification

Request body shape depends on Quidax's event payload.

Response:

```json
{
  "received": true
}
```

## Admin

### Resolve Dispute

```http
POST /api/v1/admin/escrows/orders/:reference/resolve-dispute
```

Auth: admin key required

Idempotency key: required

Use this when support/admin has reviewed evidence from both buyer and seller.

Request body:

```json
{
  "action": "REFUND",
  "resolution_note": "Seller could not provide delivery evidence."
}
```

Allowed `action` values:

```text
RELEASE
REFUND
```

Response:

```json
{
  "reference": "ESC-20260608-ABC123",
  "status": "REFUNDED",
  "resolution_note": "Seller could not provide delivery evidence."
}
```

### Verify Or Reject User KYC

```http
POST /api/v1/admin/users/:userId/verify
```

Auth: admin key required

Request body:

```json
{
  "status": "ACTIVE",
  "reason": "Documents approved successfully"
}
```

Allowed `status` values:

```text
ACTIVE
REJECTED
```

Response:

```json
{
  "success": true,
  "user_id": "d4176c58-5d37-450a-acab-dd8facf7441b",
  "status": "ACTIVE",
  "kyc_status": "VERIFIED"
}
```

## Important Edge Cases For Mobile

The app should handle these states gracefully:

1. Buyer pays, but Kora webhook is delayed.
2. Buyer closes checkout before payment completes.
3. Buyer transfers the wrong amount to a virtual account.
4. Buyer sends money twice.
5. Seller marks delivery but buyer does not respond.
6. Buyer disputes after seller has uploaded proof.
7. Admin resolves dispute in favour of seller.
8. Admin resolves dispute in favour of buyer.
9. Kora payout is processing for a long time.
10. Kora refund is processing for a long time.
11. User has not completed KYC.
12. User enters wrong PIN.
13. Token expires and user must log in again.
14. Network request times out but backend still processed the request.

For all payment mutation screens, the mobile app should use `Idempotency-Key` and retry with the same key if the request times out.

## Environment Variables Needed By Backend

```env
APP_ENV=development
PORT=8080
PUBLIC_BASE_URL=http://localhost:8080

JWT_SECRET=replace_me
ADMIN_API_KEY=replace_me

KORA_BASE_URL=https://api.korapay.com
KORA_PUBLIC_KEY=pk_test_xxx
KORA_SECRET_KEY=sk_test_xxx
KORA_WEBHOOK_SECRET=replace_with_webhook_secret_when_available

QUIDAX_BASE_URL=https://www.quidax.com/api/v1
QUIDAX_SECRET_KEY=replace_me
QUIDAX_WEBHOOK_SECRET=replace_me
```

## Mobile Developer Notes

Use secure storage for the access token.

Generate a fresh UUID for each new payment or state-changing action.

Reuse the same UUID only when retrying the exact same failed or timed-out action.

Display money from `amount_kobo` by dividing by `100`.

Do not assume payment is complete from checkout redirect alone. Wait for backend status after webhook confirmation.

Poll the escrow order after checkout until the status changes from `AWAITING_PAYMENT` to `FUNDED`, or show a pending state if Kora confirmation is delayed.

For disputes, collect evidence links such as screenshots, delivery photos, courier tracking numbers, chat exports, or short notes.
