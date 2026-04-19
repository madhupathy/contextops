# Integrating ContextOps with Your AI Agent

This guide covers every integration pattern from the simplest (post a JSON after each run) to the fully automated (instrument your framework and evaluate in CI).

---

## The core idea in one sentence

ContextOps needs to know what happened during a run — what was retrieved, what memory was used, what tools were called, and what the agent answered. You send that as a single JSON object. Everything else is optional.

---

## Step 1: Start the services

```bash
git clone https://github.com/madhupathy/contextops.git
cd contextops
cp .env.example .env          # add OPENAI_API_KEY if you want LLM-as-judge (optional)
docker compose up -d
```

Services after startup:
- API: `http://localhost:8080`
- Evaluator: `http://localhost:8081`
- Dashboard: `http://localhost:3000`

---

## Step 2: Understand the trace format

A trace is one JSON object representing a single agent run. Only two fields are required:

```json
{
  "run_id": "unique-id",
  "input": { "query": "the user's question" }
}
```

Everything else is optional — add what you have:

```json
{
  "run_id": "run-20250419-001",
  "agent":  { "id": "my-agent", "version": "0.1.0" },
  "user":   { "id": "user-123", "role": "employee", "groups": ["engineering"] },
  "input":  { "query": "What is the latest PTO policy?" },

  "retrieval": {
    "candidates": [
      {
        "doc_id": "doc-pto-2025",
        "title": "PTO Policy 2025",
        "score": 0.91,
        "selected": true,
        "acl_passed": true,
        "content_preview": "Employees receive 20 days PTO per year..."
      }
    ]
  },

  "memory": {
    "candidates": [
      {
        "memory_id": "mem-001",
        "memory_type": "episodic",
        "content": "User asked about PTO in December 2024. Policy was 15 days.",
        "relevance_score": 0.72,
        "is_stale": true,
        "stale_reason": "Policy updated January 2025",
        "selected": false
      }
    ]
  },

  "tools": [
    { "name": "search_hr_docs", "args": {"query": "PTO policy"}, "status": "success", "latency_ms": 180 }
  ],

  "reasoning_steps": [
    { "step": 1, "type": "think",    "content": "User wants the current PTO policy" },
    { "step": 2, "type": "retrieve", "content": "Searching HR document store" },
    { "step": 3, "type": "generate", "content": "Composing answer from retrieved docs" }
  ],

  "output": {
    "final_answer": "The 2025 PTO policy allows 20 days per year with unlimited carry-over.",
    "citations": ["doc-pto-2025"]
  },

  "expected_answer": "20 days per year, unlimited carry-over.",
  "expected_sources": ["doc-pto-2025"],
  "metrics": {
    "latency_ms": 1240,
    "prompt_tokens": 1800,
    "completion_tokens": 220,
    "total_tokens": 2020,
    "estimated_cost": 0.016
  }
}
```

**What each section enables:**

| Section | Evaluators unlocked |
|---------|-------------------|
| `retrieval.candidates` | `retrieval_quality`, `permission_safety`, `citation_precision`, `groundedness`, `context_poisoning` |
| `memory.candidates` | `memory_utility`, `context_poisoning`, `session_coherence` |
| `tools` | `tool_correctness`, `task_completion`, `trajectory_quality` |
| `reasoning_steps` | `trajectory_quality`, `context_poisoning` |
| `expected_answer` | `answer_correctness` |
| `expected_sources` | `retrieval_quality`, `citation_precision` |
| `metrics` | `cost_efficiency`, `agent_regression` |

---

## Step 3: Choose your integration pattern

### Pattern A — Direct HTTP (any language, any framework)

Post the trace after each run:

