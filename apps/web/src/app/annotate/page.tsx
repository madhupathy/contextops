"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Flag,
  SkipForward,
  Star,
  ThumbsDown,
  ThumbsUp,
  XCircle,
} from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Run {
  id: string;
  query: string;
  final_answer: string | null;
  expected_answer: string | null;
  status: string;
  model: string | null;
  created_at: string;
  context_manifest: Record<string, unknown> | null;
}

interface RetrievalCandidate {
  id: string;
  doc_id: string;
  title: string | null;
  content_preview: string | null;
  score: number;
  selected: boolean;
  acl_passed: boolean;
}

interface RunTimeline {
  run: Run;
  retrieval_candidates: RetrievalCandidate[];
}

interface AnnotationDraft {
  rating: number | null;        // 1–5; null = not rated
  thumbs: "up" | "down" | null; // quick thumbs
  note: string;
  is_ground_truth: boolean;
}

const EMPTY_DRAFT: AnnotationDraft = {
  rating: null,
  thumbs: null,
  note: "",
  is_ground_truth: false,
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function thumbsToRating(thumbs: "up" | "down"): number {
  return thumbs === "up" ? 5 : 1;
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StarRating({
  value,
  onChange,
}: {
  value: number | null;
  onChange: (v: number) => void;
}) {
  const [hover, setHover] = useState<number | null>(null);
  return (
    <div className="flex items-center gap-1">
      {[1, 2, 3, 4, 5].map((n) => {
        const active = (hover ?? value ?? 0) >= n;
        return (
          <button
            key={n}
            onMouseEnter={() => setHover(n)}
            onMouseLeave={() => setHover(null)}
            onClick={() => onChange(n)}
            title={`Rate ${n}/5`}
            className={`w-7 h-7 transition-colors ${active ? "text-amber-400" : "text-slate-200 hover:text-amber-300"}`}
          >
            <Star className="w-full h-full" fill={active ? "currentColor" : "none"} />
          </button>
        );
      })}
      {value && (
        <span className="text-xs text-slate-500 ml-1">{value}/5</span>
      )}
    </div>
  );
}

function ProgressBar({ done, total }: { done: number; total: number }) {
  const pct = total === 0 ? 0 : Math.round((done / total) * 100);
  return (
    <div className="flex items-center gap-3">
      <div className="flex-1 bg-slate-100 rounded-full h-2 overflow-hidden">
        <div
          className="h-2 bg-brand-600 rounded-full transition-all duration-500"
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="text-xs text-slate-500 tabular-nums whitespace-nowrap">
        {done} / {total} annotated
      </span>
    </div>
  );
}

function ContextSection({ candidates }: { candidates: RetrievalCandidate[] }) {
  const [open, setOpen] = useState(false);
  if (candidates.length === 0) return null;
  return (
    <div className="border border-slate-200 rounded-lg overflow-hidden">
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center justify-between px-4 py-3 bg-slate-50 hover:bg-slate-100 transition-colors text-left"
      >
        <span className="text-xs font-semibold text-slate-600 uppercase tracking-wider">
          Retrieved context ({candidates.length} chunks)
        </span>
        {open ? (
          <ChevronUp className="w-4 h-4 text-slate-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-slate-400" />
        )}
      </button>
      {open && (
        <div className="divide-y divide-slate-100">
          {candidates.map((c) => (
            <div key={c.id} className="px-4 py-3">
              <div className="flex items-center justify-between gap-2 mb-1">
                <p className="text-xs font-medium text-slate-700 truncate">
                  {c.title || c.doc_id}
                </p>
                <div className="flex items-center gap-1 shrink-0">
                  <span
                    className={`text-xs px-1.5 py-0.5 rounded font-medium ${
                      c.selected
                        ? "bg-blue-50 text-blue-700"
                        : "bg-slate-100 text-slate-500"
                    }`}
                  >
                    {c.selected ? "selected" : "rejected"}
                  </span>
                  {!c.acl_passed && (
                    <span className="text-xs px-1.5 py-0.5 rounded font-medium bg-red-50 text-red-600">
                      ACL blocked
                    </span>
                  )}
                  <span className="text-xs text-slate-400 tabular-nums">
                    {c.score.toFixed(3)}
                  </span>
                </div>
              </div>
              {c.content_preview && (
                <p className="text-xs text-slate-500 line-clamp-2">
                  {c.content_preview}
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function AnnotatePage() {
  const [queue, setQueue] = useState<Run[]>([]);
  const [index, setIndex] = useState(0);
  const [doneCount, setDoneCount] = useState(0);
  const [totalCount, setTotalCount] = useState(0);
  const [timeline, setTimeline] = useState<RunTimeline | null>(null);
  const [draft, setDraft] = useState<AnnotationDraft>(EMPTY_DRAFT);
  const [saving, setSaving] = useState(false);
  const [savedIds, setSavedIds] = useState<Set<string>>(new Set());
  const [loadingQueue, setLoadingQueue] = useState(true);
  const [loadingTimeline, setLoadingTimeline] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const noteRef = useRef<HTMLTextAreaElement>(null);

  // ---- Data fetching ----

  const fetchQueue = useCallback(async () => {
    setLoadingQueue(true);
    try {
      // Fetch all completed runs (the annotation filter is a server concern;
      // we do client-side filtering for needs_annotation here as a fallback).
      const res = await fetch(`${API_URL}/api/v1/runs?status=completed`);
      if (!res.ok) throw new Error("fetch failed");
      const all: Run[] = await res.json();
      // Prefer un-annotated runs first (no annotation key in context_manifest)
      const unannotated = all.filter(
        (r) => !r.context_manifest?.annotation
      );
      const annotated = all.filter((r) => r.context_manifest?.annotation);
      setQueue([...unannotated, ...annotated]);
      setTotalCount(all.length);
      setDoneCount(annotated.length);
    } catch {
      setQueue([]);
    } finally {
      setLoadingQueue(false);
    }
  }, []);

  useEffect(() => {
    fetchQueue();
  }, [fetchQueue]);

  useEffect(() => {
    const run = queue[index];
    if (!run) {
      setTimeline(null);
      return;
    }
    setLoadingTimeline(true);
    fetch(`${API_URL}/api/v1/runs/${run.id}/timeline`)
      .then((r) => (r.ok ? r.json() : null))
      .then((data: RunTimeline | null) => setTimeline(data))
      .catch(() => setTimeline(null))
      .finally(() => setLoadingTimeline(false));

    // Reset draft for new run (keep note if annotator typed something)
    setDraft(EMPTY_DRAFT);
  }, [queue, index]);

  // ---- Keyboard shortcuts ----

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Ignore when typing in textarea/input
      if (
        document.activeElement?.tagName === "TEXTAREA" ||
        document.activeElement?.tagName === "INPUT"
      )
        return;

      switch (e.key.toLowerCase()) {
        case "j":
          setIndex((i) => Math.min(i + 1, queue.length - 1));
          break;
        case "k":
          setIndex((i) => Math.max(i - 1, 0));
          break;
        case "y":
          setDraft((d) => ({ ...d, thumbs: "up", rating: 5 }));
          break;
        case "n":
          setDraft((d) => ({ ...d, thumbs: "down", rating: 1 }));
          break;
        case "s":
          handleSkip();
          break;
        default:
          break;
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queue.length]);

  // ---- Actions ----

  function showToast(msg: string) {
    setToast(msg);
    setTimeout(() => setToast(null), 2500);
  }

  function handleSkip() {
    setIndex((i) => Math.min(i + 1, queue.length - 1));
  }

  async function handleSave() {
    const run = queue[index];
    if (!run) return;
    if (draft.rating === null && draft.thumbs === null) {
      showToast("Please add a rating before saving.");
      return;
    }

    const rating =
      draft.rating ?? (draft.thumbs ? thumbsToRating(draft.thumbs) : 3);

    setSaving(true);
    try {
      const body = {
        rating,
        note: draft.note,
        is_ground_truth: draft.is_ground_truth,
      };
      const res = await fetch(`${API_URL}/api/v1/runs/${run.id}/annotate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      if (res.ok) {
        setSavedIds((s) => new Set(s).add(run.id));
        setDoneCount((c) => (savedIds.has(run.id) ? c : c + 1));
        showToast("Annotation saved");
        // Auto-advance
        setIndex((i) => Math.min(i + 1, queue.length - 1));
        setDraft(EMPTY_DRAFT);
      } else {
        const err = await res.json().catch(() => ({}));
        showToast(`Error: ${(err as { error?: string }).error ?? "save failed"}`);
      }
    } catch {
      showToast("Network error — could not save annotation.");
    } finally {
      setSaving(false);
    }
  }

  // ---- Render ----

  const run = queue[index] ?? null;
  const isAnnotated = run ? savedIds.has(run.id) : false;

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Annotation Queue</h1>
          <p className="text-sm text-slate-500 mt-1">
            Human review of agent runs — rate quality, add corrections, flag ground truth
          </p>
        </div>
        <div className="text-xs text-slate-400 bg-slate-50 border border-slate-200 rounded-lg px-3 py-2 font-mono space-y-0.5">
          <p className="font-semibold text-slate-500 font-sans mb-1">Keyboard shortcuts</p>
          <p><kbd className="bg-white border border-slate-300 rounded px-1">J</kbd> next</p>
          <p><kbd className="bg-white border border-slate-300 rounded px-1">K</kbd> prev</p>
          <p><kbd className="bg-white border border-slate-300 rounded px-1">Y</kbd> thumbs up</p>
          <p><kbd className="bg-white border border-slate-300 rounded px-1">N</kbd> thumbs down</p>
          <p><kbd className="bg-white border border-slate-300 rounded px-1">S</kbd> skip</p>
        </div>
      </div>

      {/* Progress */}
      <div className="card px-5 py-4">
        <ProgressBar done={doneCount} total={totalCount} />
      </div>

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-6 right-6 z-50 bg-slate-900 text-white text-sm px-4 py-2.5 rounded-lg shadow-lg animate-fade-in">
          {toast}
        </div>
      )}

      {loadingQueue ? (
        <div className="card p-12 text-center text-slate-400 text-sm">
          Loading annotation queue…
        </div>
      ) : queue.length === 0 ? (
        <div className="card p-12 text-center">
          <CheckCircle2 className="w-12 h-12 text-green-400 mx-auto mb-3" />
          <p className="text-slate-700 font-medium">Queue is empty</p>
          <p className="text-sm text-slate-500 mt-1">No completed runs found for annotation.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_340px] gap-5">
          {/* Main card */}
          <div className="space-y-4">
            {/* Navigation */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setIndex((i) => Math.max(i - 1, 0))}
                  disabled={index === 0}
                  className="p-1.5 rounded-md border border-slate-200 hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                >
                  <ArrowLeft className="w-4 h-4 text-slate-600" />
                </button>
                <span className="text-sm text-slate-600 tabular-nums">
                  {index + 1} / {queue.length}
                </span>
                <button
                  onClick={() => setIndex((i) => Math.min(i + 1, queue.length - 1))}
                  disabled={index === queue.length - 1}
                  className="p-1.5 rounded-md border border-slate-200 hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                >
                  <ArrowRight className="w-4 h-4 text-slate-600" />
                </button>
              </div>
              <div className="flex items-center gap-2">
                {isAnnotated && (
                  <span className="flex items-center gap-1 text-xs text-green-700 bg-green-50 border border-green-200 px-2 py-1 rounded-full">
                    <CheckCircle2 className="w-3 h-3" /> Annotated
                  </span>
                )}
                {run?.id && (
                  <a
                    href={`/runs/${run.id}`}
                    className="text-xs text-brand-600 hover:text-brand-700"
                    target="_blank"
                    rel="noreferrer"
                  >
                    View full run →
                  </a>
                )}
              </div>
            </div>

            {/* Run detail */}
            {run && (
              <div className="card p-5 space-y-4">
                {/* Query */}
                <div>
                  <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                    Question
                  </p>
                  <p className="text-base font-medium text-slate-900">{run.query}</p>
                  <div className="flex items-center gap-3 mt-2 text-xs text-slate-400">
                    <span className="font-mono">{run.id.slice(0, 16)}…</span>
                    {run.model && <span>{run.model}</span>}
                    <span>{timeAgo(run.created_at)}</span>
                  </div>
                </div>

                {/* Agent answer */}
                <div>
                  <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                    Agent Answer
                  </p>
                  {run.final_answer ? (
                    <div className="bg-slate-50 border border-slate-200 rounded-lg p-4 text-sm text-slate-800 whitespace-pre-wrap leading-relaxed">
                      {run.final_answer}
                    </div>
                  ) : (
                    <p className="text-sm text-slate-400 italic">No answer recorded</p>
                  )}
                </div>

                {/* Expected answer */}
                {run.expected_answer && (
                  <div>
                    <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-1.5">
                      Expected / Ground Truth
                    </p>
                    <div className="bg-green-50 border border-green-200 rounded-lg p-4 text-sm text-green-900 whitespace-pre-wrap leading-relaxed">
                      {run.expected_answer}
                    </div>
                  </div>
                )}

                {/* Retrieved context */}
                {loadingTimeline ? (
                  <p className="text-xs text-slate-400">Loading context…</p>
                ) : timeline ? (
                  <ContextSection candidates={timeline.retrieval_candidates} />
                ) : null}
              </div>
            )}
          </div>

          {/* Annotation panel */}
          <div className="space-y-4">
            <div className="card p-5 space-y-5">
              <h2 className="text-sm font-semibold text-slate-900">Annotation</h2>

              {/* Thumbs */}
              <div>
                <p className="text-xs font-medium text-slate-500 mb-2">Quick verdict</p>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() =>
                      setDraft((d) => ({
                        ...d,
                        thumbs: d.thumbs === "up" ? null : "up",
                        rating: d.thumbs === "up" ? null : 5,
                      }))
                    }
                    className={`flex items-center gap-1.5 px-3 py-2 rounded-lg border text-sm font-medium transition-colors ${
                      draft.thumbs === "up"
                        ? "bg-green-50 border-green-300 text-green-700"
                        : "border-slate-200 text-slate-600 hover:bg-slate-50"
                    }`}
                  >
                    <ThumbsUp className="w-4 h-4" />
                    Good
                  </button>
                  <button
                    onClick={() =>
                      setDraft((d) => ({
                        ...d,
                        thumbs: d.thumbs === "down" ? null : "down",
                        rating: d.thumbs === "down" ? null : 1,
                      }))
                    }
                    className={`flex items-center gap-1.5 px-3 py-2 rounded-lg border text-sm font-medium transition-colors ${
                      draft.thumbs === "down"
                        ? "bg-red-50 border-red-300 text-red-700"
                        : "border-slate-200 text-slate-600 hover:bg-slate-50"
                    }`}
                  >
                    <ThumbsDown className="w-4 h-4" />
                    Poor
                  </button>
                </div>
              </div>

              {/* Star rating */}
              <div>
                <p className="text-xs font-medium text-slate-500 mb-2">Quality rating</p>
                <StarRating
                  value={draft.rating}
                  onChange={(v) =>
                    setDraft((d) => ({
                      ...d,
                      rating: v,
                      thumbs: v >= 4 ? "up" : v <= 2 ? "down" : d.thumbs,
                    }))
                  }
                />
              </div>

              {/* Note */}
              <div>
                <p className="text-xs font-medium text-slate-500 mb-2">
                  Note / correction
                </p>
                <textarea
                  ref={noteRef}
                  value={draft.note}
                  onChange={(e) =>
                    setDraft((d) => ({ ...d, note: e.target.value }))
                  }
                  placeholder="The answer is correct but misses the edge case about…"
                  rows={4}
                  className="w-full text-sm border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none placeholder:text-slate-300"
                />
              </div>

              {/* Ground truth flag */}
              <label className="flex items-start gap-2.5 cursor-pointer group">
                <div className="relative mt-0.5">
                  <input
                    type="checkbox"
                    checked={draft.is_ground_truth}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        is_ground_truth: e.target.checked,
                      }))
                    }
                    className="sr-only"
                  />
                  <div
                    className={`w-4 h-4 rounded border-2 flex items-center justify-center transition-colors ${
                      draft.is_ground_truth
                        ? "bg-amber-500 border-amber-500"
                        : "border-slate-300 group-hover:border-amber-400"
                    }`}
                  >
                    {draft.is_ground_truth && (
                      <svg viewBox="0 0 10 8" className="w-2.5 h-2 text-white fill-current">
                        <path d="M1 4l3 3 5-6" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    )}
                  </div>
                </div>
                <div>
                  <div className="flex items-center gap-1.5">
                    <Flag className="w-3 h-3 text-amber-500" />
                    <span className="text-sm font-medium text-slate-700">
                      Mark as ground truth
                    </span>
                  </div>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Use this run as a baseline for regression testing
                  </p>
                </div>
              </label>

              {/* Actions */}
              <div className="flex items-center gap-2 pt-1">
                <button
                  onClick={handleSave}
                  disabled={saving || (draft.rating === null && draft.thumbs === null)}
                  className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 bg-brand-600 hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
                >
                  {saving ? (
                    <span className="inline-block w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  ) : (
                    <CheckCircle2 className="w-4 h-4" />
                  )}
                  Save &amp; Next
                </button>
                <button
                  onClick={handleSkip}
                  className="flex items-center gap-1.5 px-3 py-2.5 border border-slate-200 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
                  title="Skip (S)"
                >
                  <SkipForward className="w-4 h-4" />
                  Skip
                </button>
              </div>

              {draft.rating === null && draft.thumbs === null && (
                <p className="text-xs text-slate-400 text-center -mt-2">
                  Add a thumbs or star rating to enable save
                </p>
              )}
            </div>

            {/* Queue mini-list */}
            <div className="card overflow-hidden">
              <div className="px-4 py-3 border-b border-slate-200 flex items-center justify-between">
                <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                  Queue
                </p>
                <span className="text-xs text-slate-400">{queue.length} runs</span>
              </div>
              <div className="max-h-56 overflow-y-auto divide-y divide-slate-100">
                {queue.slice(0, 40).map((r, i) => (
                  <button
                    key={r.id}
                    onClick={() => setIndex(i)}
                    className={`w-full text-left px-4 py-2.5 flex items-center gap-2.5 hover:bg-slate-50 transition-colors ${
                      i === index ? "bg-brand-50 border-l-2 border-brand-500" : ""
                    }`}
                  >
                    <div className="shrink-0">
                      {savedIds.has(r.id) || r.context_manifest?.annotation ? (
                        <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
                      ) : (
                        <XCircle className="w-3.5 h-3.5 text-slate-300" />
                      )}
                    </div>
                    <p className="text-xs text-slate-700 truncate flex-1">{r.query}</p>
                  </button>
                ))}
                {queue.length > 40 && (
                  <p className="text-xs text-slate-400 text-center py-2">
                    +{queue.length - 40} more
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
