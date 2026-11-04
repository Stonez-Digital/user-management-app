import type { Metadata } from "next";
import "./globals.css";
import ThemeToggle from "../components/theme-toggle";
import SchoolDomainRouter from "../components/school-domain-router";

export const metadata: Metadata = {
  title: "Stonez Digital School Management Platform",
  description:
    "Access the Stonez Digital school management platform for secure school administration, teaching, student and parent workspaces.",
  applicationName: "Stonez Digital",
  icons: {
    icon: "https://raw.githubusercontent.com/Stonez-Digital/Stonez-Digital-Platform/main/public/School%20Management%20Platform%20Logo.png",
    apple: "https://raw.githubusercontent.com/Stonez-Digital/Stonez-Digital-Platform/main/public/School%20Management%20Platform%20Logo.png",
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