```python
import httpx, uuid, time

def run_agent_and_trace(query: str, user_id: str) -> str:
    trace = {
        "run_id": str(uuid.uuid4()),
        "agent": {"id": "my-agent", "version": "0.1.0"},
        "user": {"id": user_id},
        "input": {"query": query},
    }

    t0 = time.monotonic()

    # --- your agent logic here ---
    retrieval_candidates = my_retriever.retrieve(query)
    memory_candidates    = my_memory.recall(query, user_id)
    context              = build_context(retrieval_candidates, memory_candidates)
    answer, tokens       = my_llm.complete(context, query)
    # --- end agent logic ---

    elapsed_ms = int((time.monotonic() - t0) * 1000)

    trace["retrieval"] = {
        "candidates": [
            {
                "doc_id":          doc.id,
                "title":           doc.title,
                "score":           doc.score,
                "selected":        doc.selected,
                "acl_passed":      doc.acl_passed,
                "acl_reason":      doc.acl_reason,
                "content_preview": doc.content[:200],
            }
            for doc in retrieval_candidates
        ]
    }
    trace["memory"] = {
        "candidates": [
            {
                "memory_id":       mem.id,
                "memory_type":     mem.type,
                "content":         mem.content,
                "relevance_score": mem.score,
                "is_stale":        mem.is_stale,
                "stale_reason":    mem.stale_reason,
                "selected":        mem.selected,
            }
            for mem in memory_candidates
        ]
    }
    trace["output"]  = {"final_answer": answer, "citations": [d.id for d in retrieval_candidates if d.selected]}
    trace["metrics"] = {"latency_ms": elapsed_ms, "total_tokens": tokens}

    # Send to ContextOps (fire-and-forget — never block user response)
    try:
        httpx.post("http://localhost:8080/api/v1/runs", json=trace, timeout=5.0)
    except Exception:
        pass

    return answer
```

---

### Pattern B — LangChain / LangGraph callback

Add one callback handler to your existing chain — no changes to the chain itself:

```python
from langchain_core.callbacks import BaseCallbackHandler
import httpx, uuid, time

class ContextOpsCallback(BaseCallbackHandler):
    """Drop-in LangChain callback. Pass to any chain or graph as callbacks=[handler]."""

    def __init__(self, agent_id: str, api_url: str = "http://localhost:8080"):
        self.agent_id = agent_id
        self.api_url  = api_url
        self._traces: dict[str, dict] = {}

    def on_chain_start(self, serialized, inputs, run_id, **kwargs):
        self._traces[str(run_id)] = {
            "run_id": str(run_id),
            "agent":  {"id": self.agent_id},
            "input":  {"query": inputs.get("input", inputs.get("question", ""))},
            "retrieval": {"candidates": []},
            "memory":    {"candidates": []},
            "tools":     [],
            "reasoning_steps": [],
            "_t0": time.monotonic(),
        }

    def on_retriever_end(self, documents, run_id, parent_run_id, **kwargs):
        trace = self._traces.get(str(parent_run_id or run_id))
        if not trace:
            return
        for i, doc in enumerate(documents):
            trace["retrieval"]["candidates"].append({
                "doc_id":          doc.metadata.get("source", f"doc-{i}"),
                "title":           doc.metadata.get("title", ""),
                "score":           float(doc.metadata.get("score", 0.0)),
                "selected":        True,
                "acl_passed":      doc.metadata.get("acl_passed", True),
                "acl_reason":      doc.metadata.get("acl_reason"),
                "content_preview": doc.page_content[:200],
            })

    def on_tool_start(self, serialized, input_str, run_id, parent_run_id, **kwargs):
        trace = self._traces.get(str(parent_run_id or run_id))
        if trace:
            trace["tools"].append({
                "name":    serialized.get("name", "unknown"),
                "args":    {"input": input_str},
                "status":  "running",
                "_run_id": str(run_id),
                "_t":      time.monotonic(),
            })

    def on_tool_end(self, output, run_id, parent_run_id, **kwargs):
        trace = self._traces.get(str(parent_run_id or run_id))
        if not trace:
            return
        for tool in trace["tools"]:
            if tool.get("_run_id") == str(run_id):
                tool["status"]     = "success"
                tool["latency_ms"] = int((time.monotonic() - tool.pop("_t", time.monotonic())) * 1000)
                tool.pop("_run_id", None)

    def on_chain_end(self, outputs, run_id, **kwargs):
        trace = self._traces.pop(str(run_id), None)
        if not trace:
            return
        t0 = trace.pop("_t0", time.monotonic())
        trace["output"]  = {"final_answer": str(outputs.get("output", outputs.get("answer", "")))}
        trace["metrics"] = {"latency_ms": int((time.monotonic() - t0) * 1000)}
        try:
            httpx.post(f"{self.api_url}/api/v1/runs", json=trace, timeout=5.0)
        except Exception:
            pass


# Attach to any chain:
from langchain.chains import RetrievalQA

callback = ContextOpsCallback(agent_id="hr-assistant")
chain    = RetrievalQA.from_chain_type(llm=llm, retriever=retriever)
result   = chain.invoke({"query": "What is the PTO policy?"}, config={"callbacks": [callback]})
```

