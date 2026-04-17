"""Evaluator registry — central place to register and retrieve evaluators."""

from __future__ import annotations

from evaluators.base import BaseEvaluator
from evaluators.answer_correctness import AnswerCorrectnessEvaluator
from evaluators.groundedness import GroundednessEvaluator
from evaluators.retrieval import RetrievalQualityEvaluator
from evaluators.permission_safety import PermissionSafetyEvaluator
from evaluators.memory_utility import MemoryUtilityEvaluator
from evaluators.tool_correctness import ToolCorrectnessEvaluator
from evaluators.trajectory_quality import TrajectoryQualityEvaluator
from evaluators.cost_efficiency import CostEfficiencyEvaluator


class EvaluatorRegistry:
    """Registry of all available evaluators."""

    def __init__(self, openai_api_key: str = ""):
        self._evaluators: dict[str, BaseEvaluator] = {}
        self._register_defaults(openai_api_key)

    def _register_defaults(self, openai_api_key: str):
        self.register(AnswerCorrectnessEvaluator(openai_api_key=openai_api_key))
        self.register(GroundednessEvaluator(openai_api_key=openai_api_key))
        self.register(RetrievalQualityEvaluator())
        self.register(PermissionSafetyEvaluator())
        self.register(MemoryUtilityEvaluator())
        self.register(ToolCorrectnessEvaluator())
        self.register(TrajectoryQualityEvaluator())
        self.register(CostEfficiencyEvaluator())

    def register(self, evaluator: BaseEvaluator):
        self._evaluators[evaluator.category] = evaluator

    def get(self, category: str) -> BaseEvaluator | None:
        return self._evaluators.get(category)

    def list_evaluators(self) -> list[dict[str, str]]:
        return [
            {"name": e.name, "version": e.version, "category": e.category}
            for e in self._evaluators.values()
        ]

    def default_categories(self) -> list[str]:
        return list(self._evaluators.keys())
