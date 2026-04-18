import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "ContextOps — AI Agent Evaluation",
  description: "Full-stack evaluation and debugging for AI agents, RAG systems, and enterprise copilots",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-50">
        <div className="flex h-screen overflow-hidden">
          {/* Sidebar */}
          <aside className="w-56 bg-white border-r border-slate-200 flex flex-col shrink-0">
            <div className="px-4 py-5 border-b border-slate-200">
              <div className="flex items-center gap-2">
                <div className="w-7 h-7 bg-brand-600 rounded-md flex items-center justify-center">
                  <svg viewBox="0 0 24 24" fill="none" className="w-4 h-4 text-white" stroke="currentColor" strokeWidth={2.5}>
                    <circle cx="12" cy="12" r="3" />
                    <path d="M12 2v3M12 19v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M2 12h3M19 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" />
                  </svg>
                </div>
                <span className="font-bold text-slate-900 text-sm">ContextOps</span>
              </div>
              <p className="text-xs text-slate-500 mt-1">v0.2.0</p>
            </div>
            <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
              <NavItem href="/" label="Dashboard" />
              <NavItem href="/runs" label="Runs" />
              <div className="pt-3 pb-1">
                <p className="text-xs font-semibold text-slate-400 uppercase tracking-wider px-3">Evaluators</p>
              </div>
              <NavItem href="/evaluators/correctness" label="Core Quality" indent />
              <NavItem href="/evaluators/retrieval" label="Retrieval" indent />
              <NavItem href="/evaluators/memory" label="Memory & Context" indent />
              <NavItem href="/evaluators/agent" label="Agent Behaviour" indent />
            </nav>
            <div className="px-4 py-4 border-t border-slate-200">
              <a
                href="https://github.com/madhupathy/contextops"
                target="_blank"
                rel="noreferrer"
                className="text-xs text-slate-500 hover:text-slate-700"
              >
                GitHub →
              </a>
            </div>
          </aside>

          {/* Main content */}
          <main className="flex-1 overflow-y-auto">
            <div className="max-w-6xl mx-auto px-6 py-8">
              {children}
            </div>
          </main>
        </div>
      </body>
    </html>
  );
}

function NavItem({
  href,
  label,
  indent = false,
}: {
  href: string;
  label: string;
  indent?: boolean;
}) {
  return (
    <a
      href={href}
      className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm text-slate-600 hover:bg-slate-100 hover:text-slate-900 transition-colors ${
        indent ? "pl-6" : ""
      }`}
    >
      {label}
    </a>
  );
}
