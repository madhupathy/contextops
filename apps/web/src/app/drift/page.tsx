"use client";

import { useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  CheckCircle2,
  Minus,
  RefreshCw,
  TrendingDown,
  TrendingUp,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DriftPoint {
  date: string;       // "YYYY-MM-DD"
  evaluator: string;
  score: number;
}

interface DriftAlert {
  evaluator: string;
  current: number;
  baseline: number;
  delta: number;
}

interface DriftResponse {
  series: DriftPoint[];
  alerts: DriftAlert[];
}

type DayRange = 7 | 30 | 90;

// Trend entry from the existing drift endpoint (fallback shape)
interface TrendEntry {
  category: string;
  current_avg: number;
  previous_avg?: number;
  delta?: number;
  trend: string;
  current_runs: number;
}

interface AgentDriftResponse {
  drift: TrendEntry[];
  degrading: number;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Generate mock historical data for demo / when endpoint not available */
function generateMockSeries(days: DayRange): DriftPoint[] {
  const evaluators = [
    "answer_correctness",
    "groundedness",
    "retrieval_quality",
    "memory_utility",
    "tool_correctness",
    "cost_efficiency",
  ];
  const points: DriftPoint[] = [];
  const now = new Date();

  for (let d = days; d >= 0; d--) {
    const date = new Date(now);
    date.setDate(date.getDate() - d);
    const dateStr = date.toISOString().slice(0, 10);

    evaluators.forEach((ev) => {
      // Produce plausible slowly-drifting data
      const base =
        ev === "cost_efficiency"
          ? 0.85
          : ev === "tool_correctness"
          ? 0.92
          : ev === "retrieval_quality"
          ? 0.78
          : 0.82;
      // Gentle drift: retrieval_quality drops a bit near the end
      const drift =
        ev === "retrieval_quality" && d < Math.floor(days * 0.3)
          ? -0.08 * (1 - d / (days * 0.3))
          : 0;
      const noise = (Math.random() - 0.5) * 0.05;
      const score = Math.min(1, Math.max(0, base + drift + noise));
      points.push({ date: dateStr, evaluator: ev, score: +score.toFixed(3) });
    });
  }
  return points;
}

function generateMockAlerts(series: DriftPoint[]): DriftAlert[] {
  // Find evaluators where last-3-day avg is significantly below first-3-day avg
  const byEval = new Map<string, DriftPoint[]>();
  for (const p of series) {
    if (!byEval.has(p.evaluator)) byEval.set(p.evaluator, []);
    byEval.get(p.evaluator)!.push(p);
  }
  const alerts: DriftAlert[] = [];
  for (const [ev, pts] of byEval) {
    const sorted = [...pts].sort((a, b) => a.date.localeCompare(b.date));
    const first3 = sorted.slice(0, 3);
    const last3 = sorted.slice(-3);
    const baseline = first3.reduce((s, p) => s + p.score, 0) / first3.length;
    const current = last3.reduce((s, p) => s + p.score, 0) / last3.length;
    const delta = current - baseline;
    if (delta < -0.10) {
      alerts.push({
        evaluator: ev,
        current: +current.toFixed(3),
        baseline: +baseline.toFixed(3),
        delta: +delta.toFixed(3),
      });
    }
  }
  return alerts;
}

function scoreColor(score: number): string {
  if (score >= 0.8) return "#16a34a";   // green-600
  if (score >= 0.6) return "#d97706";   // amber-600
  return "#dc2626";                      // red-600
}

function trendIcon(trend: string) {
  if (trend === "improving")
    return <ArrowUpRight className="w-4 h-4 text-green-600" />;
  if (trend === "degrading")
    return <ArrowDownRight className="w-4 h-4 text-red-600" />;
  if (trend === "stable") return <Minus className="w-4 h-4 text-slate-400" />;
  return <Minus className="w-4 h-4 text-slate-300" />;
}

function statusBadge(trend: string) {
  if (trend === "improving")
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700 border border-green-200">
        <TrendingUp className="w-3 h-3" /> Improving
      </span>
    );
  if (trend === "degrading")
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-red-50 text-red-700 border border-red-200">
        <TrendingDown className="w-3 h-3" /> Degrading
      </span>
    );
  if (trend === "stable")
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-slate-50 text-slate-600 border border-slate-200">
        <Minus className="w-3 h-3" /> Stable
      </span>
    );
  return (
    <span className="text-xs text-slate-400">—</span>
  );
}

