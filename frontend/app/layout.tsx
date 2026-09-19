import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Stonez School Management", description: "School administration dashboard powered by the Stonez User Management API." };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="en"><body>{children}</body></html>; }