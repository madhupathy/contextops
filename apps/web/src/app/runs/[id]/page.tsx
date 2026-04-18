"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Cpu,
  DollarSign,
  FileText,
  Lock,
  MemoryStick,
  Shield,
  Star,
  Wrench,
  XCircle,
  Zap,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

interface Evaluation {
  category: string;
  score: number;
  passed: boolean;
  reasoning: string | null;
  details: Record<string, unknown>;
  eval_latency_ms: number;
}

interface RunTimeline {
  run: {
    id: string;
    query: string;
    final_answer: string | null;
    status: string;
    model: string | null;
    total_tokens: number;
    prompt_tokens: number;
    completion_tokens: number;
    latency_ms: number;
    estimated_cost: number;
    created_at: string;
  };
  retrieval_candidates: Array<{
    doc_id: string;
    title: string | null;
    score: number;
    selected: boolean;
    acl_passed: boolean;
    acl_reason: string | null;
    rejection_reason: string | null;
  }>;
  memory_candidates: Array<{
    memory_id: string;
    memory_type: string;
    content: string;
    relevance_score: number;
    is_stale: boolean;
    stale_reason: string | null;
    selected: boolean;
  }>;
  tool_calls: Array<{
    tool_name: string;
    step_number: number;
    status: string;
    latency_ms: number;
    error_message: string | null;
  }>;
  reasoning_steps: Array<{
    step_number: number;
    step_type: string;
    content: string;
    tokens_used: number;
  }>;
  evaluations: Evaluation[];
}

const CATEGORY_META: Record<string, { label: string; icon: JSX.Element; group: string }> = {
  answer_correctness:  { label: "Answer Correctness",  icon: <CheckCircle2 className="w-4 h-4" />, group: "core" },
  groundedness:        { label: "Groundedness",         icon: <FileText className="w-4 h-4" />,    group: "core" },
  citation_precision:  { label: "Citation Precision",   icon: <Star className="w-4 h-4" />,        group: "core" },
  task_completion:     { label: "Task Completion",      icon: <Zap className="w-4 h-4" />,         group: "core" },
  retrieval_quality:   { label: "Retrieval Quality",    icon: <ChevronRight className="w-4 h-4" />, group: "retrieval" },
  permission_safety:   { label: "Permission Safety",    icon: <Shield className="w-4 h-4" />,      group: "retrieval" },
  memory_utility:      { label: "Memory Utility",       icon: <MemoryStick className="w-4 h-4" />, group: "memory" },
  context_poisoning:   { label: "Context Poisoning",    icon: <AlertTriangle className="w-4 h-4" />, group: "memory" },
  session_coherence:   { label: "Session Coherence",    icon: <Lock className="w-4 h-4" />,        group: "memory" },
  tool_correctness:    { label: "Tool Correctness",     icon: <Wrench className="w-4 h-4" />,      group: "agent" },
  trajectory_quality:  { label: "Trajectory Quality",   icon: <Zap className="w-4 h-4" />,         group: "agent" },
  cost_efficiency:     { label: "Cost Efficiency",      icon: <DollarSign className="w-4 h-4" />,  group: "cost" },
};

const GROUP_LABELS: Record<string, string> = {
  core: "Core Quality",
  retrieval: "Retrieval Pipeline",
  memory: "Memory & Context",
  agent: "Agent Behaviour",
  cost: "Cost & Performance",
};

