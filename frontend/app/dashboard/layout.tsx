import { ReactNode } from "react";
import SchoolBrand from "../../components/school-brand";
import RoleAccessShell from "../../components/role-access-shell";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return (
    <>
      <SchoolBrand />
      <RoleAccessShell>{children}</RoleAccessShell>
    </>
  );
}
