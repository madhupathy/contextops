"""Session Coherence Evaluator.

Evaluates whether the agent maintained coherent, non-contradictory context
across a multi-turn session. This is the core gap identified by HydraDB research:
agents suffer "Groundhog Day" syndrome — each session starts fresh, losing
continuity, preferences, and prior commitments.

Based on research:
- HydraDB Cortex (composite context vs flat index)
- arxiv 2601.11653 (Agent Cognitive Compressor — memory governance)
- LoCoMo benchmark (35-session conversational memory)

What this evaluator checks:
1. Entity consistency — same entity (person, policy, value) referred to consistently
2. Preference adherence — if user preferences are in memory, are they honoured?
3. Commitment tracking — did agent follow through on prior commitments in context?
4. Temporal consistency — timestamps and dates are used in logical order
"""

from __future__ import annotations

import re
from typing import Any

from models import EvaluationResult, RunData
from evaluators.base import BaseEvaluator


class SessionCoherenceEvaluator(BaseEvaluator):
    name = "session_coherence"
    version = "v1"
    category = "session_coherence"

    async def evaluate(self, run_data: RunData) -> EvaluationResult:
        start = self._timer()
        run = run_data.run
        memory = run_data.memory_candidates
        final_answer = run.get("final_answer") or ""
        steps = run_data.reasoning_steps

        if not memory and not steps:
            return self._result(
                score=1.0, passed=True,
                details={"reason": "single_turn_no_context", "note": "Single-turn run — no session context to evaluate"},
                reasoning="No multi-turn session context present.",
                elapsed_ms=self._elapsed_ms(start),
            )

        issues: list[dict[str, Any]] = []
        signals: list[dict[str, Any]] = []

        # ── Check 1: Preference memory is selected and honoured ────────────
        preference_memories = [m for m in memory if m.get("memory_type") == "preference" and m.get("selected")]
        if preference_memories and final_answer:
            for pref_mem in preference_memories:
                content = pref_mem.get("content", "").lower()
                # Check if stated preference is reflected in answer format
                if "bullet" in content or "concise" in content or "short" in content:
                    # Answer should be relatively short/bulleted
                    word_count = len(final_answer.split())
                    has_bullets = "•" in final_answer or "- " in final_answer or "\n*" in final_answer
                    if word_count > 300 and not has_bullets:
                        issues.append({
                            "type": "preference_not_honoured",
                            "preference": content[:100],
                            "issue": f"User prefers concise/bulleted answers but response is {word_count} words without bullets",
                            "severity": "medium",
                        })
                    else:
                        signals.append({"type": "preference_honoured", "preference": content[:50]})

                if "detailed" in content or "comprehensive" in content:
                    word_count = len(final_answer.split())
                    if word_count < 50:
                        issues.append({
                            "type": "preference_not_honoured",
                            "preference": content[:100],
                            "issue": f"User prefers detailed answers but response is only {word_count} words",
                            "severity": "medium",
                        })

        # ── Check 2: Entity memory consistency ─────────────────────────────
        entity_memories = [m for m in memory if m.get("memory_type") in ("entity", "semantic") and m.get("selected")]
        entity_contradictions = []
        for em in entity_memories:
            content = em.get("content", "")
            # Extract named values from memory
            mem_numbers = re.findall(r'\b(\d+)\b', content)
            if mem_numbers and final_answer:
                ans_numbers = re.findall(r'\b(\d+)\b', final_answer)
                # If memory has specific values and answer has conflicting ones
                mem_set = set(mem_numbers)
                ans_set = set(ans_numbers)
                if mem_set and ans_set and not (mem_set & ans_set):
                    entity_contradictions.append({
                        "memory_id": em.get("memory_id"),
                        "memory_values": list(mem_set)[:5],
                        "answer_values": list(ans_set)[:5],
                    })

        if entity_contradictions:
            issues.append({
                "type": "entity_value_contradiction",
                "details": entity_contradictions[:3],
                "severity": "high",
                "explanation": "Answer values contradict values from selected entity memories",
            })

        # ── Check 3: Episodic memory continuity ────────────────────────────
        episodic_memories = [m for m in memory if m.get("memory_type") == "episodic" and m.get("selected")]
        if episodic_memories:
            # Good signal: episodic context is referenced in the answer
            for ep in episodic_memories:
                content = ep.get("content", "").lower()
                # Check for first-person reference or continuation
                keywords = [w for w in content.split() if len(w) > 4][:5]
                referenced = any(k in final_answer.lower() for k in keywords)
                if referenced:
                    signals.append({
                        "type": "episodic_context_acknowledged",
                        "memory_id": ep.get("memory_id"),
                    })

        # ── Check 4: Temporal consistency in reasoning steps ───────────────
        if steps:
            dates_by_step: list[list[str]] = []
            for step in steps:
                dates = re.findall(r'\b(20\d{2})\b', step.get("content", ""))
                if dates:
                    dates_by_step.append(dates)

            if len(dates_by_step) > 1:
                all_years = [int(y) for batch in dates_by_step for y in batch]
                if all_years and max(all_years) - min(all_years) > 2:
                    issues.append({
                        "type": "temporal_inconsistency",
                        "years_seen": sorted(set(str(y) for y in all_years)),
                        "severity": "medium",
                        "explanation": "Reasoning steps reference years more than 2 years apart — possible stale context mixing",
                    })

        # ── Scoring ────────────────────────────────────────────────────────
        critical_issues = [i for i in issues if i.get("severity") == "critical"]
        high_issues = [i for i in issues if i.get("severity") == "high"]
        medium_issues = [i for i in issues if i.get("severity") == "medium"]

        score = 1.0
        score -= len(critical_issues) * 0.4
        score -= len(high_issues) * 0.2
        score -= len(medium_issues) * 0.1
        # Positive signals boost score slightly
        score += min(0.1, len(signals) * 0.05)
        score = min(1.0, max(0.0, score))

        passed = len(critical_issues) == 0 and len(high_issues) == 0

        details = {
            "memory_candidates": len(memory),
            "preference_memories_used": len(preference_memories),
            "entity_memories_used": len(entity_memories),
            "episodic_memories_used": len(episodic_memories),
            "positive_signals": signals,
            "issues": issues,
        }

        reasoning_parts = []
        for issue in critical_issues + high_issues + medium_issues:
            reasoning_parts.append(f"{issue['severity'].upper()}: {issue['type']} — {issue.get('explanation', issue.get('issue', ''))}")
        if signals:
            reasoning_parts.append(f"{len(signals)} positive coherence signal(s) detected")
        if not reasoning_parts:
            reasoning_parts.append("Session context maintained coherently")

        return self._result(
            score=score, passed=passed, details=details,
            reasoning="; ".join(reasoning_parts),
            elapsed_ms=self._elapsed_ms(start),
        )
