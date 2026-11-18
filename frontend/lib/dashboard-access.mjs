function isPathWithin(pathname, base) {
  return pathname === base || pathname.startsWith(base + "/");
}

export function canRoleAccessPath(role, pathname) {
  if (role === "super_admin" || role === "school_admin") return true;
  if (role === "student") {
    return isPathWithin(pathname, "/dashboard/student") ||
      pathname === "/dashboard/notifications" ||
      pathname === "/dashboard";
  }
  if (role === "teacher") {
    return pathname === "/dashboard" ||
      ["/dashboard/teacher", "/dashboard/attendance", "/dashboard/assessments", "/dashboard/results", "/dashboard/timetable", "/dashboard/notifications"]
        .some((base) => isPathWithin(pathname, base));
  }
  if (role === "parent") {
    return isPathWithin(pathname, "/dashboard/parent") ||
      pathname === "/dashboard/notifications" ||
      pathname === "/dashboard";
  }
  if (role === "accountant") {
    return isPathWithin(pathname, "/dashboard/operations") ||
      isPathWithin(pathname, "/dashboard/finance") ||
      pathname === "/dashboard/notifications" ||
      pathname === "/dashboard";
  }
  if (role === "staff") {
    return isPathWithin(pathname, "/dashboard/operations") ||
      pathname === "/dashboard/notifications" ||
      pathname === "/dashboard";
  }
  return false;
}
