<div align="center">

# ContextOps

**You know your AI gave the wrong answer. ContextOps tells you exactly why.**

ContextOps is an open-source evaluation and debugging platform for AI agents, RAG systems, and enterprise copilots. It evaluates the full execution path — not just the final answer.

[![CI](https://github.com/madhupathy/contextops/actions/workflows/ci.yml/badge.svg)](https://github.com/madhupathy/contextops/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Python 3.12+](https://img.shields.io/badge/Python-3.12+-3776AB?logo=python&logoColor=white)](https://python.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Evaluators](https://img.shields.io/badge/Evaluators-12-7c3aed)](#12-evaluators)

</div>

---

## The Problem

Your RAG assistant gives an employee the wrong PTO policy. You check your eval pipeline — the answer scored **0.87 similarity**. "Looks good," it says.

Here's what actually happened:

1. The retriever fetched the **2024 policy** — not the 2025 update
2. A **stale memory** from a prior session was injected into context
3. The **correct 2025 doc was ACL-filtered out** due to a misconfigured group mapping
4. The model produced a confident, well-structured answer from wrong information

LangSmith, Langfuse, Arize Phoenix — they all answer: *"Was the final answer good?"*

ContextOps answers: **"Where in the pipeline did this fail, and how do we prevent it next time?"**

---

## What ContextOps Evaluates

```
User query
    │
    ▼
Retrieval ──────── Did we fetch the right docs?
    │               Were better docs ranked lower?
    │               Did ACL filters block the correct doc? ← permission_safety
    ▼
Memory injection ── Is this memory stale?         ← memory_utility
    │               Does it conflict with new docs? ← context_poisoning
    │               Was the user's preference honoured? ← session_coherence
    ▼
Tool calls ────────  Was the right tool selected?  ← tool_correctness
    │               Were arguments correct?
    │               Did the workflow loop?          ← trajectory_quality
    ▼
Final answer ──────  Is it factually correct?      ← answer_correctness
                    Is every claim cited?           ← citation_precision
                    Was the task actually done?     ← task_completion
                    Was it grounded in evidence?    ← groundedness
```

---

## Real Example

**Query:** "What is the latest PTO policy?"

**What happened internally:**

| Step | Result |
|---|---|
| Retrieved `PTO Policy 2024` (score: 0.83) | ✅ Selected |
| Retrieved `PTO Policy 2025` (score: 0.79) | ❌ ACL-blocked — `group-mapping-mismatch` |
| Injected episodic memory from Dec 2024 | ⚠️ Stale — policy updated since |
| Final answer: "15 days, carry over 5" | ❌ Wrong — 2025 policy is 20 days, unlimited carry-over |

**ContextOps report:**

```
retrieval_quality    0.41  FAIL  Better document blocked by ACL misconfiguration
permission_safety    0.00  FAIL  CRITICAL: ACL filter prevented correct doc from being selected
memory_utility       0.30  FAIL  Stale episodic memory injected (policy changed since last interaction)
context_poisoning    0.10  FAIL  CRITICAL: Stale value "15 days" in answer — not present in current docs
answer_correctness   0.00  FAIL  Answer contradicts expected answer (20 days)
session_coherence    1.00  PASS  No session-level contradictions
```

**Grade: F (avg: 0.30)** — that 0.87 similarity score missed everything that mattered.

---

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/madhupathy/contextops.git
cd contextops
cp .env.example .env          # add OPENAI_API_KEY for LLM-as-judge evaluators
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
contextops eval run <run-id> --explain
contextops gate check --config .contextops/gates.yaml
```

---

## Ingest a Trace

Send one JSON object capturing everything that happened in a run:

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "What is the latest PTO policy?",
    "final_answer": "The PTO policy allows 15 days...",
    "citations": ["doc-old-pto"],
    "retrieval": {
      "candidates": [
        { "doc_id": "doc-pto-2024", "score": 0.83, "selected": true, "acl_passed": true },
        { "doc_id": "doc-pto-2025", "score": 0.79, "selected": false, "acl_passed": false,
          "acl_reason": "group-mapping-mismatch" }
      ]
    },
    "memory": {
      "candidates": [
        { "memory_id": "mem-01", "memory_type": "episodic",
          "content": "User asked about PTO in 2024 — 15 days at that time",
          "is_stale": true, "stale_reason": "Policy updated Jan 2025", "selected": true }
      ]
    },
    "metrics": { "latency_ms": 1820, "total_tokens": 2780, "estimated_cost": 0.042 }
  }'
```

Then evaluate it:

```bash
# Run all 12 evaluators
curl -X POST http://localhost:8080/api/v1/runs/<run-id>/evaluate

# Run by profile
curl -X POST "http://localhost:8081/evaluate/profile?run_id=<run-id>&profile=enterprise"

# Get a grade summary
curl http://localhost:8080/api/v1/runs/<run-id>/eval-summary
```

---

## 12 Evaluators

### Core Quality
| Evaluator | What it catches |
|---|---|
| `answer_correctness` | Wrong facts, compared to expected answer or via LLM-as-judge |
| `groundedness` | Claims not supported by retrieved evidence — hallucinations |
| `citation_precision` | Hallucinated citation IDs, citations to ACL-blocked docs, unsupported citations |
| `task_completion` | Action intents that deflect to the user instead of executing; lookup answers missing the point |

### Retrieval Pipeline
| Evaluator | What it catches |
|---|---|
| `retrieval_quality` | Wrong docs fetched, better docs ranked lower, missed expected sources |
| `permission_safety` | ACL-blocked content in the answer, blocked docs cited, missing ACL data |

### Memory & Context
| Evaluator | What it catches |
|---|---|
| `memory_utility` | Stale memory selected, high-relevance memory ignored, poor selection quality |
| `context_poisoning` | Stale values contaminating the answer, numeric drift from old context, self-contradictions |
| `session_coherence` | User preferences not honoured, entity contradictions across turns, temporal inconsistency |

### Agent Behaviour
| Evaluator | What it catches |
|---|---|
| `tool_correctness` | Wrong tool selected, incorrect arguments, loops, unapproved high-risk tool calls |
| `trajectory_quality` | Excessive steps, loops, thinking without acting, incomplete execution |

### Cost & Performance
| Evaluator | What it catches |
|---|---|
| `cost_efficiency` | Token spend, latency, and cost outside acceptable bounds |

Each evaluator returns: **score (0–1)**, **pass/fail**, **reasoning**, **evidence**, and **remediation hints**.

---

## Evaluation Profiles

Run the evaluators most relevant to your use case:

```bash
# RAG pipeline quality
POST /evaluate/profile?run_id=X&profile=rag
# → retrieval_quality, groundedness, citation_precision, permission_safety, answer_correctness, context_poisoning

# Agentic task execution
POST /evaluate/profile?run_id=X&profile=agent
# → tool_correctness, trajectory_quality, task_completion, answer_correctness, cost_efficiency

# Multi-session memory systems
POST /evaluate/profile?run_id=X&profile=memory
# → memory_utility, session_coherence, context_poisoning, groundedness

# Enterprise safety gate
POST /evaluate/profile?run_id=X&profile=enterprise
# → permission_safety, citation_precision, groundedness, answer_correctness, context_poisoning, session_coherence
```

---

## CI Gates — Block Bad Releases

```yaml
# .contextops/gates.yaml
minimum_scores:
  correctness: 0.90
  groundedness: 0.92
  retrieval_recall: 0.88
  permission_safety: 1.00    # zero tolerance — any ACL leak = block
  task_completion: 0.90
  context_poisoning: 0.95

maximum_thresholds:
  latency_ms_p95: 3500
  cost_per_run_usd: 0.08
```

```bash
contextops gate check --config .contextops/gates.yaml
# Exits 1 if any threshold violated — blocks the deploy
```

---

## Benchmark Packs

Pre-built scenario suites targeting real enterprise failure modes:

| Pack | What it tests |
|---|---|
| **Enterprise Search** | Latest policy retrieval, role-sensitive answers, citation accuracy, action task completion |
| **Memory Poisoning** | Stale value override, preference persistence across sessions, ACL-blocked content in memory |
| **Memory Assistant** | Long-lived preferences, stale memory suppression, conflict resolution |
| **Workflow Agent** | Tool sequencing, approval gates, side-effect validation |
| **Document Copilot** | Clause lookup, summarization grounding, extraction accuracy |

```bash
contextops benchmark run enterprise-search
contextops benchmark run memory-poisoning
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2
```

---

## How ContextOps Compares

| Capability | LangSmith | RAGAS | Arize Phoenix | **ContextOps** |
|---|:---:|:---:|:---:|:---:|
| Final answer correctness | ✅ | ✅ | ✅ | ✅ |
| Groundedness / hallucination | Partial | ✅ | Partial | ✅ |
| Retrieval ranking diagnosis | ❌ | Partial | Partial | ✅ |
| ACL / permission safety | ❌ | ❌ | ❌ | ✅ |
| Memory staleness detection | ❌ | ❌ | ❌ | ✅ |
| Context poisoning detection | ❌ | ❌ | ❌ | ✅ |
| Session coherence / preference | ❌ | ❌ | ❌ | ✅ |
| Citation precision (not just existence) | ❌ | Partial | ❌ | ✅ |
| Task completion vs deflection | ❌ | ❌ | ❌ | ✅ |
| Regression CI gates | ❌ | ❌ | Partial | ✅ |
| Self-hosted / open-source | ❌ | ✅ | ✅ (Phoenix) | ✅ |

---

## Environment Setup

```bash
cp .env.example .env
```

| Variable | Required | Description |
|---|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Yes | Postgres credentials |
| `DATABASE_URL` | Yes | Full connection string for API and evaluator |
| `REDIS_URL` | Yes | Redis connection string |
| `OPENAI_API_KEY` | Optional | For LLM-as-judge evaluators; falls back to heuristics if unset |
| `NEXT_PUBLIC_API_URL` | Yes | API base URL for the web UI |
| `CONTEXTOPS_API_URL` | Yes | API URL for the CLI |

> **Never commit `.env`** — it is gitignored.

---

## API Reference

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/runs` | Ingest a run trace |
| `GET` | `/api/v1/runs` | List all runs |
| `GET` | `/api/v1/runs/:id/timeline` | Full debug view: run + retrieval + memory + tools + steps + evals |
| `POST` | `/api/v1/runs/:id/evaluate` | Trigger evaluation (all 12 or selected categories) |
| `GET` | `/api/v1/runs/:id/eval-summary` | Grade (A–F), avg score, critical failures |
| `POST` | `/evaluate/profile` | Evaluate by intent profile (rag/agent/memory/enterprise/full) |
| `GET` | `/evaluators/profiles` | List all profiles with category sets |
| `POST` | `/evaluate/batch` | Evaluate multiple runs in one call |
| `POST` | `/api/v1/benchmarks/:id/run` | Run a benchmark suite |
| `GET` | `/api/v1/benchmarks/:id/results` | Get benchmark results with regression detection |
| `POST` | `/api/v1/compare` | Compare two runs side-by-side |

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/               # Go REST API (Gin, multi-tenant JWT auth)
│   ├── cli/               # Go CLI (Cobra) — init, trace, eval, benchmark, gate, compare
│   ├── evaluator/         # Python evaluation engine (FastAPI, 12 evaluators)
│   └── web/               # Next.js debugger UI
├── packages/
│   ├── trace-schema/      # JSON Schema for the run trace format
│   ├── evaluator-core/    # Shared evaluator interfaces
│   ├── context-manifest/  # Context assembly specification
│   ├── benchmark-core/    # Dataset runners and scoring
│   └── policy-core/       # ACL / permission utilities
├── adapters/              # REST, LangGraph, OpenAI Agents SDK, OpenTelemetry
├── benchmarks/            # Enterprise search, memory-poisoning, memory, workflow, doc copilot
├── examples/traces/       # Ready-to-ingest example traces
└── deploy/                # Docker Compose, Kubernetes / Helm charts
```

---

## Roadmap

- [x] 12 evaluators across correctness, retrieval, memory, context, agent, cost
- [x] Context poisoning and session coherence detection
- [x] Citation precision (not just existence)
- [x] Task completion vs deflection
- [x] Intent-based evaluation profiles
- [x] CI gate enforcement
- [x] Benchmark packs with regression detection
- [x] Full run timeline API (retrieval + memory + tools + trace)
- [ ] OTEL collector adapter
- [ ] LangGraph and OpenAI Agents SDK adapters
- [ ] Dataset versioning and sampling
- [ ] Human-in-the-loop annotation queue
- [ ] Hosted SaaS

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Contributions welcome in: new evaluators, adapter integrations, benchmark datasets, and documentation.

## License

Apache 2.0 — see [LICENSE](LICENSE).
