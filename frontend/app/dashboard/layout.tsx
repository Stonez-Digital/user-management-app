"use client";
import { ReactNode } from "react";
import { AcademicContextProvider } from "../../lib/academic-context";
import AcademicSelector from "../../components/academic-selector";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  return <AcademicContextProvider><AcademicSelector />{children}</AcademicContextProvider>;
}
