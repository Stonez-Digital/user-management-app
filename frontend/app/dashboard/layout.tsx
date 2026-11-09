import { ReactNode } from "react";
import SchoolBrand from "../../components/school-brand";
import RoleAccessShell from "../../components/role-access-shell";
import { SupportWidget } from "../../components/platform-experience";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <SchoolBrand />
      <RoleAccessShell>{children}</RoleAccessShell>
      <SupportWidget />
    </>
  );
}
