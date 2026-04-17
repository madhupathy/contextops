"use client";

import { useEffect, useState } from "react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  CheckCircle2,
  Clock,
  Cpu,
  DollarSign,
  Search,
  Shield,
  XCircle,
  Zap,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Run {
  id: string;
  query: string;
  status: string;
  model: string | null;
  total_tokens: number;
  latency_ms: number;
  created_at: string;
}

interface Stats {
  total_runs: number;
  completed: number;
  failed: number;
  avg_latency: number;
  avg_tokens: number;
}

export default function Dashboard() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [stats, setStats] = useState<Stats>({
    total_runs: 0,
    completed: 0,
    failed: 0,
    avg_latency: 0,
    avg_tokens: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchDashboardData();
  }, []);

  async function fetchDashboardData() {
    try {
      const res = await fetch(`${API_URL}/api/v1/runs`);
      if (res.ok) {
        const data: Run[] = await res.json();
        setRuns(data);

        const completed = data.filter((r) => r.status === "completed").length;
        const failed = data.filter((r) => r.status === "failed").length;
        const avgLatency =
          data.length > 0
            ? Math.round(data.reduce((sum, r) => sum + (r.latency_ms || 0), 0) / data.length)
            : 0;
        const avgTokens =
          data.length > 0
            ? Math.round(data.reduce((sum, r) => sum + (r.total_tokens || 0), 0) / data.length)
            : 0;

        setStats({
          total_runs: data.length,
          completed,
          failed,
          avg_latency: avgLatency,
          avg_tokens: avgTokens,
        });
      }
    } catch (e) {
      console.error("Failed to fetch dashboard data:", e);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
        <p className="text-sm text-slate-500 mt-1">
          Monitor your AI agent runs, evaluations, and benchmarks
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
        <StatCard
          icon={<Activity className="w-5 h-5 text-brand-600" />}
          label="Total Runs"
          value={stats.total_runs.toString()}
        />
        <StatCard
          icon={<CheckCircle2 className="w-5 h-5 text-green-600" />}
          label="Completed"
          value={stats.completed.toString()}
        />
        <StatCard
          icon={<XCircle className="w-5 h-5 text-red-600" />}
          label="Failed"
          value={stats.failed.toString()}
        />
        <StatCard
          icon={<Clock className="w-5 h-5 text-amber-600" />}
          label="Avg Latency"
          value={`${stats.avg_latency}ms`}
        />
        <StatCard
          icon={<Cpu className="w-5 h-5 text-purple-600" />}
          label="Avg Tokens"
          value={stats.avg_tokens.toLocaleString()}
        />
      </div>

      {/* Evaluation Categories */}
      <div className="card p-6">
        <h2 className="text-lg font-semibold text-slate-900 mb-4">Evaluation Categories</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <CategoryCard icon={<CheckCircle2 />} name="Answer Correctness" color="text-green-600" />
          <CategoryCard icon={<Search />} name="Retrieval Quality" color="text-blue-600" />
          <CategoryCard icon={<Shield />} name="Permission Safety" color="text-red-600" />
          <CategoryCard icon={<BarChart3 />} name="Groundedness" color="text-purple-600" />
          <CategoryCard icon={<Zap />} name="Memory Utility" color="text-amber-600" />
          <CategoryCard icon={<Activity />} name="Tool Correctness" color="text-indigo-600" />
          <CategoryCard icon={<AlertTriangle />} name="Trajectory Quality" color="text-orange-600" />
          <CategoryCard icon={<DollarSign />} name="Cost Efficiency" color="text-emerald-600" />
        </div>
      </div>

      {/* Recent Runs */}
      <div className="card">
        <div className="px-6 py-4 border-b border-slate-200">
          <h2 className="text-lg font-semibold text-slate-900">Recent Runs</h2>
        </div>
        {loading ? (
          <div className="p-8 text-center text-slate-500">Loading...</div>
        ) : runs.length === 0 ? (
          <div className="p-8 text-center">
            <Activity className="w-12 h-12 text-slate-300 mx-auto mb-3" />
            <p className="text-slate-500 text-sm">No runs yet. Ingest your first agent run via the API.</p>
            <code className="text-xs bg-slate-100 px-3 py-2 rounded mt-3 inline-block">
              POST /api/v1/runs
            </code>
          </div>
        ) : (
          <div className="divide-y divide-slate-100">
            {runs.slice(0, 10).map((run) => (
              <a
                key={run.id}
                href={`/runs/${run.id}`}
                className="flex items-center justify-between px-6 py-3 hover:bg-slate-50 transition-colors"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-slate-900 truncate">{run.query}</p>
                  <p className="text-xs text-slate-500 mt-0.5">
                    {run.model || "unknown"} &middot; {run.total_tokens} tokens &middot;{" "}
                    {run.latency_ms}ms
                  </p>
                </div>
                <div className="flex items-center gap-3 ml-4">
                  <StatusBadge status={run.status} />
                  <span className="text-xs text-slate-400">
                    {new Date(run.created_at).toLocaleString()}
                  </span>
                </div>
              </a>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="card p-4">
      <div className="flex items-center gap-3">
        {icon}
        <div>
          <p className="text-2xl font-bold text-slate-900">{value}</p>
          <p className="text-xs text-slate-500">{label}</p>
        </div>
      </div>
    </div>
  );
}

function CategoryCard({
  icon,
  name,
  color,
}: {
  icon: React.ReactNode;
  name: string;
  color: string;
}) {
  return (
    <div className="flex items-center gap-2 p-3 rounded-lg border border-slate-100 hover:border-slate-200 transition-colors">
      <span className={color}>{icon}</span>
      <span className="text-sm font-medium text-slate-700">{name}</span>
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
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
        styles[status] || styles.pending
      }`}
    >
      {status}
    </span>
  );
}