---

### Pattern C — OpenAI Agents SDK

```python
import contextlib, httpx, uuid, time
from openai import OpenAI

client = OpenAI()

@contextlib.contextmanager
def contextops_trace(agent_id: str, query: str, user_id: str = ""):
    run_id = str(uuid.uuid4())
    trace  = {
        "run_id": run_id,
        "agent":  {"id": agent_id},
        "user":   {"id": user_id},
        "input":  {"query": query},
        "_t0":    time.monotonic(),
    }
    try:
        yield trace
    finally:
        t0 = trace.pop("_t0")
        trace.setdefault("metrics", {})["latency_ms"] = int((time.monotonic() - t0) * 1000)
        try:
            httpx.post("http://localhost:8080/api/v1/runs", json=trace, timeout=5.0)
        except Exception:
            pass


# Usage:
def run_agent(query: str) -> str:
    with contextops_trace("my-agent", query) as trace:
        response = client.responses.create(model="gpt-4o", input=query)

        answer = response.output_text
        trace["output"]  = {"final_answer": answer}
        trace["metrics"] = {"total_tokens": response.usage.total_tokens}

        # Capture file citations if using file_search tool
        for item in response.output:
            if hasattr(item, "file_citation"):
                trace.setdefault("retrieval", {"candidates": []})
                trace["retrieval"]["candidates"].append({
                    "doc_id":   item.file_citation.file_id,
                    "selected": True,
                    "acl_passed": True,
                })

        return answer
```

---

### Pattern D — Absolute minimum (any framework)

```python
import httpx, uuid

httpx.post("http://localhost:8080/api/v1/runs", json={
    "run_id":          str(uuid.uuid4()),
    "agent":           {"id": "my-agent"},
    "input":           {"query": user_query},
    "output":          {"final_answer": agent_answer},
    "expected_answer": ground_truth,      # optional — enables answer_correctness
    "metrics": {
        "latency_ms":    latency_ms,
        "total_tokens":  token_count,
        "estimated_cost": cost,
    },
})
```

Unlocks: `answer_correctness`, `task_completion`, `response_completeness`, `hallucination_risk`, `cost_efficiency`.

---

## Step 4: Trigger evaluation

```bash
# All 15 evaluators
curl -X POST http://localhost:8080/api/v1/runs/YOUR_RUN_ID/evaluate

# Focused profile (faster)
curl "http://localhost:8081/evaluate/profile?run_id=YOUR_RUN_ID&profile=rag"

# Via CLI
contextops eval run YOUR_RUN_ID
```

Profiles: `rag` · `agent` · `memory` · `enterprise` · `production` · `full`

---

## Step 5: View results

```bash
contextops eval run YOUR_RUN_ID --explain
```

```
retrieval_quality    0.41  FAIL  Better document blocked by ACL misconfiguration
permission_safety    0.00  FAIL  CRITICAL: ACL blocked correct document
memory_utility       0.28  FAIL  Stale memory selected (policy updated Jan 2025)
context_poisoning    0.20  FAIL  Stale value "15 days" in answer, not in current docs
answer_correctness   0.91  PASS
groundedness         0.88  PASS
cost_efficiency      0.82  PASS
```

