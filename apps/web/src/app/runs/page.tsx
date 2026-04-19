"use client";

import { useEffect, useState } from "react";
import {
  Activity, AlertTriangle, CheckCircle2, Clock, Cpu, Search,
  ShieldAlert, TrendingDown, XCircle,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Run {
  id: string;
  query: string;
  status: string;
  model: string | null;
  agent_id: string | null;
  total_tokens: number;
  latency_ms: number;
  estimated_cost: number;
  created_at: string;
}

interface EvalSummary {
  grade: string;
  avg_score: string;
  passed: number;
  failed: number;
  critical_failures: Array<{ category: string }>;
}

const GRADE_COLORS: Record<string, string> = {
  A: "text-green-600 bg-green-50",
  B: "text-blue-600 bg-blue-50",
  C: "text-amber-600 bg-amber-50",
  D: "text-orange-600 bg-orange-50",
  F: "text-red-600 bg-red-50",
};

export default function RunsPage() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [evalSummaries, setEvalSummaries] = useState<Record<string, EvalSummary>>({});
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");

  useEffect(() => {
    fetchRuns();
  }, []);

  async function fetchRuns() {
    try {
      const res = await fetch(`${API_URL}/api/v1/runs`);
      if (res.ok) {
        const data = await res.json();
        setRuns(data);
        // Fetch eval summaries for completed runs (background, non-blocking)
        fetchEvalSummaries(data.filter((r: Run) => r.status === "completed").slice(0, 20));
      }
    } catch (e) {
      console.error("Failed to fetch runs:", e);
    } finally {
      setLoading(false);
    }
  }

  async function fetchEvalSummaries(completedRuns: Run[]) {
    const summaries: Record<string, EvalSummary> = {};
    await Promise.allSettled(
      completedRuns.map(async (run) => {
        try {
          const res = await fetch(`${API_URL}/api/v1/runs/${run.id}/eval-summary`);
          if (res.ok) {
            summaries[run.id] = await res.json();
          }
        } catch {}
      })
    );
    setEvalSummaries(summaries);
  }

  const statusOptions = ["all", "completed", "failed", "running", "pending", "timeout"];

  const filtered = runs.filter((r) => {
    const matchesText = !filter ||
      r.query.toLowerCase().includes(filter.toLowerCase()) ||
      r.status.toLowerCase().includes(filter.toLowerCase());
    const matchesStatus = statusFilter === "all" || r.status === statusFilter;
    return matchesText && matchesStatus;
  });

  const failedCount = runs.filter((r) => r.status === "failed").length;
  const hasCritical = Object.values(evalSummaries).some(
    (s) => s.critical_failures?.length > 0
  );

  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Runs</h1>
          <p className="text-sm text-slate-500 mt-1">
            {runs.length} total · {runs.filter((r) => r.status === "completed").length} completed
            {failedCount > 0 && (
              <span className="text-red-600 ml-2">· {failedCount} failed</span>
            )}
          </p>
        </div>
        {hasCritical && (
          <div className="flex items-center gap-2 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
            <ShieldAlert className="w-4 h-4 text-red-600" />
            <span className="text-xs text-red-700 font-medium">Critical violations detected</span>
          </div>
        )}
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-sm">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="Search runs..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-9 pr-4 py-2 w-full text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div className="flex gap-1">
          {statusOptions.map((s) => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors capitalize ${
                statusFilter === s
                  ? "bg-slate-900 text-white"
                  : "bg-white border border-slate-200 text-slate-600 hover:bg-slate-50"
              }`}
            >
              {s}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="text-center py-12 text-slate-500 text-sm">Loading runs...</div>
      ) : filtered.length === 0 ? (
        <div className="card p-12 text-center">
          <Activity className="w-12 h-12 text-slate-300 mx-auto mb-3" />
          <p className="text-slate-500 text-sm">No runs found</p>
        </div>
      ) : (
        <div className="card divide-y divide-slate-100">
          {/* Table header */}
          <div className="grid grid-cols-[1fr_80px_80px_64px_64px_80px_64px] gap-3 px-5 py-2.5 bg-slate-50 text-xs font-medium text-slate-500 uppercase tracking-wider rounded-t-lg">
            <div>Query</div>
            <div>Status</div>
            <div>Grade</div>
            <div>Tokens</div>
            <div>Latency</div>
            <div>Cost</div>
            <div>Age</div>
          </div>

          {filtered.map((run) => {
            const evalSum = evalSummaries[run.id];
            return (
              <a
                key={run.id}
                href={`/runs/${run.id}`}
                className="grid grid-cols-[1fr_80px_80px_64px_64px_80px_64px] gap-3 px-5 py-3 hover:bg-slate-50 transition-colors items-center"
              >
                {/* Query */}
                <div className="min-w-0">
                  <p className="text-sm font-medium text-slate-900 truncate">{run.query}</p>
                  <p className="text-xs text-slate-400 mt-0.5 font-mono truncate">
                    {run.id.slice(0, 16)}…
                  </p>
                </div>

                {/* Status */}
                <StatusBadge status={run.status} />

                {/* Eval grade */}
                <div>
                  {evalSum ? (
                    <div className="flex flex-col items-start gap-0.5">
                      <span
                        className={`text-xs font-bold px-2 py-0.5 rounded ${
                          GRADE_COLORS[evalSum.grade] || "text-slate-600 bg-slate-100"
                        }`}
                      >
                        {evalSum.grade}
                      </span>
                      {evalSum.critical_failures?.length > 0 && (
                        <span className="text-xs text-red-600 flex items-center gap-0.5">
                          <AlertTriangle className="w-3 h-3" />
                          {evalSum.critical_failures.length}
                        </span>
                      )}
                    </div>
                  ) : run.status === "completed" ? (
                    <span className="text-xs text-slate-300">—</span>
                  ) : null}
                </div>

                {/* Tokens */}
                <div className="text-xs text-slate-600 flex items-center gap-1">
                  <Cpu className="w-3 h-3 text-slate-400" />
                  {((run.total_tokens || 0) / 1000).toFixed(1)}k
                </div>

                {/* Latency */}
                <div className="text-xs text-slate-600 flex items-center gap-1">
                  <Clock className="w-3 h-3 text-slate-400" />
                  {run.latency_ms < 1000
                    ? `${run.latency_ms}ms`
                    : `${(run.latency_ms / 1000).toFixed(1)}s`}
                </div>

                {/* Cost */}
                <div className="text-xs text-slate-600">
                  ${(run.estimated_cost || 0).toFixed(4)}
                </div>

                {/* Age */}
                <div className="text-xs text-slate-400">{timeAgo(run.created_at)}</div>
              </a>
            );
          })}
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const config: Record<string, { color: string; icon: JSX.Element }> = {
    completed: { color: "text-green-700 bg-green-50", icon: <CheckCircle2 className="w-3 h-3" /> },
    failed:    { color: "text-red-700 bg-red-50",   icon: <XCircle className="w-3 h-3" /> },
    running:   { color: "text-blue-700 bg-blue-50",  icon: <Activity className="w-3 h-3 animate-pulse" /> },
    timeout:   { color: "text-amber-700 bg-amber-50", icon: <Clock className="w-3 h-3" /> },
    pending:   { color: "text-slate-600 bg-slate-100", icon: <Clock className="w-3 h-3" /> },
  };
  const c = config[status] ?? config.pending;
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${c.color}`}>
      {c.icon} {status}
    </span>
  );
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "now";
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  return `${Math.floor(hrs / 24)}d`;
}
