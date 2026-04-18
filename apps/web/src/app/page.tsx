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
  MemoryStick,
  Shield,
  Star,
  Wrench,
  XCircle,
  Zap,
  FileText,
  Lock,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Run {
  id: string;
  query: string;
  status: string;
  model: string | null;
  total_tokens: number;
  latency_ms: number;
  estimated_cost: number;
  created_at: string;
}

interface Stats {
  total_runs: number;
  completed: number;
  failed: number;
  avg_latency: number;
  avg_tokens: number;
  total_cost: number;
}

const EVAL_GROUPS = [
  {
    label: "Core Quality",
    color: "text-green-600",
    items: [
      { key: "answer_correctness", label: "Answer Correctness", icon: <CheckCircle2 className="w-4 h-4" /> },
      { key: "groundedness",       label: "Groundedness",        icon: <FileText className="w-4 h-4" /> },
      { key: "citation_precision", label: "Citation Precision",  icon: <Star className="w-4 h-4" /> },
      { key: "task_completion",    label: "Task Completion",     icon: <Zap className="w-4 h-4" /> },
    ],
  },
  {
    label: "Retrieval Pipeline",
    color: "text-blue-600",
    items: [
      { key: "retrieval_quality", label: "Retrieval Quality",  icon: <BarChart3 className="w-4 h-4" /> },
      { key: "permission_safety", label: "Permission Safety",  icon: <Shield className="w-4 h-4" /> },
    ],
  },
  {
    label: "Memory & Context",
    color: "text-purple-600",
    items: [
      { key: "memory_utility",    label: "Memory Utility",     icon: <MemoryStick className="w-4 h-4" /> },
      { key: "context_poisoning", label: "Context Poisoning",  icon: <AlertTriangle className="w-4 h-4" /> },
      { key: "session_coherence", label: "Session Coherence",  icon: <Lock className="w-4 h-4" /> },
    ],
  },
  {
    label: "Agent Behaviour",
    color: "text-indigo-600",
    items: [
      { key: "tool_correctness",  label: "Tool Correctness",   icon: <Wrench className="w-4 h-4" /> },
      { key: "trajectory_quality",label: "Trajectory Quality", icon: <Activity className="w-4 h-4" /> },
    ],
  },
  {
    label: "Cost & Performance",
    color: "text-emerald-600",
    items: [
      { key: "cost_efficiency", label: "Cost Efficiency", icon: <DollarSign className="w-4 h-4" /> },
    ],
  },
];

export default function Dashboard() {
  const [runs, setRuns] = useState<Run[]>([]);
  const [stats, setStats] = useState<Stats>({
    total_runs: 0, completed: 0, failed: 0,
    avg_latency: 0, avg_tokens: 0, total_cost: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`${API_URL}/api/v1/runs`)
      .then((r) => r.ok ? r.json() : [])
      .then((data: Run[]) => {
        setRuns(data);
        const completed = data.filter((r) => r.status === "completed").length;
        const failed = data.filter((r) => r.status === "failed").length;
        const totalCost = data.reduce((s, r) => s + (r.estimated_cost || 0), 0);
        setStats({
          total_runs: data.length,
          completed,
          failed,
          avg_latency: data.length ? Math.round(data.reduce((s, r) => s + (r.latency_ms || 0), 0) / data.length) : 0,
          avg_tokens: data.length ? Math.round(data.reduce((s, r) => s + (r.total_tokens || 0), 0) / data.length) : 0,
          total_cost: totalCost,
        });
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
        <p className="text-sm text-slate-500 mt-1">
          Full-stack AI agent evaluation — retrieval, memory, context, tools, cost
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        {[
          { icon: <Activity className="w-5 h-5 text-brand-600" />, label: "Total Runs", value: stats.total_runs.toString() },
          { icon: <CheckCircle2 className="w-5 h-5 text-green-600" />, label: "Completed", value: stats.completed.toString() },
          { icon: <XCircle className="w-5 h-5 text-red-600" />, label: "Failed", value: stats.failed.toString() },
          { icon: <Clock className="w-5 h-5 text-amber-600" />, label: "Avg Latency", value: `${stats.avg_latency}ms` },
          { icon: <Cpu className="w-5 h-5 text-purple-600" />, label: "Avg Tokens", value: stats.avg_tokens.toLocaleString() },
          { icon: <DollarSign className="w-5 h-5 text-emerald-600" />, label: "Total Cost", value: `$${stats.total_cost.toFixed(3)}` },
        ].map((s) => (
          <div key={s.label} className="card p-4">
            <div className="flex items-center gap-2">
              {s.icon}
              <div>
                <p className="text-xl font-bold text-slate-900">{s.value}</p>
                <p className="text-xs text-slate-500">{s.label}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Evaluator grid */}
      <div className="card p-6">
        <h2 className="text-base font-semibold text-slate-900 mb-4">
          12 Evaluators — Full AI Agent Quality Stack
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          {EVAL_GROUPS.map((group) => (
            <div key={group.label}>
              <p className={`text-xs font-semibold uppercase tracking-wider mb-2 ${group.color}`}>
                {group.label}
              </p>
              <div className="space-y-1.5">
                {group.items.map((item) => (
                  <div
                    key={item.key}
                    className="flex items-center gap-2 px-3 py-2 rounded-md border border-slate-100 hover:border-slate-200 bg-slate-50 hover:bg-white transition-colors"
                  >
                    <span className={group.color}>{item.icon}</span>
                    <span className="text-xs font-medium text-slate-700">{item.label}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Recent runs */}
      <div className="card">
        <div className="px-6 py-4 border-b border-slate-200 flex items-center justify-between">
          <h2 className="text-base font-semibold text-slate-900">Recent Runs</h2>
          <a href="/runs" className="text-xs text-brand-600 hover:text-brand-700">View all →</a>
        </div>
        {loading ? (
          <div className="p-8 text-center text-slate-500 text-sm">Loading...</div>
        ) : runs.length === 0 ? (
          <div className="p-8 text-center">
            <Activity className="w-12 h-12 text-slate-300 mx-auto mb-3" />
            <p className="text-slate-500 text-sm">No runs yet.</p>
            <code className="text-xs bg-slate-100 px-3 py-2 rounded mt-3 inline-block text-slate-600">
              POST /api/v1/runs
            </code>
          </div>
        ) : (
          <div className="divide-y divide-slate-100">
            {runs.slice(0, 8).map((run) => (
              <a
                key={run.id}
                href={`/runs/${run.id}`}
                className="flex items-center justify-between px-6 py-3 hover:bg-slate-50 transition-colors"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-slate-900 truncate">{run.query}</p>
                  <p className="text-xs text-slate-500 mt-0.5">
                    {run.model || "—"} · {(run.total_tokens || 0).toLocaleString()} tokens · {run.latency_ms}ms
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

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    completed: "bg-green-100 text-green-700",
    failed:    "bg-red-100 text-red-700",
    running:   "bg-blue-100 text-blue-700",
    pending:   "bg-slate-100 text-slate-600",
    timeout:   "bg-amber-100 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status] || styles.pending}`}>
      {status}
    </span>
  );
}
