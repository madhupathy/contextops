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

## What ContextOps Evaluates

```
User query
    │
    ▼
Retrieval candidates ←── Was the right document retrieved?
    │                     Were better-scoring docs rejected by ACL?
    │                     Did ranking put lower-quality docs first?
    ▼
Memory injection ←──────  Was this memory stale?
    │                     Is it contradicting current retrieved evidence?
    │                     Was a useful preference ignored?
    ▼
Context assembly ←──────  Did a stale value from memory contaminate the answer?
    │                     Is early-turn context poisoning late-turn reasoning?
    ▼
Tool calls ←────────────  Wrong tool selected? Wrong arguments?
    │                     Action intent but tool not called?
    ▼
Final answer ←──────────  Is every claim grounded in retrieved evidence?
                          Did any ACL-blocked document get cited?
                          Did the agent actually complete the task?
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
# UI:        http://localhost:3000
```

### CLI

```bash
go install github.com/madhupathy/contextops/apps/cli@latest

contextops init
contextops trace ingest ./examples/traces/pto-policy-failure.json
contextops eval run <run-id>
contextops gate check --config .contextops/gates.yaml
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

## 12 Evaluators

### Core Quality

| Evaluator | What it measures |
|-----------|-----------------|
| `answer_correctness` | Is the final answer factually correct? Heuristic + optional LLM-as-judge |
| `groundedness` | Is every claim in the answer supported by retrieved evidence? Detects hallucinations |
| `citation_precision` | Are citations real, selected, and actually relevant? Catches hallucinated doc IDs |
| `task_completion` | Did the agent *complete* the task — or just describe it? Distinguishes execution from deflection |

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

# Everything
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
| CI gate blocking | ❌ | ❌ | ❌ | ✅ |
| Evaluation profiles | ❌ | ❌ | ❌ | ✅ |
| Self-hosted / open source | ❌ | ✅ | ✅ | ✅ |

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
| `NEXT_PUBLIC_API_URL` | Yes | API base URL for the web UI |
| `CONTEXTOPS_API_URL` | Yes | API base URL for the CLI |

> **Never commit `.env`** — it is gitignored.

---

## API Reference

### Core run flow

```
POST /api/v1/runs                     Ingest a trace (run + retrieval + memory + tools)
GET  /api/v1/runs/:id/timeline        Full debug view: run + all sub-tables + evaluations
POST /api/v1/runs/:id/evaluate        Trigger evaluation (all 12 evaluators or specific set)
GET  /api/v1/runs/:id/eval-summary    Grade (A–F), avg score, critical failure highlights
GET  /api/v1/runs/:id/evaluations     Per-evaluator scores, reasoning, and details
```

### Evaluation profiles

```
POST /evaluate/profile?run_id=X&profile=rag|agent|memory|enterprise|full
GET  /evaluators/profiles             List all profiles with their category sets
GET  /evaluators                      List all 12 registered evaluators
```

### Benchmarks & datasets

```
POST /api/v1/benchmarks               Create a benchmark suite
POST /api/v1/benchmarks/:id/run       Run a suite against your agent
GET  /api/v1/benchmarks/:id/results   Aggregate results with regression detection
POST /api/v1/compare                  Compare two runs side by side
```

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/              # Go REST API (Gin, multi-tenant, JWT, audit log)
│   ├── cli/              # Go CLI (init, trace, eval, benchmark, gate, compare, report)
│   ├── evaluator/        # Python evaluation engine (FastAPI, 12 async evaluators)
│   └── web/              # Next.js 14 debugger UI with run timeline, eval groups, trace view
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

## Contributing

Contributions welcome in: new evaluators, adapter integrations, benchmark datasets, UI improvements. See [CONTRIBUTING.md](CONTRIBUTING.md). Run `go test ./...` and `pytest` before submitting.

## License

Apache 2.0 — see [LICENSE](LICENSE).
