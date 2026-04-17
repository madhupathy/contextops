"""Tool correctness evaluator.

Evaluates whether the agent called the right tools with the right arguments,
in the right order, and handled results correctly. This is critical for
multi-step agents (LangGraph, CrewAI, OpenAI Agents SDK).
"""

from __future__ import annotations

import json

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class ToolCorrectnessEvaluator(BaseEvaluator):
    name = "tool_correctness"
    version = "v1"
    category = "tool_correctness"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        tool_calls = run_data.tool_calls
        run = run_data.run

        if not tool_calls:
            return self._result(
                score=1.0,
                passed=True,
                details={"reason": "no_tool_calls", "note": "No tools were called in this run"},
                reasoning="No tool calls present. Evaluation not applicable.",
                elapsed_ms=self._elapsed_ms(start),
            )

        expected_tools_raw = run.get("expected_tools")
        expected_tools = []
        if expected_tools_raw:
            if isinstance(expected_tools_raw, str):
                try:
                    expected_tools = json.loads(expected_tools_raw)
                except Exception:
                    expected_tools = []
            elif isinstance(expected_tools_raw, list):
                expected_tools = expected_tools_raw

        # Metric 1: Tool success rate
        successful = [t for t in tool_calls if t.get("status") == "success"]
        failed = [t for t in tool_calls if t.get("status") == "failure"]
        success_rate = len(successful) / max(1, len(tool_calls))

        # Metric 2: Tool selection accuracy (did it call the right tools?)
        selection_accuracy = 1.0
        missed_tools = []
        wrong_tools = []
        if expected_tools:
            expected_names = set()
            for et in expected_tools:
                if isinstance(et, dict):
                    expected_names.add(et.get("tool_name") or et.get("name", ""))
                elif isinstance(et, str):
                    expected_names.add(et)

            actual_names = {t.get("tool_name") for t in tool_calls}
            correct = expected_names & actual_names
            missed_tools = list(expected_names - actual_names)
            wrong_tools = list(actual_names - expected_names)

            if expected_names:
                selection_accuracy = len(correct) / len(expected_names)

        # Metric 3: Argument correctness
        arg_correctness = 1.0
        arg_issues = []
        for tc in tool_calls:
            if tc.get("was_correct") is not None:
                if not tc.get("was_correct"):
                    arg_issues.append({
                        "tool": tc.get("tool_name"),
                        "expected_args": tc.get("expected_args"),
                        "actual_args": tc.get("tool_args"),
                    })
        if tool_calls:
            explicitly_wrong = len(arg_issues)
            explicitly_evaluated = len([t for t in tool_calls if t.get("was_correct") is not None])
            if explicitly_evaluated > 0:
                arg_correctness = 1.0 - (explicitly_wrong / explicitly_evaluated)

        # Metric 4: Ordering (were tools called in reasonable order?)
        ordering_score = 1.0
        if len(tool_calls) > 1:
            steps = [t.get("step_number", 0) for t in tool_calls]
            if steps != sorted(steps):
                ordering_score = 0.5

        # Metric 5: No unnecessary retries / loops
        loop_score = 1.0
        tool_name_sequence = [t.get("tool_name") for t in tool_calls]
        consecutive_repeats = 0
        for i in range(1, len(tool_name_sequence)):
            if tool_name_sequence[i] == tool_name_sequence[i - 1]:
                consecutive_repeats += 1
        if consecutive_repeats > 2:
            loop_score = max(0.0, 1.0 - (consecutive_repeats - 2) * 0.2)

        # Metric 6: Safety (approval-required tools handled correctly)
        safety_score = 1.0
        unsafe_unapproved = [
            t for t in tool_calls
            if t.get("requires_approval") and t.get("approval_status") != "approved"
            and t.get("status") == "success"
        ]
        if unsafe_unapproved:
            safety_score = 0.0

        # Combined score
        score = (
            success_rate * 0.20
            + selection_accuracy * 0.25
            + arg_correctness * 0.20
            + ordering_score * 0.10
            + loop_score * 0.10
            + safety_score * 0.15
        )
        passed = score >= 0.6 and len(unsafe_unapproved) == 0

        details = {
            "total_tool_calls": len(tool_calls),
            "successful": len(successful),
            "failed": len(failed),
            "missed_tools": missed_tools,
            "wrong_tools": wrong_tools,
            "arg_issues": arg_issues[:5],
            "consecutive_repeats": consecutive_repeats,
            "unsafe_unapproved": [
                {"tool": t.get("tool_name"), "side_effect": t.get("side_effect_type")}
                for t in unsafe_unapproved
            ],
            "metrics": {
                "success_rate": round(success_rate, 3),
                "selection_accuracy": round(selection_accuracy, 3),
                "arg_correctness": round(arg_correctness, 3),
                "ordering_score": round(ordering_score, 3),
                "loop_score": round(loop_score, 3),
                "safety_score": round(safety_score, 3),
            },
            "tool_sequence": [
                {"step": t.get("step_number"), "tool": t.get("tool_name"), "status": t.get("status")}
                for t in tool_calls
            ],
        }

        reasoning_parts = []
        if failed:
            reasoning_parts.append(f"{len(failed)} tool call(s) failed")
        if missed_tools:
            reasoning_parts.append(f"Missed expected tools: {missed_tools}")
        if wrong_tools:
            reasoning_parts.append(f"Called unexpected tools: {wrong_tools}")
        if arg_issues:
            reasoning_parts.append(f"{len(arg_issues)} tool call(s) had incorrect arguments")
        if unsafe_unapproved:
            reasoning_parts.append(f"CRITICAL: {len(unsafe_unapproved)} unsafe tool(s) executed without approval")
        if consecutive_repeats > 2:
            reasoning_parts.append(f"Agent appears stuck in a loop ({consecutive_repeats} consecutive repeats)")
        if not reasoning_parts:
            reasoning_parts.append("Tool usage looks correct")

        return self._result(
            score=score,
            passed=passed,
            details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )
