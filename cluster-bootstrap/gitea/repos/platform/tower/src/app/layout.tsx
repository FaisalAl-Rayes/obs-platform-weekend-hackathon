import type { Metadata } from "next";
import { Geist } from "next/font/google";
import "./globals.css";
import Sidebar from "@/components/sidebar";

const geist = Geist({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Tower",
  description: "obs-platform Developer Portal",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${geist.className} dark`}>
      <body className="min-h-screen flex bg-background text-foreground">
        <Sidebar />
        <main className="flex-1 overflow-auto">{children}</main>
      </body>
    </html>
  );
}
