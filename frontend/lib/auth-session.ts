export type AuthUser = {
  id: string;
  name?: string;
  email?: string;
  role: string;
  active?: boolean;
  school_id?: string | null;
};

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" ? value as Record<string, unknown> : null;
}

function readString(value: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const candidate = value[key];
    if (typeof candidate === "string" && candidate.trim()) return candidate;
  }
  return undefined;
}

export function normalizeAuthUser(payload: unknown): AuthUser | null {
  const root = asRecord(payload);
  if (!root) return null;

  const candidates: unknown[] = [
    root,
    root.user,
    root.data,
    asRecord(root.data)?.user,
  ];

  for (const candidate of candidates) {
    const value = asRecord(candidate);
    if (!value) continue;

    // Accept both the explicit API contract ("id"/"role") and legacy Go
    // JSON serialization ("ID"/"Role") so an older edge deployment cannot
    // discard an otherwise valid authenticated session.
    const id = readString(value, "id", "ID", "user_id", "userId");
    const role = readString(value, "role", "Role");
    if (!id || !role) continue;

    const name = readString(value, "name", "Name");
    const email = readString(value, "email", "Email");
    const schoolId = readString(value, "school_id", "SchoolID", "schoolId");

    return {
      id,
      name,
      email,
      role,
      active: typeof value.active === "boolean"
        ? value.active
        : typeof value.Active === "boolean"
          ? value.Active
          : undefined,
      school_id: schoolId ?? null,
    };
  }

  return null;
}

export function landingPathForRole(role: string): string {
  switch (role) {
    case "parent":
      return "/parent";
    case "student":
      return "/student";
    case "teacher":
      return "/dashboard/teacher";
    case "super_admin":
    case "school_admin":
    case "accountant":
    case "staff":
      return "/dashboard";
    default:
      return "/dashboard";
  }
}
