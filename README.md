# ContextOps

> **Open-source evaluation, debugging, and regression testing for AI agents, RAG systems, and enterprise copilots.**

[![CI](https://github.com/madhupathy/contextops/actions/workflows/ci.yml/badge.svg)](https://github.com/madhupathy/contextops/actions/workflows/ci.yml)
[![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-blue)](https://go.dev/)
[![Python 3.12+](https://img.shields.io/badge/Python-3.12%2B-blue)](https://python.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue)](LICENSE)

ContextOps helps teams understand **why** an AI system failed — not just whether the final answer looked good. It evaluates and inspects the full AI execution path: retrieval quality, memory usage, permission safety, tool calls, groundedness, cost, and regressions.

---

## Screenshots

> Add screenshots of your ContextOps dashboard to `docs/screenshots/` and update the paths below.
>
> **Runs dashboard** — inspect trace details, evaluation scores, and failure breakdowns  
> ![ContextOps Dashboard](docs/screenshots/dashboard.png)

---

## Why ContextOps?

Most AI evaluation tools answer: *"Was the answer good?"*

ContextOps answers: *"Why did this system fail, where did it fail, and how do we prevent it again?"*

| Capability | Traditional RAG Eval | ContextOps |
|---|---|---|
| Answer correctness | ✅ | ✅ |
| Groundedness | ✅ | ✅ |
| Retrieval ranking diagnosis | Limited | ✅ |
| Memory evaluation | ❌ | ✅ |
| Permission / ACL evaluation | ❌ | ✅ |
| Tool-call evaluation | ❌ | ✅ |
| Workflow trajectory evaluation | ❌ | ✅ |
| Regression testing | Partial | ✅ |
| Full-stack debugging | ❌ | ✅ |

---

## Quick Start

### Option 1 — Docker Compose (recommended)

```bash
git clone https://github.com/madhupathy/contextops.git
cd contextops
cp .env.example .env   # fill in OPENAI_API_KEY if using LLM-as-judge evaluators
docker compose up -d

# API:       http://localhost:8080
# Evaluator: http://localhost:8081
# UI:        http://localhost:3000
```

### Option 2 — CLI

```bash
go install github.com/madhupathy/contextops/apps/cli@latest

contextops server start
contextops init
contextops trace ingest ./examples/traces/support-agent-success.json
contextops eval run <run-id>
contextops compare <run-a> <run-b>
contextops gate check --config .contextops/gates.yaml
```

---

## Environment Setup

```bash
cp .env.example .env
```

| Variable | Required | Description |
|---|---|---|
| `POSTGRES_USER` | Yes | Postgres username |
| `POSTGRES_PASSWORD` | Yes | Postgres password |
| `POSTGRES_DB` | Yes | Postgres database name |
| `DATABASE_URL` | Yes | Full connection string for the API service |
| `REDIS_URL` | Yes | Redis connection string |
| `OPENAI_API_KEY` | Optional | Required for LLM-as-judge evaluators; falls back to heuristics if unset |
| `NEXT_PUBLIC_API_URL` | Yes | API base URL for the web UI |
| `CONTEXTOPS_API_URL` | Yes | API URL for the CLI |
| `CONTEXTOPS_TENANT` | Yes | Tenant UUID for CLI isolation |

> ⚠️ **Never commit `.env`** — it is gitignored. Use `.env.example` to document variables only.

---

## Architecture

```
                       +----------------------+
                       |   Agent / RAG App    |
                       | LangGraph / SDK /    |
                       | custom workflow      |
                       +----------+-----------+
                                  │
                                  │ traces / callbacks / events
                                  ▼
+----------------+   +--------------------------+   +------------------+
| Connectors /   │──▶│    ContextOps API        │──▶│   Postgres       │
| Adapters       │   │ traces, runs, evals,     │   │ traces, scores,  │
|                │   │ manifests, benchmarks    │   │ configs, datasets│
+----------------+   +------------+-------------+   +------------------+
                                  │
                                  ▼
                     +-----------------------------+
                     │ Evaluation Engine (Python)  │
                     │ correctness, retrieval,     │
                     │ memory, ACL, workflow       │
                     +-------------+---------------+
                                   │
                                   ▼
                     +-----------------------------+
                     │ Debug UI / CLI / CI Gates   │
                     │ inspect, compare, regress   │
                     +-----------------------------+
```

### Tech Stack

| Layer | Technology |
|---|---|
| API + CLI | Go (Gin, Cobra) |
| Evaluator | Python (FastAPI) |
| UI | Next.js + TypeScript + Tailwind CSS |
| Database | PostgreSQL + pgvector |
| Queue | Redis Streams |
| Auth | JWT with tenant isolation |

---

## CLI Reference

```bash
# Traces
contextops trace ingest ./run.json
contextops trace ingest ./runs/ --recursive
contextops trace show <run-id>
contextops trace list --agent policy-assistant --status failed --since 7d
contextops trace validate ./run.json

# Evaluation
contextops eval run <run-id>
contextops eval run <run-id> --only correctness,groundedness --explain
contextops eval dataset enterprise-search-v1

# Benchmarks
contextops benchmark run enterprise-search
contextops benchmark run workflow-agent --dataset ./datasets/workflows/

# Compare
contextops compare <run-a> <run-b>
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2

# Gates (CI/CD)
contextops gate check --config .contextops/gates.yaml

# Reports
contextops report generate <run-id> --format markdown
```

---

## Evaluators

| Category | What it measures |
|---|---|
| `correctness` | Is the final answer right? |
| `groundedness` | Is the answer supported by evidence? |
| `retrieval_precision` | Were retrieved docs relevant? |
| `retrieval_recall` | Were the best docs found? |
| `memory_relevance` | Was selected memory useful? |
| `memory_staleness` | Was stale memory avoided? |
| `permission_safety` | Was restricted content excluded? |
| `trajectory_quality` | Was the reasoning path efficient? |
| `tool_argument_correctness` | Were tool calls correct? |
| `cost_efficiency` | Was token spend reasonable? |

Each evaluator produces: numeric score, pass/fail, explanation, supporting evidence, and remediation hints.

---

## CI Gates

Block bad releases in GitHub Actions:

```yaml
# .contextops/gates.yaml
minimum_scores:
  correctness: 0.90
  groundedness: 0.92
  retrieval_recall: 0.88
  permission_safety: 1.00
  task_completion: 0.90

maximum_thresholds:
  latency_ms_p95: 3500
  cost_per_run_usd: 0.08
  memory_staleness_rate: 0.05
```

```bash
contextops gate check --config .contextops/gates.yaml
```

---

## Benchmark Packs

| Pack | Scenarios |
|---|---|
| Enterprise Search | Latest policy retrieval, role-sensitive answers, doc freshness |
| Memory Assistant | Long-lived preferences, stale memory suppression, conflict resolution |
| Workflow Agent | Tool sequencing, approvals, side effects, completion accuracy |
| Document Copilot | Clause lookup, summarization grounding, extraction correctness |

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/          # Go REST API
│   ├── cli/          # Go CLI (Cobra)
│   ├── evaluator/    # Python evaluation engine (FastAPI)
│   └── web/          # Next.js debugger UI
├── packages/
│   ├── trace-schema/       # Shared run schema + JSON Schema
│   ├── evaluator-core/     # Evaluation interfaces
│   ├── context-manifest/   # Context assembly spec
│   ├── benchmark-core/     # Datasets, runners, scoring
│   ├── policy-core/        # ACL / permissions checks
│   └── adapter-sdk/        # Instrumentation SDK
├── adapters/         # REST, LangGraph, OpenAI Agents, OTEL
├── benchmarks/       # Enterprise search, memory, workflow, doc copilot
├── docs/             # Architecture, CLI, evaluator, benchmark docs
├── deploy/           # Docker + Kubernetes / Helm
├── docker-compose.yml
└── Makefile
```

---

## Roadmap

- [x] Trace ingestion and run viewer
- [x] Basic evaluators, CLI, and Docker environment
- [x] Context manifests and benchmark packs
- [x] Gate checks and REST adapter
- [ ] Memory evaluators and permission simulators
- [ ] Dataset management
- [ ] OTEL support
- [ ] LangGraph and OpenAI Agents SDK adapters
- [ ] Advanced dashboards

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Contributions welcome in: evaluators, adapters, benchmark datasets, memory diagnostics, permissions test harnesses, and docs.

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
