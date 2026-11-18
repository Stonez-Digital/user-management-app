"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { schoolPathForHostname } from "../lib/school-domain.js";

export default function SchoolDomainRouter() {
  const pathname = usePathname();

  useEffect(() => {
    const destination = schoolPathForHostname(
      window.location.hostname,
      window.location.pathname,
      window.location.search,
    );
    if (destination && destination !== window.location.pathname + window.location.search) {
      window.location.replace(destination);
    }
  }, [pathname]);

  return null;
}
