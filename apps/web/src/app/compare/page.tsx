"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  ArrowLeft,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  Activity,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface EvalResult {
  category: string;
  score: number;
  passed: boolean;
  explanation?: string;
  is_critical?: boolean;
}

interface EvalSummary {
  grade: string;
  avg_score: string;
  passed: number;
  failed: number;
  critical_failures: Array<{ category: string }>;
  results: EvalResult[];
}

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

interface RunData {
  run: Run | null;
  summary: EvalSummary | null;
  loading: boolean;
  error: string | null;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function scoreColor(score: number): string {
  if (score >= 0.8) return "text-green-700";
  if (score >= 0.6) return "text-amber-600";
  return "text-red-600";
}

function deltaBadge(delta: number) {
  if (Math.abs(delta) < 0.005) {
    return (
      <span className="inline-flex items-center gap-0.5 text-xs text-slate-400 font-medium">
        <Minus className="w-3 h-3" />
        {delta === 0 ? "—" : delta.toFixed(3)}
      </span>
    );
  }
  if (delta > 0) {
    return (
      <span className="inline-flex items-center gap-0.5 text-xs text-green-700 font-semibold">
        <ArrowUpRight className="w-3 h-3" />+{delta.toFixed(3)}
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-0.5 text-xs text-red-600 font-semibold">
      <ArrowDownRight className="w-3 h-3" />
      {delta.toFixed(3)}
    </span>
  );
}

function rowBg(delta: number): string {
  if (Math.abs(delta) < 0.005) return "";
  return delta > 0 ? "bg-green-50" : "bg-red-50";
}

function gradeColor(grade: string): string {
  const map: Record<string, string> = {
    A: "text-green-700 bg-green-50 border-green-200",
    B: "text-blue-700 bg-blue-50 border-blue-200",
    C: "text-amber-700 bg-amber-50 border-amber-200",
    D: "text-orange-700 bg-orange-50 border-orange-200",
    F: "text-red-700 bg-red-50 border-red-200",
  };
  return map[grade] ?? "text-slate-600 bg-slate-50 border-slate-200";
}

function StatusBadge({ status }: { status: string }) {
  const config: Record<string, { color: string; icon: React.ReactNode }> = {
    completed: { color: "text-green-700 bg-green-50", icon: <CheckCircle2 className="w-3 h-3" /> },
    failed:    { color: "text-red-700 bg-red-50",     icon: <XCircle className="w-3 h-3" /> },
    running:   { color: "text-blue-700 bg-blue-50",   icon: <Activity className="w-3 h-3 animate-pulse" /> },
    pending:   { color: "text-slate-600 bg-slate-100", icon: <Minus className="w-3 h-3" /> },
  };
  const c = config[status] ?? config.pending;
  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${c.color}`}>
      {c.icon} {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Summary sentence
// ---------------------------------------------------------------------------

function buildSummary(
  aResults: EvalResult[],
  bResults: EvalResult[],
  labelA: string,
  labelB: string
): string {
  const aMap = new Map(aResults.map((r) => [r.category, r.score]));
  const bMap = new Map(bResults.map((r) => [r.category, r.score]));

  const improvements: string[] = [];
  const regressions: string[] = [];

  for (const [cat, bScore] of bMap) {
    const aScore = aMap.get(cat);
    if (aScore === undefined) continue;
    const delta = bScore - aScore;
    if (delta > 0.005) {
      improvements.push(`${cat.replace(/_/g, " ")} by +${delta.toFixed(2)}`);
    } else if (delta < -0.005) {
      regressions.push(`${cat.replace(/_/g, " ")} by ${delta.toFixed(2)}`);
    }
  }

  if (improvements.length === 0 && regressions.length === 0) {
    return `${labelB} scores are equivalent to ${labelA} — no significant changes detected.`;
  }

  const parts: string[] = [];
  if (improvements.length > 0) {
    parts.push(`${labelB} improved ${improvements.slice(0, 3).join(", ")}`);
  }
  if (regressions.length > 0) {
    parts.push(`regressed ${regressions.slice(0, 3).join(", ")}`);
  }
  return parts.join(" and ") + ".";
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function ComparePage() {
  const searchParams = useSearchParams();
  const runAId = searchParams.get("a") ?? "";
  const runBId = searchParams.get("b") ?? "";

  const [dataA, setDataA] = useState<RunData>({ run: null, summary: null, loading: true, error: null });
  const [dataB, setDataB] = useState<RunData>({ run: null, summary: null, loading: true, error: null });

  async function fetchRunData(id: string, setter: (d: RunData) => void) {
    if (!id) {
      setter({ run: null, summary: null, loading: false, error: "No run ID provided" });
      return;
    }
    try {
      const [runRes, sumRes] = await Promise.all([
        fetch(`${API_URL}/api/v1/runs/${id}`),
        fetch(`${API_URL}/api/v1/runs/${id}/eval-summary`),
      ]);

      const run: Run | null = runRes.ok ? await runRes.json() : null;
      const summary: EvalSummary | null = sumRes.ok ? await sumRes.json() : null;

      setter({ run, summary, loading: false, error: run ? null : `Run ${id} not found` });
    } catch (e) {
      setter({ run: null, summary: null, loading: false, error: String(e) });
    }
  }

  useEffect(() => {
    fetchRunData(runAId, setDataA);
  }, [runAId]);

  useEffect(() => {
    fetchRunData(runBId, setDataB);
  }, [runBId]);

  const loading = dataA.loading || dataB.loading;

  // Merge categories from both runs for the comparison table
  const allCategories = Array.from(
    new Set([
      ...(dataA.summary?.results ?? []).map((r) => r.category),
      ...(dataB.summary?.results ?? []).map((r) => r.category),
    ])
  ).sort();

  const aMap = new Map((dataA.summary?.results ?? []).map((r) => [r.category, r]));
  const bMap = new Map((dataB.summary?.results ?? []).map((r) => [r.category, r]));

  const labelA = runAId ? `Run A (…${runAId.slice(-8)})` : "Run A";
  const labelB = runBId ? `Run B (…${runBId.slice(-8)})` : "Run B";

  const summaryLine =
    !loading && dataA.summary && dataB.summary
      ? buildSummary(dataA.summary.results, dataB.summary.results, labelA, labelB)
      : null;

  const missingRunId = !runAId || !runBId;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <a
            href="/runs"
            className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 mb-2"
          >
            <ArrowLeft className="w-3 h-3" /> Back to Runs
          </a>
          <h1 className="text-2xl font-bold text-slate-900">Run Comparison</h1>
          <p className="text-sm text-slate-500 mt-1">
            Side-by-side evaluation scores for two runs
          </p>
        </div>
      </div>

      {/* No IDs warning */}
      {missingRunId && (
        <div className="card p-6 text-center">
          <AlertTriangle className="w-10 h-10 text-amber-400 mx-auto mb-3" />
          <p className="text-slate-700 font-medium">Two run IDs are required</p>
          <p className="text-sm text-slate-500 mt-1">
            Navigate here with <code className="text-xs bg-slate-100 px-1 rounded">?a=&lt;run_id&gt;&amp;b=&lt;run_id&gt;</code> in the URL,
            or use the Compare button on the Runs page.
          </p>
        </div>
      )}

      {!missingRunId && (
        <>
          {/* Run metadata cards */}
          <div className="grid grid-cols-2 gap-4">
            {([
              { label: "Run A (baseline)", data: dataA, id: runAId },
              { label: "Run B (candidate)", data: dataB, id: runBId },
            ] as const).map(({ label, data, id }) => (
              <div key={id} className="card p-5">
                <div className="flex items-center justify-between mb-3">
                  <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">{label}</p>
                  {data.summary && (
                    <span
                      className={`text-sm font-bold px-2.5 py-0.5 rounded border ${gradeColor(data.summary.grade)}`}
                    >
                      {data.summary.grade}
                    </span>
                  )}
                </div>
                {data.loading ? (
                  <p className="text-sm text-slate-400">Loading…</p>
                ) : data.error ? (
                  <p className="text-sm text-red-600">{data.error}</p>
                ) : data.run ? (
                  <div className="space-y-1.5">
                    <p className="text-sm font-medium text-slate-900 truncate" title={data.run.query}>
                      {data.run.query}
                    </p>
                    <p className="text-xs text-slate-400 font-mono">{id}</p>
                    <div className="flex items-center gap-3 mt-2 flex-wrap">
                      <StatusBadge status={data.run.status} />
                      {data.run.model && (
                        <span className="text-xs text-slate-500">{data.run.model}</span>
                      )}
                      <span className="text-xs text-slate-500">
                        {(data.run.total_tokens || 0).toLocaleString()} tokens
                      </span>
                      <span className="text-xs text-slate-500">
                        {data.run.latency_ms}ms
                      </span>
                    </div>
                    {data.summary && (
                      <div className="flex items-center gap-3 mt-1 text-xs text-slate-500">
                        <span className="text-green-700">{data.summary.passed} passed</span>
                        <span className="text-red-600">{data.summary.failed} failed</span>
                        <span>avg {parseFloat(data.summary.avg_score).toFixed(3)}</span>
                      </div>
                    )}
                    <a
                      href={`/runs/${id}`}
                      className="inline-flex items-center gap-1 text-xs text-brand-600 hover:text-brand-700 mt-1"
                    >
                      View full report <ArrowUpRight className="w-3 h-3" />
                    </a>
                  </div>
                ) : (
                  <p className="text-sm text-slate-400">Run not found</p>
                )}
              </div>
            ))}
          </div>

          {/* Summary sentence */}
          {summaryLine && (
            <div className="card px-5 py-4 bg-slate-50 border-slate-200">
              <p className="text-sm text-slate-700">
                <span className="font-semibold">Summary:</span> {summaryLine}
              </p>
            </div>
          )}

          {/* Comparison table */}
          {loading ? (
            <div className="card p-8 text-center text-sm text-slate-400">Loading evaluation data…</div>
          ) : allCategories.length === 0 ? (
            <div className="card p-8 text-center">
              <Activity className="w-10 h-10 text-slate-300 mx-auto mb-3" />
              <p className="text-slate-500 text-sm">No evaluation results available for these runs.</p>
              <p className="text-xs text-slate-400 mt-1">
                Trigger evaluations via <code className="bg-slate-100 px-1 rounded">POST /api/v1/runs/:id/evaluate</code>
              </p>
            </div>
          ) : (
            <div className="card overflow-hidden">
              <div className="px-6 py-4 border-b border-slate-200">
                <h2 className="text-base font-semibold text-slate-900">Evaluator Scores</h2>
                <p className="text-xs text-slate-500 mt-0.5">
                  Green rows = improvement in Run B. Red rows = regression in Run B.
                </p>
              </div>

              {/* Table header */}
              <div className="grid grid-cols-[1fr_100px_100px_96px] gap-3 px-6 py-2.5 bg-slate-50 text-xs font-semibold text-slate-500 uppercase tracking-wider border-b border-slate-200">
                <div>Evaluator</div>
                <div className="text-center">Run A</div>
                <div className="text-center">Run B</div>
                <div className="text-center">Delta</div>
              </div>

              <div className="divide-y divide-slate-100">
                {allCategories.map((cat) => {
                  const a = aMap.get(cat);
                  const b = bMap.get(cat);
                  const aScore = a?.score ?? null;
                  const bScore = b?.score ?? null;
                  const delta =
                    aScore !== null && bScore !== null ? bScore - aScore : null;

                  return (
                    <div
                      key={cat}
                      className={`grid grid-cols-[1fr_100px_100px_96px] gap-3 px-6 py-3 items-center transition-colors ${
                        delta !== null ? rowBg(delta) : ""
                      }`}
                    >
                      {/* Evaluator name */}
                      <div>
                        <p className="text-sm font-medium text-slate-800 capitalize">
                          {cat.replace(/_/g, " ")}
                        </p>
                        {(a?.is_critical || b?.is_critical) && (
                          <span className="inline-flex items-center gap-0.5 text-xs text-red-600">
                            <AlertTriangle className="w-3 h-3" /> critical
                          </span>
                        )}
                      </div>

                      {/* Run A score */}
                      <div className="text-center">
                        {aScore !== null ? (
                          <div className="space-y-0.5">
                            <p className={`text-sm font-semibold tabular-nums ${scoreColor(aScore)}`}>
                              {aScore.toFixed(3)}
                            </p>
                            {a && (
                              <span
                                className={`text-xs font-medium ${
                                  a.passed ? "text-green-600" : "text-red-500"
                                }`}
                              >
                                {a.passed ? "pass" : "fail"}
                              </span>
                            )}
                          </div>
                        ) : (
                          <span className="text-slate-300 text-sm">—</span>
                        )}
                      </div>

                      {/* Run B score */}
                      <div className="text-center">
                        {bScore !== null ? (
                          <div className="space-y-0.5">
                            <p className={`text-sm font-semibold tabular-nums ${scoreColor(bScore)}`}>
                              {bScore.toFixed(3)}
                            </p>
                            {b && (
                              <span
                                className={`text-xs font-medium ${
                                  b.passed ? "text-green-600" : "text-red-500"
                                }`}
                              >
                                {b.passed ? "pass" : "fail"}
                              </span>
                            )}
                          </div>
                        ) : (
                          <span className="text-slate-300 text-sm">—</span>
                        )}
                      </div>

                      {/* Delta */}
                      <div className="text-center">
                        {delta !== null ? deltaBadge(delta) : <span className="text-slate-300 text-sm">—</span>}
                      </div>
                    </div>
                  );
                })}
              </div>

              {/* Footer totals */}
              {dataA.summary && dataB.summary && (
                <div className="grid grid-cols-[1fr_100px_100px_96px] gap-3 px-6 py-3 border-t border-slate-200 bg-slate-50 items-center">
                  <p className="text-xs font-semibold text-slate-600 uppercase tracking-wider">
                    Overall average
                  </p>
                  <p className={`text-center text-sm font-bold tabular-nums ${scoreColor(parseFloat(dataA.summary.avg_score))}`}>
                    {parseFloat(dataA.summary.avg_score).toFixed(3)}
                  </p>
                  <p className={`text-center text-sm font-bold tabular-nums ${scoreColor(parseFloat(dataB.summary.avg_score))}`}>
                    {parseFloat(dataB.summary.avg_score).toFixed(3)}
                  </p>
                  <div className="text-center">
                    {deltaBadge(
                      parseFloat(dataB.summary.avg_score) - parseFloat(dataA.summary.avg_score)
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}
