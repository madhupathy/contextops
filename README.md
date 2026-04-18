<div align="center">

# ContextOps

**You know your AI gave the wrong answer. ContextOps tells you exactly why.**

ContextOps is an open-source evaluation and debugging platform for AI agents, RAG systems, and enterprise copilots. It evaluates the full execution path — retrieval quality, memory, ACL safety, tool calls, cost — not just whether the final answer looked correct.

[![CI](https://github.com/madhupathy/contextops/actions/workflows/ci.yml/badge.svg)](https://github.com/madhupathy/contextops/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Python 3.12+](https://img.shields.io/badge/Python-3.12+-3776AB?logo=python&logoColor=white)](https://python.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

---

## The Problem

Your RAG-based enterprise assistant confidently tells an employee the wrong PTO policy. You check the logs: the final answer got a 0.87 similarity score from your evaluation pipeline. "Looks good," it says.

But here's what actually happened:
1. The retriever fetched the **2024 policy** instead of the **2025 update**
2. A **stale memory** from last quarter got injected into the context
3. The **correct document was ACL-filtered out** due to a misconfigured group mapping
4. The model produced a confident, well-structured answer based on wrong information

Existing evaluation tools — RAGAS, LangSmith evals, custom scripts — answer one question: *"Was the final answer good?"* They miss everything that happened before the answer was generated.

ContextOps answers: **"Where in the pipeline did this fail, and how do we catch it before it reaches production?"**

---

## What ContextOps Evaluates

```
User query
    │
    ▼
Retrieval candidates ←── ContextOps checks:
    │                      • Did we fetch the right docs?
    │                      • Were better docs ranked lower?
    │                      • Did ACL filters block the correct doc?
    ▼
Memory injection ←──────  ContextOps checks:
    │                      • Is this memory stale?
    │                      • Does it conflict with newer context?
    │                      • Was useful memory ignored?
    ▼
Tool calls ←────────────  ContextOps checks:
    │                      • Was the right tool selected?
    │                      • Were arguments correct?
    │                      • Did the workflow loop unnecessarily?
    ▼
Final answer ←──────────  ContextOps checks:
                           • Is the answer grounded in evidence?
                           • Does it cite restricted documents?
                           • Did accuracy regress vs. last version?
```

---

## Real Example

**Query:** "What is the latest PTO policy?"

**What the system did:**
1. Retrieved `PTO Policy 2024` (score: 0.83) — selected
2. Retrieved `PTO Policy 2025` (score: 0.79) — **filtered by ACL bug**
3. Injected stale memory: "User asked about PTO last year"
4. Produced confident answer based on the wrong document

**ContextOps report:**
```
retrieval_quality    0.41  FAIL  Better document rejected by ACL filter
memory_staleness     0.20  FAIL  Injected episodic memory is 9 months old
permission_safety    0.00  FAIL  CRITICAL: ACL misconfiguration blocked correct document
correctness          0.00  FAIL  Final answer contradicts current policy
```

**That level of visibility** is what ContextOps provides. Not a final-answer score. A full-stack diagnosis.

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
contextops compare <run-a> <run-b>
contextops gate check --config .contextops/gates.yaml
```

---

## Ingesting a Trace

Send a single JSON object capturing everything that happened in one AI run:

```json
{
  "run_id": "run-001",
  "agent": { "id": "policy-assistant", "version": "0.2.1" },
  "input": { "query": "What is the latest PTO policy?" },
  "retrieval": {
    "candidates": [
      { "doc_id": "doc-pto-2024", "score": 0.83, "selected": true, "acl_passed": true },
      { "doc_id": "doc-pto-2025", "score": 0.79, "selected": false, "acl_passed": false,
        "acl_reason": "group-mapping-mismatch" }
    ]
  },
  "memory": {
    "candidates": [
      { "memory_id": "mem-01", "summary": "User asked about PTO last year",
        "age_days": 287, "selected": true }
    ]
  },
  "output": {
    "final_answer": "The PTO policy allows 15 days...",
    "citations": ["doc-pto-2024"]
  },
  "metrics": { "latency_ms": 1820, "prompt_tokens": 2400 }
}
```

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H 'Content-Type: application/json' \
  -d @my-run.json
```

---

## Evaluators

| Evaluator | What it measures | Key signal |
|-----------|-----------------|------------|
| `correctness` | Is the final answer factually right? | Compared against ground truth |
| `groundedness` | Is every claim supported by retrieved evidence? | Citation coverage |
| `retrieval_precision` | Were retrieved docs relevant to the query? | Relevance scoring |
| `retrieval_recall` | Were the *best* docs actually retrieved? | Missed expected sources |
| `memory_relevance` | Was selected memory helpful, not distracting? | Memory signal quality |
| `memory_staleness` | Was outdated memory injected? | Memory age + conflict detection |
| `permission_safety` | Did any ACL-blocked content appear in the answer? | Critical — zero tolerance |
| `trajectory_quality` | Was the reasoning path efficient? | Loops, redundant steps |
| `tool_correctness` | Were tool calls right (name + arguments)? | Tool call accuracy |
| `cost_efficiency` | Was token spend reasonable for the task? | Tokens per outcome |

Each evaluator returns: **numeric score (0–1)**, **pass/fail**, **explanation**, **evidence**, and **remediation hints**.

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

maximum_thresholds:
  latency_ms_p95: 3500
  cost_per_run_usd: 0.08
  memory_staleness_rate: 0.05
```

```bash
# In your CI pipeline
contextops gate check --config .contextops/gates.yaml
# Exits 1 if any threshold is violated
```

---

## Benchmark Packs

Pre-built scenario suites you can run against your system out of the box:

| Pack | Tests |
|------|-------|
| **Enterprise Search** | Latest policy retrieval, role-sensitive answers, doc freshness, citation accuracy |
| **Memory Assistant** | Long-lived user preferences, stale memory suppression, conflict resolution |
| **Workflow Agent** | Tool sequencing, approval gates, side-effect validation, completion accuracy |
| **Document Copilot** | Clause lookup, summarization grounding, extraction accuracy, ACL enforcement |

```bash
contextops benchmark run enterprise-search
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2
```

---

## How ContextOps Compares

| Capability | LangSmith | RAGAS | ContextOps |
|------------|-----------|-------|------------|
| Final answer correctness | ✅ | ✅ | ✅ |
| Groundedness / hallucination | Partial | ✅ | ✅ |
| Retrieval ranking diagnosis | ❌ | Partial | ✅ |
| ACL / permission safety | ❌ | ❌ | ✅ |
| Memory evaluation | ❌ | ❌ | ✅ |
| Tool call evaluation | Partial | ❌ | ✅ |
| Regression testing via CI gates | ❌ | ❌ | ✅ |
| Self-hosted / open source | ❌ | ✅ | ✅ |

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/           # Go REST API (Gin, multi-tenant, JWT auth)
│   ├── cli/           # Go CLI (Cobra) — init, trace, eval, benchmark, gate, compare
│   ├── evaluator/     # Python evaluation engine (FastAPI + async evaluators)
│   └── web/           # Next.js debugger UI
├── packages/
│   ├── trace-schema/  # JSON Schema for the run trace format
│   ├── evaluator-core/ # Shared evaluator interfaces
│   ├── context-manifest/ # Context assembly specification
│   ├── benchmark-core/ # Dataset runners and scoring
│   └── policy-core/   # ACL / permission check utilities
├── adapters/          # REST, LangGraph, OpenAI Agents SDK, OpenTelemetry
├── benchmarks/        # Enterprise search, memory, workflow, doc copilot suites
└── deploy/            # Docker Compose, Kubernetes / Helm charts
```

---

## Environment Setup

```bash
cp .env.example .env
```

| Variable | Required | Description |
|----------|----------|-------------|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Yes | Postgres credentials |
| `DATABASE_URL` | Yes | Full Postgres connection string for the API |
| `REDIS_URL` | Yes | Redis connection string |
| `OPENAI_API_KEY` | Optional | For LLM-as-judge evaluators; falls back to heuristics if unset |
| `NEXT_PUBLIC_API_URL` | Yes | API base URL for the web UI |
| `CONTEXTOPS_API_URL` | Yes | API URL for the CLI |

> **Never commit `.env`** — it is gitignored.

---

## Contributing

Contributions welcome in: new evaluators, adapter integrations, benchmark datasets, UI improvements, and documentation. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache 2.0 — see [LICENSE](LICENSE).
