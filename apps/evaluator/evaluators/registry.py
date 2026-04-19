"""Evaluator registry — central place to register and retrieve evaluators.

Registered evaluators (17 total):
Core quality:
  answer_correctness      - Is the final answer factually correct?
  groundedness            - Is every claim supported by retrieved evidence?
  citation_precision      - Are citations accurate and not hallucinated?
  task_completion         - Did the agent actually complete the user's intent?
  response_completeness   - Were ALL parts of a multi-part query answered?
  hallucination_risk      - Do specific claim types signal likely hallucination?

Retrieval pipeline:
  retrieval_quality       - Were the right docs retrieved in the right order?
  permission_safety       - Did any ACL-blocked content leak into the answer?

Memory & context:
  memory_utility          - Was memory helpful, fresh, and correctly selected?
  context_poisoning       - Did stale context contaminate the answer?
  session_coherence       - Was context maintained coherently across turns?

Agent behaviour:
  tool_correctness        - Were tool calls correct (name, args, ordering)?
  trajectory_quality      - Was the reasoning path efficient and loop-free?

Cost & performance:
  cost_efficiency         - Was token spend and latency within acceptable bounds?

Production:
  agent_regression        - Did quality degrade vs a recorded baseline run?

Autonomous agents:
  plan_adherence          - Did the agent follow the plan it created (plan-and-execute)?
  agent_handoff_quality   - Was context passed correctly between agents (multi-agent)?
"""

from __future__ import annotations

from evaluators.base import BaseEvaluator
from evaluators.agent_handoff_quality import AgentHandoffQualityEvaluator
from evaluators.agent_regression import AgentRegressionEvaluator
from evaluators.plan_adherence import PlanAdherenceEvaluator
from evaluators.answer_correctness import AnswerCorrectnessEvaluator
from evaluators.citation_precision import CitationPrecisionEvaluator
from evaluators.context_poisoning import ContextPoisoningEvaluator
from evaluators.cost_efficiency import CostEfficiencyEvaluator
from evaluators.groundedness import GroundednessEvaluator
from evaluators.hallucination_risk import HallucinationRiskEvaluator
from evaluators.memory_utility import MemoryUtilityEvaluator
from evaluators.permission_safety import PermissionSafetyEvaluator
from evaluators.response_completeness import ResponseCompletenessEvaluator
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
        self.register(ResponseCompletenessEvaluator())
        self.register(HallucinationRiskEvaluator())
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
        # Production
        self.register(AgentRegressionEvaluator())
        # Autonomous agents
        self.register(PlanAdherenceEvaluator())
        self.register(AgentHandoffQualityEvaluator())

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
        profiles: dict[str, list[str]] = {
            "rag": [
                "retrieval_quality", "groundedness", "citation_precision",
                "permission_safety", "answer_correctness", "context_poisoning",
                "hallucination_risk",
            ],
            "agent": [
                "tool_correctness", "trajectory_quality", "task_completion",
                "response_completeness", "answer_correctness", "cost_efficiency",
                "agent_regression",
            ],
            "memory": [
                "memory_utility", "session_coherence", "context_poisoning",
                "groundedness",
            ],
            "enterprise": [
                "permission_safety", "citation_precision", "groundedness",
                "answer_correctness", "context_poisoning", "session_coherence",
                "hallucination_risk", "response_completeness",
            ],
            "production": [
                "agent_regression", "cost_efficiency", "answer_correctness",
                "groundedness", "task_completion", "hallucination_risk",
            ],
            "autonomous": [
                "plan_adherence", "agent_handoff_quality", "trajectory_quality",
                "tool_correctness", "task_completion", "agent_regression",
                "response_completeness",
            ],
            "full": list(self._evaluators.keys()),
        }
        return profiles.get(intent, list(self._evaluators.keys()))
