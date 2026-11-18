-- Reconstructed source for the already-applied Supabase migration.
-- Verified against project tthzvaguegpmcbxqfcms on 2026-11-12:
-- bucket exists, is public, is limited to 2 MiB, and accepts PNG/JPEG/WebP.
-- ON CONFLICT preserves existing bucket settings; this migration is historical
-- source restoration and is not intended to mutate the already-migrated project.

INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
  'school-logos',
  'school-logos',
  true,
  2097152,
  ARRAY['image/png', 'image/jpeg', 'image/webp']
)
ON CONFLICT (id) DO NOTHING;
