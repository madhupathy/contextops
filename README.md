# ContextOps

**Open-source evaluation, debugging, and regression testing for AI agents, RAG systems, and enterprise copilots.**

ContextOps helps teams understand **why** an AI system failed — not just whether the final answer looked good.

It evaluates and inspects the full AI execution path:

- **Retrieval quality** — wrong docs, missed docs, bad ranking
- **Memory usage** — stale memory, conflicts, missed personalization
- **Permission / ACL safety** — restricted content leaking into prompts
- **Tool calls and workflows** — wrong tools, bad arguments, infinite loops
- **Grounding and correctness** — hallucinations, unsupported claims
- **Cost, latency, and regressions** — quality drops across versions

---

## Why ContextOps?

Most AI evaluation tools answer:

> "Was the answer good?"

ContextOps answers:

> "Why did this system fail, where did it fail, and how do we prevent it from happening again?"

That makes it useful for **RAG systems**, **AI agents**, **workflow agents**, **enterprise search copilots**, **memory-heavy assistants**, and **tool-using automation systems**.

---

## What Problems ContextOps Solves

### 1. Retrieval Failures
- Wrong documents retrieved
- Better documents missed
- Bad ranking or stale content preferred
- Irrelevant chunks included in context

### 2. Memory Failures
- Stale memory used
- Conflicting memory injected
- Useful memory ignored
- Over-reliance on past context

### 3. Permission Failures
- Restricted documents leaking into prompts
- ACL-filtered results degrading answer quality
- Wrong user/role simulation
- Unsafe citations

### 4. Agent / Workflow Failures
- Wrong tool selected or wrong arguments
- Infinite or wasteful loops
- Incorrect execution path
- Correct answer with incorrect side effects

### 5. Regression Failures
- A newer version performs worse
- Retrieval quality drops
- Memory conflict rate rises
- Cost increases or latency spikes unexpectedly

---

## Example Failure Analysis

**User asks:** `What is the latest PTO policy?`

**System behavior:**
1. Retrieves old HR document
2. Misses newer document
3. Injects stale memory from prior run
4. Filters the correct doc because of a permission mapping bug
5. Produces a confident but incorrect answer

**ContextOps report:**
- Retrieval ranking issue detected
- Stale memory used
- ACL filtering misconfiguration suspected
- Final answer incorrect
- Regression from previous release

That level of visibility is the point of ContextOps.

---

## Core Concepts

### Trace
A complete record of one AI run — input query, user identity, retrieved candidates, selected documents, memory candidates, ACL filters, tool calls, model outputs, final answer, citations, cost, and latency.

### Context Manifest
A machine-readable explanation of how the final context was assembled: which sources were considered, chosen, rejected, why they were rejected, prompt budget per source, and which memory items were injected.

### Evaluators
Pluggable scoring modules for correctness, groundedness, retrieval relevance, citation quality, permission safety, memory quality, trajectory quality, task completion, and efficiency.

### Benchmarks
Reusable scenario packs for enterprise search assistants, document copilots, workflow agents, memory-heavy assistants, support automation, and policy retrieval.

### Gates
CI checks that block deployment when quality or safety thresholds are violated.

### Adapters
Integrations for common agent stacks: generic REST callbacks, OpenAI Agents SDK, LangGraph, custom RAG pipelines, and OTEL-compatible traces.

---

## How ContextOps Is Different

| Capability | Traditional RAG Eval | ContextOps |
|------------|----------------------|------------|
| Answer correctness | Yes | Yes |
| Groundedness | Yes | Yes |
| Retrieval ranking diagnosis | Limited | Yes |
| Memory evaluation | No | Yes |
| Permission / ACL evaluation | No | Yes |
| Tool-call evaluation | No | Yes |
| Workflow trajectory evaluation | No | Yes |
| Regression testing | Partial | Yes |
| Full-stack debugging | No | Yes |

---

## Architecture

