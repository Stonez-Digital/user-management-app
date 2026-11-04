import { headers } from "next/headers";
import type { MetadataRoute } from "next";

const PUBLIC_HOST = "app.stonezdigital.com.ng";
const PUBLIC_ORIGIN = `https://${PUBLIC_HOST}`;

export const dynamic = "force-dynamic";

export default async function robots(): Promise<MetadataRoute.Robots> {
  const requestHeaders = await headers();
  const forwardedHost = requestHeaders.get("x-forwarded-host")?.split(",")[0]?.trim();
  const host = (forwardedHost || requestHeaders.get("host") || "")
    .split(":")[0]
    .toLowerCase();

  // School-owned custom domains share this application. Keep their private
  // tenant entry points out of search until a school explicitly publishes them.
  if (host !== PUBLIC_HOST) {
    return {
      rules: {
        userAgent: "*",
        disallow: "/",
      },
    };
  }

  return {
    rules: {
      userAgent: "*",
      allow: ["/", "/school-signup"],
      disallow: [
        "/backend/",
        "/dashboard/",
        "/platform/",
        "/school/",
        "/student/",
        "/parent/",
        "/forgot-password",
        "/reset-password",
      ],
    },
    sitemap: `${PUBLIC_ORIGIN}/sitemap.xml`,
    host: PUBLIC_ORIGIN,
  };
}