export default function RunDetailPage() {
  const params = useParams();
  const runId = params.id as string;
  const [timeline, setTimeline] = useState<RunTimeline | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"evaluations" | "retrieval" | "memory" | "trace">("evaluations");

  useEffect(() => {
    if (!runId) return;
    fetch(`${API_URL}/api/v1/runs/${runId}/timeline`)
      .then((r) => r.ok ? r.json() : null)
      .then((d) => { if (d) setTimeline(d); })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [runId]);

  if (loading) {
    return <div className="p-8 text-center text-slate-500">Loading run details...</div>;
  }
  if (!timeline) {
    return (
      <div className="p-8 text-center">
        <XCircle className="w-12 h-12 text-red-300 mx-auto mb-3" />
        <p className="text-slate-500">Run not found</p>
        <a href="/runs" className="text-brand-600 text-sm mt-2 inline-block">← Back to Runs</a>
      </div>
    );
  }

  const { run, evaluations } = timeline;
  const passed = evaluations.filter((e) => e.passed).length;
  const avgScore = evaluations.length > 0
    ? evaluations.reduce((s, e) => s + e.score, 0) / evaluations.length
    : 0;

  const criticalFails = evaluations.filter(
    (e) => !e.passed && ["permission_safety", "context_poisoning"].includes(e.category)
  );

  // Group evaluations
  const grouped: Record<string, Evaluation[]> = {};
  for (const ev of evaluations) {
    const group = CATEGORY_META[ev.category]?.group ?? "other";
    (grouped[group] = grouped[group] || []).push(ev);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <a href="/runs" className="flex items-center gap-1 text-sm text-slate-500 hover:text-slate-700 mb-2">
            <ArrowLeft className="w-4 h-4" /> Back to Runs
          </a>
          <h1 className="text-xl font-bold text-slate-900 line-clamp-2">{run.query}</h1>
          <div className="flex items-center gap-4 mt-2 text-xs text-slate-500">
            <span className="flex items-center gap-1"><Cpu className="w-3 h-3" /> {run.total_tokens.toLocaleString()} tokens</span>
            <span className="flex items-center gap-1"><Clock className="w-3 h-3" /> {run.latency_ms}ms</span>
            <span className="flex items-center gap-1"><DollarSign className="w-3 h-3" /> ${(run.estimated_cost || 0).toFixed(4)}</span>
            <StatusBadge status={run.status} />
          </div>
        </div>
        {/* Score summary */}
        <div className="text-right ml-6 shrink-0">
          <div className={`text-3xl font-bold ${scoreColor(avgScore)}`}>
            {(avgScore * 100).toFixed(0)}
          </div>
          <div className="text-xs text-slate-500 mt-0.5">{passed}/{evaluations.length} passed</div>
        </div>
      </div>

      {/* Critical failure banner */}
      {criticalFails.length > 0 && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-red-600 shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-semibold text-red-800">Critical failures detected</p>
              {criticalFails.map((f) => (
                <p key={f.category} className="text-sm text-red-700 mt-1">
                  <strong>{CATEGORY_META[f.category]?.label ?? f.category}:</strong> {f.reasoning}
                </p>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Final answer */}
      {run.final_answer && (
        <div className="card p-4">
          <h2 className="text-sm font-semibold text-slate-700 mb-2">Final Answer</h2>
          <p className="text-sm text-slate-600 whitespace-pre-wrap">{run.final_answer}</p>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-slate-200">
        <nav className="flex gap-6">
          {(["evaluations", "retrieval", "memory", "trace"] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`pb-3 text-sm font-medium border-b-2 capitalize transition-colors ${
                activeTab === tab
                  ? "border-brand-600 text-brand-600"
                  : "border-transparent text-slate-500 hover:text-slate-700"
              }`}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab content */}
      {activeTab === "evaluations" && (
        <div className="space-y-4">
          {Object.entries(GROUP_LABELS).map(([groupKey, groupLabel]) => {
            const evals = grouped[groupKey];
            if (!evals?.length) return null;
            return (
              <div key={groupKey}>
                <h3 className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-2">
                  {groupLabel}
                </h3>
                <div className="space-y-2">
                  {evals.map((ev) => (
                    <EvalRow key={ev.category} ev={ev} />
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {activeTab === "retrieval" && (
        <RetrievalTab candidates={timeline.retrieval_candidates} />
      )}

      {activeTab === "memory" && (
        <MemoryTab candidates={timeline.memory_candidates} />
      )}

      {activeTab === "trace" && (
        <TraceTab steps={timeline.reasoning_steps} toolCalls={timeline.tool_calls} />
      )}
    </div>
  );
}

function EvalRow({ ev }: { ev: Evaluation }) {
  const [open, setOpen] = useState(false);
  const meta = CATEGORY_META[ev.category];

  return (
    <div className={`card border ${ev.passed ? "border-slate-100" : "border-red-100 bg-red-50/30"}`}>
      <button
        className="w-full flex items-center justify-between p-4"
        onClick={() => setOpen((o) => !o)}
      >
        <div className="flex items-center gap-3">
          <span className={ev.passed ? "text-slate-400" : "text-red-500"}>
            {meta?.icon}
          </span>
          <span className="text-sm font-medium text-slate-800">
            {meta?.label ?? ev.category}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <ScoreBar score={ev.score} passed={ev.passed} />
          <span className={`text-sm font-bold ${scoreColor(ev.score)}`}>
            {(ev.score * 100).toFixed(0)}
          </span>
          {ev.passed
            ? <CheckCircle2 className="w-4 h-4 text-green-500" />
            : <XCircle className="w-4 h-4 text-red-500" />}
          {open
            ? <ChevronDown className="w-4 h-4 text-slate-400" />
            : <ChevronRight className="w-4 h-4 text-slate-400" />}
        </div>
      </button>
      {open && ev.reasoning && (
        <div className="px-4 pb-4 border-t border-slate-100 pt-3">
          <p className="text-sm text-slate-600">{ev.reasoning}</p>
        </div>
      )}
    </div>
  );
}

function RetrievalTab({ candidates }: { candidates: RunTimeline["retrieval_candidates"] }) {
  if (!candidates.length) {
    return <div className="card p-8 text-center text-slate-500 text-sm">No retrieval candidates in this run.</div>;
  }
  return (
    <div className="card divide-y divide-slate-100">
      {candidates.map((c, i) => (
        <div key={i} className="flex items-start justify-between px-4 py-3 gap-4">
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-slate-800 truncate">{c.title || c.doc_id}</p>
            <p className="text-xs text-slate-500 mt-0.5 font-mono">{c.doc_id}</p>
            {c.acl_reason && (
              <p className="text-xs text-red-600 mt-0.5">ACL: {c.acl_reason}</p>
            )}
            {c.rejection_reason && (
              <p className="text-xs text-amber-600 mt-0.5">Rejected: {c.rejection_reason}</p>
            )}
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="text-xs text-slate-500 font-mono">{c.score.toFixed(3)}</span>
            {c.selected && <span className="badge-green">selected</span>}
            {!c.acl_passed && <span className="badge-red">ACL blocked</span>}
          </div>
        </div>
      ))}
    </div>
  );
}

function MemoryTab({ candidates }: { candidates: RunTimeline["memory_candidates"] }) {
  if (!candidates.length) {
    return <div className="card p-8 text-center text-slate-500 text-sm">No memory candidates in this run.</div>;
  }
  return (
    <div className="card divide-y divide-slate-100">
      {candidates.map((m, i) => (
        <div key={i} className="px-4 py-3">
          <div className="flex items-center justify-between mb-1">
            <span className="text-xs font-medium text-slate-600 uppercase">{m.memory_type}</span>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-500">rel: {m.relevance_score.toFixed(2)}</span>
              {m.selected && <span className="badge-green">used</span>}
              {m.is_stale && <span className="badge-red">stale</span>}
            </div>
          </div>
          <p className="text-sm text-slate-700">{m.content}</p>
          {m.stale_reason && (
            <p className="text-xs text-red-500 mt-1">Stale reason: {m.stale_reason}</p>
          )}
        </div>
      ))}
    </div>
  );
}

function TraceTab({
  steps,
  toolCalls,
}: {
  steps: RunTimeline["reasoning_steps"];
  toolCalls: RunTimeline["tool_calls"];
}) {
  if (!steps.length && !toolCalls.length) {
    return <div className="card p-8 text-center text-slate-500 text-sm">No reasoning trace available.</div>;
  }

  // Interleave steps and tool calls by step_number
  type TraceItem =
    | { kind: "step"; data: RunTimeline["reasoning_steps"][0] }
    | { kind: "tool"; data: RunTimeline["tool_calls"][0] };

  const items: TraceItem[] = [
    ...steps.map((s) => ({ kind: "step" as const, data: s })),
    ...toolCalls.map((t) => ({ kind: "tool" as const, data: t })),
  ].sort((a, b) => a.data.step_number - b.data.step_number);

  return (
    <div className="space-y-2">
      {items.map((item, i) => (
        <div key={i} className="card px-4 py-3">
          {item.kind === "step" ? (
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-xs font-medium text-brand-600 uppercase">{item.data.step_type}</span>
                <span className="text-xs text-slate-400">step {item.data.step_number}</span>
                {item.data.tokens_used > 0 && (
                  <span className="text-xs text-slate-400">{item.data.tokens_used} tokens</span>
                )}
              </div>
              <p className="text-sm text-slate-700">{item.data.content}</p>
            </div>
          ) : (
            <div>
              <div className="flex items-center gap-2 mb-1">
                <Wrench className="w-3 h-3 text-slate-400" />
                <span className="text-xs font-medium text-slate-700">{item.data.tool_name}</span>
                <span className="text-xs text-slate-400">step {item.data.step_number}</span>
                <StatusBadge status={item.data.status} />
                <span className="text-xs text-slate-400">{item.data.latency_ms}ms</span>
              </div>
              {item.data.error_message && (
                <p className="text-xs text-red-600">{item.data.error_message}</p>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function ScoreBar({ score, passed }: { score: number; passed: boolean }) {
  return (
    <div className="w-20 bg-slate-100 rounded-full h-1.5 overflow-hidden">
      <div
        className={`h-full rounded-full ${passed ? "bg-green-500" : score > 0.5 ? "bg-amber-500" : "bg-red-500"}`}
        style={{ width: `${Math.round(score * 100)}%` }}
      />
    </div>
  );
}

function scoreColor(score: number): string {
  if (score >= 0.8) return "text-green-600";
  if (score >= 0.6) return "text-amber-600";
  return "text-red-600";
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    completed: "bg-green-100 text-green-700",
    success:   "bg-green-100 text-green-700",
    failed:    "bg-red-100 text-red-700",
    failure:   "bg-red-100 text-red-700",
    running:   "bg-blue-100 text-blue-700",
    pending:   "bg-slate-100 text-slate-600",
    timeout:   "bg-amber-100 text-amber-700",
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status] ?? styles.pending}`}>
      {status}
    </span>
  );
}
