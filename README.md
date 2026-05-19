<div align="center">

# ContextOps

**Your AI agent gave the wrong answer. ContextOps tells you exactly where in the pipeline it went wrong — and blocks the next release if it happens again.**

ContextOps is an open-source evaluation and debugging platform for AI agents, RAG pipelines, and enterprise copilots. It evaluates the full execution path — retrieval, memory, permissions, tool calls, context assembly — not just whether the final answer looked correct.

[![CI](https://github.com/madhupathy/contextops/actions/workflows/ci.yml/badge.svg)](https://github.com/madhupathy/contextops/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Python 3.12+](https://img.shields.io/badge/Python-3.12+-3776AB?logo=python&logoColor=white)](https://python.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

---

## The Problem

Your enterprise RAG assistant confidently told an employee the PTO policy allows 15 days. It's actually 20 days as of January 2025. Your evaluation pipeline gave the run a 0.87 correctness score. Everything looks fine.

Here's what actually happened:

```
1. Retriever fetched PTO Policy 2024 (score: 0.83)  ← selected
2. PTO Policy 2025 (score: 0.79)                    ← ACL-filtered: group-mapping-mismatch
3. Stale memory from Dec 2024 injected              ← "User was told 15 days last year"
4. Model answered confidently from wrong context
```

**The final answer scored 0.87.** The pipeline failed completely.

This happens because LangSmith, Langfuse, RAGAS, and Arize Phoenix all evaluate the final answer. None of them evaluate *how the context was assembled* — which retrieval candidates were rejected, why an ACL filter blocked the right document, whether injected memory was stale.

ContextOps evaluates the pipeline, not just the output.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         ContextOps                              │
│                                                                  │
│  ┌──────────┐   ┌──────────────┐   ┌──────────────────────────┐ │
│  │  CLI     │   │  Web UI      │   │  REST API (Go / Gin)     │ │
│  │  (Go)    │   │  (Next.js)   │   │  Multi-tenant · Audit    │ │
│  │          │   │  localhost:  │   │  localhost:8080           │ │
│  │  trace   │   │  3000        │   │                          │ │
│  │  eval    │   │              │   │  /api/v1/runs            │ │
│  │  gate    │   │  Dashboard   │   │  /api/v1/evaluations     │ │
│  │  compare │   │  Runs        │   │  /api/v1/drift           │ │
│  │  report  │   │  Compare     │   │  /api/v1/benchmarks      │ │
│  └──────────┘   │  Annotate    │   │  /api/v1/traces          │ │
│                 │  Drift       │   └──────────┬───────────────┘ │
│                 └──────────────┘              │                  │
│                                               ▼                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │              Evaluator Service (Python / FastAPI)          │ │
│  │              17 async evaluators · localhost:8081          │ │
│  │                                                            │ │
│  │  Core Quality    Retrieval      Memory & Context           │ │
│  │  Agent Behaviour Cost/Perf      Production · Autonomous    │ │
│  └────────────────────────────────────────────────────────────┘ │
│                               │                                  │
│                               ▼                                  │
│  ┌──────────────┐   ┌─────────────────┐   ┌───────────────────┐ │
│  │  PostgreSQL  │   │  Redis          │   │  Adapters         │ │
│  │  (runs,      │   │  (async eval    │   │  LangChain        │ │
│  │  evals,      │   │  queue)         │   │  OpenAI Agents    │ │
│  │  benchmarks) │   └─────────────────┘   │  OpenTelemetry    │ │
│  └──────────────┘                         └───────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Quick Start

### Docker Compose

```bash
git clone https://github.com/madhupathy/contextops.git
cd contextops
cp .env.example .env          # add OPENAI_API_KEY for LLM-as-judge evaluators (optional)
docker compose up -d

# API:       http://localhost:8080
# Evaluator: http://localhost:8081
# Web UI:    http://localhost:3000
```

Open `http://localhost:3000` in your browser to see the Dashboard, Runs, Compare, Annotate, and Drift pages.

### CLI

```bash
go install github.com/madhupathy/contextops/apps/cli@latest

contextops init
contextops trace ingest ./examples/traces/pto-policy-failure.json
contextops eval run <run-id>
contextops gate check --config .contextops/gates.yaml
```

---

## Integrating with your agent

There are four ways to plug ContextOps into any existing agent, from a two-line minimal wrapper to a full framework callback. **[Full integration guide →](docs/integration/README.md)**

### The one-minute version

After each agent run, POST a single JSON trace:

```python
import httpx, uuid

httpx.post("http://localhost:8080/api/v1/runs", json={
    "run_id":  str(uuid.uuid4()),
    "agent":   {"id": "my-agent"},
    "input":   {"query": user_query},
    "output":  {"final_answer": agent_answer},
    "metrics": {"latency_ms": latency_ms, "total_tokens": token_count},
})
```

Then evaluate:
```bash
contextops eval run YOUR_RUN_ID
```

### Supported patterns

| Pattern | When to use |
|---------|-------------|
| **Direct HTTP** ([guide](docs/integration/README.md#pattern-a--direct-http-any-language-any-framework)) | Any language, any framework — wrap your run in a few lines |
| **LangChain / LangGraph callback** ([guide](docs/integration/README.md#pattern-b--langchain--langgraph-callback)) | Drop a callback handler into your existing chain — no changes to chain code |
| **OpenAI Agents SDK** ([guide](docs/integration/README.md#pattern-c--openai-agents-sdk)) | Wrap your agent run with a context manager |
| **Absolute minimum** ([guide](docs/integration/README.md#pattern-d--absolute-minimum-any-framework)) | Just query + answer + tokens — enables 5 evaluators with two fields |

### OpenTelemetry adapter

Emit ContextOps traces directly from your existing OTel instrumentation:

```python
from contextops.adapters.otel import ContextOpsSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

provider = TracerProvider()
provider.add_span_processor(
    BatchSpanProcessor(
        ContextOpsSpanExporter(endpoint="http://localhost:8080/api/v1/traces")
    )
)

# Your existing OTel-instrumented code now sends traces to ContextOps automatically
```

### GitHub Action

Block CI/CD on quality regressions:

```yaml
# .github/workflows/eval.yml
name: ContextOps Evaluation Gate

on: [push, pull_request]

jobs:
  eval-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run evaluation gate
        uses: ./.github/actions/contextops-gate
        with:
          api_url: ${{ secrets.CONTEXTOPS_API_URL }}
          config: .contextops/gates.yaml
          fail_on_regression: true
```

```yaml
# .contextops/gates.yaml
minimum_scores:
  answer_correctness: 0.90
  groundedness: 0.85
  retrieval_quality: 0.80
  permission_safety: 1.00   # zero tolerance
  task_completion: 0.85

maximum_thresholds:
  latency_ms_p95: 5000
  cost_per_run_usd: 0.10
```

### What to add to unlock each evaluator

```
retrieval.candidates  →  retrieval_quality, permission_safety, citation_precision,
                         groundedness, context_poisoning
memory.candidates     →  memory_utility, context_poisoning, session_coherence
tools                 →  tool_correctness, task_completion, trajectory_quality
reasoning_steps       →  trajectory_quality, context_poisoning
expected_answer       →  answer_correctness (LLM-as-judge)
expected_sources      →  retrieval_quality (recall measurement)
metrics.total_tokens  →  cost_efficiency, agent_regression
```

---

## Sending a Trace

Send one JSON object capturing everything that happened in a single AI run:

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H 'Content-Type: application/json' \
  -d '{
    "run_id": "run-001",
    "agent": { "id": "policy-assistant", "version": "0.2.1" },
    "input": { "query": "What is the latest PTO policy?" },
    "retrieval": {
      "candidates": [
        { "doc_id": "doc-pto-2024", "score": 0.83, "selected": true,  "acl_passed": true },
        { "doc_id": "doc-pto-2025", "score": 0.79, "selected": false, "acl_passed": false,
          "acl_reason": "group-mapping-mismatch" }
      ]
    },
    "memory": {
      "candidates": [
        { "memory_id": "mem-01", "memory_type": "episodic",
          "content": "User was told 15 days PTO in Dec 2024",
          "is_stale": true, "stale_reason": "Policy updated Jan 2025", "selected": true }
      ]
    },
    "output": {
      "final_answer": "The PTO policy allows 15 days per year.",
      "citations": ["doc-pto-2024"]
    },
    "expected_answer": "The 2025 policy allows 20 days with unlimited carry-over.",
    "expected_sources": ["doc-pto-2025"],
    "metrics": { "latency_ms": 1820, "total_tokens": 2780, "estimated_cost": 0.042 }
  }'
```

Then trigger evaluation:

```bash
contextops eval run run-001
```

Output:

```
retrieval_quality    0.41  FAIL  Better document (doc-pto-2025) blocked by ACL misconfiguration
memory_utility       0.28  FAIL  Stale memory used (policy updated Jan 2025); stale avoidance: 0.0
context_poisoning    0.20  FAIL  Stale value "15 days" contaminated answer (not in current docs)
permission_safety    0.00  FAIL  CRITICAL: ACL misconfiguration blocked the correct document
answer_correctness   0.10  FAIL  Answer contradicts expected (15 days vs 20 days)
groundedness         0.45  FAIL  Answer not grounded in selected evidence
session_coherence    0.85  PASS  No multi-session context issues
tool_correctness     1.00  PASS  Tool calls correct
cost_efficiency      0.82  PASS  Latency and token spend within bounds
```

---

## 17 Evaluators

### Core Quality

| Evaluator | What it measures |
|-----------|-----------------|
| `answer_correctness` | Is the final answer factually correct? Heuristic + optional LLM-as-judge |
| `groundedness` | Is every claim in the answer supported by retrieved evidence? Detects hallucinations |
| `citation_precision` | Are citations real, selected, and actually relevant? Catches hallucinated doc IDs |
| `task_completion` | Did the agent *complete* the task — or just describe it? Distinguishes execution from deflection |
| `response_completeness` | Were ALL sub-questions in a multi-part query answered? Penalizes partial answers |
| `hallucination_risk` | Detects high-risk claim patterns — statistics, URLs, quotes not grounded in evidence |

### Retrieval Pipeline

| Evaluator | What it measures |
|-----------|-----------------|
| `retrieval_quality` | Recall (were the right docs found?), ranking (were better docs ranked lower?), ACL impact |
| `permission_safety` | Did any ACL-blocked document appear in the answer or citations? Zero tolerance |

### Memory & Context

| Evaluator | What it measures |
|-----------|-----------------|
| `memory_utility` | Was memory helpful? Were stale items used? Were relevant items ignored? |
| `context_poisoning` | Did a stale value (e.g. "15 days" from 2024 memory) contaminate the final answer? |
| `session_coherence` | Were user preferences honoured? Are entity values consistent across turns? |

### Agent Behaviour

| Evaluator | What it measures |
|-----------|-----------------|
| `tool_correctness` | Right tool? Right arguments? Correct ordering? No unsafe unapproved executions? |
| `trajectory_quality` | Was the reasoning path efficient? Any loops? Did it complete cleanly? |

### Cost & Performance

| Evaluator | What it measures |
|-----------|-----------------|
| `cost_efficiency` | Token spend, latency, and cost within configured thresholds |
| `agent_regression` | Did quality degrade vs a recorded baseline run? |

### Autonomous Agent

| Evaluator | What it measures |
|-----------|-----------------|
| `plan_adherence` | Did a multi-step agent follow its declared plan? Detects deviation and improvisation |
| `agent_handoff_quality` | In multi-agent pipelines, was context preserved and handoff intent clear? |

---

## Web UI

Open `http://localhost:3000` after `docker compose up -d`.

### Dashboard

Overview of all runs — total, completed, failed, average latency, token spend, and total cost. The full 17-evaluator grid shows which categories are active.

### Runs

Table view of all ingested runs with status, eval grade (A–F), token count, latency, and cost. Supports text and status filters.

### Compare

Select any two runs in the Runs page and click **Compare selected** to see a side-by-side evaluator score table with per-metric deltas highlighted green (improvement) or red (regression).

```
http://localhost:3000/compare?a=<run-id-a>&b=<run-id-b>
```

### Annotate

Human labeling queue for teams that want manual oversight of borderline runs.

- Shows a queue of completed runs with the question, agent answer, and retrieved context
- Annotators rate quality with thumbs up/down or 1–5 stars, add a correction note, and optionally flag a run as **ground truth** to use as a regression baseline
- Progress bar shows X of Y annotated
- Keyboard shortcuts: **J/K** navigate, **Y/N** thumbs up/down, **S** skip

Annotations are saved via `POST /api/v1/runs/{id}/annotate`:

```json
{
  "rating": 4,
  "note": "The answer is correct but misses the edge case about carry-over.",
  "is_ground_truth": false
}
```

### Drift Monitoring

Time-series view showing how evaluator scores trend over time.

- SVG line chart (no external dependencies) showing score trends per evaluator
- Time range selector: **7 days / 30 days / 90 days**
- Alert badges when a score has drifted more than 10% within the window
- "Drift detected" banner if any metric crossed the regression threshold in the last 24 h
- Table showing evaluator | current score | 7-day avg | trend | status

Powered by `GET /api/v1/drift?days=30&project=<agent-id>`:

```json
{
  "series": [
    {"date": "2026-05-12", "evaluator": "answer_correctness", "score": 0.84},
    {"date": "2026-05-13", "evaluator": "answer_correctness", "score": 0.81}
  ],
  "alerts": [
    {"evaluator": "retrieval_quality", "current": 0.71, "baseline": 0.83, "delta": -0.12}
  ]
}
```

---

## Evaluation Profiles

Run only the evaluators relevant to your system type:

```bash
# RAG pipeline debug
POST /evaluate/profile?run_id=X&profile=rag
# → retrieval_quality, groundedness, citation_precision, permission_safety,
#   answer_correctness, context_poisoning

# Agentic task execution
POST /evaluate/profile?run_id=X&profile=agent
# → tool_correctness, trajectory_quality, task_completion,
#   answer_correctness, cost_efficiency

# Multi-session memory system
POST /evaluate/profile?run_id=X&profile=memory
# → memory_utility, session_coherence, context_poisoning, groundedness

# Enterprise safety gate
POST /evaluate/profile?run_id=X&profile=enterprise
# → permission_safety, citation_precision, groundedness,
#   answer_correctness, context_poisoning, session_coherence

# Everything (all 17 evaluators)
POST /evaluate/profile?run_id=X&profile=full
```

---

## CI Gates

Block releases when quality or safety thresholds are violated:

```yaml
# .contextops/gates.yaml
minimum_scores:
  correctness: 0.90
  groundedness: 0.85
  retrieval_recall: 0.80
  permission_safety: 1.00    # zero tolerance — any ACL leak = block
  task_completion: 0.85

maximum_thresholds:
  latency_ms_p95: 5000
  cost_per_run_usd: 0.10
  memory_staleness_rate: 0.10
```

```bash
contextops gate check --config .contextops/gates.yaml
# Exits 1 if any threshold is violated — plugs into any CI pipeline
```

---

## Benchmark Suites

Five pre-built scenario packs you can run against your system:

| Suite | What it tests |
|-------|--------------|
| `enterprise-search` | Policy retrieval, role-sensitive answers, doc freshness, citation accuracy |
| `memory-assistant` | Preference persistence, stale memory suppression, conflict resolution |
| `memory-poisoning` | Stale value override, ACL-blocked memory leakage, preference honoured across turns |
| `workflow-agent` | Tool sequencing, approval gates, side-effect accuracy, completion rate |
| `document-copilot` | Clause lookup, summarization grounding, extraction accuracy |

```bash
contextops benchmark run enterprise-search
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2
```

---

## How It Compares

| Capability | LangSmith | RAGAS | Arize Phoenix | **ContextOps** |
|------------|:---------:|:-----:|:-------------:|:--------------:|
| Final answer correctness | ✅ | ✅ | ✅ | ✅ |
| Groundedness / hallucination | Partial | ✅ | ✅ | ✅ |
| Retrieval ranking diagnosis | ❌ | Partial | Partial | ✅ |
| ACL / permission safety | ❌ | ❌ | ❌ | ✅ |
| Memory staleness evaluation | ❌ | ❌ | ❌ | ✅ |
| Context poisoning detection | ❌ | ❌ | ❌ | ✅ |
| Session coherence | ❌ | ❌ | ❌ | ✅ |
| Citation accuracy (not just existence) | ❌ | Partial | ❌ | ✅ |
| Task completion vs deflection | ❌ | ❌ | ❌ | ✅ |
| Autonomous agent evaluation | ❌ | ❌ | ❌ | ✅ |
| Human annotation queue | ❌ | ❌ | ❌ | ✅ |
| Drift monitoring dashboard | Partial | ❌ | Partial | ✅ |
| Run comparison (side-by-side) | Partial | ❌ | Partial | ✅ |
| CI gate blocking | ❌ | ❌ | ❌ | ✅ |
| Evaluation profiles | ❌ | ❌ | ❌ | ✅ |
| OpenTelemetry adapter | ❌ | ❌ | ✅ | ✅ |
| Self-hosted / open source | ❌ | ✅ | ✅ | ✅ |

---

## API Reference

### Core run flow

```
POST /api/v1/runs                     Ingest a trace (run + retrieval + memory + tools)
GET  /api/v1/runs                     List runs (filter: ?status=, ?agent_id=)
GET  /api/v1/runs/:id                 Get a single run
GET  /api/v1/runs/:id/timeline        Full debug view: run + all sub-tables + evaluations
POST /api/v1/runs/:id/evaluate        Trigger evaluation (all 17 evaluators or specific set)
GET  /api/v1/runs/:id/eval-summary    Grade (A–F), avg score, critical failure highlights
GET  /api/v1/runs/:id/evaluations     Per-evaluator scores, reasoning, and details
POST /api/v1/runs/:id/annotate        Save human annotation (rating, note, is_ground_truth)
```

### Evaluators and drift

```
GET  /api/v1/evaluators               List all 17 registered evaluators
GET  /api/v1/evaluations              List evaluations (filter: ?category=)
GET  /api/v1/drift                    Time-series scores per evaluator + alerts (drift > 10%)
GET  /api/v1/drift/window             Single-window comparison (legacy)
```

### Comparison

```
POST /api/v1/compare                  Compare two runs — returns delta summary
GET  /compare?a=<id>&b=<id>          Web UI side-by-side comparison view
```

### Benchmarks and datasets

```
POST /api/v1/benchmarks               Create a benchmark suite
POST /api/v1/benchmarks/:id/run       Run a suite against your agent
GET  /api/v1/benchmarks/:id/results   Aggregate results with regression detection
POST /api/v1/datasets                 Create a dataset
POST /api/v1/traces                   Ingest canonical (nested) trace format
```

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/              # Go REST API (Gin, multi-tenant, JWT, audit log)
│   │   └── internal/
│   │       ├── handler/  # All HTTP handlers including annotate + drift
│   │       ├── db/       # pgx connection pool
│   │       └── model/    # Shared Go structs
│   ├── cli/              # Go CLI (init, trace, eval, benchmark, gate, compare, report)
│   ├── evaluator/        # Python evaluation engine (FastAPI, 17 async evaluators)
│   └── web/              # Next.js 14 web UI
│       └── src/app/
│           ├── page.tsx          # Dashboard
│           ├── runs/             # Run list + detail
│           ├── compare/          # Side-by-side run comparison
│           ├── annotate/         # Human annotation queue
│           └── drift/            # Drift monitoring dashboard
├── packages/
│   ├── trace-schema/     # JSON Schema for the run trace format
│   ├── evaluator-core/   # Shared evaluator interfaces
│   ├── context-manifest/ # Context assembly spec
│   ├── benchmark-core/   # Dataset runners and scoring
│   └── policy-core/      # ACL / permission check utilities
├── adapters/             # REST, LangGraph, OpenAI Agents SDK, OpenTelemetry
├── benchmarks/           # Enterprise search, memory, memory-poisoning, workflow, doc copilot
└── deploy/               # Docker Compose, Kubernetes / Helm charts
```

---

## Environment Setup

```bash
cp .env.example .env
```

| Variable | Required | Description |
|----------|----------|-------------|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Yes | Postgres credentials |
| `DATABASE_URL` | Yes | Full connection string for the API service |
| `REDIS_URL` | Yes | Redis connection string |
| `OPENAI_API_KEY` | Optional | Enables LLM-as-judge for `answer_correctness` and `groundedness`; falls back to heuristics if unset |
| `NEXT_PUBLIC_API_URL` | Yes | API base URL for the web UI (e.g. `http://localhost:8080`) |
| `CONTEXTOPS_API_URL` | Yes | API base URL for the CLI |

> **Never commit `.env`** — it is gitignored.

---

## Contributing

Contributions welcome in: new evaluators, adapter integrations, benchmark datasets, UI improvements. See [CONTRIBUTING.md](CONTRIBUTING.md). Run `go test ./...` and `pytest` before submitting.

## License

Apache 2.0 — see [LICENSE](LICENSE).
