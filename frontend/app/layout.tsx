import type { Metadata } from "next";
import "./globals.css";
import ThemeToggle from "../components/theme-toggle";

export const metadata: Metadata = {
  title: "Stonez School Management",
  description: "School administration dashboard powered by the Stonez User Management API.",
  icons: { icon: "/stonez-digital-logo.svg", apple: "/stonez-digital-logo.svg" },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en" suppressHydrationWarning><head><script dangerouslySetInnerHTML={{__html:`(function(){try{var t=localStorage.getItem("theme");document.documentElement.dataset.theme=t==="light"?"light":"dark"}catch(e){document.documentElement.dataset.theme="dark"}})()`}} /></head><body><ThemeToggle />{children}</body></html>;
}