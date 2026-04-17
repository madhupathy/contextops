# ContextOps Evaluators

## Overview

ContextOps ships with 14 pluggable evaluator categories. Each evaluator produces:

- **Numeric score** (0.0 – 1.0)
- **Pass/fail status** (based on configurable thresholds)
- **Explanation** (human-readable reasoning)
- **Supporting evidence** (specific data points)
- **Remediation hints** (actionable next steps)

## Categories

### Correctness Evaluators

| Evaluator | Description |
|-----------|-------------|
| `correctness` | Is the final answer factually correct? Compares against ground truth when available. |
| `groundedness` | Is the answer supported by retrieved evidence? Detects hallucinations. |

### Retrieval Evaluators

| Evaluator | Description |
|-----------|-------------|
| `retrieval_precision` | Were the retrieved documents relevant to the query? |
| `retrieval_recall` | Were the best available documents found? |
| `citation_precision` | Are citations accurate and pointing to correct sources? |
| `citation_coverage` | Are all claims in the answer backed by citations? |

### Memory Evaluators

| Evaluator | Description |
|-----------|-------------|
| `memory_relevance` | Was the selected memory useful for this query? |
| `memory_staleness` | Was stale or outdated memory correctly avoided? |

### Safety Evaluators

| Evaluator | Description |
|-----------|-------------|
| `permission_safety` | Was restricted/ACL-blocked content excluded from the answer? |

### Agent Evaluators

| Evaluator | Description |
|-----------|-------------|
| `trajectory_quality` | Was the reasoning path efficient without loops or waste? |
| `tool_argument_correctness` | Were tools called with correct names and arguments? |
| `task_completion` | Did the agent successfully complete the requested task? |

### Efficiency Evaluators

| Evaluator | Description |
|-----------|-------------|
| `latency_efficiency` | Was the response time within acceptable bounds? |
| `cost_efficiency` | Was the token spend reasonable for the task? |

## Adding a Custom Evaluator

1. Create a new file in `apps/evaluator/evaluators/`
2. Extend `BaseEvaluator`:

```python
from evaluators.base import BaseEvaluator
from models import RunData, EvaluationResult

class MyEvaluator(BaseEvaluator):
    name = "my_evaluator"
    version = "v1"
    category = "my_category"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        # Your evaluation logic
        score = 0.85
        return self.result(
            score=score,
            passed=score >= 0.7,
            reasoning="Explanation of the score",
            details={"key": "value"},
        )
```

3. Register in `evaluators/registry.py`:

```python
from evaluators.my_evaluator import MyEvaluator

class EvaluatorRegistry:
    def __init__(self, ...):
        self._register_defaults()
        self.register(MyEvaluator())
```

## Evaluation Modes

- **Heuristic**: Rule-based scoring using keyword matching, overlap, etc.
- **LLM-as-Judge**: Uses an LLM to semantically evaluate (requires `OPENAI_API_KEY`)
- **Hybrid**: Heuristic first, LLM fallback for ambiguous cases

Set via environment variable `OPENAI_API_KEY`. If not set, evaluators fall back to heuristic mode.
