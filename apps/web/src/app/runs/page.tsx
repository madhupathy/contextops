"use client";

import { useEffect, useState } from "react";
import { Activity, Clock, Cpu, Search } from "lucide-react";

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

export default function RunsPage() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    fetchRuns();
  }, []);

  async function fetchRuns() {
    try {
      const res = await fetch(`${API_URL}/api/v1/runs`);
      if (res.ok) setRuns(await res.json());
    } catch (e) {
      console.error("Failed to fetch runs:", e);
    } finally {
      setLoading(false);
    }
  }

  const filtered = filter
    ? runs.filter(
        (r) =>
          r.query.toLowerCase().includes(filter.toLowerCase()) ||
          r.status.toLowerCase().includes(filter.toLowerCase())
      )
    : runs;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Runs</h1>
          <p className="text-sm text-slate-500 mt-1">All agent run traces ingested into the system</p>
        </div>
        <div className="relative">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="Filter runs..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="pl-9 pr-4 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          />
        </div>
      </div>

      {loading ? (
        <div className="text-center py-12 text-slate-500">Loading runs...</div>
      ) : filtered.length === 0 ? (
        <div className="card p-12 text-center">
          <Activity className="w-12 h-12 text-slate-300 mx-auto mb-3" />
          <p className="text-slate-500">No runs found</p>
        </div>
      ) : (
        <div className="card divide-y divide-slate-100">
          <div className="grid grid-cols-12 gap-4 px-6 py-3 bg-slate-50 text-xs font-medium text-slate-500 uppercase tracking-wider rounded-t-lg">
            <div className="col-span-5">Query</div>
            <div className="col-span-1">Status</div>
            <div className="col-span-1">Model</div>
            <div className="col-span-1">Tokens</div>
            <div className="col-span-1">Latency</div>
            <div className="col-span-1">Cost</div>
            <div className="col-span-2">Created</div>
          </div>
          {filtered.map((run) => (
            <a
              key={run.id}
              href={`/runs/${run.id}`}
              className="grid grid-cols-12 gap-4 px-6 py-3 hover:bg-slate-50 transition-colors items-center"
            >
              <div className="col-span-5 text-sm text-slate-900 truncate">{run.query}</div>
              <div className="col-span-1">
                <StatusBadge status={run.status} />
              </div>
              <div className="col-span-1 text-xs text-slate-500 truncate">{run.model || "-"}</div>
              <div className="col-span-1 text-xs text-slate-600 flex items-center gap-1">
                <Cpu className="w-3 h-3" /> {(run.total_tokens || 0).toLocaleString()}
              </div>
              <div className="col-span-1 text-xs text-slate-600 flex items-center gap-1">
                <Clock className="w-3 h-3" /> {run.latency_ms}ms
              </div>
              <div className="col-span-1 text-xs text-slate-600">
                ${(run.estimated_cost || 0).toFixed(4)}
              </div>
              <div className="col-span-2 text-xs text-slate-400">
                {new Date(run.created_at).toLocaleString()}
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    completed: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
    running: "bg-blue-100 text-blue-700",
    pending: "bg-slate-100 text-slate-600",
    timeout: "bg-amber-100 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status] || styles.pending}`}>
      {status}
    </span>
  );
}
