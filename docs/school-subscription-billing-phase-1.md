# School subscription billing — Phase 1

## Scope

This phase introduces a subscription domain separate from student-fee finance.

- billing_plans stores the term-based catalogue and plan entitlements.
- school_subscriptions stores a school's plan for one academic term.
- subscription_events records lifecycle transitions.
- The Go service starts a one-term trial only against a current academic term owned by the same school.
- The service records expiry when an expired subscription is queried.
- Quota checks are available in the service layer for student, teacher and storage usage.

Existing student fee items, student invoices and student payments are unchanged.

## Agreed catalogue

Amounts are stored as integer minor units (kobo), not floating-point naira.

| Plan | Price / term | Students | Teachers | Storage |
|---|---:|---:|---:|---:|
| Free Trial | Free | 50 | 5 | 1 GB |
| Basic | ₦65,000 | 200 | 15 | 5 GB |
| Professional | ₦100,000 | 500 | 40 | 20 GB |
| Premium | ₦150,000 | 1,500 | 100 | 50 GB |
| Enterprise | Custom | Custom | Custom | Custom |

## Trial policy

A trial is not activated merely because registration stored requested_plan = trial.

The trial service requires:
1. A valid school.
2. A current academic term belonging to that school.
3. No previous trial for that school.
4. No other active subscription.
5. An active trial plan in the catalogue.

The trial expires at the selected academic term's end date. This avoids inventing a fixed number of trial days that may not match the school's academic calendar.

## Security and rollout gate

The SQL file under migrations/ is a review artifact only. It is deliberately not registered in the automatic GORM migration runner. This PR must not change production schema merely by deploying the application.

Before any production application:
- Review the SQL and the plan values.
- Confirm the production backup and rollback procedure.
- Confirm the backend database role retains required access while anon and authenticated remain denied.
- Apply the migration only after explicit approval.
- Then wire the service into school-admin onboarding/billing routes in a separate reviewed step.

## Explicitly out of scope

- Paystack or other payment provider integration.
- Paid subscription activation.
- Subscription invoices, receipts and payment webhooks.
- Automatic renewal, grace periods or reminders.
- Automatic enforcement in every student, teacher and media creation endpoint.
- Frontend billing screens and platform-admin reporting.

The existing requested_plan school field remains a registration preference only.
