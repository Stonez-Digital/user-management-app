import { ReactNode } from "react";
import SchoolBrand from "../../components/school-brand";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return <><SchoolBrand />{children}</>;
}
