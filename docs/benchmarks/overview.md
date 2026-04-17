# ContextOps Benchmarks

## Overview

Benchmarks are reusable scenario packs that test specific AI system capabilities. Each benchmark contains test cases with known inputs, expected outputs, and pass criteria.

## Built-in Benchmark Packs

### Enterprise Search (`benchmarks/enterprise-search/`)
Tests for enterprise search and policy retrieval systems.
- Latest policy retrieval
- Role-sensitive answers
- Document freshness
- Citation correctness

### Document Copilot (`benchmarks/document-copilot/`)
Tests for document analysis and copilot systems.
- Clause lookup
- Summarization grounding
- Extraction correctness
- Access control

### Memory Assistant (`benchmarks/memory-assistant/`)
Tests for memory-heavy AI assistants.
- Long-lived preferences
- Stale memory suppression
- Conflict resolution
- Personalization quality

### Workflow Agent (`benchmarks/workflow-agent/`)
Tests for tool-using workflow agents.
- Tool sequencing
- Approval workflows
- Side effect detection
- Completion accuracy

## Benchmark Case Format

Each case is a JSON file in `benchmarks/<suite>/cases/`:

```json
{
  "name": "case-name",
  "description": "What this test checks",
  "query": "The input query",
  "simulate_user_roles": ["employee"],
  "simulate_user_groups": ["engineering"],
  "expected_answer": "The correct answer",
  "expected_sources": ["doc-id"],
  "expected_tools": ["tool-name"],
  "forbidden_sources": ["restricted-doc"],
  "forbidden_content": ["sensitive phrase"],
  "pass_criteria": {
    "correctness": 0.90,
    "permission_safety": 1.00
  },
  "seed_documents": [...],
  "seed_memories": [...]
}
```

## Running Benchmarks

```bash
# List available suites
contextops benchmark list

# Run a benchmark
contextops benchmark run enterprise-search

# Run with custom dataset
contextops benchmark run workflow-agent --dataset ./my-cases/
```

## Creating Custom Benchmarks

1. Create a directory under `benchmarks/<name>/`
2. Add `README.md` describing the benchmark
3. Add test case JSON files in `benchmarks/<name>/cases/`
4. Each case must include: `query`, `expected_answer`, and `pass_criteria`

## Regression Testing

Compare benchmark results across versions:

```bash
contextops compare benchmark enterprise-search --baseline v0.1 --candidate v0.2
```

This shows score deltas per category, highlighting regressions in red.
