# API proxy security and shared rate limiting

## What this protects

The Cloudflare Worker signs each forwarded API request with HMAC-SHA256. The signature covers the HTTP method, exact upstream path and query string, Unix timestamp, one-time nonce, and SHA-256 digest of the request body. The Go API verifies the signature in constant time and rejects missing, invalid, expired, or replayed requests.

A shared Redis-compatible Valkey store provides:
- Atomic, cross-instance fixed-window counters for authentication and public school-enquiry endpoints.
- One-time nonce claims with a two-minute TTL, preventing a captured signed request from being replayed against another API instance.
- Fail-closed behavior when the shared store is unavailable while either security feature is enabled.

The existing in-memory limiter remains available only when shared rate limiting is not required. It is not a production substitute for shared counters.

## Required configuration

Use the same high-entropy random secret of at least 32 bytes in both environments. Generate it locally with a cryptographically secure generator; do not commit it or paste it into GitHub issues or chat.

### Render API service

Provision a Render Key Value (Valkey) instance in the same region as the API service. Keep external access disabled and use its internal connection URL.

Configure these environment variables on the API service:

| Variable | Value |
| --- | --- |
| `REDIS_URL` | Valkey internal connection URL |
| `API_PROXY_HMAC_SECRET` | Same secret configured on the Worker |
| `API_PROXY_AUTH_REQUIRED` | `true` |
| `REQUIRE_SHARED_RATE_LIMIT` | `true` |

The API validates these settings at startup. It refuses to start in required mode if the secret or shared store URL is missing or malformed. Store connection failures produce HTTP 503 for protected requests rather than silently falling back to local counters.

### Cloudflare Worker

Set the same secret as a Worker secret:

```sh
cd frontend
npx wrangler secret put API_PROXY_HMAC_SECRET
```

Deploy the Worker after adding the secret. The Worker runtime configuration enables `nodejs_compat_populate_process_env` so the route handler can read the secret at runtime. Never define the secret as a plaintext Wrangler variable.

## Safe rollout sequence

1. Create Valkey and configure `REDIS_URL` on Render. Enable `REQUIRE_SHARED_RATE_LIMIT=true` first; validate that the API starts and that auth/enquiry requests still work.
2. Add `API_PROXY_HMAC_SECRET` as a Cloudflare Worker secret and deploy the Worker. At this stage the API may still accept unsigned requests, so complete the next step promptly.
3. Add the same `API_PROXY_HMAC_SECRET` to Render and set `API_PROXY_AUTH_REQUIRED=true`. The API will now reject direct unsigned requests. Health checks at `/healthz` remain available to Render.
4. Verify browser login, school signup (including multipart uploads), token refresh, logout, public school enquiries, and direct unsigned calls to the Render origin.
5. Confirm rate-limit counters are shared across API instances by sending requests through separate instances and checking that the combined limit is enforced.

Do not enable mandatory API verification before the Worker secret has been deployed. Otherwise all proxied API requests will receive HTTP 401.

## Current rate-limit scope

The shared counter is currently applied to `/auth/*` and `/public/schools/*/enquiries`, at 10 requests per visitor IP per 60-second fixed window. Authentication and public-enquiry traffic use separate namespaces. Other API routes are not yet assigned rate-limit policies.

Cloudflare's `CF-Connecting-IP` is used for visitor-level rate limiting only after the HMAC proxy request has been verified. Without proxy authentication, the API falls back to the socket peer and does not trust a client-supplied forwarded IP.

## Operational notes

- The Valkey service must be reachable from the Render API over the private network.
- Use an internal `redis://` URL where Render provides private networking; use `rediss://` only when the service URL requires TLS.
- The limiter uses atomic Redis Lua operations and short-lived keys. It does not require a schema migration.
- A Redis outage causes protected requests to fail with HTTP 503 when shared rate limiting or signed proxy authentication is enabled. Alert on these errors.
- Rotate the secret by deploying a coordinated Worker/API change. A rotation that leaves the two sides with different secrets will reject API requests.
- `/healthz` is intentionally exempt from proxy authentication for platform health checks and must not expose application data.
