import type { Metadata } from "next";
import "./globals.css";
import ThemeToggle from "../components/theme-toggle";
import SchoolDomainRouter from "../components/school-domain-router";

export const metadata: Metadata = {
  metadataBase: new URL("https://app.stonezdigital.com.ng"),
  title: {
    default: "School Management Platform | Stonez Digital",
    template: "%s | Stonez Digital",
  },
  description:
    "A connected school management platform for administration, academics, finance, teachers, students and parents.",
  applicationName: "Stonez Digital",
  icons: {
    icon: "/stonez-digital-logo.svg",
    apple: "/stonez-digital-logo.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <SchoolDomainRouter />
        <ThemeToggle />
        {children}
      </body>
    </html>
  );
}