// ---------------------------------------------------------------------------
// SVG Line Chart (no external deps)
// ---------------------------------------------------------------------------

interface LineChartProps {
  series: DriftPoint[];
  days: DayRange;
  height?: number;
}

const CHART_COLORS: Record<string, string> = {
  answer_correctness: "#2563eb",
  groundedness: "#16a34a",
  retrieval_quality: "#9333ea",
  memory_utility: "#0891b2",
  tool_correctness: "#d97706",
  cost_efficiency: "#64748b",
  permission_safety: "#dc2626",
  context_poisoning: "#ea580c",
  session_coherence: "#0d9488",
  hallucination_risk: "#b45309",
  task_completion: "#6d28d9",
  response_completeness: "#0284c7",
  trajectory_quality: "#7c3aed",
  agent_regression: "#be123c",
  plan_adherence: "#15803d",
  agent_handoff_quality: "#1d4ed8",
};

function fallbackColor(ev: string): string {
  return CHART_COLORS[ev] ?? "#94a3b8";
}

function LineChart({ series, days, height = 220 }: LineChartProps) {
  const W = 600;
  const H = height;
  const PAD = { top: 16, right: 12, bottom: 28, left: 36 };
  const chartW = W - PAD.left - PAD.right;
  const chartH = H - PAD.top - PAD.bottom;

  // Group by evaluator
  const byEval = useMemo(() => {
    const m = new Map<string, DriftPoint[]>();
    for (const p of series) {
      if (!m.has(p.evaluator)) m.set(p.evaluator, []);
      m.get(p.evaluator)!.push(p);
    }
    for (const pts of m.values()) {
      pts.sort((a, b) => a.date.localeCompare(b.date));
    }
    return m;
  }, [series]);

  // Date range
  const allDates = useMemo(() => {
    const set = new Set(series.map((p) => p.date));
    return Array.from(set).sort();
  }, [series]);

  if (allDates.length < 2) {
    return (
      <div
        className="flex items-center justify-center text-slate-400 text-sm"
        style={{ height }}
      >
        Not enough data to draw a chart
      </div>
    );
  }

  const dateIndex = new Map(allDates.map((d, i) => [d, i]));
  const n = allDates.length;

  function xOf(date: string): number {
    return PAD.left + ((dateIndex.get(date) ?? 0) / (n - 1)) * chartW;
  }
  function yOf(score: number): number {
    return PAD.top + chartH - score * chartH;
  }

  // Y grid lines: 0.2 / 0.4 / 0.6 / 0.8 / 1.0
  const yTicks = [0, 0.2, 0.4, 0.6, 0.8, 1.0];
  // X grid: show ~5 date labels
  const xTickStep = Math.max(1, Math.floor(n / 5));
  const xTicks = allDates.filter((_, i) => i % xTickStep === 0 || i === n - 1);

  const evalNames = Array.from(byEval.keys());

  return (
    <div className="w-full overflow-x-auto">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        width="100%"
        style={{ fontFamily: "inherit" }}
        aria-label="Drift line chart"
      >
        {/* Grid lines */}
        {yTicks.map((v) => (
          <g key={v}>
            <line
              x1={PAD.left}
              x2={W - PAD.right}
              y1={yOf(v)}
              y2={yOf(v)}
              stroke="#e2e8f0"
              strokeWidth={1}
            />
            <text
              x={PAD.left - 4}
              y={yOf(v) + 4}
              textAnchor="end"
              fontSize={10}
              fill="#94a3b8"
            >
              {v.toFixed(1)}
            </text>
          </g>
        ))}

        {/* X axis labels */}
        {xTicks.map((d) => (
          <text
            key={d}
            x={xOf(d)}
            y={H - 4}
            textAnchor="middle"
            fontSize={9}
            fill="#94a3b8"
          >
            {d.slice(5)} {/* MM-DD */}
          </text>
        ))}

        {/* Lines */}
        {evalNames.map((ev) => {
          const pts = byEval.get(ev)!;
          const d = pts
            .map((p, i) => `${i === 0 ? "M" : "L"} ${xOf(p.date)} ${yOf(p.score)}`)
            .join(" ");
          return (
            <path
              key={ev}
              d={d}
              fill="none"
              stroke={fallbackColor(ev)}
              strokeWidth={1.8}
              strokeLinejoin="round"
              strokeLinecap="round"
              opacity={0.85}
            />
          );
        })}

        {/* Dots on last point */}
        {evalNames.map((ev) => {
          const pts = byEval.get(ev)!;
          if (pts.length === 0) return null;
          const last = pts[pts.length - 1];
          return (
            <circle
              key={ev}
              cx={xOf(last.date)}
              cy={yOf(last.score)}
              r={3}
              fill={fallbackColor(ev)}
            />
          );
        })}
      </svg>

      {/* Legend */}
      <div className="flex flex-wrap gap-3 mt-2 px-1">
        {evalNames.map((ev) => (
          <div key={ev} className="flex items-center gap-1.5">
            <span
              className="inline-block w-3 h-0.5 rounded"
              style={{ backgroundColor: fallbackColor(ev) }}
            />
            <span className="text-xs text-slate-500">
              {ev.replace(/_/g, " ")}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function DriftPage() {
  const [days, setDays] = useState<DayRange>(30);
  const [project, setProject] = useState("");
  const [data, setData] = useState<DriftResponse | null>(null);
  const [tableRows, setTableRows] = useState<TrendEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [usingMock, setUsingMock] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  async function fetchData(d: DayRange, proj: string) {
    setLoading(true);
    setUsingMock(false);

    try {
      // Try the new /api/v1/drift endpoint (returns { series, alerts })
      const qs = new URLSearchParams({ days: String(d) });
      if (proj) qs.set("project", proj);
      const res = await fetch(`${API_URL}/api/v1/drift?${qs}`);

      if (res.ok) {
        const json: DriftResponse = await res.json();
        // If the response has a `drift` array (legacy shape), convert it
        if (!json.series && (json as unknown as AgentDriftResponse).drift) {
          const legacy = json as unknown as AgentDriftResponse;
          setTableRows(legacy.drift ?? []);
          // Generate mock series since old endpoint doesn't provide time-series
          const mockSeries = generateMockSeries(d);
          const mockAlerts = generateMockAlerts(mockSeries);
          setData({ series: mockSeries, alerts: mockAlerts });
          setUsingMock(true);
        } else {
          setData(json);
          // Build tableRows from alerts + series
          const byEval = new Map<string, number[]>();
          for (const p of json.series) {
            if (!byEval.has(p.evaluator)) byEval.set(p.evaluator, []);
            byEval.get(p.evaluator)!.push(p.score);
          }
          const rows: TrendEntry[] = [];
          for (const [ev, scores] of byEval) {
            const cur = scores[scores.length - 1];
            const prev = scores.length > 1 ? scores[0] : undefined;
            const delta = prev !== undefined ? cur - prev : undefined;
            const trend =
              delta === undefined
                ? "insufficient_data"
                : delta > 0.05
                ? "improving"
                : delta < -0.05
                ? "degrading"
                : "stable";
            rows.push({
              category: ev,
              current_avg: cur,
              previous_avg: prev,
              delta,
              trend,
              current_runs: scores.length,
            });
          }
          setTableRows(rows.sort((a, b) => a.category.localeCompare(b.category)));
        }
      } else {
        throw new Error("endpoint unavailable");
      }
    } catch {
      // Fall back to mock data + note
      const mockSeries = generateMockSeries(d);
      const mockAlerts = generateMockAlerts(mockSeries);
      setData({ series: mockSeries, alerts: mockAlerts });

      const byEval = new Map<string, number[]>();
      for (const p of mockSeries) {
        if (!byEval.has(p.evaluator)) byEval.set(p.evaluator, []);
        byEval.get(p.evaluator)!.push(p.score);
      }
      const rows: TrendEntry[] = [];
      for (const [ev, scores] of byEval) {
        const cur = scores[scores.length - 1];
        const prev = scores[0];
        const delta = cur - prev;
        const trend =
          delta > 0.05 ? "improving" : delta < -0.05 ? "degrading" : "stable";
        rows.push({
          category: ev,
          current_avg: cur,
          previous_avg: prev,
          delta,
          trend,
          current_runs: scores.length,
        });
      }
      setTableRows(rows.sort((a, b) => a.category.localeCompare(b.category)));
      setUsingMock(true);
    } finally {
      setLoading(false);
      setLastUpdated(new Date());
    }
  }

  useEffect(() => {
    fetchData(days, project);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days, project]);

  const alerts = data?.alerts ?? [];
  const series = data?.series ?? [];
  const hasDriftAlert = alerts.length > 0;
  const degradingCount = tableRows.filter((r) => r.trend === "degrading").length;

  // 7-day averages for the table
  const sevenDayAvg = useMemo(() => {
    const cutoff = new Date();
    cutoff.setDate(cutoff.getDate() - 7);
    const cutStr = cutoff.toISOString().slice(0, 10);
    const byEval = new Map<string, number[]>();
    for (const p of series) {
      if (p.date >= cutStr) {
        if (!byEval.has(p.evaluator)) byEval.set(p.evaluator, []);
        byEval.get(p.evaluator)!.push(p.score);
      }
    }
    const result = new Map<string, number>();
    for (const [ev, scores] of byEval) {
      result.set(ev, scores.reduce((s, v) => s + v, 0) / scores.length);
    }
    return result;
  }, [series]);

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Drift Monitoring</h1>
          <p className="text-sm text-slate-500 mt-1">
            Track evaluator score trends over time — detect quality regressions before they reach production
          </p>
        </div>
        <div className="flex items-center gap-2">
          {lastUpdated && (
            <span className="text-xs text-slate-400">
              Updated {lastUpdated.toLocaleTimeString()}
            </span>
          )}
          <button
            onClick={() => fetchData(days, project)}
            disabled={loading}
            className="flex items-center gap-1.5 px-3 py-2 border border-slate-200 rounded-lg text-sm text-slate-600 hover:bg-slate-50 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} />
            Refresh
          </button>
        </div>
      </div>

      {/* Mock data notice */}
      {usingMock && (
        <div className="flex items-start gap-3 px-4 py-3 bg-amber-50 border border-amber-200 rounded-lg">
          <AlertTriangle className="w-4 h-4 text-amber-500 mt-0.5 shrink-0" />
          <p className="text-sm text-amber-800">
            <span className="font-semibold">Demo data</span> — the{" "}
            <code className="text-xs bg-amber-100 px-1 rounded">GET /api/v1/drift</code> endpoint
            returned the legacy single-window format. For full time-series data, update the backend
            to return <code className="text-xs bg-amber-100 px-1 rounded">{"{ series: [...], alerts: [...] }"}</code>.
          </p>
        </div>
      )}

      {/* Drift detected banner */}
      {hasDriftAlert && !loading && (
        <div className="flex items-center gap-3 px-4 py-3 bg-red-50 border border-red-300 rounded-lg">
          <TrendingDown className="w-5 h-5 text-red-600 shrink-0" />
          <div>
            <p className="text-sm font-semibold text-red-800">
              Drift detected — {alerts.length} metric{alerts.length > 1 ? "s" : ""} crossed the regression threshold
            </p>
            <p className="text-xs text-red-600 mt-0.5">
              {alerts.map((a) => `${a.evaluator.replace(/_/g, " ")} (${(a.delta * 100).toFixed(1)}%)`).join(" · ")}
            </p>
          </div>
        </div>
      )}

      {/* Controls */}
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-1 bg-white border border-slate-200 rounded-lg p-1">
          {([7, 30, 90] as DayRange[]).map((d) => (
            <button
              key={d}
              onClick={() => setDays(d)}
              className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                days === d
                  ? "bg-slate-900 text-white"
                  : "text-slate-600 hover:bg-slate-50"
              }`}
            >
              {d}d
            </button>
          ))}
        </div>
        <input
          type="text"
          placeholder="Filter by project / agent ID…"
          value={project}
          onChange={(e) => setProject(e.target.value)}
          className="px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500 w-56"
        />
        {degradingCount > 0 && (
          <span className="flex items-center gap-1 text-sm font-medium text-red-700 bg-red-50 border border-red-200 px-3 py-1.5 rounded-lg">
            <TrendingDown className="w-4 h-4" />
            {degradingCount} degrading
          </span>
        )}
      </div>

      {/* Chart */}
      <div className="card p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-semibold text-slate-900">
            Score trends — last {days} days
          </h2>
        </div>
        {loading ? (
          <div className="h-56 flex items-center justify-center text-slate-400 text-sm">
            Loading chart data…
          </div>
        ) : (
          <LineChart series={series} days={days} />
        )}
      </div>

      {/* Alert badges */}
      {alerts.length > 0 && !loading && (
        <div className="card p-5">
          <h2 className="text-sm font-semibold text-slate-900 mb-3">
            Active Alerts — metrics with &gt;10% drift
          </h2>
          <div className="flex flex-wrap gap-3">
            {alerts.map((a) => (
              <div
                key={a.evaluator}
                className="flex items-center gap-2 px-3 py-2 bg-red-50 border border-red-200 rounded-lg"
              >
                <AlertTriangle className="w-4 h-4 text-red-500 shrink-0" />
                <div>
                  <p className="text-xs font-semibold text-red-800">
                    {a.evaluator.replace(/_/g, " ")}
                  </p>
                  <p className="text-xs text-red-600">
                    {a.current.toFixed(3)} vs baseline {a.baseline.toFixed(3)}{" "}
                    <span className="font-semibold">
                      ({(a.delta * 100).toFixed(1)}%)
                    </span>
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Table */}
      <div className="card overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-200">
          <h2 className="text-sm font-semibold text-slate-900">
            Evaluator Scores — {days}-day window
          </h2>
        </div>

        {/* Table header */}
        <div className="grid grid-cols-[1fr_90px_90px_90px_120px] gap-3 px-5 py-2.5 bg-slate-50 text-xs font-semibold text-slate-500 uppercase tracking-wider border-b border-slate-200">
          <div>Evaluator</div>
          <div className="text-right">Current</div>
          <div className="text-right">7-day avg</div>
          <div className="text-right">Trend</div>
          <div className="text-right">Status</div>
        </div>

        {loading ? (
          <div className="p-8 text-center text-slate-400 text-sm">Loading…</div>
        ) : tableRows.length === 0 ? (
          <div className="p-8 text-center text-slate-400 text-sm">No data available</div>
        ) : (
          <div className="divide-y divide-slate-100">
            {tableRows.map((row) => {
              const sevenAvg = sevenDayAvg.get(row.category);
              const isDegrading = row.trend === "degrading";
              const isAlert = alerts.some((a) => a.evaluator === row.category);
              return (
                <div
                  key={row.category}
                  className={`grid grid-cols-[1fr_90px_90px_90px_120px] gap-3 px-5 py-3 items-center transition-colors ${
                    isDegrading ? "bg-red-50/40" : "hover:bg-slate-50"
                  }`}
                >
                  {/* Name */}
                  <div className="flex items-center gap-2">
                    <span
                      className="inline-block w-2.5 h-2.5 rounded-full shrink-0"
                      style={{ backgroundColor: fallbackColor(row.category) }}
                    />
                    <span className="text-sm font-medium text-slate-800 capitalize">
                      {row.category.replace(/_/g, " ")}
                    </span>
                    {isAlert && (
                      <AlertTriangle className="w-3.5 h-3.5 text-red-500 shrink-0" />
                    )}
                  </div>

                  {/* Current score */}
                  <div className="text-right">
                    <span
                      className="text-sm font-semibold tabular-nums"
                      style={{ color: scoreColor(row.current_avg) }}
                    >
                      {row.current_avg.toFixed(3)}
                    </span>
                  </div>

                  {/* 7-day avg */}
                  <div className="text-right">
                    {sevenAvg !== undefined ? (
                      <span
                        className="text-sm tabular-nums"
                        style={{ color: scoreColor(sevenAvg) }}
                      >
                        {sevenAvg.toFixed(3)}
                      </span>
                    ) : (
                      <span className="text-slate-300 text-sm">—</span>
                    )}
                  </div>

                  {/* Trend delta */}
                  <div className="text-right flex items-center justify-end gap-1">
                    {trendIcon(row.trend)}
                    {row.delta !== undefined ? (
                      <span
                        className={`text-xs font-medium tabular-nums ${
                          row.delta > 0
                            ? "text-green-700"
                            : row.delta < 0
                            ? "text-red-600"
                            : "text-slate-400"
                        }`}
                      >
                        {row.delta > 0 ? "+" : ""}
                        {(row.delta * 100).toFixed(1)}%
                      </span>
                    ) : (
                      <span className="text-xs text-slate-300">—</span>
                    )}
                  </div>

                  {/* Status */}
                  <div className="text-right">{statusBadge(row.trend)}</div>
                </div>
              );
            })}
          </div>
        )}

        {/* Footer summary */}
        {!loading && tableRows.length > 0 && (
          <div className="px-5 py-3 border-t border-slate-200 bg-slate-50 flex items-center gap-4 text-xs text-slate-500">
            <span className="flex items-center gap-1">
              <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
              {tableRows.filter((r) => r.trend === "stable" || r.trend === "improving").length} stable/improving
            </span>
            <span className="flex items-center gap-1">
              <TrendingDown className="w-3.5 h-3.5 text-red-500" />
              {degradingCount} degrading
            </span>
            <span className="flex items-center gap-1">
              <AlertTriangle className="w-3.5 h-3.5 text-amber-500" />
              {tableRows.filter((r) => r.trend === "insufficient_data").length} insufficient data
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
