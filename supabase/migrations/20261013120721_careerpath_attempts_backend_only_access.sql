-- Reconstructed source for the already-applied CareerPath migration.
-- Catalog inspection of project tthzvaguegpmcbxqfcms on 2026-11-12
-- confirmed this table shape, constraints, indexes, and backend-only grants.
-- IF NOT EXISTS keeps this safe when the schema is already provisioned.

CREATE TABLE IF NOT EXISTS public.career_path_attempts (
  id uuid NOT NULL,
  school_id uuid NOT NULL,
  student_id uuid NOT NULL,
  user_id uuid NOT NULL,
  version varchar(32) NOT NULL,
  status varchar(20) NOT NULL,
  responses text NOT NULL DEFAULT '{}'::text,
  results text NOT NULL DEFAULT '[]'::text,
  submitted_at timestamptz,
  created_at timestamptz,
  updated_at timestamptz,
  CONSTRAINT career_path_attempts_pkey PRIMARY KEY (id),
  CONSTRAINT fk_career_path_attempts_school
    FOREIGN KEY (school_id) REFERENCES public.schools(id)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT fk_career_path_attempts_student
    FOREIGN KEY (student_id) REFERENCES public.students(id)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT fk_career_path_attempts_tenant_student
    FOREIGN KEY (school_id, student_id, user_id)
    REFERENCES public.students(school_id, id, user_id)
    ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_career_attempt_owner
  ON public.career_path_attempts (school_id, student_id);
CREATE INDEX IF NOT EXISTS idx_career_path_attempts_status
  ON public.career_path_attempts (status);
CREATE INDEX IF NOT EXISTS idx_career_path_attempts_user_id
  ON public.career_path_attempts (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_careerpath_one_draft
  ON public.career_path_attempts (school_id, student_id, version)
  WHERE status = 'in_progress';

ALTER TABLE public.career_path_attempts ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.career_path_attempts FROM anon, authenticated;
GRANT ALL PRIVILEGES ON TABLE public.career_path_attempts TO service_role;
