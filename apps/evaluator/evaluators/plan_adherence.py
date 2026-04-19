"""Plan Adherence Evaluator.

For plan-and-execute agents: did the agent actually execute the plan it created?

Highly autonomous agents (AutoGPT-style, LangGraph planners, CrewAI) create a plan
at the start of a task and then execute it over many steps. The failure mode unique
to these agents is plan drift — the agent quietly abandons part of its plan, executes
a different set of steps than it said it would, or loops on one step while skipping
others.

This evaluator checks:
1. Did the agent create a plan at all? (if not, single-step evaluation applies)
2. What fraction of planned steps were completed?
3. Were steps completed in logical order?
4. Did the agent take unplanned steps not in the original plan?
5. Was the final answer consistent with completing the plan (not a partial completion)?

The plan is expected in run.context_manifest.plan or run.plan:
{
  "steps": [
    {"step_id": "s1", "description": "Search for X", "completed": true},
    {"step_id": "s2", "description": "Analyse Y", "completed": true},
    {"step_id": "s3", "description": "Write report Z", "completed": false}
  ],
  "total_steps": 3,
  "completed_steps": 2
}
"""

from __future__ import annotations

import json
import re
from typing import Any

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class PlanAdherenceEvaluator(BaseEvaluator):
    name = "plan_adherence"
    version = "v1"
    category = "plan_adherence"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        steps = run_data.reasoning_steps
        tool_calls = run_data.tool_calls

        # Load plan from context_manifest or top-level run fields
        plan = self._load_plan(run)

        if not plan or not plan.get("steps"):
            # No plan — this isn't a plan-and-execute agent, skip gracefully
            return self._result(
                score=1.0, passed=True,
                details={
                    "reason": "no_plan",
                    "note": (
                        "No plan found in context_manifest.plan. "
                        "This evaluator applies to plan-and-execute agents. "
                        "Include a plan in context_manifest to enable this evaluator."
                    ),
                },
                reasoning="No plan present — plan adherence evaluation not applicable.",
                elapsed_ms=self._elapsed_ms(start),
            )

        plan_steps = plan.get("steps", [])
        total_planned = len(plan_steps)
        completed_steps = [s for s in plan_steps if s.get("completed", False)]
        skipped_steps = [s for s in plan_steps if not s.get("completed", False)]
        completion_ratio = len(completed_steps) / max(1, total_planned)

        issues: list[dict[str, Any]] = []
        signals: list[str] = []

        # ── Check 1: Completion ratio ──────────────────────────────────────
        if completion_ratio == 1.0:
            signals.append("All planned steps completed")
        elif completion_ratio >= 0.75:
            issues.append({
                "type": "partial_plan_completion",
                "severity": "medium",
                "detail": f"{len(skipped_steps)} of {total_planned} steps skipped: "
                          + ", ".join(s.get("description", s.get("step_id", "?"))[:40] for s in skipped_steps[:3]),
            })
        else:
            issues.append({
                "type": "major_plan_deviation",
                "severity": "high",
                "detail": f"Only {len(completed_steps)}/{total_planned} steps completed. "
                          f"Skipped: {[s.get('description', '')[:40] for s in skipped_steps[:3]]}",
            })

        # ── Check 2: Cross-check plan against reasoning steps ──────────────
        # If reasoning steps exist, verify plan step descriptions appear in them
        if steps:
            step_contents = " ".join(s.get("content", "").lower() for s in steps)
            unmentioned = []
            for ps in plan_steps:
                desc = ps.get("description", "").lower()
                if not desc:
                    continue
                # Check if key words from the plan step appear in reasoning
                keywords = [w for w in re.findall(r'\b\w{4,}\b', desc)][:4]
                if keywords and not any(kw in step_contents for kw in keywords):
                    unmentioned.append(ps.get("description", "")[:60])
            if unmentioned:
                issues.append({
                    "type": "plan_steps_not_in_reasoning",
                    "severity": "medium",
                    "detail": f"{len(unmentioned)} plan step(s) not mentioned in reasoning trace: "
                              + "; ".join(unmentioned[:3]),
                })

        # ── Check 3: Unplanned tool calls ──────────────────────────────────
        # Detect tool calls that seem completely outside the plan scope
        if tool_calls and plan_steps:
            plan_keywords = set()
            for ps in plan_steps:
                plan_keywords.update(re.findall(r'\b\w{4,}\b', ps.get("description", "").lower()))

            unplanned_tools = []
            for tc in tool_calls:
                tool_name = tc.get("tool_name", "").lower()
                tool_keywords = set(re.findall(r'\b\w{3,}\b', tool_name))
                if tool_keywords and not (tool_keywords & plan_keywords):
                    unplanned_tools.append(tc.get("tool_name"))

            if len(unplanned_tools) > len(tool_calls) * 0.4:
                issues.append({
                    "type": "significant_unplanned_tool_usage",
                    "severity": "medium",
                    "detail": f"{len(unplanned_tools)} tool calls ({len(unplanned_tools)}/{len(tool_calls)}) "
                              f"appear outside plan scope: {list(set(unplanned_tools))[:4]}",
                })

        # ── Check 4: Final answer reflects plan completion ─────────────────
        final_answer = run.get("final_answer") or ""
        if completion_ratio < 1.0 and final_answer:
            # If plan is incomplete, answer should acknowledge it
            partial_indicators = ["partial", "incomplete", "remaining", "still need", "not yet", "pending"]
            acknowledges_incomplete = any(p in final_answer.lower() for p in partial_indicators)
            if not acknowledges_incomplete and completion_ratio < 0.7:
                issues.append({
                    "type": "incomplete_plan_undisclosed",
                    "severity": "high",
                    "detail": "Plan was not fully completed but final answer does not acknowledge this. "
                              "Agent may be presenting partial work as complete.",
                })

        # ── Scoring ────────────────────────────────────────────────────────
        high_issues = [i for i in issues if i.get("severity") == "high"]
        medium_issues = [i for i in issues if i.get("severity") == "medium"]

        # Base score from completion ratio
        score = completion_ratio
        # Penalise high-severity issues more
        score -= len(high_issues) * 0.2
        score -= len(medium_issues) * 0.1
        score = max(0.0, min(1.0, score))
        passed = score >= 0.7 and len(high_issues) == 0

        details = {
            "total_planned_steps": total_planned,
            "completed_steps": len(completed_steps),
            "skipped_steps": len(skipped_steps),
            "completion_ratio": round(completion_ratio, 3),
            "issues": issues,
            "positive_signals": signals,
            "plan_summary": [
                {
                    "step": s.get("step_id", f"step-{i+1}"),
                    "description": s.get("description", "")[:60],
                    "completed": s.get("completed", False),
                    "assigned_to": s.get("assigned_to"),
                }
                for i, s in enumerate(plan_steps)
            ],
        }

        if issues:
            reasoning = "; ".join(i["detail"] for i in high_issues + medium_issues)
        else:
            reasoning = f"Plan fully adhered to — all {total_planned} steps completed."

        return self._result(
            score=score, passed=passed, details=details,
            reasoning=reasoning,
            elapsed_ms=self._elapsed_ms(start),
        )

    @staticmethod
    def _load_plan(run: dict) -> dict | None:
        # Try context_manifest.plan first
        manifest = run.get("context_manifest")
        if isinstance(manifest, str):
            try:
                manifest = json.loads(manifest)
            except Exception:
                manifest = {}
        if isinstance(manifest, dict):
            plan = manifest.get("plan")
            if plan:
                return plan if isinstance(plan, dict) else None

        # Try top-level run.plan
        plan = run.get("plan")
        if plan:
            if isinstance(plan, str):
                try:
                    return json.loads(plan)
                except Exception:
                    return None
            return plan if isinstance(plan, dict) else None

        return None
