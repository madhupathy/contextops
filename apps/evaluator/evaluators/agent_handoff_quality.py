"""Agent Handoff Quality Evaluator.

For multi-agent systems (CrewAI, AutoGen, OpenAI Swarm, LangGraph multi-agent):
evaluates whether context was passed correctly between agents, whether
handoffs were necessary, and whether receiving agents built on (rather than
repeated or contradicted) the work of the agents that came before.

The failure modes unique to multi-agent systems:
1. Context loss — Agent B starts over instead of building on Agent A's work
2. Contradiction — Agent B produces output contradicting Agent A's findings
3. Unnecessary delegation — orchestrator delegates a task any single agent could do
4. Missing handoff — agent completes a partial task and declares done without passing on
5. Circular handoff — A hands to B, B hands back to A, indefinite loop

This evaluator uses:
- run.handoffs[] — explicit handoff records
- run.parent_run_id — presence indicates this run is a sub-agent
- run.agent_role — orchestrator vs subagent vs specialist
- reasoning_steps with content — checks for re-doing prior work
"""

from __future__ import annotations

import json
import re
from typing import Any

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class AgentHandoffQualityEvaluator(BaseEvaluator):
    name = "agent_handoff_quality"
    version = "v1"
    category = "agent_handoff_quality"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        steps = run_data.reasoning_steps
        tool_calls = run_data.tool_calls

        # Load handoff records
        handoffs_raw = run.get("handoffs") or []
        if isinstance(handoffs_raw, str):
            try:
                handoffs_raw = json.loads(handoffs_raw)
            except Exception:
                handoffs_raw = []

        agent_role = run.get("agent_role", "standalone")
        parent_run_id = run.get("parent_run_id")
        is_subagent = parent_run_id is not None or agent_role in ("subagent", "specialist", "executor")

        # If standalone agent with no handoffs, this evaluator doesn't apply
        if not handoffs_raw and not is_subagent and agent_role == "standalone":
            return self._result(
                score=1.0, passed=True,
                details={
                    "reason": "standalone_agent",
                    "note": (
                        "No handoffs and agent_role is 'standalone'. "
                        "Set agent_role and parent_run_id to enable multi-agent evaluation. "
                        "Include handoffs[] records to evaluate inter-agent context transfer."
                    ),
                },
                reasoning="Standalone agent — handoff quality evaluation not applicable.",
                elapsed_ms=self._elapsed_ms(start),
            )

        issues: list[dict[str, Any]] = []
        signals: list[str] = []

        # ── Check 1: Handoff completeness ──────────────────────────────────
        if handoffs_raw:
            handoffs_without_context = [
                h for h in handoffs_raw
                if not h.get("context_passed") and not h.get("reason")
            ]
            if handoffs_without_context:
                issues.append({
                    "type": "context_missing_in_handoff",
                    "severity": "medium",
                    "count": len(handoffs_without_context),
                    "detail": f"{len(handoffs_without_context)} handoff(s) passed no context to the receiving agent.",
                })

            # Check for circular handoffs (A→B→A)
            agent_chain = []
            for h in handoffs_raw:
                agent_chain.append(h.get("from_agent", "?"))
                agent_chain.append(h.get("to_agent", "?"))
            seen = set()
            for a in agent_chain:
                if a in seen and a != "?":
                    issues.append({
                        "type": "circular_handoff_detected",
                        "severity": "high",
                        "detail": f"Agent '{a}' appears multiple times in handoff chain — possible circular delegation.",
                    })
                    break
                seen.add(a)

            signals.append(f"{len(handoffs_raw)} handoff(s) recorded")

        # ── Check 2: Sub-agent redundancy ──────────────────────────────────
        # If this is a sub-agent, check whether its steps repeat tool calls
        # that should have been done by the orchestrator (common re-do pattern)
        if is_subagent and steps:
            step_contents = [s.get("content", "").lower() for s in steps]
            re_doing_signals = [
                "start", "begin", "first", "initialize", "setup",
                "gather requirements", "understand the task", "analyse the problem"
            ]
            re_doing_steps = [
                c for c in step_contents
                if any(sig in c for sig in re_doing_signals)
            ]
            if re_doing_steps:
                issues.append({
                    "type": "subagent_restarting_from_scratch",
                    "severity": "medium",
                    "detail": (
                        f"Sub-agent appears to restart from scratch ({len(re_doing_steps)} restart-like steps). "
                        "Context from parent may not have been passed correctly."
                    ),
                })

        # ── Check 3: Tool call duplication across agents ───────────────────
        # Heuristic: if a sub-agent calls the same tool as the parent likely did
        # (indicated by having retrieval candidates identical to what orchestrator would use),
        # flag potential duplication
        if is_subagent and tool_calls:
            retrieval_tools = [
                tc for tc in tool_calls
                if any(kw in tc.get("tool_name", "").lower()
                       for kw in ["search", "retrieve", "fetch", "get", "find", "query", "lookup"])
            ]
            if len(retrieval_tools) > len(tool_calls) * 0.6:
                issues.append({
                    "type": "subagent_heavy_retrieval",
                    "severity": "low",
                    "detail": (
                        f"{len(retrieval_tools)}/{len(tool_calls)} tool calls are retrieval operations. "
                        "Verify the orchestrator is passing retrieved context rather than requiring sub-agents to re-retrieve."
                    ),
                })

        # ── Check 4: Role appropriateness ──────────────────────────────────
        if agent_role == "orchestrator" and not handoffs_raw and tool_calls:
            # Orchestrator doing direct tool calls without delegating = possible design issue
            heavy_work_tools = [
                tc for tc in tool_calls
                if tc.get("status") == "success" and
                not any(kw in tc.get("tool_name", "").lower()
                        for kw in ["delegate", "assign", "spawn", "create_task", "handoff"])
            ]
            if len(heavy_work_tools) > 5:
                issues.append({
                    "type": "orchestrator_doing_specialist_work",
                    "severity": "low",
                    "detail": (
                        f"Orchestrator made {len(heavy_work_tools)} direct tool calls without delegating. "
                        "Consider delegating specialist work to sub-agents."
                    ),
                })

        # ── Scoring ────────────────────────────────────────────────────────
        high_issues = [i for i in issues if i.get("severity") == "high"]
        medium_issues = [i for i in issues if i.get("severity") == "medium"]
        low_issues = [i for i in issues if i.get("severity") == "low"]

        score = 1.0
        score -= len(high_issues) * 0.35
        score -= len(medium_issues) * 0.15
        score -= len(low_issues) * 0.05
        score = max(0.0, min(1.0, score))
        passed = score >= 0.6 and len(high_issues) == 0

        details = {
            "agent_role": agent_role,
            "is_subagent": is_subagent,
            "parent_run_id": parent_run_id,
            "handoff_count": len(handoffs_raw),
            "issues": issues,
            "signals": signals,
        }

        if issues:
            reasoning = "; ".join(
                i["detail"] for i in (high_issues + medium_issues + low_issues)
            )
        else:
            reasoning = (
                f"Handoff quality looks good. Agent role '{agent_role}', "
                f"{len(handoffs_raw)} handoff(s) recorded."
                + (" No issues detected." if not issues else "")
            )

        return self._result(
            score=score, passed=passed, details=details,
            reasoning=reasoning,
            elapsed_ms=self._elapsed_ms(start),
        )
