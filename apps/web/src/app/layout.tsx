import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "ContextOps",
  description: "Open-source benchmark and debugging layer for enterprise AI agents",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-50">
        <nav className="bg-white border-b border-slate-200 sticky top-0 z-50">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex justify-between h-14">
              <div className="flex items-center gap-8">
                <a href="/" className="flex items-center gap-2">
                  <div className="w-8 h-8 bg-brand-600 rounded-lg flex items-center justify-center">
                    <span className="text-white font-bold text-sm">CO</span>
                  </div>
                  <span className="font-semibold text-slate-900">ContextOps</span>
                </a>
                <div className="hidden sm:flex items-center gap-1">
                  <NavLink href="/" label="Dashboard" />
                  <NavLink href="/runs" label="Runs" />
                  <NavLink href="/benchmarks" label="Benchmarks" />
                  <NavLink href="/evaluations" label="Evaluations" />
                  <NavLink href="/audit" label="Audit" />
                </div>
              </div>
              <div className="flex items-center gap-3">
                <span className="text-xs text-slate-500 bg-slate-100 px-2 py-1 rounded">v0.1.0</span>
              </div>
            </div>
          </div>
        </nav>
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          {children}
        </main>
      </body>
    </html>
  );
}

function NavLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      className="px-3 py-2 text-sm font-medium text-slate-600 hover:text-slate-900 hover:bg-slate-50 rounded-md transition-colors"
    >
      {label}
    </a>
  );
}
