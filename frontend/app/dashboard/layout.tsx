import Link from "next/link";
import { ReactNode } from "react";
import SchoolBrand from "../../components/school-brand";

const dashboardLinks = [
  ["Overview", "/dashboard"],
  ["Curriculum", "/dashboard/curriculum"],
  ["Academic", "/dashboard/academic"],
  ["Students", "/dashboard/students"],
  ["Enrollment", "/dashboard/enrollments"],
  ["Teacher Assignments", "/dashboard/teacher-assignments"],
  ["Attendance", "/dashboard/attendance"],
  ["Assessments", "/dashboard/assessments"],
  ["Results", "/dashboard/results"],
  ["Timetable", "/dashboard/timetable"],
  ["Operations", "/dashboard/operations"],
  ["Users & Roles", "/dashboard/users"],
  ["Onboarding", "/dashboard/onboarding"],
  ["Audit Logs", "/dashboard/audit"],
] as const;

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <SchoolBrand />
      <nav className="dashboard-section-nav" aria-label="School management navigation">
        {dashboardLinks.map(([label, href]) => (
          <Link key={href} href={href}>{label}</Link>
        ))}
      </nav>
      {children}
    </>
  );
}
