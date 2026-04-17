"""Cost efficiency evaluator.

Evaluates whether the agent completed the task within reasonable
token budget and latency constraints.
"""

from __future__ import annotations

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class CostEfficiencyEvaluator(BaseEvaluator):
    name = "cost_efficiency"
    version = "v1"
    category = "cost_efficiency"

    # Configurable thresholds
    MAX_TOKENS_GOOD = 10000
    MAX_TOKENS_ACCEPTABLE = 50000
    MAX_LATENCY_GOOD_MS = 5000
    MAX_LATENCY_ACCEPTABLE_MS = 30000
    MAX_COST_GOOD = 0.01
    MAX_COST_ACCEPTABLE = 0.10

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run

        total_tokens = run.get("total_tokens", 0) or 0
        latency_ms = run.get("latency_ms", 0) or 0
        estimated_cost = run.get("estimated_cost", 0) or 0
        prompt_tokens = run.get("prompt_tokens", 0) or 0
        completion_tokens = run.get("completion_tokens", 0) or 0

        # Token efficiency
        if total_tokens <= self.MAX_TOKENS_GOOD:
            token_score = 1.0
        elif total_tokens <= self.MAX_TOKENS_ACCEPTABLE:
            token_score = 1.0 - ((total_tokens - self.MAX_TOKENS_GOOD) / (self.MAX_TOKENS_ACCEPTABLE - self.MAX_TOKENS_GOOD)) * 0.5
        else:
            token_score = max(0.0, 0.5 - ((total_tokens - self.MAX_TOKENS_ACCEPTABLE) / self.MAX_TOKENS_ACCEPTABLE) * 0.5)

        # Latency efficiency
        if latency_ms <= self.MAX_LATENCY_GOOD_MS:
            latency_score = 1.0
        elif latency_ms <= self.MAX_LATENCY_ACCEPTABLE_MS:
            latency_score = 1.0 - ((latency_ms - self.MAX_LATENCY_GOOD_MS) / (self.MAX_LATENCY_ACCEPTABLE_MS - self.MAX_LATENCY_GOOD_MS)) * 0.5
        else:
            latency_score = max(0.0, 0.5 - ((latency_ms - self.MAX_LATENCY_ACCEPTABLE_MS) / self.MAX_LATENCY_ACCEPTABLE_MS) * 0.5)

        # Cost efficiency
        if estimated_cost <= self.MAX_COST_GOOD:
            cost_score = 1.0
        elif estimated_cost <= self.MAX_COST_ACCEPTABLE:
            cost_score = 1.0 - ((estimated_cost - self.MAX_COST_GOOD) / (self.MAX_COST_ACCEPTABLE - self.MAX_COST_GOOD)) * 0.5
        else:
            cost_score = max(0.0, 0.5 - ((estimated_cost - self.MAX_COST_ACCEPTABLE) / self.MAX_COST_ACCEPTABLE) * 0.5)

        # Prompt/completion ratio (ideally completion << prompt for grounded answers)
        ratio_score = 1.0
        if prompt_tokens > 0 and completion_tokens > prompt_tokens * 2:
            ratio_score = 0.6  # Generating too much relative to context

        # Combined score
        score = token_score * 0.30 + latency_score * 0.30 + cost_score * 0.25 + ratio_score * 0.15
        passed = score >= 0.5

        details = {
            "total_tokens": total_tokens,
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "latency_ms": latency_ms,
            "estimated_cost": estimated_cost,
            "metrics": {
                "token_score": round(token_score, 3),
                "latency_score": round(latency_score, 3),
                "cost_score": round(cost_score, 3),
                "ratio_score": round(ratio_score, 3),
            },
            "thresholds": {
                "tokens_good": self.MAX_TOKENS_GOOD,
                "tokens_acceptable": self.MAX_TOKENS_ACCEPTABLE,
                "latency_good_ms": self.MAX_LATENCY_GOOD_MS,
                "latency_acceptable_ms": self.MAX_LATENCY_ACCEPTABLE_MS,
                "cost_good": self.MAX_COST_GOOD,
                "cost_acceptable": self.MAX_COST_ACCEPTABLE,
            },
        }

        reasoning_parts = []
        if total_tokens > self.MAX_TOKENS_ACCEPTABLE:
            reasoning_parts.append(f"Token usage ({total_tokens}) exceeds acceptable threshold")
        if latency_ms > self.MAX_LATENCY_ACCEPTABLE_MS:
            reasoning_parts.append(f"Latency ({latency_ms}ms) exceeds acceptable threshold")
        if estimated_cost > self.MAX_COST_ACCEPTABLE:
            reasoning_parts.append(f"Cost (${estimated_cost:.4f}) exceeds acceptable threshold")
        if ratio_score < 1.0:
            reasoning_parts.append("Completion tokens are disproportionately high vs prompt tokens")
        if not reasoning_parts:
            reasoning_parts.append("Cost and latency are within acceptable bounds")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )
