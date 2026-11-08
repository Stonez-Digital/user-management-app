"use client";

import { installAuthFetch } from "../lib/auth-fetch.mjs";

if (typeof window !== "undefined") {
  installAuthFetch();
  window.addEventListener("stonez:session-expired", () => {
    if (window.location.pathname.startsWith("/login")) return;
    window.location.assign("/login?reason=session-expired");
  });
}

export default function AuthFetchBootstrap() {
  return null;
}
