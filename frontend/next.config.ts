import type { NextConfig } from "next";
const apiTarget = process.env.API_SERVER_URL || "http://localhost:8080";
const nextConfig: NextConfig = { async rewrites() { return [{ source: "/backend/:path*", destination: apiTarget + "/:path*" }]; } };
export default nextConfig;