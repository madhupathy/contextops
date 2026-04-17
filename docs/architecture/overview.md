# ContextOps Architecture

## Overview

ContextOps is a multi-service platform for evaluating, debugging, and regression-testing AI agents.

```text
                           +----------------------+
                           |   Agent / RAG App    |
                           +----------+-----------+
                                      |
                                      | traces / callbacks
                                      v
+----------------+       +--------------------------+       +------------------+
| Adapters       | ----> |    ContextOps API        | ----> |   PostgreSQL     |
|                |       |    (Go / Gin)            |       |   + pgvector     |
+----------------+       +------------+-------------+       +------------------+
                                      |
                                      v
                         +-----------------------------+
                         | Evaluation Engine            |
                         | (Python / FastAPI)           |
                         +-------------+---------------+
                                       |
                                       v
                         +-----------------------------+
                         | Debug UI / CLI / CI Gates   |
                         +-----------------------------+
```

## Services

### API Service (`apps/api/`)
- **Language**: Go
- **Framework**: Gin
- **Port**: 8080
- **Responsibilities**: Trace ingestion, run management, evaluation orchestration, benchmark execution, audit logging
- **Database**: PostgreSQL with pgvector for embeddings

### Evaluation Engine (`apps/evaluator/`)
- **Language**: Python
- **Framework**: FastAPI
- **Port**: 8081
- **Responsibilities**: Pluggable evaluator execution, scoring, LLM-as-judge support
- **Evaluators**: 14 categories (correctness, groundedness, retrieval, memory, permissions, tools, trajectory, cost)

### CLI (`apps/cli/`)
- **Language**: Go
- **Framework**: Cobra
- **Responsibilities**: Project init, trace ingestion/inspection, evaluation, comparison, gate checks, reporting

### Debug UI (`apps/web/`)
- **Language**: TypeScript
- **Framework**: Next.js + Tailwind CSS
- **Port**: 3000
- **Responsibilities**: Run timeline viewer, retrieval inspector, memory inspector, evaluation dashboards

## Data Flow

1. Agent runs are captured as **traces** via adapters or direct JSON ingestion
2. Traces are stored in PostgreSQL with full audit trail and tenant isolation
3. **Evaluators** score traces across multiple categories
4. Results are viewable via **UI**, **CLI**, or **API**
5. **Gates** enforce quality thresholds in CI/CD pipelines
6. **Benchmarks** provide repeatable scenario packs for regression testing

## Tenant Isolation

All data is scoped by `tenant_id`. The API uses an `X-Tenant-ID` header for routing.

## Key Design Decisions

- **Pluggable evaluators**: Each evaluator is independent and can be enabled/disabled per tenant
- **Vendor-neutral**: No dependency on specific LLM providers or agent frameworks
- **CI-friendly**: Gate checks return non-zero exit codes on failure
- **Schema-first**: Canonical trace schema in `packages/trace-schema/`
