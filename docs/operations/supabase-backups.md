# Supabase production database backups

The `Supabase Daily Encrypted Backup` GitHub Actions workflow creates a PostgreSQL custom-format dump, encrypts it with age, verifies that the encrypted file can be decrypted and its archive index read, uploads the encrypted artifact to Cloudflare R2, checks the remote object size, and removes expired backup objects older than 30 days.

- Schedule: daily at 02:17 UTC (03:17 Nigeria time).
- Manual execution: GitHub → Actions → Supabase Daily Encrypted Backup → Run workflow.
- R2 object prefix: `supabase-production/`.
- Encryption: age; R2 receives only the encrypted `.dump.age` file.
- Retention: 30 days, restricted to backup objects matching the expected filename pattern in the dedicated prefix.
- Production safety: the workflow only reads the database. It does not run migrations, write application rows, or restore production.

## Required GitHub Actions secrets

Set these in the repository's **Settings → Secrets and variables → Actions → New repository secret**. Never commit these values to the repository, issue comments, workflow files, or logs.

| Secret | Value |
|---|---|
| `SUPABASE_DB_URL` | Production PostgreSQL connection URI with TLS required (`sslmode=require`). Use the direct connection or a session-mode pooler, not transaction-mode pooling. The DB role must be able to dump the required schemas, tables, sequences, and data. Prefer a dedicated least-privilege backup role if your Supabase configuration supports it. |
| `BACKUP_AGE_RECIPIENT` | Public age recipient, usually beginning with `age1`. |
| `BACKUP_AGE_IDENTITY` | Private age identity generated for this backup. Treat as a high-value secret. |
| `R2_ACCOUNT_ID` | Cloudflare account ID that owns the bucket. |
| `R2_ACCESS_KEY_ID` | R2 API token access key ID scoped to the backup bucket. |
| `R2_SECRET_ACCESS_KEY` | Matching R2 API token secret. |
| `R2_BUCKET` | Existing R2 bucket name. |

## Generate age encryption keys safely

On a trusted local machine with the age utility installed:

```sh
age-keygen -o age-identity.txt
age-keygen -y age-identity.txt
```

- Put the output of `age-keygen -y age-identity.txt` into `BACKUP_AGE_RECIPIENT`.
- Put the complete contents of `age-identity.txt` into `BACKUP_AGE_IDENTITY`.
- Store the private identity separately in an offline password manager or other protected recovery location. If the private identity is lost, the encrypted backups cannot be decrypted.
- Do not upload or commit `age-identity.txt`.

## Cloudflare R2 permissions

Create an R2 API token limited to the existing backup bucket with object read/write permission. The workflow needs list, upload, head-object, and delete permission for retention. Do not use an account-wide administrator token. R2 encryption at rest is provided by Cloudflare; age adds client-side encryption before upload.

## First-run verification

1. Add all seven secrets above.
2. Merge this workflow through the normal pull-request review.
3. In GitHub Actions, manually run **Supabase Daily Encrypted Backup** once.
4. Confirm the job succeeds, including archive-index validation, upload, and remote size check.
5. In R2, confirm a new `supabase-production/database-....dump.age` object exists.
6. Keep the age private identity outside GitHub and test a restore into a separate disposable database before relying on this backup for recovery.

A successful workflow run proves the dump was created, encrypted, archive-index validated, uploaded, and size-checked. It does **not** prove a full restore is successful. Test restoration separately in a non-production project.

## Restore outline (never target production for the first test)

Download a selected `.dump.age` object from R2, decrypt it with the private age identity, and inspect it with PostgreSQL 17's `pg_restore --list`. For a real recovery drill, restore into a separate empty test database and validate representative row counts, schema objects, and application behavior. Review any production recovery plan with the project owner before restoring production.

## Important scope notes

- This is a logical database backup; it does not include Supabase Storage objects, project settings, or secrets.
- Keep a separate backup of important Storage objects if the app relies on them.
- GitHub-hosted runner disk is ephemeral; temporary plaintext files are permission-restricted and removed when the job exits.
- Failed runs should be investigated immediately. Do not assume a backup exists merely because the workflow is scheduled.
