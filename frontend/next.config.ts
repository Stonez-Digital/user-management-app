import type { NextConfig } from "next";

const productionApiTarget = "https://stonez-digital-school-api.onrender.com";
const apiTarget =
 process.env.API_SERVER_URL ||
 (process.env.NODE_ENV === "production" ? productionApiTarget : "http://localhost:8080");

const nextConfig: NextConfig = {
 async rewrites() {
  return [{ source: "/backend/:path*", destination: apiTarget + "/:path*" }];
 }
};

export default nextConfig;