Dashboard: `http://localhost:3000` → Runs → click any run → four tabs: Evaluations, Retrieval, Memory, Trace.

---

## Step 6: Add a CI gate

```yaml
# .contextops/gates.yaml
minimum_scores:
  answer_correctness: 0.85
  groundedness:       0.80
  permission_safety:  1.00   # zero tolerance
  task_completion:    0.80

maximum_thresholds:
  latency_ms_p95:    5000
  cost_per_run_usd:  0.10
```

```yaml
# .github/workflows/quality.yml
- run: |
    contextops benchmark run enterprise-search
    contextops gate check --config .contextops/gates.yaml
```

Exit code 1 if any gate fails.

---

## What to capture and why

| You have this | Capture it as | Evaluators enabled |
|---------------|---------------|-------------------|
| Vector search results | `retrieval.candidates[].score` + `selected` | `retrieval_quality` — detects ranking failures |
| ACL/permission filters | `retrieval.candidates[].acl_passed` + `acl_reason` | `permission_safety` — #1 enterprise concern |
| Memory/context injected | `memory.candidates[]` with `is_stale` | `context_poisoning` — stale values in answers |
| Tool calls made | `tools[]` with `name`, `args`, `status` | `tool_correctness` — wrong tool, missing action |
| Chain-of-thought | `reasoning_steps[]` | `trajectory_quality` — loops, inefficiency |
| Expected answer | `expected_answer` | `answer_correctness` — factual accuracy |
| Expected sources | `expected_sources` | `retrieval_quality` — recall measurement |
| Token counts | `metrics.total_tokens` | `cost_efficiency`, `agent_regression` |

---

## Multi-session agents

For agents that remember context across turns:

```python
trace = {
    "run_id": str(uuid.uuid4()),
    "input":  {"query": current_query},
    "output": {"final_answer": answer},
    "context_manifest": {
        "session_id":    "session-abc-123",
        "session_turns": 3,
        # For regression detection against previous runs
        "baseline_scores": {
            "answer_correctness": 0.88,
            "groundedness":       0.91,
            "latency_ms":         1800,
        }
    }
}
```

The `session_coherence` evaluator uses prior-turn memories in `memory.candidates` to check preferences and entity consistency across turns.

---

## Annotating runs as golden test cases

```bash
# Mark correct, add to a dataset
curl -X POST http://localhost:8080/api/v1/runs/RUN_ID/annotate \
  -d '{"annotation": "correct", "add_to_dataset_id": "YOUR_DATASET_ID"}'

# Mark incorrect, capture the right answer
curl -X POST http://localhost:8080/api/v1/runs/RUN_ID/annotate \
  -d '{
    "annotation": "incorrect",
    "correct_answer": "The 2025 policy allows 20 days, not 15.",
    "add_to_dataset_id": "YOUR_DATASET_ID"
  }'
```

This grows a golden evaluation dataset from production traffic.

---

## Monitoring quality drift

```bash
curl "http://localhost:8080/api/v1/drift?agent_id=my-agent&window_hours=24"
```

```json
{
  "degrading": 1,
  "drift": [
    { "category": "answer_correctness", "current": 0.71, "previous": 0.88, "delta": -0.17, "trend": "degrading" },
    { "category": "groundedness",        "current": 0.85, "previous": 0.84, "delta":  0.01, "trend": "stable"    },
    { "category": "cost_efficiency",     "current": 0.90, "previous": 0.82, "delta":  0.08, "trend": "improving" }
  ]
}
```

---

## Minimum viable integration checklist

- [ ] ContextOps running (`docker compose up -d`)
- [ ] Trace sent after each run (`POST /api/v1/runs`)
- [ ] `retrieval.candidates` with `score`, `selected`, `acl_passed`
- [ ] `output.final_answer` populated
- [ ] `metrics.total_tokens` and `metrics.latency_ms` populated
- [ ] Evaluation triggered (`POST /runs/:id/evaluate`)
- [ ] Gate config created (`.contextops/gates.yaml`)
- [ ] Gate check in CI (`contextops gate check`)
