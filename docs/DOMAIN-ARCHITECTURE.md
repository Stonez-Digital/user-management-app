# Stonez Digital school-domain architecture

## Domain ownership and boundaries

- `stonezdigital.com.ng` remains the Stonez Digital organisation and portfolio website.
- `www.stonezdigital.com.ng` remains an alias for that organisation website.
- `app.stonezdigital.com.ng` is the central school-platform entry point (platform login and school registration).
- Every registered school reserves a name-first hostname such as `https://greenfield-academy.stonezdigital.com.ng`; the canonical DNS label is stored on its `schools` row. The hostname becomes routable when platform approval provisions it.
- The existing `stonez-school-management.onojamondayojonugba.workers.dev` address remains available as a fallback.

Do not point the apex or `www` records at the school-management Worker. They currently serve the Stonez Digital portfolio through Cloudflare Pages.

## Routing behavior

The frontend's school-domain router recognizes exactly one label below `stonezdigital.com.ng`. It excludes reserved organisation/platform hosts such as `app`, `www`, `api`, and `mail`, then maps requests to the existing `/school/{slug}` route family. Paths and query strings are preserved. An explicit `/school/{different-slug}` path is pinned back to the slug represented by the hostname.

The frontend uses the hostname only to select a public school website. It does not set or override an authenticated user's tenant. The Go API continues to derive protected tenant context from the authenticated user's database `school_id`; never accept a school ID or tenant identity from a request hostname, query parameter, or browser storage for authorization.

## Cloudflare attachment and school onboarding

The Worker is attached to `app.stonezdigital.com.ng` as a separate custom domain in `frontend/wrangler.jsonc`. The apex and `www` are intentionally not included.

Cloudflare Worker custom domains are attached to individual hostnames. Signup reserves a unique name-first slug and returns the assigned hostname, but does not expose a pending school. When a platform administrator changes the school status to `active`, the Go API attaches that exact hostname to the `stonez-school-management` Worker through the Cloudflare Workers Domains API. The school status is not changed if provisioning fails. The API call is idempotent so an administrator can retry activation after a transient Cloudflare failure.

Configure the Render API service with `CLOUDFLARE_API_TOKEN` (a narrowly scoped token allowed to manage Worker custom domains for this account), `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_WORKER_NAME=stonez-school-management`, and optionally `SCHOOL_BASE_DOMAIN=stonezdigital.com.ng`. Never expose these values to the frontend or commit the token. Do not install a blanket `*.stonezdigital.com.ng` Worker route: it could divert the existing `www` portfolio hostname.

## Safe rollout

1. Merge only after the frontend domain-routing tests and both frontend build checks pass.
2. Deploy the Worker with the new `app` custom domain; leave the current Worker.dev hostname untouched.
3. Configure the Cloudflare API credentials on the backend service and verify `app.stonezdigital.com.ng` login and school signup.
4. Approve one pilot school through the platform UI; confirm its exact hostname is attached automatically, then verify public pages and branded login.
5. Confirm the apex and `www` still resolve to the Stonez Digital portfolio before onboarding additional schools.

## Read-only production reconciliation audit

Before repairing legacy school slugs or attaching missing school hostnames, run the read-only inventory tool against the intended production database and Cloudflare account:

```bash
DB_DRIVER=postgres \
DATABASE_URL="$DATABASE_URL" \
CLOUDFLARE_API_TOKEN="$CLOUDFLARE_API_TOKEN" \
CLOUDFLARE_ACCOUNT_ID="$CLOUDFLARE_ACCOUNT_ID" \
CLOUDFLARE_WORKER_NAME=stonez-school-management \
SCHOOL_BASE_DOMAIN=stonezdigital.com.ng \
go run ./cmd/school-domain-audit
```

The command requires PostgreSQL explicitly and refuses SQLite. It reads school IDs, names, codes, slugs and statuses, then lists custom domains attached to the configured Worker. It prints a JSON report containing invalid legacy slugs, collision-aware proposed replacements, active schools with missing Worker attachments, and attachments that are enabled for the expected Worker.

**This command is strictly read-only.** It does not update school rows, call the Cloudflare attachment endpoint, create DNS records, activate schools, or modify Worker configuration. It accepts no apply flag. Review the generated report and approve a separate, narrowly scoped repair before changing any production slug or attaching any missing hostname. Do not infer that an active school is ready for public traffic merely because its Worker attachment exists; validate DNS/TLS, school routing, branded login, and tenant isolation independently.

The slug proposal uses the school's normalized name first and adds a normalized school-code suffix only when the name slug is already reserved or occupied. Existing canonical slugs are not rewritten automatically.

