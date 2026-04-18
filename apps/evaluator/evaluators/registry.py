"""Evaluator registry — central place to register and retrieve evaluators.

Registered evaluators (12 total):
Core quality:
  answer_correctness  - Is the final answer factually correct?
  groundedness        - Is every claim supported by retrieved evidence?
  citation_precision  - Are citations accurate and not hallucinated?
  task_completion     - Did the agent actually complete the user's intent?

Retrieval pipeline:
  retrieval_quality   - Were the right docs retrieved in the right order?
  permission_safety   - Did any ACL-blocked content leak into the answer?

Memory & context:
  memory_utility      - Was memory helpful, fresh, and correctly selected?
  context_poisoning   - Did stale context contaminate the answer?
  session_coherence   - Was context maintained coherently across turns?

Agent behaviour:
  tool_correctness    - Were tool calls correct (name, args, ordering)?
  trajectory_quality  - Was the reasoning path efficient and loop-free?

Cost & performance:
  cost_efficiency     - Was token spend and latency within acceptable bounds?
"""

from __future__ import annotations

from evaluators.base import BaseEvaluator
from evaluators.answer_correctness import AnswerCorrectnessEvaluator
from evaluators.citation_precision import CitationPrecisionEvaluator
from evaluators.context_poisoning import ContextPoisoningEvaluator
from evaluators.cost_efficiency import CostEfficiencyEvaluator
from evaluators.groundedness import GroundednessEvaluator
from evaluators.memory_utility import MemoryUtilityEvaluator
from evaluators.permission_safety import PermissionSafetyEvaluator
from evaluators.retrieval import RetrievalQualityEvaluator
from evaluators.session_coherence import SessionCoherenceEvaluator
from evaluators.task_completion import TaskCompletionEvaluator
from evaluators.tool_correctness import ToolCorrectnessEvaluator
from evaluators.trajectory_quality import TrajectoryQualityEvaluator


class EvaluatorRegistry:
    """Registry of all available evaluators."""

    def __init__(self, openai_api_key: str = ""):
        self._evaluators: dict[str, BaseEvaluator] = {}
        self._register_defaults(openai_api_key)

    def _register_defaults(self, openai_api_key: str):
        # Core quality
        self.register(AnswerCorrectnessEvaluator(openai_api_key=openai_api_key))
        self.register(GroundednessEvaluator(openai_api_key=openai_api_key))
        self.register(CitationPrecisionEvaluator())
        self.register(TaskCompletionEvaluator())
        # Retrieval pipeline
        self.register(RetrievalQualityEvaluator())
        self.register(PermissionSafetyEvaluator())
        # Memory & context
        self.register(MemoryUtilityEvaluator())
        self.register(ContextPoisoningEvaluator())
        self.register(SessionCoherenceEvaluator())
        # Agent behaviour
        self.register(ToolCorrectnessEvaluator())
        self.register(TrajectoryQualityEvaluator())
        # Cost & performance
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

    def categories_for_intent(self, intent: str) -> list[str]:
        """Return the most relevant evaluator categories for a given use case."""
        profiles = {
            "rag": [
                "retrieval_quality", "groundedness", "citation_precision",
                "permission_safety", "answer_correctness", "context_poisoning",
            ],
            "agent": [
                "tool_correctness", "trajectory_quality", "task_completion",
                "answer_correctness", "cost_efficiency",
            ],
            "memory": [
                "memory_utility", "session_coherence", "context_poisoning",
                "groundedness",
            ],
            "enterprise": [
                "permission_safety", "citation_precision", "groundedness",
                "answer_correctness", "context_poisoning", "session_coherence",
            ],
            "full": list(self._evaluators.keys()),
        }
        return profiles.get(intent, list(self._evaluators.keys()))
