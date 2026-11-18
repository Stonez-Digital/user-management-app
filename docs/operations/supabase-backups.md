# Supabase production database backups

## Source of truth

Production database backup automation is managed in the private repository [Stonez-Digital/production-db-backups](https://github.com/Stonez-Digital/production-db-backups). The obsolete Cloudflare R2-based workflow in this application repository has been removed. Do not recreate the old R2 workflow or add R2 secrets for the current backup path.

- [Daily encrypted custom-format backup workflow](https://github.com/Stonez-Digital/production-db-backups/blob/main/.github/workflows/supabase-daily-backup.yml): scheduled daily at 02:17 UTC (03:17 Nigeria time).
- [Weekly Supabase-filtered compatibility backup workflow](https://github.com/Stonez-Digital/production-db-backups/blob/main/.github/workflows/supabase-filtered-backup-compatibility.yml): scheduled Sundays at 04:17 UTC (05:17 Nigeria time).
- [Encrypted disposable PostgreSQL restore test](https://github.com/Stonez-Digital/production-db-backups/blob/main/.github/workflows/encrypted-backup-restore-test.yml): manually triggered; restores compatible objects into a disposable PostgreSQL 17 service, never production.
- [Restore-drill runbook](https://github.com/Stonez-Digital/production-db-backups/blob/main/docs/restore-drill-runbook.md).
- [Backup repository README](https://github.com/Stonez-Digital/production-db-backups/blob/main/README.md).

## Required secrets

Configure secrets in **Settings → Secrets and variables → Actions** of the private `production-db-backups` repository only. Secret values must never be committed, pasted into issues/chat, or printed in logs.

| Secret | Purpose |
|---|---|
| `SUPABASE_DB_URL` | Production PostgreSQL connection URI; require TLS and use a direct connection or session-mode pooler, not transaction-mode pooling. |
| `BACKUP_AGE_RECIPIENT` | Public age encryption recipient. |
| `BACKUP_AGE_IDENTITY` | Private age identity used to decrypt backups; retain a protected offline recovery copy. |

GitHub Actions artifacts have a 30-day retention target and are not a permanent archive. Keep the repository private and review artifact expiry/storage limits.

## What has been verified

- The encrypted custom-format artifact workflow validates that the dump can be decrypted and its PostgreSQL archive index can be read.
- The disposable PostgreSQL 17 compatibility restore test succeeded on 2026-10-19 in 51 seconds using artifact `supabase-production-39218658661-1`. It excludes Supabase-managed extension registration and Vault schema objects.
- The separate local Supabase restore drill and public-schema row-count checks are documented in the restore-drill runbook.

These checks are different levels of evidence. A valid encrypted archive is not by itself a successful restore; a stock PostgreSQL restore does not prove Supabase Vault, Storage object payloads, project settings, Auth configuration, or recovery into a fresh managed Supabase project.

## Safe operating rules

1. Use the backup repository's workflows as the source of truth; do not re-enable the removed R2-based workflow in this app repository.
2. Never restore to production during a test.
3. Do not create a paid Supabase project or branch without explicit approval.
4. The database dump does not include Supabase Storage file payloads, project settings, or secrets. Storage-object recovery is tracked separately.
5. Verify actual scheduled workflow runs and unexpired encrypted artifacts before declaring scheduled backup health.
