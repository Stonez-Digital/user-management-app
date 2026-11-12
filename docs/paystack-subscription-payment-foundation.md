# Paystack provider foundation

This change adds a server-side Paystack transaction client for the school-subscription billing work.

## Security boundary

- The Paystack secret key is read only by the Go API from `PAYSTACK_SECRET_KEY`; never expose it to the browser or commit it to Git.
- Checkout initialization accepts an amount in integer kobo. The calling billing service must load the active plan from the database and supply its server-side `amount_minor`; never trust a client-supplied amount.
- Checkout is initialized with NGN and a unique server-generated reference.
- Callback URLs, when supplied, must use HTTPS.
- The checkout response is accepted only when Paystack returns an HTTPS URL on `checkout.paystack.com` and echoes the exact reference.
- Verification calls Paystack's API from the backend and accepts only a successful transaction whose reference matches exactly, whose amount is positive, and whose currency is NGN.
- Verified amounts are exposed as both integer `AmountMinor` and major-unit `Amount`. Subscription billing must compare `AmountMinor` to the plan/subscription amount exactly.
- Network errors, non-2xx responses, malformed payloads, unsuccessful transactions, mismatched references, and non-NGN transactions fail closed.

## Render configuration

Add `PAYSTACK_SECRET_KEY` to the backend Render service environment. Start with a Paystack test secret key (`sk_test_...`) in a non-production/test environment. Keep the live key (`sk_live_...`) out of test environments and enable it only after test-mode end-to-end validation and an explicit production rollout decision.

No key value belongs in this repository, frontend environment variables, logs, or issue comments.

## Scope of this phase

This is the provider client and its unit-test foundation only. It does not yet create pending subscription-payment records, expose checkout/verification HTTP endpoints, process signed webhooks, activate paid subscriptions, or apply the review-only subscription SQL migration. Those steps require a separate PR with persistent idempotency, school-admin authorization, transactional activation, and webhook replay protection.

## Paystack references

- Transaction initialize and verify: https://paystack.com/docs/api/transaction/
- Accept payments and verify amount/status before delivering value: https://paystack.com/docs/payments/accept-payments/
- API key handling: https://paystack.com/docs/api/authentication/
