import { headers } from "next/headers";
import type { MetadataRoute } from "next";

const PUBLIC_HOST = "app.stonezdigital.com.ng";
const PUBLIC_ORIGIN = `https://${PUBLIC_HOST}`;

export const dynamic = "force-dynamic";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const requestHeaders = await headers();
  const forwardedHost = requestHeaders.get("x-forwarded-host")?.split(",")[0]?.trim();
  const host = (forwardedHost || requestHeaders.get("host") || "")
    .split(":")[0]
    .toLowerCase();

  // Do not publish the platform sitemap on school-owned custom domains.
  if (host !== PUBLIC_HOST) return [];

  return [
    {
      url: `${PUBLIC_ORIGIN}/`,
      changeFrequency: "monthly",
      priority: 1,
    },
    {
      url: `${PUBLIC_ORIGIN}/school-signup`,
      changeFrequency: "yearly",
      priority: 0.6,
    },
  ];
}
