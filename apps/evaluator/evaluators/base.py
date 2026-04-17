"""Base evaluator interface."""

from __future__ import annotations

import abc
import time
from typing import Any

from models import EvaluationResult, RunData


class BaseEvaluator(abc.ABC):
    """Abstract base class for all evaluators."""

    name: str = "base"
    version: str = "v1"
    category: str = "unknown"

    @abc.abstractmethod
    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        """Evaluate a run and return a result."""
        ...

    def _result(
        self,
        score: float,
        passed: bool,
        details: dict[str, Any] | None = None,
        reasoning: str | None = None,
        model_used: str | None = None,
        eval_tokens: int = 0,
        elapsed_ms: int = 0,
    ) -> EvaluationResult:
        return EvaluationResult(
            evaluator_name=self.name,
            evaluator_version=self.version,
            category=self.category,
            score=max(0.0, min(1.0, score)),
            passed=passed,
            details=details or {},
            reasoning=reasoning,
            model_used=model_used,
            eval_tokens=eval_tokens,
            eval_latency_ms=elapsed_ms,
        )

    @staticmethod
    def _timer() -> float:
        return time.monotonic()

    @staticmethod
    def _elapsed_ms(start: float) -> int:
        return int((time.monotonic() - start) * 1000)
