"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { landingPathForRole, normalizeAuthUser } from "../lib/auth-session";

type NavItem = { label: string; href: string };

const adminLinks: NavItem[] = [
  { label: "Overview", href: "/dashboard" },
  { label: "Curriculum", href: "/dashboard/curriculum" },
  { label: "Academic", href: "/dashboard/academic" },
  { label: "Students", href: "/dashboard/students" },
  { label: "Enrollment", href: "/dashboard/enrollments" },
  { label: "Teacher Assignments", href: "/dashboard/teacher-assignments" },
  { label: "Attendance", href: "/dashboard/attendance" },
  { label: "Assessments", href: "/dashboard/assessments" },
  { label: "Results", href: "/dashboard/results" },
  { label: "Timetable", href: "/dashboard/timetable" },
  { label: "Operations", href: "/dashboard/operations" },
  { label: "Users & Roles", href: "/dashboard/users" },
  { label: "Onboarding", href: "/dashboard/onboarding" },
  { label: "Audit Logs", href: "/dashboard/audit" },
];

const studentLinks: NavItem[] = [
  { label: "My Dashboard", href: "/dashboard/student" },
  { label: "CareerPath", href: "/dashboard/student/careerpath" },
  { label: "Notifications", href: "/dashboard/notifications" },
];

const roleLinks: Record<string, NavItem[]> = {
  student: studentLinks,
  teacher: [
    { label: "My Workspace", href: "/dashboard/teacher" },
    { label: "Attendance", href: "/dashboard/attendance" },
    { label: "Assessments", href: "/dashboard/assessments" },
    { label: "Results", href: "/dashboard/results" },
    { label: "Timetable", href: "/dashboard/timetable" },
    { label: "Notifications", href: "/dashboard/notifications" },
  ],
  parent: [
    { label: "My Family", href: "/dashboard/parent" },
    { label: "Notifications", href: "/dashboard/notifications" },
  ],
  accountant: [
    { label: "Operations", href: "/dashboard/operations" },
    { label: "Finance", href: "/dashboard/finance" },
    { label: "Notifications", href: "/dashboard/notifications" },
  ],
  staff: [
    { label: "Operations", href: "/dashboard/operations" },
    { label: "Notifications", href: "/dashboard/notifications" },
  ],
};

function isPathWithin(pathname: string, base: string) {
  return pathname === base || pathname.startsWith(base + "/");
}

export function canRoleAccessPath(role: string, pathname: string): boolean {
  if (role === "super_admin" || role === "school_admin") return true;
  if (role === "student") {
    return isPathWithin(pathname, "/dashboard/student") ||
      pathname === "/dashboard/notifications" ||
      pathname === "/dashboard";
  }
  if (role === "teacher") {
    return ["/dashboard/teacher", "/dashboard/attendance", "/dashboard/assessments", "/dashboard/results", "/dashboard/timetable", "/dashboard/notifications", "/dashboard"].some((base) => isPathWithin(pathname, base));
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

export default function RoleAccessShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [role, setRole] = useState("");
  const [ready, setReady] = useState(false);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const token = localStorage.getItem("access_token");
        if (!token) throw new Error("Your session has expired. Please sign in again.");
        const response = await fetch("/backend/me", {
          headers: { Authorization: "Bearer " + token },
          cache: "no-store",
        });
        if (response.status === 401) throw new Error("Your session has expired. Please sign in again.");
        if (!response.ok) throw new Error("Unable to verify your account permissions.");
        const user = normalizeAuthUser(await response.json());
        if (!user) throw new Error("Unable to verify your account permissions.");
        if (!active) return;
        setRole(user.role);
        setReady(true);
      } catch (error) {
        if (!active) return;
        setFailure(error instanceof Error ? error.message : "Unable to verify your account permissions.");
        setReady(true);
      }
    })();
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!ready || !role) return;
    if (!canRoleAccessPath(role, pathname)) router.replace(landingPathForRole(role));
    else if (pathname === "/dashboard" && ["student", "teacher", "parent", "accountant", "staff"].includes(role)) router.replace(landingPathForRole(role));
  }, [pathname, ready, role, router]);

  if (!ready) return <main className="content"><section className="panel"><strong>Verifying access…</strong><p className="muted">Loading the workspace permitted for your account.</p></section></main>;
  if (failure || !role) return <main className="content"><section className="panel" role="alert"><h1>Unable to verify access</h1><p>{failure || "Please sign in again to continue."}</p></section></main>;
  if (!canRoleAccessPath(role, pathname) || (pathname === "/dashboard" && ["student", "teacher", "parent", "accountant", "staff"].includes(role))) {
    return <main className="content"><section className="panel"><strong>Opening your workspace…</strong></section></main>;
  }

  const links = role === "school_admin" || role === "super_admin"
    ? adminLinks
    : roleLinks[role] || [];
  return (
    <>
      <nav className="dashboard-section-nav" aria-label="Workspace navigation">
        {links.map(({ label, href }) => (
          <Link key={href} href={href} aria-current={isPathWithin(pathname, href) ? "page" : undefined}>{label}</Link>
        ))}
      </nav>
      {children}
    </>
  );
}
