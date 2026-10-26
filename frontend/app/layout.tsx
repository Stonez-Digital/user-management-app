import type { Metadata } from "next";
import "./globals.css";
import ThemeToggle from "../components/theme-toggle";

export const metadata: Metadata = {
  title: "Stonez School Management",
  description: "School administration dashboard powered by the Stonez User Management API.",
  icons: { icon: "https://raw.githubusercontent.com/Stonez-Digital/Stonez-Digital-Platform/main/public/School%20Management%20Platform%20Logo.png", apple: "https://raw.githubusercontent.com/Stonez-Digital/Stonez-Digital-Platform/main/public/School%20Management%20Platform%20Logo.png" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body><ThemeToggle />{children}</body></html>;
}
