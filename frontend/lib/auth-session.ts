export type AuthUser = {
  id: string;
  name?: string;
  email?: string;
  role: string;
  active?: boolean;
  school_id?: string | null;
};

export function normalizeAuthUser(payload: unknown): AuthUser | null {
  if (!payload || typeof payload !== "object") return null;

  const root = payload as Record<string, unknown>;
  const candidates = [
    root,
    root.user,
    root.data,
    typeof root.data === "object" && root.data !== null
      ? (root.data as Record<string, unknown>).user
      : null,
  ];

  for (const candidate of candidates) {
    if (!candidate || typeof candidate !== "object") continue;
    const value = candidate as Record<string, unknown>;
    if (typeof value.id === "string" && typeof value.role === "string" && value.role.trim()) {
      return {
        id: value.id,
        name: typeof value.name === "string" ? value.name : undefined,
        email: typeof value.email === "string" ? value.email : undefined,
        role: value.role,
        active: typeof value.active === "boolean" ? value.active : undefined,
        school_id: typeof value.school_id === "string" ? value.school_id : null,
      };
    }
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
