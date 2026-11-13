# Production Deployment

## Architecture

Browser -> Cloudflare Worker -> Go API -> Supabase PostgreSQL

The Go API must be publicly reachable over HTTPS before the frontend can be deployed.

## Required API environment

- DB_DRIVER=postgres
- DATABASE_URL=<Supabase PostgreSQL connection string for the API runtime>
- MIGRATION_DATABASE_URL=<optional separate PostgreSQL connection with schema migration privileges>
- JWT_SECRET=<strong random production secret>
- APP_PORT=8080

The API should listen on the platform-provided port when one is supplied. Never commit production secrets.

`MIGRATION_DATABASE_URL` is optional only while `REQUIRE_NOBYPASSRLS` is disabled. When configured, startup runs versioned migrations and the one-time platform-admin bootstrap using this separate PostgreSQL connection, then closes it before serving requests. The API continues to use `DATABASE_URL` for normal application queries. When `REQUIRE_NOBYPASSRLS=true`, startup now fails unless `MIGRATION_DATABASE_URL` is configured; this prevents schema migrations or platform bootstrap from silently running with the restricted API runtime credential. The fallback to `DATABASE_URL` remains for legacy deployments only.

`REQUIRE_NOBYPASSRLS=true` is an opt-in fail-closed startup gate. It makes the API refuse to start if the current or session PostgreSQL role is a superuser, has `BYPASSRLS`, can assume another privileged role, or cannot be inspected. Keep it `false` until the tenant-aware policy and request-transaction rollout is complete; enabling it against the current production `postgres` connection will intentionally stop the API from starting. This flag does not create roles, change grants, or make existing RLS policies tenant-aware.

## Container

The repository root contains a production Dockerfile. Build locally with:

    docker build -t stonez-school-api .

Run locally with:

    docker run --rm -p 8080:8080 \
      -e DB_DRIVER=postgres \
      -e DATABASE_URL='...' \
      -e JWT_SECRET='...' \
      -e APP_PORT=8080 \
      stonez-school-api

Verify:

    curl -i http://localhost:8080/healthz

Expected response is HTTP 200 with JSON containing status=ok.

## Cloudflare frontend

Configure the GitHub Actions production environment secrets:

- CLOUDFLARE_API_TOKEN
- CLOUDFLARE_ACCOUNT_ID
- API_SERVER_URL=https://<public-api-host>

After the API is reachable, run the Cloudflare Deployment workflow manually. It will build the frontend, deploy the Worker, and list the deployed Worker revisions.

## Production checklist

- [ ] Supabase PostgreSQL connection tested
- [ ] JWT_SECRET configured
- [ ] API health endpoint returns 200
- [ ] Public API URL uses HTTPS
- [ ] CORS origin is restricted to the production frontend
- [ ] Cloudflare secrets configured in GitHub production
- [ ] Cloudflare deployment workflow succeeds
- [ ] Browser login and /backend/me verified
