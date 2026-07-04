import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "lokal",
  description: "Step-through debugger for CI pipelines",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${geistSans.variable} ${geistMono.variable}`}>
      <body className="min-h-screen" style={{ background: "var(--gh-bg)", color: "var(--gh-text)" }}>
        <header style={{ borderBottom: "1px solid var(--gh-border)" }} className="px-6 py-3 flex items-center gap-3">
          <a href="/" className="flex items-center gap-2 no-underline">
            <span className="text-lg font-bold" style={{ color: "var(--gh-text)" }}>lokal</span>
            <span className="text-xs px-2 py-0.5 rounded-full font-medium" style={{ background: "var(--gh-surface)", color: "var(--gh-muted)", border: "1px solid var(--gh-border)" }}>beta</span>
          </a>
          <span style={{ color: "var(--gh-muted)" }} className="text-sm ml-1">— step-through CI debugger</span>
        </header>
        {children}
      </body>
    </html>
  );
}
