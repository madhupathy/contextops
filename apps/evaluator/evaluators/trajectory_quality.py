"""Trajectory quality evaluator.

Evaluates the agent's reasoning path — was it efficient, logical,
and did it avoid unnecessary steps or loops?
"""

from __future__ import annotations

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class TrajectoryQualityEvaluator(BaseEvaluator):
    name = "trajectory_quality"
    version = "v1"
    category = "trajectory_quality"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        steps = run_data.reasoning_steps
        tool_calls = run_data.tool_calls
        run = run_data.run

        if not steps and not tool_calls:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "no_trajectory", "note": "Single-step run, no trajectory to evaluate"},
                reasoning="No multi-step trajectory present.",
                elapsed_ms=self._elapsed_ms(start),
            )

        all_steps = steps or []
        total_steps = len(all_steps) + len(tool_calls)

        # Metric 1: Step efficiency (penalize excessive steps)
        efficiency = 1.0
        if total_steps > 20:
            efficiency = max(0.2, 1.0 - (total_steps - 20) * 0.05)
        elif total_steps > 10:
            efficiency = max(0.5, 1.0 - (total_steps - 10) * 0.03)

        # Metric 2: Loop detection
        loop_score = 1.0
        step_types = [s.get("step_type", "") for s in all_steps]
        tool_names = [t.get("tool_name", "") for t in tool_calls]
        sequence = step_types + tool_names

        max_consecutive = _max_consecutive_repeats(sequence)
        if max_consecutive > 3:
            loop_score = max(0.0, 1.0 - (max_consecutive - 3) * 0.2)

        # Metric 3: Progress (are steps making forward progress?)
        progress_score = 1.0
        if all_steps:
            think_steps = [s for s in all_steps if s.get("step_type") == "think"]
            action_steps = [s for s in all_steps if s.get("step_type") in ("retrieve", "tool_call", "act")]
            if think_steps and not action_steps:
                progress_score = 0.3  # Thinking without acting
            elif len(think_steps) > len(action_steps) * 3:
                progress_score = 0.5  # Too much thinking, not enough doing

        # Metric 4: Completion (did the trajectory reach a conclusion?)
        completion_score = 1.0
        status = run.get("status", "")
        if status == "failed":
            completion_score = 0.0
        elif status == "timeout":
            completion_score = 0.2
        elif not run.get("final_answer"):
            completion_score = 0.3

        # Metric 5: Token efficiency
        total_tokens = sum(s.get("tokens_used", 0) for s in all_steps)
        total_latency = sum(s.get("latency_ms", 0) for s in all_steps)
        token_efficiency = 1.0
        if total_tokens > 50000:
            token_efficiency = max(0.3, 1.0 - (total_tokens - 50000) / 100000)

        # Combined score
        score = (
            efficiency * 0.25
            + loop_score * 0.25
            + progress_score * 0.20
            + completion_score * 0.20
            + token_efficiency * 0.10
        )
        passed = score >= 0.5

        details = {
            "total_reasoning_steps": len(all_steps),
            "total_tool_calls": len(tool_calls),
            "total_steps": total_steps,
            "max_consecutive_repeats": max_consecutive,
            "total_trajectory_tokens": total_tokens,
            "total_trajectory_latency_ms": total_latency,
            "step_type_distribution": _count_types(step_types),
            "metrics": {
                "efficiency": round(efficiency, 3),
                "loop_score": round(loop_score, 3),
                "progress_score": round(progress_score, 3),
                "completion_score": round(completion_score, 3),
                "token_efficiency": round(token_efficiency, 3),
            },
        }

        reasoning_parts = []
        if max_consecutive > 3:
            reasoning_parts.append(f"Agent appears stuck in a loop ({max_consecutive} consecutive repeats)")
        if total_steps > 20:
            reasoning_parts.append(f"Trajectory is long ({total_steps} steps) — may be inefficient")
        if progress_score < 0.5:
            reasoning_parts.append("Agent spent too much time thinking without taking action")
        if completion_score < 1.0:
            reasoning_parts.append(f"Trajectory did not complete cleanly (status: {status})")
        if not reasoning_parts:
            reasoning_parts.append("Trajectory looks efficient and well-structured")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )


def _max_consecutive_repeats(sequence: list[str]) -> int:
    if not sequence:
        return 0
    max_count = 1
    current_count = 1
    for i in range(1, len(sequence)):
        if sequence[i] == sequence[i - 1] and sequence[i]:
            current_count += 1
            max_count = max(max_count, current_count)
        else:
            current_count = 1
    return max_count


def _count_types(types: list[str]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for t in types:
        counts[t] = counts.get(t, 0) + 1
    return counts