```text
                           +----------------------+
                           |   Agent / RAG App    |
                           | LangGraph / SDK /    |
                           | custom workflow      |
                           +----------+-----------+
                                      |
                                      | traces / callbacks / events
                                      v
+----------------+       +--------------------------+       +------------------+
| Connectors /   | ----> |    ContextOps API        | ----> |   Postgres       |
| Adapters       |       | traces, runs, evals,     |       | traces, scores,  |
|                |       | manifests, benchmarks    |       | configs, datasets|
+----------------+       +------------+-------------+       +------------------+
                                      |
                                      v
                         +-----------------------------+
                         | Evaluation Engine           |
                         | correctness, retrieval,     |
                         | memory, ACL, workflow       |
                         +-------------+---------------+
                                       |
                                       v
                         +-----------------------------+
                         | Debug UI / CLI / CI Gates   |
                         | inspect, compare, regress   |
                         +-----------------------------+
```

---

## Quick Start

### Option 1: CLI

```bash
# Install
go install github.com/contextops/contextops/apps/cli@latest

# Start local services
contextops server start

# Initialize a project
contextops init

# Ingest a trace
contextops trace ingest ./examples/run.json

# Evaluate a run
contextops eval run <run-id>

# Compare two runs
contextops compare <run-a> <run-b>

# Gate a release
contextops gate check --config .contextops/gates.yaml
```

### Option 2: Docker Compose

```bash
git clone https://github.com/contextops/contextops.git
cd contextops
docker compose up -d

# API:       http://localhost:8080
# Evaluator: http://localhost:8081
# UI:        http://localhost:3000
```

---

## CLI Reference

```text
contextops [command]

Available Commands:
  init        Initialize a ContextOps project
  server      Run local ContextOps services
  trace       Ingest, validate, and inspect traces
  manifest    Inspect context manifests
  eval        Run evaluators on runs or datasets
  benchmark   Execute benchmark suites
  compare     Compare runs, datasets, or benchmarks
  gate        Enforce quality and safety thresholds
  report      Generate reports
  adapter     Scaffold and validate adapters
  dataset     Manage evaluation datasets
  config      View and update configuration
```

### Key Commands

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
contextops benchmark list
contextops benchmark run enterprise-search
contextops benchmark run workflow-agent --dataset ./datasets/workflows/

# Compare
contextops compare <run-a> <run-b>
contextops compare <run-a> <run-b> --metric correctness
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2

# Gates
contextops gate check --config .contextops/gates.yaml
contextops gate check --benchmark enterprise-search

# Reports
contextops report generate <run-id>
contextops report generate <run-id> --format markdown
contextops report generate benchmark enterprise-search --format html
```

---

## Run Schema

```json
{
  "tenant_id": "tenant-acme",
  "run_id": "run-001",
  "user": {
    "id": "u-123",
    "role": "employee"
  },
  "agent": {
    "id": "policy-assistant",
    "version": "0.1.0"
  },
  "input": {
    "query": "What is the latest PTO policy?"
  },
  "retrieval": {
    "candidates": [
      { "doc_id": "doc-old-pto", "title": "PTO Policy 2024", "score": 0.83 },
      { "doc_id": "doc-new-pto", "title": "PTO Policy 2025", "score": 0.79 }
    ],
    "selected": ["doc-old-pto"]
  },
  "memory": {
    "candidates": [
      { "memory_id": "mem-01", "type": "episodic", "summary": "User asked about PTO policy last year" }
    ],
    "selected": ["mem-01"]
  },
  "permissions": {
    "acl_filters": [
      { "doc_id": "doc-new-pto", "result": "filtered", "reason": "group-mapping-mismatch" }
    ]
  },
  "tools": [
    { "name": "search_hr_docs", "status": "success" }
  ],
  "output": {
    "final_answer": "The PTO policy allows 15 days...",
    "citations": ["doc-old-pto"]
  },
  "metrics": {
    "latency_ms": 1820,
    "prompt_tokens": 2400,
    "completion_tokens": 380
  }
}
```

---

## Evaluators

ContextOps ships with these evaluator categories:

- **correctness** — Is the final answer right?
- **groundedness** — Is the answer supported by evidence?
- **retrieval_precision** — Were retrieved docs relevant?
- **retrieval_recall** — Were the best docs found?
- **citation_precision** — Are citations accurate?
- **citation_coverage** — Are all claims cited?
- **memory_relevance** — Was selected memory useful?
- **memory_staleness** — Was stale memory avoided?
- **permission_safety** — Was restricted content excluded?
- **trajectory_quality** — Was the reasoning path efficient?
- **tool_argument_correctness** — Were tool calls correct?
- **task_completion** — Did the agent finish the job?
- **latency_efficiency** — Was response time acceptable?
- **cost_efficiency** — Was token spend reasonable?

Each evaluator produces: numeric score, pass/fail status, explanation, supporting evidence, and remediation hints.

---

## Benchmark Packs

### Enterprise Search
- Latest policy retrieval, role-sensitive answers, doc freshness, citation correctness

### Memory Assistant
- Long-lived preferences, stale memory suppression, conflict resolution, personalization quality

### Workflow Agent
- Tool sequencing, approvals, side effects, completion accuracy

### Document Copilot
- Clause lookup, summarization grounding, extraction correctness, access control

---

## Gates

Use ContextOps in CI/CD to block bad releases.

**Example `.contextops/gates.yaml`:**

```yaml
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

