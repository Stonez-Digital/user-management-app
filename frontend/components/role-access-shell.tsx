"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { landingPathForRole, normalizeAuthUser } from "../lib/auth-session";
import { canRoleAccessPath } from "../lib/dashboard-access.mjs";

type NavItem = { label: string; href: string };
type NavGroup = { label: string; items: NavItem[] };

const adminGroups: NavGroup[] = [
  { label: "People", items: [
    { label: "Students", href: "/dashboard/students" },
    { label: "Enrollment", href: "/dashboard/enrollments" },
    { label: "Onboarding & Imports", href: "/dashboard/onboarding" },
  ] },
  { label: "Academics", items: [
    { label: "Curriculum", href: "/dashboard/curriculum" },
    { label: "Academic Setup", href: "/dashboard/academic" },
    { label: "Teacher Assignments", href: "/dashboard/teacher-assignments" },
  ] },
  { label: "Examinations & Results", items: [
    { label: "Assessments", href: "/dashboard/assessments" },
    { label: "Results & Report Cards", href: "/dashboard/results" },
  ] },
  { label: "Attendance & Operations", items: [
    { label: "Attendance", href: "/dashboard/attendance" },
    { label: "Timetable", href: "/dashboard/timetable" },
    { label: "Operations", href: "/dashboard/operations" },
  ] },
  { label: "Finance", items: [
    { label: "Fees, Invoices & Payments", href: "/dashboard/finance" },
  ] },
  { label: "Communication", items: [
    { label: "Blog & Gallery", href: "/dashboard/content" },
    { label: "Notifications", href: "/dashboard/notifications" },
  ] },
  { label: "Administration", items: [
    { label: "Users & Roles", href: "/dashboard/users" },
    { label: "Audit Logs", href: "/dashboard/audit" },
  ] },
];

const roleGroups: Record<string, NavGroup[]> = {
  student: [
    { label: "My Learning", items: [
      { label: "My Dashboard", href: "/dashboard/student" },
      { label: "CareerPath", href: "/dashboard/student/careerpath" },
    ] },
    { label: "Updates", items: [{ label: "Notifications", href: "/dashboard/notifications" }] },
  ],
  teacher: [
    { label: "Teaching Workspace", items: [
      { label: "My Workspace", href: "/dashboard/teacher" },
      { label: "Attendance", href: "/dashboard/attendance" },
      { label: "Timetable", href: "/dashboard/timetable" },
    ] },
    { label: "Examinations & Results", items: [
      { label: "Assessments", href: "/dashboard/assessments" },
      { label: "Results", href: "/dashboard/results" },
    ] },
    { label: "Updates", items: [{ label: "Notifications", href: "/dashboard/notifications" }] },
  ],
  parent: [
    { label: "Family", items: [{ label: "My Family", href: "/dashboard/parent" }] },
    { label: "Updates", items: [{ label: "Notifications", href: "/dashboard/notifications" }] },
  ],
  accountant: [
    { label: "Finance", items: [
      { label: "Operations", href: "/dashboard/operations" },
      { label: "Fees, Invoices & Payments", href: "/dashboard/finance" },
    ] },
    { label: "Updates", items: [{ label: "Notifications", href: "/dashboard/notifications" }] },
  ],
  staff: [
    { label: "Operations", items: [{ label: "Operations Workspace", href: "/dashboard/operations" }] },
    { label: "Updates", items: [{ label: "Notifications", href: "/dashboard/notifications" }] },
  ],
};

function isPathWithin(pathname: string, base: string) {
  return pathname === base || pathname.startsWith(base + "/");
}

function isCurrentLink(pathname: string, href: string) {
  return href === "/dashboard" ? pathname === href : isPathWithin(pathname, href);
}

export default function RoleAccessShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [role, setRole] = useState("");
  const [schoolSlug, setSchoolSlug] = useState("");
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
        const payload = await response.json();
        const user = normalizeAuthUser(payload);
        if (!user) throw new Error("Unable to verify your account permissions.");
        if (!active) return;
        setRole(user.role);
        setSchoolSlug(typeof payload.school_slug === "string" ? payload.school_slug : "");
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
    else if (pathname === "/dashboard" && landingPathForRole(role) !== pathname && ["student", "teacher", "parent", "accountant", "staff"].includes(role)) router.replace(landingPathForRole(role));
  }, [pathname, ready, role, router]);

  if (!ready) return <main className="content"><section className="panel"><strong>Verifying access…</strong><p className="muted">Loading the workspace permitted for your account.</p></section></main>;
  if (failure || !role) return <main className="content" ><section className="panel" role="alert"><h1>Unable to verify access</h1><p>{failure || "Please sign in again to continue."}</p></section></main>;
  if (!canRoleAccessPath(role, pathname) || (pathname === "/dashboard" && landingPathForRole(role) !== pathname && ["student", "teacher", "parent", "accountant", "staff"].includes(role))) {
    return <main className="content"><section className="panel"><strong>Opening your workspace…</strong></section></main>;
  }

  const isSchoolAdmin = role === "school_admin";
  const groups = isSchoolAdmin || role === "super_admin" ? adminGroups : roleGroups[role] || [];

  return (
    <>
      <nav className="dashboard-section-nav" aria-label="Workspace navigation">
        <Link className="dashboard-nav-home" href={landingPathForRole(role)} aria-current={isCurrentLink(pathname, landingPathForRole(role)) ? "page" : undefined}>
          {isSchoolAdmin || role === "super_admin" ? "Overview" : "My Workspace"}
        </Link>
        {groups.map((group) => {
          const active = group.items.some((item) => isPathWithin(pathname, item.href));
          return (
            <details className="dashboard-nav-group" key={group.label} open={active}>
              <summary aria-current={active ? "location" : undefined}>{group.label}</summary>
              <div className="dashboard-nav-menu">
                {group.items.map(({ label, href }) => (
                  <Link key={href} href={href} aria-current={isCurrentLink(pathname, href) ? "page" : undefined}>{label}</Link>
                ))}
              </div>
            </details>
          );
        })}
        {isSchoolAdmin && schoolSlug && (
          <Link className="dashboard-nav-external" href={"/school/" + encodeURIComponent(schoolSlug)} target="_blank" rel="noreferrer">
            School Homepage ↗
          </Link>
        )}
      </nav>
      {children}
    </>
  );
}
