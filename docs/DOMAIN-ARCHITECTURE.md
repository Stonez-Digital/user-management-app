# Stonez Digital school-domain architecture

## Domain ownership and boundaries

- `stonezdigital.com.ng` remains the Stonez Digital organisation and portfolio website.
- `www.stonezdigital.com.ng` remains an alias for that organisation website.
- `app.stonezdigital.com.ng` is the central school-platform entry point (platform login and school registration).
- An approved school receives `https://{school-slug}.stonezdigital.com.ng`, using the canonical slug stored on its `schools` row.
- The existing `stonez-school-management.onojamondayojonugba.workers.dev` address remains available as a fallback.

Do not point the apex or `www` records at the school-management Worker. They currently serve the Stonez Digital portfolio through Cloudflare Pages.

## Routing behavior

The frontend's school-domain router recognizes exactly one label below `stonezdigital.com.ng`. It excludes reserved organisation/platform hosts such as `app`, `www`, `api`, and `mail`, then maps requests to the existing `/school/{slug}` route family. Paths and query strings are preserved. An explicit `/school/{different-slug}` path is pinned back to the slug represented by the hostname.

The frontend uses the hostname only to select a public school website. It does not set or override an authenticated user's tenant. The Go API continues to derive protected tenant context from the authenticated user's database `school_id`; never accept a school ID or tenant identity from a request hostname, query parameter, or browser storage for authorization.

## Cloudflare attachment and school onboarding

The Worker is attached to `app.stonezdigital.com.ng` as a separate custom domain in `frontend/wrangler.jsonc`. The apex and `www` are intentionally not included.

Cloudflare Worker custom domains are attached to individual hostnames. Attach each approved school's exact hostname (for example, `bedrock.stonezdigital.com.ng`) to the `stonez-school-management` Worker when that school is approved and its slug has passed the platform's uniqueness checks. Do not install a blanket `*.stonezdigital.com.ng` Worker route: it would also match the existing `www` portfolio hostname and could divert organisation traffic.

The current signup flow creates schools in `pending` status. Domain attachment is an operational approval step; do not expose a school hostname before the platform administrator activates the school. DNS and Worker-domain provisioning are not performed by the application request handler and require an explicit platform deployment/operations action.

## Safe rollout

1. Merge only after the frontend domain-routing tests and both frontend build checks pass.
2. Deploy the Worker with the new `app` custom domain; leave the current Worker.dev hostname untouched.
3. Verify `app.stonezdigital.com.ng` login and school signup.
4. Approve one pilot school, attach only its exact hostname, and verify public pages and branded login.
5. Confirm the apex and `www` still resolve to the Stonez Digital portfolio before onboarding additional schools.