## Adapters

ContextOps supports multiple integration patterns:

- **Direct JSON trace ingestion** — POST traces from any system
- **REST callback adapter** — webhook-based integration
- **OpenTelemetry-compatible spans** — OTEL collector support
- **Framework-specific adapters** — LangGraph, OpenAI Agents SDK
- **Batch evaluation from datasets** — bulk offline evaluation

---

## Project Structure

```
contextops/
├── apps/
│   ├── api/                # Go REST API service
│   ├── cli/                # contextops CLI (Go, Cobra)
│   ├── evaluator/          # Python evaluation engine
│   └── web/                # Next.js debugger UI
├── packages/
│   ├── trace-schema/       # Shared run schema definition
│   ├── evaluator-core/     # Evaluation interfaces
│   ├── context-manifest/   # Context assembly spec
│   ├── benchmark-core/     # Datasets, runners, scoring
│   ├── policy-core/        # ACL / permissions checks
│   └── adapter-sdk/        # Instrumentation SDK
├── adapters/
│   ├── rest/
│   ├── langgraph/
│   ├── openai-agents/
│   └── otel/
├── benchmarks/
│   ├── enterprise-search/
│   ├── document-copilot/
│   ├── memory-assistant/
│   └── workflow-agent/
├── docs/
│   ├── architecture/
│   ├── cli/
│   ├── evaluators/
│   └── benchmarks/
├── deploy/
│   ├── docker/
│   └── k8s/
├── docker-compose.yml
└── Makefile
```

## Tech Stack

- **API + CLI**: Go (Gin, Cobra)
- **Evaluator**: Python (FastAPI)
- **UI**: Next.js + TypeScript + Tailwind CSS
- **Database**: PostgreSQL + pgvector
- **Queue**: Redis Streams
- **Auth**: JWT with tenant isolation

---

## Use Cases

### For Platform Teams
- Benchmark agent systems before release
- Compare retrievers, rerankers, models, prompts
- Validate permission safety

### For AI Product Teams
- Debug poor answers faster
- See why context assembly failed
- Catch regressions early

### For Enterprises
- Verify AI behavior under real access controls
- Test role-based quality degradation
- Measure trustworthiness across departments

### For Open-Source Builders
- Add evals, traces, and benchmarks to agent repos
- Expose meaningful metrics in CI

---

## Roadmap

### v0.1
- Trace ingestion and run viewer
- Basic evaluators and CLI
- Compare runs
- Local Docker environment

### v0.2
- Context manifests and benchmark packs
- Gate checks and REST adapter
- Evaluator SDK

### v0.3
- Memory evaluators and permission simulators
- Workflow trajectory evaluators
- Dataset management

### v0.4
- OTEL support
- LangGraph and OpenAI Agents adapters
- Advanced dashboards

---

## Contributing

We welcome contributions in: evaluators, adapters, benchmark datasets, memory diagnostics, permissions test harnesses, docs and examples.

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## Design Principles

- Vendor-neutral
- Framework-agnostic
- Debuggable by default
- CI-friendly
- Enterprise-safe
- Open and extensible

---

## Vision

ContextOps aims to become the standard open layer for inspecting AI context assembly, evaluating agents and workflows, preventing regressions, and improving trust in enterprise AI systems.

If observability changed cloud systems, ContextOps can do the same for AI systems.

---

## License

Apache-2.0
