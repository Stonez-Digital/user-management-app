# School subscription checkout — phase 2

## Scope

This phase adds authenticated school-admin endpoints for subscription plan discovery, pending checkout creation, and server-verified activation. It is separate from the existing student-fee invoice/payment workflow.

## Endpoints

All endpoints are under the existing authenticated `/admin` group and require the `school_admin` role.

- `GET /admin/billing/plans` — lists active subscription plans.
- `GET /admin/billing/subscription` — returns the current active subscription.
- `POST /admin/billing/checkout` — accepts `plan_code` and `term_id`; the server loads amount and currency from `billing_plans`, takes the contact email from the authenticated school administrator, creates a pending subscription and unique payment reference, then initializes Paystack.
- `POST /admin/billing/verify` — accepts a payment `reference`; the server verifies directly with Paystack and activates only when reference, integer kobo amount, currency, pending state, and school ownership all match.

The checkout response includes the Paystack authorization URL and reference. The frontend must redirect the user to the returned authorization URL. It must never send an amount, provider status, or paid timestamp as authoritative input.

## Persistence migration

`migrations/20261112_subscription_checkout_payments.sql` is a review-only migration. It creates `subscription_payments`, a unique provider reference, lookup indexes, and one-pending-subscription-per-school protection. RLS is enabled and direct grants to PUBLIC, anon, and authenticated are revoked.

The migration is intentionally not part of GORM automatic migration. It must be separately reviewed and explicitly approved before production application. Until it is applied, the billing checkout endpoints cannot persist transactions and will fail closed.

## Safety and idempotency

- Amounts come from active database plans and remain integer kobo.
- Trial and custom Enterprise plans cannot be purchased through this Paystack endpoint.
- Only an active term belonging to the authenticated school is eligible.
- One pending checkout per school prevents duplicate concurrent checkout creation.
- The payment reference is generated server-side and unique in the database.
- Payment verification runs server-to-server; client redirects are not proof of payment.
- Amount mismatch, reference mismatch, non-NGN currency, non-success status, and cross-school references are rejected.
- Activation and payment success are committed in one database transaction.
- Repeated verification of an already successful payment returns the same active subscription without recording another activation event.
- Existing student-fee invoices and payments are untouched.

## Environment and rollout

The Go API reads `PAYSTACK_SECRET_KEY` only on the backend. Configure a test key in a non-production environment. `PAYSTACK_CALLBACK_URL` is optional and, when configured, must be HTTPS.

Do not add a live key to test environments or enable live checkout until test-mode end-to-end validation and a separately approved production rollout. Do not log keys or payment authorization secrets.

## Next phase

Add signed webhook verification, replay protection, payment-state reconciliation, invoice/receipt generation, and billing UI. Webhook handling should reuse the same transactional activation invariants rather than implement a second activation path.
