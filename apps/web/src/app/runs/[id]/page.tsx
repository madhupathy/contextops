"use client";

import { useEffect, useState } from "react";
import {
  AlertTriangle,
  ArrowLeft,
  BookOpen,
  Brain,
  CheckCircle2,
  Clock,
  Cpu,
  Database,
  DollarSign,
  Eye,
  EyeOff,
  Shield,
  ShieldAlert,
  Wrench,
  XCircle,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Timeline {
  run: any;
  retrieval_candidates: any[];
  memory_candidates: any[];
  tool_calls: any[];
  reasoning_steps: any[];
  evaluations: any[];
}

export default function RunDetailPage({ params }: { params: { id: string } }) {
  const [timeline, setTimeline] = useState<Timeline | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("overview");

  useEffect(() => {
    fetchTimeline();
  }, [params.id]);

  async function fetchTimeline() {
    try {
      const res = await fetch(`${API_URL}/api/v1/runs/${params.id}/timeline`);
      if (res.ok) {
        setTimeline(await res.json());
      }
    } catch (e) {
      console.error("Failed to fetch timeline:", e);
    } finally {
      setLoading(false);
    }
  }

  async function triggerEvaluation() {
    try {
      const res = await fetch(`${API_URL}/api/v1/runs/${params.id}/evaluate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });
      if (res.ok) {
        setTimeout(fetchTimeline, 2000);
      }
    } catch (e) {
      console.error("Failed to trigger evaluation:", e);
    }
  }

  if (loading) {
    return <div className="text-center py-12 text-slate-500">Loading run timeline...</div>;
  }

  if (!timeline) {
    return <div className="text-center py-12 text-slate-500">Run not found</div>;
  }

  const { run, retrieval_candidates, memory_candidates, tool_calls, reasoning_steps, evaluations } =
    timeline;

  const tabs = [
    { id: "overview", label: "Overview", icon: <Eye className="w-4 h-4" /> },
    {
      id: "retrieval",
      label: `Retrieval (${retrieval_candidates.length})`,
      icon: <Database className="w-4 h-4" />,
    },
    {
      id: "memory",
      label: `Memory (${memory_candidates.length})`,
      icon: <Brain className="w-4 h-4" />,
    },
    {
      id: "tools",
      label: `Tools (${tool_calls.length})`,
      icon: <Wrench className="w-4 h-4" />,
    },
    {
      id: "trajectory",
      label: `Trajectory (${reasoning_steps.length})`,
      icon: <BookOpen className="w-4 h-4" />,
    },
    {
      id: "evaluations",
      label: `Evaluations (${evaluations.length})`,
      icon: <CheckCircle2 className="w-4 h-4" />,
    },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <a href="/runs" className="inline-flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 mb-2">
            <ArrowLeft className="w-4 h-4" /> Back to runs
          </a>
          <h1 className="text-xl font-bold text-slate-900 mt-1">Run Timeline</h1>
          <p className="text-sm text-slate-500 mt-1 font-mono">{run.id}</p>
        </div>
        <button
          onClick={triggerEvaluation}
          className="px-4 py-2 bg-brand-600 text-white text-sm font-medium rounded-lg hover:bg-brand-700 transition-colors"
        >
          Run Evaluation
        </button>
      </div>

      {/* Run Summary */}
      <div className="card p-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <h3 className="text-sm font-medium text-slate-500 mb-1">Query</h3>
            <p className="text-base text-slate-900">{run.query}</p>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <MiniStat icon={<Clock className="w-4 h-4" />} label="Latency" value={`${run.latency_ms}ms`} />
            <MiniStat icon={<Cpu className="w-4 h-4" />} label="Tokens" value={run.total_tokens?.toLocaleString() || "0"} />
            <MiniStat icon={<DollarSign className="w-4 h-4" />} label="Cost" value={`$${(run.estimated_cost || 0).toFixed(4)}`} />
            <MiniStat icon={<Shield className="w-4 h-4" />} label="Status" value={run.status} />
          </div>
        </div>
        {run.final_answer && (
          <div className="mt-4 pt-4 border-t border-slate-100">
            <h3 className="text-sm font-medium text-slate-500 mb-1">Final Answer</h3>
            <p className="text-sm text-slate-700 bg-slate-50 rounded-lg p-3">{run.final_answer}</p>
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-slate-200">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? "border-brand-600 text-brand-600"
                : "border-transparent text-slate-500 hover:text-slate-700"
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div>
        {activeTab === "overview" && <OverviewTab timeline={timeline} />}
        {activeTab === "retrieval" && <RetrievalTab candidates={retrieval_candidates} />}
        {activeTab === "memory" && <MemoryTab candidates={memory_candidates} />}
        {activeTab === "tools" && <ToolsTab calls={tool_calls} />}
        {activeTab === "trajectory" && <TrajectoryTab steps={reasoning_steps} />}
        {activeTab === "evaluations" && <EvaluationsTab evaluations={evaluations} />}
      </div>
    </div>
  );
}

function MiniStat({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-slate-400">{icon}</span>
      <div>
        <p className="text-sm font-semibold text-slate-900">{value}</p>
        <p className="text-xs text-slate-500">{label}</p>
      </div>
    </div>
  );
}

function OverviewTab({ timeline }: { timeline: Timeline }) {
  const { retrieval_candidates, memory_candidates, tool_calls, reasoning_steps } = timeline;
  const selected = retrieval_candidates.filter((c) => c.selected);
  const aclBlocked = retrieval_candidates.filter((c) => !c.acl_passed);
  const staleMemory = memory_candidates.filter((m) => m.is_stale && m.selected);
  const failedTools = tool_calls.filter((t) => t.status === "failure");

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div className="card p-4">
        <h3 className="font-medium text-slate-900 mb-3 flex items-center gap-2">
          <Database className="w-4 h-4 text-blue-600" /> Retrieval Summary
        </h3>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between"><span className="text-slate-500">Total candidates</span><span className="font-medium">{retrieval_candidates.length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Selected</span><span className="font-medium text-green-600">{selected.length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">ACL blocked</span><span className={`font-medium ${aclBlocked.length > 0 ? "text-red-600" : "text-slate-900"}`}>{aclBlocked.length}</span></div>
        </div>
      </div>

      <div className="card p-4">
        <h3 className="font-medium text-slate-900 mb-3 flex items-center gap-2">
          <Brain className="w-4 h-4 text-purple-600" /> Memory Summary
        </h3>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between"><span className="text-slate-500">Total memories</span><span className="font-medium">{memory_candidates.length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Selected</span><span className="font-medium">{memory_candidates.filter((m) => m.selected).length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Stale used</span><span className={`font-medium ${staleMemory.length > 0 ? "text-amber-600" : "text-slate-900"}`}>{staleMemory.length}</span></div>
        </div>
      </div>

      <div className="card p-4">
        <h3 className="font-medium text-slate-900 mb-3 flex items-center gap-2">
          <Wrench className="w-4 h-4 text-indigo-600" /> Tool Calls Summary
        </h3>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between"><span className="text-slate-500">Total calls</span><span className="font-medium">{tool_calls.length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Successful</span><span className="font-medium text-green-600">{tool_calls.filter((t) => t.status === "success").length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Failed</span><span className={`font-medium ${failedTools.length > 0 ? "text-red-600" : "text-slate-900"}`}>{failedTools.length}</span></div>
        </div>
      </div>

      <div className="card p-4">
        <h3 className="font-medium text-slate-900 mb-3 flex items-center gap-2">
          <BookOpen className="w-4 h-4 text-orange-600" /> Trajectory Summary
        </h3>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between"><span className="text-slate-500">Total steps</span><span className="font-medium">{reasoning_steps.length}</span></div>
          <div className="flex justify-between"><span className="text-slate-500">Total tokens</span><span className="font-medium">{reasoning_steps.reduce((s: number, r: any) => s + (r.tokens_used || 0), 0).toLocaleString()}</span></div>
        </div>
      </div>
    </div>
  );
}

function RetrievalTab({ candidates }: { candidates: any[] }) {
  if (!candidates.length) return <EmptyState message="No retrieval candidates" />;

  return (
    <div className="space-y-3">
      {candidates.map((c, i) => (
        <div key={c.id || i} className={`card p-4 ${!c.acl_passed ? "border-red-200 bg-red-50" : c.selected ? "border-green-200 bg-green-50" : ""}`}>
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="font-medium text-sm text-slate-900">{c.title || c.doc_id}</span>
                {c.selected && <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Selected</span>}
                {!c.acl_passed && <span className="text-xs bg-red-100 text-red-700 px-1.5 py-0.5 rounded flex items-center gap-1"><ShieldAlert className="w-3 h-3" /> ACL Blocked</span>}
              </div>
              <p className="text-xs text-slate-500 mt-1">Score: {c.score?.toFixed(3)} | Rank: #{c.rank || i + 1} | Method: {c.retrieval_method || "unknown"}</p>
              {c.content_preview && <p className="text-xs text-slate-600 mt-2 line-clamp-2">{c.content_preview}</p>}
              {c.acl_reason && <p className="text-xs text-red-600 mt-1">ACL reason: {c.acl_reason}</p>}
              {c.rejection_reason && <p className="text-xs text-amber-600 mt-1">Rejected: {c.rejection_reason}</p>}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function MemoryTab({ candidates }: { candidates: any[] }) {
  if (!candidates.length) return <EmptyState message="No memory candidates" />;

  return (
    <div className="space-y-3">
      {candidates.map((m, i) => (
        <div key={m.id || i} className={`card p-4 ${m.is_stale && m.selected ? "border-amber-200 bg-amber-50" : m.selected ? "border-green-200 bg-green-50" : ""}`}>
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xs bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded">{m.memory_type}</span>
            {m.selected && <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Selected</span>}
            {m.is_stale && <span className="text-xs bg-amber-100 text-amber-700 px-1.5 py-0.5 rounded flex items-center gap-1"><AlertTriangle className="w-3 h-3" /> Stale</span>}
          </div>
          <p className="text-sm text-slate-700">{m.content}</p>
          <p className="text-xs text-slate-500 mt-2">Relevance: {m.relevance_score?.toFixed(3)} | Recency: {m.recency_score?.toFixed(3)}</p>
          {m.stale_reason && <p className="text-xs text-amber-600 mt-1">Stale reason: {m.stale_reason}</p>}
        </div>
      ))}
    </div>
  );
}

function ToolsTab({ calls }: { calls: any[] }) {
  if (!calls.length) return <EmptyState message="No tool calls" />;

  return (
    <div className="space-y-3">
      {calls.map((t, i) => (
        <div key={t.id || i} className={`card p-4 ${t.status === "failure" ? "border-red-200 bg-red-50" : ""}`}>
          <div className="flex items-center gap-2 mb-2">
            <span className="font-mono text-sm font-medium text-slate-900">{t.tool_name}</span>
            <span className={`text-xs px-1.5 py-0.5 rounded ${t.status === "success" ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"}`}>{t.status}</span>
            <span className="text-xs text-slate-400">Step {t.step_number}</span>
            {t.requires_approval && <span className="text-xs bg-amber-100 text-amber-700 px-1.5 py-0.5 rounded">Requires Approval</span>}
          </div>
          <div className="grid grid-cols-2 gap-4 mt-2">
            <div>
              <p className="text-xs text-slate-500 mb-1">Arguments</p>
              <pre className="text-xs bg-slate-100 p-2 rounded overflow-x-auto">{JSON.stringify(t.tool_args, null, 2)}</pre>
            </div>
            {t.tool_result && (
              <div>
                <p className="text-xs text-slate-500 mb-1">Result</p>
                <pre className="text-xs bg-slate-100 p-2 rounded overflow-x-auto">{JSON.stringify(t.tool_result, null, 2)}</pre>
              </div>
            )}
          </div>
          {t.error_message && <p className="text-xs text-red-600 mt-2">Error: {t.error_message}</p>}
          <p className="text-xs text-slate-400 mt-2">Latency: {t.latency_ms}ms</p>
        </div>
      ))}
    </div>
  );
}

function TrajectoryTab({ steps }: { steps: any[] }) {
  if (!steps.length) return <EmptyState message="No reasoning steps recorded" />;

  const typeColors: Record<string, string> = {
    think: "bg-purple-100 text-purple-700",
    retrieve: "bg-blue-100 text-blue-700",
    remember: "bg-amber-100 text-amber-700",
    tool_call: "bg-indigo-100 text-indigo-700",
    tool_result: "bg-slate-100 text-slate-700",
    generate: "bg-green-100 text-green-700",
    decide: "bg-orange-100 text-orange-700",
    act: "bg-red-100 text-red-700",
  };

  return (
    <div className="relative">
      <div className="absolute left-6 top-0 bottom-0 w-px bg-slate-200" />
      <div className="space-y-4">
        {steps.map((s, i) => (
          <div key={s.id || i} className="relative flex gap-4 ml-2">
            <div className="w-8 h-8 rounded-full bg-white border-2 border-slate-200 flex items-center justify-center z-10 flex-shrink-0">
              <span className="text-xs font-bold text-slate-500">{s.step_number}</span>
            </div>
            <div className="card p-3 flex-1">
              <div className="flex items-center gap-2 mb-1">
                <span className={`text-xs px-1.5 py-0.5 rounded ${typeColors[s.step_type] || "bg-slate-100 text-slate-600"}`}>{s.step_type}</span>
                <span className="text-xs text-slate-400">{s.latency_ms}ms | {s.tokens_used} tokens</span>
              </div>
              <p className="text-sm text-slate-700">{s.content}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function EvaluationsTab({ evaluations }: { evaluations: any[] }) {
  if (!evaluations.length) {
    return (
      <div className="text-center py-12">
        <CheckCircle2 className="w-12 h-12 text-slate-300 mx-auto mb-3" />
        <p className="text-slate-500 text-sm">No evaluations yet. Click &quot;Run Evaluation&quot; to start.</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      {evaluations.map((ev, i) => (
        <div key={ev.id || i} className={`card p-4 ${ev.passed ? "border-green-200" : "border-red-200"}`}>
          <div className="flex items-center justify-between mb-2">
            <span className="font-medium text-sm text-slate-900">{ev.category.replace(/_/g, " ")}</span>
            <div className="flex items-center gap-2">
              <ScoreBar score={ev.score} />
              {ev.passed ? (
                <CheckCircle2 className="w-4 h-4 text-green-600" />
              ) : (
                <XCircle className="w-4 h-4 text-red-600" />
              )}
            </div>
          </div>
          {ev.reasoning && (
            <p className="text-xs text-slate-600 mt-1">{ev.reasoning}</p>
          )}
          <p className="text-xs text-slate-400 mt-2">
            {ev.evaluator_name} {ev.evaluator_version} | {ev.eval_latency_ms}ms
          </p>
        </div>
      ))}
    </div>
  );
}

function ScoreBar({ score }: { score: number }) {
  const pct = Math.round(score * 100);
  const color = pct >= 80 ? "bg-green-500" : pct >= 60 ? "bg-amber-500" : "bg-red-500";
  return (
    <div className="flex items-center gap-2">
      <div className="w-16 h-2 bg-slate-100 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs font-medium text-slate-700">{pct}%</span>
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="text-center py-12">
      <EyeOff className="w-12 h-12 text-slate-300 mx-auto mb-3" />
      <p className="text-slate-500 text-sm">{message}</p>
    </div>
  );
}
